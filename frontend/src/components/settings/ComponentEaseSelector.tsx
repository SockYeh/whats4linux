import { useEffect, useRef, useState, useCallback, Fragment } from "react"
import { gsap } from "gsap"
import { CustomEase } from "gsap/CustomEase"
import { PathEditor } from "gsap/utils/PathEditor"
import { useEaseStore } from "../../store"
import { getEase } from "../../store/useEaseStore"
import ToggleButton from "./ToggleButton"
import DropDown from "./DropDown"
import { DEFAULT_EASES } from "../../theme.config"

gsap.registerPlugin(CustomEase, PathEditor)

const PRESETS: Record<string, string> = {
  "power0.out": "M0,500 C0,500 500,0 500,0",
  "power1.out": "M0,500 C52,398 246,0 500,0",
  "power2.out": "M0,500 C63,309 141,163 220,89 316,-1 409,-0.499 500,0",
  "power3.out": "M0,500 C41.5,353 91,141 224,46 289.5,-0.499 376,0 500,0",
  "power4.out": "M0,500 C55,253 96,137 159,74 225,8 252,0 500,0",
  "back.out": "M0,500 C64,214 128.5,-8 256,-45 336,-68 419,0 500,0",
  "bounce.out":
    "M0,500 C70,500 121,281 136,219.5 156.5,136 177,18.5 181,0 185,7.5 207,63.5 227.5,94.5 255,137 286.5,123.5 293,119 331,94 359.5,9.5 363,1 394,43 420,32 429.5,25 439,18 448.5,7.5 455.5,1 461,3 469.5,8 477,8 484.5,8 500,0 500,0",
  "circ.out": "M0,500 C0,296 121,171.5 147.5,145.5 173,120 292,0 500,0",
  "expo.out": "M0,500 C42,195 107,99 140,72 178,41 187,0 500,0",
  "sine.out": "M0,500 C133,294 218,173 282.5,112.5 304.5,92 390,0 500,0",
  "back.inOut":
    "M0,500 C34,500 64,530.5 87.5,540.5 112,551 133.5,553.5 157.5,532.5 192,502 224.5,373.5 232.5,338.5 252.5,249.5 260.5,199 280,110.5 294,46 325.5,-21 352.5,-41 374,-57 399.5,-47 408.5,-42.5 434,-30.5 469,1 500,0",
}

const INITIAL_GRID_PATH = PRESETS["power2.out"]
const INITIAL_EASE_STRING = "M0,0,C0.126,0.382,0.282,0.674,0.44,0.822,0.632,1.002,0.818,1.001,1,1"

const COMPONENTS = {
  DropDown: ["open", "close", "rotate"],
  ToggleButton: ["slide"],
  CallOverlay: ["open", "close"],
} as const

