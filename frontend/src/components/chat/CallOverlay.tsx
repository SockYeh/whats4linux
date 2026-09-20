import { useEffect, useRef, useState } from "react"
import clsx from "clsx"
import gsap from "gsap"
import { useGSAP } from "@gsap/react"
import {
  AnswerCall,
  GetCallInfo,
  HangUp,
  ListAudioDevices,
  RejectCall,
  SetAudioInput,
  SetAudioOutput,
} from "../../../wailsjs/go/api/Api"
import { EventsOn } from "../../../wailsjs/runtime/runtime"
import { useChatStore } from "../../store"
import { getEase } from "../../store/useEaseStore"
import { displayCallName } from "../../lib/utils"

export type CallInfo = {
  id?: string
  peer?: string
  chat_jid?: string
  name?: string
  direction?: string
  state?: string
  input_id?: string
  output_id?: string
}

type AudioDevice = {
  id: string
  name: string
  kind: string
  default: boolean
}

function normalizeCall(info: any): CallInfo {
  if (!info) return { state: "idle" }
  return {
    id: info.id ?? info.ID,
    peer: info.peer ?? info.Peer,
    chat_jid: info.chat_jid ?? info.ChatJID,
    name: info.name ?? info.Name,
    direction: info.direction ?? info.Direction,
    state: info.state ?? info.State ?? "idle",
    input_id: info.input_id ?? info.InputID,
    output_id: info.output_id ?? info.OutputID,
  }
}

function normalizeDevice(device: any): AudioDevice {
  return {
    id: device.id ?? device.ID ?? "",
    name: device.name ?? device.Name ?? "Device",
    kind: device.kind ?? device.Kind ?? "",
    default: Boolean(device.default ?? device.Default),
  }
}

type Props = {
  call: CallInfo
}

const CheckIcon = () => (
  <svg viewBox="0 0 24 24" width="14" height="14" className="fill-current">
    <path d="M9.55 18.2 3.65 12.3l1.4-1.4 4.5 4.5 9.4-9.4 1.4 1.4z" />
  </svg>
)

const ChevronIcon = ({ open }: { open: boolean }) => (
  <svg
    viewBox="0 0 24 24"
    width="16"
    height="16"
    className={clsx("fill-current transition-transform", open && "rotate-180")}
  >
    <path d="M7.4 15.4 6 14l6-6 6 6-1.4 1.4L12 10.8z" />
  </svg>
)

function DeviceList({
  label,
  devices,
  selected,
  onSelect,
}: {
  label: string
  devices: AudioDevice[]
  selected: string
  onSelect: (id: string) => void
}) {
  return (
    <div className="mt-2">
      <div className="px-1 pb-1 text-[10px] font-semibold uppercase tracking-wide text-gray-500 dark:text-[#8696a0]">
        {label}
      </div>
      <div>
        {devices.map(device => {
          const active = (device.id || "") === (selected || "")
          return (
            <button
              key={device.id || "default"}
              type="button"
              onClick={() => onSelect(device.id || "")}
              className={clsx(
                "flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs",
                active
                  ? "bg-[#d9fdd3] text-[#0a1014] dark:bg-[#005c4b] dark:text-white"
                  : "text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-white/5",
              )}
            >
              <span className={clsx("w-3.5", active ? "opacity-100" : "opacity-0")}>
                <CheckIcon />
              </span>
              <span className="truncate">{device.name}</span>
            </button>
          )
        })}
      </div>
    </div>
  )
}

