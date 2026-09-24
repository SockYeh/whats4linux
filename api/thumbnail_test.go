package api

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
)

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func jpegWithOrientation(t *testing.T, img image.Image, orientation int) []byte {
	t.Helper()
	encoded := encodeJPEG(t, img)
	app1 := buildExifAPP1(orientation)
	out := make([]byte, 0, 2+len(app1)+len(encoded)-2)
	out = append(out, 0xFF, 0xD8)
	out = append(out, app1...)
	out = append(out, encoded[2:]...)
	return out
}

func buildExifAPP1(orientation int) []byte {
	tiff := []byte{
		'M', 'M',
		0x00, 0x2A,
		0x00, 0x00, 0x00, 0x08,
		0x00, 0x01,
		0x01, 0x12, 0x00, 0x03, 0x00, 0x00, 0x00, 0x01,
		0x00, byte(orientation), 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	payload := append([]byte("Exif\x00\x00"), tiff...)
	app1 := make([]byte, 4+len(payload))
	app1[0], app1[1] = 0xFF, 0xE1
	binary.BigEndian.PutUint16(app1[2:4], uint16(2+len(payload)))
	copy(app1[4:], payload)
	return app1
}

func pngIHDR(width, height uint32) []byte {
	sig := []byte{137, 80, 78, 71, 13, 10, 26, 10}
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:4], width)
	binary.BigEndian.PutUint32(ihdr[4:8], height)
	ihdr[8] = 8
	ihdr[9] = 2
	crc := crc32.ChecksumIEEE(append([]byte("IHDR"), ihdr...))
	chunk := make([]byte, 4+4+13+4)
	binary.BigEndian.PutUint32(chunk[0:4], 13)
	copy(chunk[4:8], "IHDR")
	copy(chunk[8:21], ihdr)
	binary.BigEndian.PutUint32(chunk[21:25], crc)
	return append(sig, chunk...)
}

func decodeThumb(t *testing.T, data []byte) image.Image {
	t.Helper()
	if len(data) == 0 {
		t.Fatal("empty thumbnail")
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("thumbnail is not a JPEG: %v", err)
	}
	return img
}

func TestGenerateThumbnailAspectRatioAndMinHeight(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1600, 4))
	for x := 0; x < 1600; x++ {
		img.Set(x, 0, color.RGBA{R: 255, A: 255})
		img.Set(x, 1, color.RGBA{G: 255, A: 255})
		img.Set(x, 2, color.RGBA{B: 255, A: 255})
		img.Set(x, 3, color.RGBA{R: 255, G: 255, A: 255})
	}
	thumb := generateThumbnail(encodePNG(t, img), "image/png")
	got := decodeThumb(t, thumb)
	b := got.Bounds()
	if b.Dx() < 1 || b.Dy() < 1 {
		t.Fatalf("thumbnail dims %dx%d, want at least 1x1", b.Dx(), b.Dy())
	}
	if b.Dx() > thumbnailMaxDim {
		t.Fatalf("width %d exceeds max %d", b.Dx(), thumbnailMaxDim)
	}
}

func TestGenerateThumbnailCompositesAlphaOnWhite(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 0})
		}
	}
	thumb := generateThumbnail(encodePNG(t, img), "image/png")
	got := decodeThumb(t, thumb)
	r, g, b, _ := got.At(got.Bounds().Min.X, got.Bounds().Min.Y).RGBA()
	if r < 0xC000 || g < 0xC000 || b < 0xC000 {
		t.Fatalf("transparent PNG thumbnail pixel = %d,%d,%d want near-white", r, g, b)
	}
}

func TestGenerateThumbnailAppliesEXIFOrientation(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 20, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			src.Set(x, y, color.RGBA{R: 0, G: 0, B: 255, A: 255})
		}
	}
	for y := 0; y < 10; y++ {
		src.Set(0, y, color.RGBA{R: 255, A: 255})
	}
	thumb := generateThumbnail(jpegWithOrientation(t, src, 6), "image/jpeg")
	got := decodeThumb(t, thumb)
	if got.Bounds().Dx() >= got.Bounds().Dy() {
		t.Fatalf("orientation 6 should swap axes, got %dx%d", got.Bounds().Dx(), got.Bounds().Dy())
	}
}

func TestGenerateThumbnailRejectsHugeDimensions(t *testing.T) {
	data := pngIHDR(20000, 20000)
	if thumb := generateThumbnail(data, "image/png"); thumb != nil {
		t.Fatal("expected nil thumbnail for oversized PNG header")
	}
}

func TestGenerateThumbnailRejectsNonImage(t *testing.T) {
	if thumb := generateThumbnail([]byte("%PDF-1.4"), "application/pdf"); thumb != nil {
		t.Fatal("expected nil thumbnail for PDF")
	}
	if thumb := generateThumbnail([]byte("not an image"), "image/png"); thumb != nil {
		t.Fatal("expected nil thumbnail for invalid PNG")
	}
}

func TestGenerateThumbnailGIF(t *testing.T) {
	src := image.NewPaletted(image.Rect(0, 0, 16, 16), color.Palette{color.White, color.Black})
	var buf bytes.Buffer
	if err := gif.Encode(&buf, src, nil); err != nil {
		t.Fatal(err)
	}
	thumb := generateThumbnail(buf.Bytes(), "image/gif")
	_ = decodeThumb(t, thumb)
}

func TestGenerateThumbnailCapsSourceBytes(t *testing.T) {
	data := bytes.Repeat([]byte{0xFF, 0xD8}, thumbnailMaxSourceBytes/2+1)
	if thumb := generateThumbnail(data, "image/jpeg"); thumb != nil {
		t.Fatal("expected nil thumbnail for oversized payload")
	}
}

func TestSafeDownloadName(t *testing.T) {
	if got := safeDownloadName("../.ssh/authorized_keys", "image/jpeg", ""); got == "../.ssh/authorized_keys.jpg" || bytes.Contains([]byte(got), []byte("..")) {
		t.Fatalf("unsanitised name: %q", got)
	}
	if got := safeDownloadName("abc", "image/png", "photo.png"); got != "photo.png" {
		t.Fatalf("stored name = %q", got)
	}
	if got := safeDownloadName("id", "application/pdf", ""); got != "id.jpg" {
		t.Fatalf("unknown mime defaulted to %q, want id.jpg", got)
	}
}
