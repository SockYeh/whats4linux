import { clsx, type ClassValue } from "clsx"

export function cn(...inputs: ClassValue[]) {
  return clsx(inputs)
}

/**
 * Format a bare phone number (digits, as stored in JIDs) the way WhatsApp
 * displays it: "+<country code> XXXXX XXXXX". Falls back to "+digits" when
 * the number is too short to split.
 */
export function formatPhone(digits: string): string {
  const clean = digits.replace(/\D/g, "")
  if (!clean) return ""
  if (clean.length <= 10) return `+${clean}`
  const cc = clean.slice(0, clean.length - 10)
  const rest = clean.slice(-10)
  return `+${cc} ${rest.slice(0, 5)} ${rest.slice(5)}`
}

/**
 * Extract the phone number from a user JID ("<digits>@s.whatsapp.net").
 * Returns "" for non-phone JIDs (e.g. "@lid" senders).
 */
export function phoneFromJID(jid: string): string {
  if (!jid.endsWith("@s.whatsapp.net")) return ""
  return jid.split("@")[0]
}

/** True for raw WhatsApp JIDs that should not be shown as a person's name. */
export function isRawJID(value: string): boolean {
  return /@(lid|s\.whatsapp\.net|g\.us|hosted\.lid)\b/i.test(value)
}

/** Prefer the chat-list / contact-store name; never fall back to a LID/JID. */
export function displayCallName(chatName?: string, contactName?: string): string {
  for (const candidate of [chatName, contactName]) {
    const name = candidate?.trim()
    if (name && !isRawJID(name)) return name
  }
  return "Call"
}

/**
 * WhatsApp default avatar / community placeholder pastels (light mode).
 * Soft backgrounds with a dark silhouette icon — matches current WA chat list.
 * Order is fixed; index is chosen by hashing the JID.
 */
export const AVATAR_COLORS_LIGHT = [
  "#E5CDB0", // beige / tan (community)
  "#A8E0B8", // mint green
  "#7EC4E8", // sky blue
  "#F0B4B4", // soft coral
  "#C9B8E8", // soft lavender
  "#F5D08A", // soft gold
  "#9DD9D0", // soft teal
  "#E8B8D0", // soft pink
  "#B8D4A8", // soft sage
  "#A8C8E8", // soft periwinkle
  "#E8C8A0", // warm sand
  "#D0E8A8", // lime mist
  "#D8B8E0", // soft orchid
  "#A8D8E8", // powder blue
  "#E8D0B8", // peach sand
  "#B8E0D0", // seafoam
] as const

/**
 * Dark-mode variants of the same palette — slightly deeper so pastels still
 * read on WhatsApp’s near-black chat list without looking neon.
 */
export const AVATAR_COLORS_DARK = [
  "#C4A882",
  "#6FB88A",
  "#5A9EC4",
  "#C47A7A",
  "#9A88C4",
  "#C4A85A",
  "#6BB0A8",
  "#C488A8",
  "#88B078",
  "#7898C4",
  "#C4A878",
  "#A0B878",
  "#A888B8",
  "#78B0C4",
  "#C4B090",
  "#88B8A8",
] as const

/** @deprecated Use getAvatarColor — kept for sender-name color tests. */
export const PROFILE_COLORS = AVATAR_COLORS_LIGHT

/** Minimal SHA-1 (bytes) — deterministic JID → palette index. */
function sha1Bytes(message: string): Uint8Array {
  const ml = message.length
  const words: number[] = []
  for (let i = 0; i < ml; i++) {
    words[i >> 2] |= (message.charCodeAt(i) & 0xff) << (24 - (i % 4) * 8)
  }
  words[ml >> 2] |= 0x80 << (24 - (ml % 4) * 8)
  const bitLen = ml * 8
  const totalWords = (((ml + 8) >> 6) + 1) * 16
  words[totalWords - 1] = bitLen

  let h0 = 0x67452301
  let h1 = 0xefcdab89
  let h2 = 0x98badcfe
  let h3 = 0x10325476
  let h4 = 0xc3d2e1f0

  const w = new Array<number>(80)
  const rotl = (n: number, s: number) => (n << s) | (n >>> (32 - s))

  for (let i = 0; i < totalWords; i += 16) {
    for (let t = 0; t < 16; t++) w[t] = words[i + t] | 0
    for (let t = 16; t < 80; t++) {
      w[t] = rotl(w[t - 3] ^ w[t - 8] ^ w[t - 14] ^ w[t - 16], 1)
    }

    let a = h0
    let b = h1
    let c = h2
    let d = h3
    let e = h4

    for (let t = 0; t < 80; t++) {
      let f: number
      let k: number
      if (t < 20) {
        f = (b & c) | (~b & d)
        k = 0x5a827999
      } else if (t < 40) {
        f = b ^ c ^ d
        k = 0x6ed9eba1
      } else if (t < 60) {
        f = (b & c) | (b & d) | (c & d)
        k = 0x8f1bbcdc
      } else {
        f = b ^ c ^ d
        k = 0xca62c1d6
      }
      const temp = (rotl(a, 5) + f + e + k + w[t]) | 0
      e = d
      d = c
      c = rotl(b, 30)
      b = a
      a = temp
    }

    h0 = (h0 + a) | 0
    h1 = (h1 + b) | 0
    h2 = (h2 + c) | 0
    h3 = (h3 + d) | 0
    h4 = (h4 + e) | 0
  }

  const out = new Uint8Array(20)
  const write = (offset: number, v: number) => {
    out[offset] = (v >>> 24) & 0xff
    out[offset + 1] = (v >>> 16) & 0xff
    out[offset + 2] = (v >>> 8) & 0xff
    out[offset + 3] = v & 0xff
  }
  write(0, h0)
  write(4, h1)
  write(8, h2)
  write(12, h3)
  write(16, h4)
  return out
}

