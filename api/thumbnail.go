package api

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"sync"

	_ "golang.org/x/image/webp"
)

const (
	thumbnailMaxDim         = 320
	thumbnailMaxSourceBytes = 8 << 20
	thumbnailMaxPixels      = 4096 * 4096
	thumbnailJPEGQuality    = 60
)

// thumbnailUnavailable is stored when generation failed so GetThumbnail
// short-circuits instead of re-decoding the same file on every open.
var thumbnailUnavailable = []byte{0}

func isUnavailableThumbnail(data []byte) bool {
	return len(data) == 1 && data[0] == 0
}

var thumbnailDecodeMu sync.Mutex

func generateThumbnail(data []byte, mime string) []byte {
	if len(data) == 0 || len(data) > thumbnailMaxSourceBytes {
		return nil
	}
	if mime != "" && !isImageMime(mime) {
		return nil
	}

	thumbnailDecodeMu.Lock()
	defer thumbnailDecodeMu.Unlock()

	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return nil
	}
	if int64(cfg.Width)*int64(cfg.Height) > thumbnailMaxPixels {
		return nil
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}

	img = applyOrientation(img, jpegOrientation(data))
	img = flattenOnWhite(img)

	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW <= 0 || srcH <= 0 {
		return nil
	}

	scale := float64(thumbnailMaxDim) / float64(srcW)
	if srcH > srcW {
		scale = float64(thumbnailMaxDim) / float64(srcH)
	}
	if scale >= 1 {
		return encodeThumbnailJPEG(img)
	}

	dstW := int(float64(srcW) * scale)
	dstH := int(float64(srcH) * scale)
	if dstW < 1 {
		dstW = 1
	}
	if dstH < 1 {
		dstH = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for y := 0; y < dstH; y++ {
		srcY := bounds.Min.Y + y*srcH/dstH
		for x := 0; x < dstW; x++ {
			srcX := bounds.Min.X + x*srcW/dstW
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}
	return encodeThumbnailJPEG(dst)
}

func encodeThumbnailJPEG(img image.Image) []byte {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: thumbnailJPEGQuality}); err != nil {
		return nil
	}
	return buf.Bytes()
}

func flattenOnWhite(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Over)
	return dst
}

func applyOrientation(img image.Image, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	switch orientation {
	case 2: // mirror horizontal
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(x, y, img.At(b.Min.X+w-1-x, b.Min.Y+y))
			}
		}
		return dst
	case 3: // rotate 180
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(x, y, img.At(b.Min.X+w-1-x, b.Min.Y+h-1-y))
			}
		}
		return dst
	case 4: // mirror vertical
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(x, y, img.At(b.Min.X+x, b.Min.Y+h-1-y))
			}
		}
		return dst
	case 5: // mirror horizontal + rotate 270 CW (transpose)
		dst := image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(y, x, img.At(b.Min.X+x, b.Min.Y+y))
			}
		}
		return dst
	case 6: // rotate 90 CW
		dst := image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(h-1-y, x, img.At(b.Min.X+x, b.Min.Y+y))
			}
		}
		return dst
	case 7: // mirror horizontal + rotate 90 CW (transverse)
		dst := image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(h-1-y, w-1-x, img.At(b.Min.X+x, b.Min.Y+y))
			}
		}
		return dst
	case 8: // rotate 270 CW
		dst := image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(y, w-1-x, img.At(b.Min.X+x, b.Min.Y+y))
			}
		}
		return dst
	default:
		return img
	}
}

func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			return 1
		}
		marker := data[i+1]
		if marker == 0xDA {
			return 1
		}
		seglen := int(data[i+2])<<8 | int(data[i+3])
		if seglen < 2 || i+2+seglen > len(data) {
			return 1
		}
		if marker == 0xE1 {
			payload := data[i+4 : i+2+seglen]
			if len(payload) >= 6 && string(payload[:6]) == "Exif\x00\x00" {
				if o := tiffOrientation(payload[6:]); o >= 1 && o <= 8 {
					return o
				}
			}
		}
		i += 2 + seglen
	}
	return 1
}

func tiffOrientation(tiff []byte) int {
	if len(tiff) < 8 {
		return 0
	}
	var bigEndian bool
	switch {
	case tiff[0] == 'M' && tiff[1] == 'M':
		bigEndian = true
	case tiff[0] == 'I' && tiff[1] == 'I':
		bigEndian = false
	default:
		return 0
	}
	u16 := func(off int) uint16 {
		if off+1 >= len(tiff) {
			return 0
		}
		if bigEndian {
			return uint16(tiff[off])<<8 | uint16(tiff[off+1])
		}
		return uint16(tiff[off]) | uint16(tiff[off+1])<<8
	}
	u32 := func(off int) uint32 {
		if off+3 >= len(tiff) {
			return 0
		}
		if bigEndian {
			return uint32(tiff[off])<<24 | uint32(tiff[off+1])<<16 | uint32(tiff[off+2])<<8 | uint32(tiff[off+3])
		}
		return uint32(tiff[off]) | uint32(tiff[off+1])<<8 | uint32(tiff[off+2])<<16 | uint32(tiff[off+3])<<24
	}
	ifd := int(u32(4))
	if ifd < 8 || ifd+2 > len(tiff) {
		return 0
	}
	n := int(u16(ifd))
	for e := 0; e < n; e++ {
		off := ifd + 2 + e*12
		if off+12 > len(tiff) {
			return 0
		}
		if u16(off) != 0x0112 {
			continue
		}
		typ := u16(off + 2)
		count := u32(off + 4)
		if count < 1 {
			return 0
		}
		var val uint16
		if typ == 4 {
			val = uint16(u32(off + 8))
		} else {
			val = u16(off + 8)
		}
		if val >= 1 && val <= 8 {
			return int(val)
		}
		return 0
	}
	return 0
}