const CallOverlayPreview = () => {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const easeOpenRef = useRef(getEase("CallOverlay", "open"))
  const easeCloseRef = useRef(getEase("CallOverlay", "close"))

  useEffect(() => {
    easeOpenRef.current = getEase("CallOverlay", "open")
    easeCloseRef.current = getEase("CallOverlay", "close")
  })

  useEffect(() => {
    const el = ref.current
    if (!el) return

    gsap.killTweensOf(el)
    if (open) {
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
  }, [open])

  return (
    <div className="w-60 rounded-xl border border-black/20 bg-white p-3 text-left dark:border-white/10 dark:bg-[#1a1a1a]">
      <button
        type="button"
        onClick={() => setOpen(v => !v)}
        className="flex w-full items-center justify-between text-sm font-medium text-gray-800 dark:text-gray-200"
      >
        <span>Audio devices</span>
        <span>{open ? "▲" : "▼"}</span>
      </button>
      <div ref={ref} className="overflow-hidden" style={{ height: 0, opacity: 0 }}>
        <div className="mt-2 border-t border-gray-100 pt-1 dark:border-white/5">
          <div className="px-1 pb-1 text-[10px] uppercase tracking-wide text-gray-400">Output</div>
          <div className="rounded-md px-2 py-1 text-xs text-gray-700 dark:text-gray-300">
            Speakers
          </div>
          <div className="px-1 pb-1 pt-1 text-[10px] uppercase tracking-wide text-gray-400">
            Input
          </div>
          <div className="rounded-md px-2 py-1 text-xs text-gray-700 dark:text-gray-300">
            Microphone
          </div>
        </div>
      </div>
    </div>
  )
}

const GSAPMasterVisualizer = () => {
  const [component, setComponent] = useState<keyof typeof COMPONENTS | null>(null)
  const [prop, setProp] = useState<string | null>(null)
  const { eases, updateEase: saveEaseToStore } = useEaseStore()

  const [easeString, setEaseString] = useState(INITIAL_EASE_STRING)
  const [dirty, setDirty] = useState(false)
  const [isInvalid, setIsInvalid] = useState(false)
  const duration = 2.5

  const mainPathRef = useRef<SVGPathElement>(null)
  const revealPathRef = useRef<SVGPathElement>(null)
  const jointRef = useRef<SVGCircleElement>(null)
  const horizontalFillRef = useRef<HTMLDivElement>(null)
  const progressTextRef = useRef<HTMLSpanElement>(null)
  const valueTextRef = useRef<HTMLSpanElement>(null)

  const tweenRef = useRef<gsap.core.Tween | null>(null)
  const editorRef = useRef<any>(null)

  // Load stored ease when selection changes
  useEffect(() => {
    if (component && prop && (eases as any)[component][prop]) {
      const storedPath = (eases as any)[component][prop]
      setEaseString(storedPath)
      setDirty(false)
      // Ideally, you'd convert 0-1 string back to 0-500 SVG grid path here
    }
  }, [component, prop, eases])

  const refreshPreviewEase = useCallback(() => {
    if (!editorRef.current || !jointRef.current) return

    let errored = false
    const onError = () => {
      errored = true
    }

    let normalized: string
    try {
      normalized = editorRef.current.getNormalizedSVG(500, 500, true, onError)
    } catch {
      errored = true
      normalized = INITIAL_EASE_STRING
    }

    setIsInvalid(errored)
    setEaseString(normalized)

    if (!errored) {
      setDirty(true)
      const newEase = CustomEase.create(`liveEase_${Date.now()}`, normalized)

      // FIX: Kill old and restart new loop
      tweenRef.current?.kill()
      gsap.set(jointRef.current, { attr: { cy: 500 } })

      tweenRef.current = gsap.to(jointRef.current, {
        duration,
        repeat: -1,
        attr: { cy: 0 },
        ease: newEase,
        onUpdate() {
          const p = this.progress()
          const cy = gsap.getProperty(jointRef.current, "cy") as number
          const val = 500 - cy
          if (progressTextRef.current) progressTextRef.current.textContent = p.toFixed(2)
          if (valueTextRef.current) valueTextRef.current.textContent = Math.round(val).toString()
          if (horizontalFillRef.current) gsap.set(horizontalFillRef.current, { scaleX: p })
        },
      })
    }
  }, [duration])

  const handlePresetChange = useCallback(
    (name: string) => {
      const path = PRESETS[name]
      if (!path || !mainPathRef.current || !revealPathRef.current) return

      gsap.to([mainPathRef.current, revealPathRef.current], {
        attr: { d: path },
        duration: 0.4,
        ease: "power2.inOut",
        onComplete: () => {
          editorRef.current?.init()
          refreshPreviewEase()
        },
      })
    },
    [refreshPreviewEase],
  )

  useEffect(() => {
    if (!mainPathRef.current || !jointRef.current) return

    // Setup initial circle pos
    gsap.set(jointRef.current, { attr: { cx: 500, cy: 500 } })

    // Initial loop
    tweenRef.current = gsap.to(jointRef.current, {
      duration,
      repeat: -1,
      attr: { cy: 0 },
      ease: CustomEase.create("initial", INITIAL_EASE_STRING),
      onUpdate() {
        const p = this.progress()
        const cy = gsap.getProperty(jointRef.current, "cy") as number
        if (progressTextRef.current) progressTextRef.current.textContent = p.toFixed(2)
        if (valueTextRef.current) valueTextRef.current.textContent = Math.round(500 - cy).toString()
        if (horizontalFillRef.current) gsap.set(horizontalFillRef.current, { scaleX: p })
      },
    })

    // Setup PathEditor with Snap Logic
    editorRef.current = PathEditor.create(mainPathRef.current, {
      handleSize: 12,
      draggable: true,
      selected: true,
      anchorSnap: (p: { x: number; y: number }) => {
        // 1. Hard Corner Snapping
        if (p.x * p.x + (p.y - 500) * (p.y - 500) < 256) {
          p.x = 0
          p.y = 500
        }
        if (Math.pow(p.x - 500, 2) + p.y * p.y < 256) {
          p.x = 500
          p.y = 0
        }
        // 2. Bound X movement
        if (p.x < 0) p.x = 0
        if (p.x > 500) p.x = 500
      },
      onUpdate: (path: string) => {
        gsap.set(revealPathRef.current, { attr: { d: path } })
        refreshPreviewEase()
      },
      onPress: () => tweenRef.current?.pause(),
      onRelease: () => tweenRef.current?.resume(),
    })

    return () => {
      tweenRef.current?.kill()
      editorRef.current?.kill()
    }
  }, [duration, refreshPreviewEase])

  return (
    <div className="flex gap-8 p-10 bg-[#0e100f] h-fit text-[#bbbaa6] font-mono select-none">
      {/* SIDEBAR */}
      <div className="w-1/4 space-y-6">
        <DropDown
          title="Component"
          elements={Object.keys(COMPONENTS)}
          onToggle={v => {
            setComponent(v as any)
            setProp(null)
          }}
        />
        {component && (
          <DropDown
            title="Animation"
            elements={[...COMPONENTS[component]]}
            onToggle={v => setProp(v)}
          />
        )}
        <DropDown title="Presets" elements={Object.keys(PRESETS)} onToggle={handlePresetChange} />

        {component && (
          <div className="flex flex-col gap-2">
            <button
              disabled={!dirty || !component || !prop}
              className="py-2 rounded bg-emerald-600 disabled:opacity-40"
              onClick={() => {
                if (!component || !prop) return
                ;(saveEaseToStore as (g: string, a: string, e: string) => Promise<void>)(
                  component,
                  prop,
                  easeString,
                )
                setDirty(false)
              }}
            >
              Save Changes
            </button>
            <button
              disabled={!prop}
              className="py-2 rounded bg-zinc-800 disabled:opacity-40"
              onClick={() => {
                const def = (DEFAULT_EASES as any)[component!][prop!]
                setEaseString(def)
                setDirty(true)
              }}
            >
              Reset to Default
            </button>
          </div>
        )}
      </div>

      {/* EDITOR GRID */}
      <div className="flex-1 space-y-6">
        <div className="relative aspect-square bg-black border border-[#42433d] p-12 overflow-visible shadow-2xl">
          <div className="absolute inset-0 pointer-events-none text-lg uppercase tracking-widest text-[#7c7c6f]">
            <div className="absolute left-0 top-1/2 -rotate-90 origin-bottom-left whitespace-nowrap px-4">
              value:{" "}
              <span ref={valueTextRef} className="text-[#0ae448]">
                0
              </span>
            </div>
            <div className="absolute bottom-2 left-0 w-full text-center">
              progress:{" "}
              <span ref={progressTextRef} className="text-[#0ae448]">
                0.00
              </span>
            </div>
          </div>

          <svg className="w-full h-full overflow-visible relative z-10" viewBox="0 0 500 500">
            <g className="opacity-10" stroke="#bbbaa6" strokeWidth="1">
              {[...Array(11)].map((_, i) => (
                <Fragment key={i}>
                  <line x1={i * 50} x2={i * 50} y1="0" y2="500" />
                  <line x1="0" x2="500" y1={i * 50} y2={i * 50} />
                </Fragment>
              ))}
            </g>
            <path
              ref={mainPathRef}
              d={INITIAL_GRID_PATH}
              fill="none"
              stroke="transparent"
              strokeWidth="28"
              className="cursor-crosshair"
            />
            <path
              ref={revealPathRef}
              d={INITIAL_GRID_PATH}
              fill="none"
              stroke={isInvalid ? "#f10c00" : "#0ae448"}
              strokeWidth="3"
              className="pointer-events-none shadow-[0_0_15px_#0ae448]"
            />
            <circle
              ref={jointRef}
              cx="500"
              cy="500"
              r="12"
              fill="#0ae448"
              style={{ filter: "drop-shadow(0 0 15px #0ae448)" }}
            />
          </svg>

          <div className="absolute bottom-11.5 left-12 right-12 h-px bg-[#222]">
            <div ref={horizontalFillRef} className="h-full bg-[#0ae448] scale-x-0 origin-left" />
          </div>
        </div>
      </div>

      {/* PREVIEW PANEL */}
      <div className="w-1/4 flex items-center justify-center border-l border-[#42433d]/30">
        {!component && <div className="opacity-40 text-sm">Pick a component to preview</div>}
        {component === "ToggleButton" && <ToggleButton isEnabled={true} onToggle={() => {}} />}
        {component === "DropDown" && (
          <DropDown title="Preview" elements={["Option 1", "Option 2"]} onToggle={() => {}} />
        )}
        {component === "CallOverlay" && <CallOverlayPreview />}
      </div>
    </div>
  )
}

export default GSAPMasterVisualizer