function colorIndex(jid: string, paletteLen: number): number {
  if (!jid) return 0
  const hash = sha1Bytes(jid)
  const hashInt = ((hash[0] << 24) | (hash[1] << 16) | (hash[2] << 8) | hash[3]) >>> 0
  return hashInt % paletteLen
}

/**
 * WhatsApp default avatar / community placeholder color for a JID.
 * Uses soft pastels (screenshot-matched). Pass `dark` for dark-theme variants.
 */
export function getAvatarColor(jid: string, dark = false): string {
  const palette = dark ? AVATAR_COLORS_DARK : AVATAR_COLORS_LIGHT
  return palette[colorIndex(jid, palette.length)]
}

/**
 * @deprecated Prefer getAvatarColor(jid, isDark). Kept for sender-name styling
 * which still uses the light palette as a saturated accent.
 */
export function getProfileColor(jid: string): string {
  return getAvatarColor(jid, false)
}

/** Dark silhouette on pastel avatars (both themes, matches WA). */
export const AVATAR_ICON_COLOR = "#111b21"
/** Light icon on the small dark group disc in stacked community logos. */
export const AVATAR_ICON_ON_DARK = "#ffffff"

const ISODAY = /^(\d{4})-(\d{2})-(\d{2})$/

/**
 * Format a unix-epoch-seconds timestamp for the chat list (WhatsApp-style):
 * - Today → "HH:MM"
 * - Yesterday → "Yesterday"
 * - Older → locale date (e.g. "dd/mm/yyyy").
 * - Invalid / zero → "".
 */
export function formatChatTimestamp(ts: number): string {
  if (!ts) return ""
  try {
    const d = new Date(ts * 1000)
    if (isNaN(d.getTime())) return ""
    const now = new Date()
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
    const yesterday = new Date(today.getTime() - 86400000)
    const msgDay = new Date(d.getFullYear(), d.getMonth(), d.getDate())
    if (msgDay.getTime() === today.getTime()) {
      return d.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })
    }
    if (msgDay.getTime() === yesterday.getTime()) return "Yesterday"
    return d.toLocaleDateString([], { day: "numeric", month: "numeric", year: "numeric" })
  } catch {
    return ""
  }
}

/**
 *Take an ISO date string ("YYYY-MM-DD") and return a human-readable label
 * matching WhatsApp's date-separator style:
 * - Today → "Today"
 * - Yesterday → "Yesterday"
 * - This week → locale weekday name
 * - Otherwise → locale date (e.g. "31/07/2026")
 */
export function formatDateSeparator(iso: string): string {
  try {
    const m = ISODAY.exec(iso)
    if (!m) return iso
    const d = new Date(+m[1], +m[2] - 1, +m[3])
    if (isNaN(d.getTime())) return iso
    const now = new Date()
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
    const yesterday = new Date(today.getTime() - 86400000)
    const msgDay = new Date(d.getFullYear(), d.getMonth(), d.getDate())
    const diff = (today.getTime() - msgDay.getTime()) / 86400000
    if (diff === 0) return "Today"
    if (diff === 1) return "Yesterday"
    if (diff < 7) return d.toLocaleDateString([], { weekday: "long" })
    return d.toLocaleDateString([], { day: "numeric", month: "numeric", year: "numeric" })
  } catch {
    return iso
  }
}

/**
 *Extract "YYYY-MM-DD" from a message timestamp string (ISO8601). Returns
 * the ISO date part or "" if the timestamp is unparseable.
 */
export function dayKey(ts: string): string {
  try {
    const d = new Date(ts)
    if (isNaN(d.getTime())) return ""
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`
  } catch {
    return ""
  }
}