export function CallOverlay({ call }: Props) {
  const [devicesOpen, setDevicesOpen] = useState(false)
  const [inputs, setInputs] = useState<AudioDevice[]>([])
  const [outputs, setOutputs] = useState<AudioDevice[]>([])
  const chat = useChatStore(
    s => s.getChat(call.chat_jid || "") || s.getChat(call.peer || ""),
  )
  const devicesRef = useRef<HTMLDivElement>(null)

  const easeOpenRef = useRef(getEase("CallOverlay", "open"))
  const easeCloseRef = useRef(getEase("CallOverlay", "close"))

  useEffect(() => {
    easeOpenRef.current = getEase("CallOverlay", "open")
    easeCloseRef.current = getEase("CallOverlay", "close")
  })

  useGSAP(
    () => {
      const el = devicesRef.current
      if (!el) return
      gsap.killTweensOf(el)
      if (devicesOpen) {
        gsap.to(el, {
          height: "auto",
          opacity: 1,
          duration: 0.3,
          ease: easeOpenRef.current,
          overwrite: "auto",
        })
      } else {
        gsap.to(el, {
          height: 0,
          opacity: 0,
          duration: 0.3,
          ease: easeCloseRef.current,
          overwrite: "auto",
        })
      }
    },
    { dependencies: [devicesOpen, inputs.length, outputs.length] },
  )

  const name = displayCallName(chat?.name, call.name)
  const avatar = chat?.avatar
  const ringing = call.state === "ringing" && call.direction === "incoming"

  useEffect(() => {
    let cancelled = false
    ListAudioDevices()
      .then((devs: any) => {
        if (cancelled) return
        setInputs((devs.inputs || devs.Inputs || []).map(normalizeDevice))
        setOutputs((devs.outputs || devs.Outputs || []).map(normalizeDevice))
      })
      .catch(err => console.error("list audio devices failed:", err))
    return () => {
      cancelled = true
    }
  }, [devicesOpen, call.input_id, call.output_id])

  return (
    <div className="m-2 rounded-2xl border border-gray-200 bg-white p-3 shadow-lg dark:border-white/10 dark:bg-[#1a1a1a]">
      <div ref={devicesRef} className="overflow-hidden" style={{ height: 0, opacity: 0 }}>
        <div className="mb-2 max-h-56 overflow-y-auto border-b border-gray-100 pb-1 dark:border-white/5">
          <DeviceList
            label="Output"
            devices={outputs}
            selected={call.output_id || ""}
            onSelect={id => SetAudioOutput(id).catch(err => console.error(err))}
          />
          <DeviceList
            label="Input"
            devices={inputs}
            selected={call.input_id || ""}
            onSelect={id => SetAudioInput(id).catch(err => console.error(err))}
          />
        </div>
      </div>

      <div className="flex items-center gap-3">
        <div className="h-10 w-10 shrink-0 overflow-hidden rounded-full bg-gray-300 dark:bg-gray-600">
          {avatar ? (
            <img src={avatar} alt="" className="h-full w-full object-cover" />
          ) : (
            <div className="flex h-full w-full items-center justify-center text-sm font-semibold text-white">
              {name.slice(0, 1).toUpperCase()}
            </div>
          )}
        </div>
        <div className="min-w-0 flex-1">
          <div className="truncate text-sm font-medium text-light-text dark:text-dark-text">
            {name}
          </div>
          <div className="text-xs capitalize text-[#1b9a58] dark:text-[#21c063]">
            {ringing ? "Incoming call" : call.state?.replace("_", " ")}
          </div>
        </div>
        <button
          type="button"
          title="Audio devices"
          onClick={() => setDevicesOpen(v => !v)}
          className="rounded-full p-1.5 text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-white/10"
        >
          <ChevronIcon open={devicesOpen} />
        </button>
      </div>

      <div className="mt-3 flex gap-2">
        {ringing ? (
          <>
            <button
              type="button"
              onClick={() => AnswerCall().catch(err => console.error(err))}
              className="flex-1 rounded-full bg-[#21c063] px-3 py-1.5 text-sm font-medium text-[#0a1014]"
            >
              Answer
            </button>
            <button
              type="button"
              onClick={() => RejectCall().catch(err => console.error(err))}
              className="flex-1 rounded-full bg-[#ea0038] px-3 py-1.5 text-sm font-medium text-white"
            >
              Reject
            </button>
          </>
        ) : (
          <button
            type="button"
            onClick={() => HangUp().catch(err => console.error(err))}
            className="flex-1 rounded-full bg-[#ea0038] px-3 py-1.5 text-sm font-medium text-white"
          >
            Hang up
          </button>
        )}
      </div>
    </div>
  )
}

export function useActiveCall() {
  const [call, setCall] = useState<CallInfo>({ state: "idle" })

  useEffect(() => {
    const apply = (info: CallInfo) => setCall(normalizeCall(info))
    GetCallInfo()
      .then(apply)
      .catch(() => {})
    const unsub = EventsOn("wa:call", apply)
    return () => unsub()
  }, [])

  const active = Boolean(call.state && call.state !== "idle")
  return { call, active }
}
