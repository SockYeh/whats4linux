import { describe, it, expect } from "vitest"
import { mediaBox, imageMediaIDs, visibleImageIDs } from "./MediaContent"

// Rendered bounds mirrored from MediaContent: min-w 300, max-w 330, max-h 400.
describe("mediaBox", () => {
  it("returns null when dimensions are missing or invalid", () => {
    expect(mediaBox(undefined, undefined)).toBeNull()
    expect(mediaBox(0, 100)).toBeNull()
    expect(mediaBox(100, 0)).toBeNull()
    expect(mediaBox(-5, 10)).toBeNull()
  })

  it("scales large square images down to the max width", () => {
    expect(mediaBox(1000, 1000)).toEqual({ width: 330, height: 330 })
  })

  it("clamps tall images to max height then widens to min width", () => {
    // 500x4000 -> height-limited to 50x400 -> widened to 300, height clamped.
    expect(mediaBox(500, 4000)).toEqual({ width: 300, height: 400 })
  })

  it("keeps very wide images at their scaled height", () => {
    // 4000x500 -> width-limited: 330 x ~41
    const box = mediaBox(4000, 500)
    expect(box?.width).toBe(330)
    expect(box?.height).toBe(41)
  })

  it("widens small images to the min width, scaling height", () => {
    // 100x100 renders at min-width 300; height follows to 300.
    expect(mediaBox(100, 100)).toEqual({ width: 300, height: 300 })
  })

  it("leaves mid-size images within bounds untouched", () => {
    expect(mediaBox(320, 200)).toEqual({ width: 320, height: 200 })
  })
})

describe("imageMediaIDs", () => {
  it("keeps image and sticker ids and drops temp rows", () => {
    expect(
      imageMediaIDs([
        { Info: { ID: "img-1" }, Content: { imageMessage: {} } },
        { Info: { ID: "txt-1" }, Content: {} },
        { Info: { ID: "stk-1" }, Content: { stickerMessage: {} } },
        { Info: { ID: "temp-1" }, Content: { imageMessage: {} } },
      ]),
    ).toEqual(["img-1", "stk-1"])
  })

  it("takes the last N image ids for the visible page", () => {
    const messages = Array.from({ length: 12 }, (_, i) => ({
      Info: { ID: `img-${i}` },
      Content: { imageMessage: {} },
    }))
    expect(visibleImageIDs(messages, 8)).toEqual(messages.slice(-8).map(message => message.Info.ID))
  })
})
