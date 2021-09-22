package storages

import (
	"errors"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/jitsucom/jitsu/server/timestamp"
	"time"

	"github.com/jitsucom/jitsu/server/adapters"
	"github.com/jitsucom/jitsu/server/events"
	"github.com/jitsucom/jitsu/server/logging"
	"github.com/jitsucom/jitsu/server/schema"
)

//S3 stores files to aws s3 in batch mode
type S3 struct {
	Abstract

	s3Adapter *adapters.S3
}

func init() {
	RegisterStorage(StorageType{typeName: S3Type, createFunc: NewS3})
}

func NewS3(config *Config) (Storage, error) {
	if config.streamMode {
		if config.eventQueue != nil {
			config.eventQueue.Close()
		}
		return nil, fmt.Errorf("S3 destination doesn't support %s mode", StreamMode)
	}
	s3Config := config.destination.S3
	if err := s3Config.Validate(); err != nil {
		return nil, err
	}

	s3Adapter, err := adapters.NewS3(s3Config)
	if err != nil {
		return nil, err
	}

	s3 := &S3{
		s3Adapter: s3Adapter,
	}

	//Abstract (SQLAdapters and tableHelpers and archive logger are omitted)
	s3.destinationID = config.destinationID
	s3.processor = config.processor
	s3.fallbackLogger = config.loggerFactory.CreateFailedLogger(config.destinationID)
	s3.eventsCache = config.eventsCache
	s3.uniqueIDField = config.uniqueIDField
	s3.staged = config.destination.Staged
	s3.cachingConfiguration = config.destination.CachingConfiguration

	return s3, nil
}

func (s3 *S3) DryRun(payload events.Event) ([]adapters.TableField, error) {
	return nil, errors.New("s3 does not support dry run functionality")
}

//Store process events and stores with storeTable() func
//returns store result per table, failed events (group of events which are failed to process) and err
func (s3 *S3) Store(fileName string, objects []map[string]interface{}, alreadyUploadedTables map[string]bool) (map[string]*StoreResult, *events.FailedEvents, *events.SkippedEvents, error) {
	processedFiles, failedEvents, skippedEvents, err := s3.processor.ProcessEvents(fileName, objects, alreadyUploadedTables)
	if err != nil {
		return nil, nil, nil, err
	}

	//update cache with failed events
	for _, failedEvent := range failedEvents.Events {
		s3.eventsCache.Error(s3.IsCachingDisabled(), s3.ID(), failedEvent.EventID, failedEvent.Error)
	}
	//update cache and counter with skipped events
	for _, skipEvent := range skippedEvents.Events {
		s3.eventsCache.Skip(s3.IsCachingDisabled(), s3.ID(), skipEvent.EventID, skipEvent.Error)
	}

	storeFailedEvents := true
	tableResults := map[string]*StoreResult{}
	marshaller := s3.marshaller()
	for _, fdata := range processedFiles {
		b := fdata.GetPayloadBytes(marshaller)
		fileName := s3.fileName(fdata)
		err := s3.s3Adapter.UploadBytes(fileName, b)

		tableResults[fdata.BatchHeader.TableName] = &StoreResult{Err: err, RowsCount: fdata.GetPayloadLen(), EventsSrc: fdata.GetEventsPerSrc()}
		if err != nil {
			logging.Errorf("[%s] Error storing file %s: %v", s3.ID(), fileName, err)
			storeFailedEvents = false
		}

		//events cache
		for _, object := range fdata.GetPayload() {
			if err != nil {
				s3.eventsCache.Error(s3.IsCachingDisabled(), s3.ID(), s3.uniqueIDField.Extract(object), err.Error())
			} else {
				s3.eventsCache.Succeed(&adapters.EventContext{
					CacheDisabled:  s3.IsCachingDisabled(),
					DestinationID:  s3.ID(),
					EventID:        s3.uniqueIDField.Extract(object),
					ProcessedEvent: object,
					Table:          nil,
				})
			}
		}
	}

	//store failed events to fallback only if other events have been inserted ok
	if storeFailedEvents {
		return tableResults, failedEvents, skippedEvents, nil
	}

	return tableResults, nil, skippedEvents, nil
}

func (s3 *S3) marshaller() schema.Marshaller {
	if s3.s3Adapter.Format() == adapters.S3FormatCSV {
		return schema.CSVMarshallerInstance
	} else {
		return schema.JSONMarshallerInstance
	}
}

func (s3 *S3) fileName(fdata *schema.ProcessedFile) string {
	start, end := findStartEndTimestamp(fdata.GetPayload())
	return fmt.Sprintf("%s-start-%s-end-%s.log", fdata.BatchHeader.TableName, timestamp.ToISOFormat(start), timestamp.ToISOFormat(end))
}

func findStartEndTimestamp(fdata []map[string]interface{}) (time.Time, time.Time) {
	var start, end time.Time
	for _, it := range fdata {
		if objectTSValue, ok := it[timestamp.Key]; ok {
			var datetime time.Time

			switch objectTSValueInType := objectTSValue.(type) {
			case time.Time:
				datetime = objectTSValueInType
			case *time.Time:
				datetime = *objectTSValueInType
			case string:
				parsed, err := time.Parse(time.RFC3339Nano, objectTSValueInType)
				if err != nil {
					logging.SystemErrorf("Error parsing %s value under %s key with time.RFC3339Nano template: %v", objectTSValueInType, timestamp.Key, err)
				} else {
					datetime = parsed
				}
			}

			if start.IsZero() || datetime.Before(start) {
				start = datetime
			}
			if end.IsZero() || datetime.After(end) {
				end = datetime
			}
		}
	}
	if start.IsZero() || end.IsZero() {
		now := time.Now()
		start = now
		end = now
	}

	return start, end
}

//SyncStore isn't supported
func (s3 *S3) SyncStore(overriddenDataSchema *schema.BatchHeader, objects []map[string]interface{}, timeIntervalValue string, cacheTable bool) error {
	return errors.New("S3 doesn't support sync store")
}

//Update isn't suported
func (s3 *S3) Update(object map[string]interface{}) error {
	return errors.New("S3 doesn't support updates")
}

//GetUsersRecognition returns disabled users recognition configuration
func (s3 *S3) GetUsersRecognition() *UserRecognitionConfiguration {
	return disabledRecognitionConfiguration
}

//Type returns S3 type
func (s3 *S3) Type() string {
	return S3Type
}

//Close closes fallback logger
func (s3 *S3) Close() (multiErr error) {
	if err := s3.s3Adapter.Close(); err != nil {
		multiErr = multierror.Append(multiErr, fmt.Errorf("[%s] Error closing s3 adapter: %v", s3.ID(), err))
	}
	if err := s3.close(); err != nil {
		multiErr = multierror.Append(multiErr, err)
	}
	return
}
