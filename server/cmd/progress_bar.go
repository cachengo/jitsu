package cmd

import (
	au "github.com/logrusorgru/aurora"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
	"io"
	"path/filepath"
)

const (
	mpbType   = "mpb"
	dummyType = "dummy"
)

type ProgressBar interface {
	SetCurrent(current int64)
	createKBFileBar(filePath string, fileSize int64) ProgressBar
	createPartFileBar(filePath string, fileSize int64) ProgressBar

	ProxyReader(r io.Reader) io.ReadCloser

	Type() string
}

type DummyProgressBar struct {
}

func (d *DummyProgressBar) SetCurrent(current int64) {}

//createKBFileBar returns dummy progress bar
func (d *DummyProgressBar) createKBFileBar(filePath string, fileSize int64) ProgressBar {
	return &DummyProgressBar{}
}

//createPartFileBar returns dummy progress bar
func (d *DummyProgressBar) createPartFileBar(filePath string, fileSize int64) ProgressBar {
	return &DummyProgressBar{}
}

func (d *DummyProgressBar) ProxyReader(r io.Reader) io.ReadCloser { return nil }

func (d *DummyProgressBar) Type() string { return dummyType }

type MultiProgressBar struct {
	progress *mpb.Progress
	bar      *mpb.Bar
}

func NewParentMultiProgressBar(capacity int64) *MultiProgressBar {
	progress := mpb.New()
	bar := createProcessingBar(progress, capacity)
	return &MultiProgressBar{progress: progress, bar: bar}
}

func (mp *MultiProgressBar) SetCurrent(current int64) {
	mp.bar.SetCurrent(current)
}

//createKBFileBar creates progress bar per file which counts parts
func (mp *MultiProgressBar) createKBFileBar(filePath string, fileSize int64) ProgressBar {
	fileBar := mp.progress.Add(fileSize,
		mpb.NewBarFiller(mpb.BarStyle().Lbound("╢").Filler(au.Index(99, "█").String()).Tip("").Padding(au.Index(104, "░").String()).Rbound("╟")),
		mpb.BarFillerClearOnComplete(),
		mpb.PrependDecorators(
			decor.Name(filepath.Base(filePath)),
			decor.Percentage(decor.WCSyncSpace),
		),
		mpb.AppendDecorators(
			decor.OnComplete(
				decor.CountersKiloByte("%d / %d", decor.WCSyncWidth), au.Green("✓ done").String(),
			),
		),
	)
	return &MultiProgressBar{progress: mp.progress, bar: fileBar}
}

//createPartFileBar creates progress bar per file which counts parts
func (mp *MultiProgressBar) createPartFileBar(filePath string, fileSize int64) ProgressBar {
	fileBar := mp.progress.Add(fileSize,
		mpb.NewBarFiller(mpb.BarStyle().Lbound("╢").Filler(au.Index(99, "█").String()).Tip("").Padding(au.Index(104, "░").String()).Rbound("╟")),
		mpb.BarFillerClearOnComplete(),
		mpb.PrependDecorators(
			decor.Name(filepath.Base(filePath)),
			decor.Percentage(decor.WCSyncSpace),
		),
		mpb.AppendDecorators(
			decor.OnComplete(
				decor.CountersNoUnit("%d / %d parts", decor.WCSyncWidth), au.Green("✓ done").String(),
			),
		),
	)
	return &MultiProgressBar{progress: mp.progress, bar: fileBar}
}

func (mp *MultiProgressBar) ProxyReader(r io.Reader) io.ReadCloser {
	return mp.bar.ProxyReader(r)
}

func (mp *MultiProgressBar) Type() string {
	return mpbType
}

//check all available colors:
//for i:=0;i<255;i++{
//		fmt.Println(i, " --- ", au.Index(uint8(i), "████████████████").String())
//	}
//createProcessingBar creates global progress bar
func createProcessingBar(p *mpb.Progress, allFilesSize int64) *mpb.Bar {
	return p.Add(allFilesSize,
		mpb.NewBarFiller(mpb.BarStyle().Lbound("╢").Filler(au.Index(93, "█").String()).Tip("").Padding(au.Index(99, "░").String()).Rbound("╟")),
		mpb.PrependDecorators(
			decor.Name("replay"),
			decor.Percentage(decor.WCSyncSpace),
		),
		mpb.AppendDecorators(
			decor.OnComplete(
				decor.CountersNoUnit("%d / %d files", decor.WCSyncWidth), au.Green("✓ done").String(),
			),
		),
		mpb.BarPriority(9999999),
	)
}
