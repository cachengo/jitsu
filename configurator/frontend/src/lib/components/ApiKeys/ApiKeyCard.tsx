import { apiKeysStore } from "../../../stores/apiKeys"
import { ReactNode, useState } from "react"
import { useServices } from "../../../hooks/useServices"
import { flowResult } from "mobx"
import { Button, Menu, Tooltip } from "antd"
import { ConnectionCard } from "../../../ui/components/ConnectionCard/ConnectionCard"
import { apiKeysReferenceMap } from "../../../catalog/apiKeys/lib"
import { DeleteOutlined, EditOutlined } from "@ant-design/icons"
import { copyToClipboard, reactElementToString, trimMiddle } from "../../commons/utils"
import styles from "./ApiKeys.module.less"
import { generatePath, NavLink } from "react-router-dom"
import { apiKeysRoutes } from "./ApiKeyEditor"
import { confirmDelete } from "../../commons/deletionConfirmation"
import { actionNotification } from "ui/components/ActionNotification/ActionNotification"
import { APIKeyUtil } from "../../../utils/apiKeys.utils"

type ApiKeyCardProps = {
  apiKey: APIKey
  showDocumentation: () => void
}

export function ApiKeyCard({ apiKey: key, showDocumentation }: ApiKeyCardProps) {
  const [loading, setLoading] = useState(false)
  const services = useServices()
  let keysBackend = services.storageService.table<APIKey>("api_keys")
  const rotateKey = async (key: APIKey, type: "jsAuth" | "serverAuth"): Promise<string> => {
    let newKey = apiKeysStore.generateApiToken(type === "jsAuth" ? "js" : "s2s")
    await keysBackend.patch(key.uid, { [type]: newKey })
    await flowResult(apiKeysStore.pullApiKeys())
    actionNotification.info("New key has been generated and saved")
    return newKey
  }

  let deleteAction = async () => {
    confirmDelete({
      entityName: "api key",
      action: async () => {
        setLoading(true)
        try {
          await keysBackend.remove(key.uid)
          await flowResult(apiKeysStore.pullApiKeys())
        } finally {
          setLoading(false)
        }
      },
    })
  }
  let editLink = generatePath(apiKeysRoutes.editExact, { id: key.uid.replace(".", "-") })
  return (
    <ConnectionCard
      loading={loading}
      title={APIKeyUtil.getDisplayName(key)}
      icon={apiKeysReferenceMap.js.icon}
      deleteAction={deleteAction}
      editAction={editLink}
      menuOverlay={
        <Menu>
          <Menu.Item icon={<EditOutlined />}>
            <NavLink to={editLink}>Edit</NavLink>
          </Menu.Item>
          <Menu.Item icon={<DeleteOutlined />} onClick={deleteAction}>
            Delete
          </Menu.Item>
        </Menu>
      }
      rename={async newName => {
        await keysBackend.patch(key.uid, { comment: newName })
        await flowResult(apiKeysStore.pullApiKeys())
      }}
      subtitle={<a onClick={showDocumentation}>Show connection instructions→</a>}
      status={
        <>
          <div className="text-xs">
            <div className="flex flex-nowrap items-center">
              <span className="inline-block whitespace-nowrap w-16 text-xxs">Server Key</span>{" "}
              <SecretKey rotateKey={() => rotateKey(key, "serverAuth")}>{key.serverAuth}</SecretKey>
            </div>
            <div className="flex flex-nowrap items-center pt-2">
              <span className="inline-block whitespace-nowrap w-16 text-xxs">JS Key</span>
              <SecretKey rotateKey={() => rotateKey(key, "jsAuth")}>{key.jsAuth}</SecretKey>
            </div>
          </div>
        </>
      }
    />
  )
}

type SecretKeyProps = {
  //Key
  children: ReactNode
  //Function that generates newKey
  rotateKey: () => Promise<string>
}

function SecretKey({ children, rotateKey }: SecretKeyProps) {
  let keyStr = reactElementToString(children)

  const [copied, setCopied] = useState(false)
  return (
    <div className="flex flex-nowrap flex w-78">
      <Tooltip
        overlayClassName={`${styles.keyTooltip}`}
        trigger={"click"}
        overlay={
          <div className="flex flex-nowrap items-center space-x-4 justify-center">
            <span className="text-base font-semibold rounded font-monospace text-secondaryText">{children}</span>
            <Button
              size="middle"
              onClick={e => {
                e.stopPropagation()
                e.preventDefault()
                setCopied(true)
                setTimeout(() => setCopied(false), 1000)
                copyToClipboard(keyStr)
              }}
            >
              {copied ? "Copied!" : "Copy"}
            </Button>
          </div>
        }
      >
        <span className="cursor-pointer border px-1 py-0.5 mr-0.5 rounded font-monospace text-secondaryText flex-grow w-56 text-center">
          {trimMiddle(keyStr, 29, "•••")}
        </span>
      </Tooltip>
      {/*<Button*/}
      {/*  size="small"*/}
      {/*  type="link"*/}
      {/*  icon={<ReloadOutlined />}*/}
      {/*  onClick={() => {*/}
      {/*    Modal.confirm({*/}
      {/*      title: "Please confirm key rotation",*/}
      {/*      icon: <ExclamationCircleOutlined />,*/}
      {/*      content:*/}
      {/*        "Are you sure you want to rotate the key? Previously generated key will be lost and you'll need to reconfigure ALL clients",*/}
      {/*      okText: "Generate new key",*/}
      {/*      cancelText: "Cancel",*/}
      {/*      onOk: async () => {*/}
      {/*        setCurrentKeyDisplay("Generating...")*/}
      {/*        try {*/}
      {/*          let newKey = await rotateKey()*/}
      {/*          setCurrentKeyDisplay(newKey)*/}
      {/*        } catch (e) {*/}
      {/*          setCurrentKeyDisplay("Error!")*/}
      {/*          console.log(e)*/}
      {/*        }*/}
      {/*      },*/}
      {/*      onCancel: () => {},*/}
      {/*    })*/}
      {/*  }}*/}
      {/*/>*/}
    </div>
  )
}
