package imageutil

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/d2cTool/goprofile/internal/domain"
)

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 30, G: 180, B: 160, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDetectAndInspect(t *testing.T) {
	t.Parallel()
	data := testPNG(t, 32, 24)
	info, err := Inspect(data)
	if err != nil {
		t.Fatal(err)
	}
	if info.MIME != MIMEPNG || info.Width != 32 || info.Height != 24 {
		t.Fatalf("%+v", info)
	}
	if _, err := DetectMIME([]byte("not-an-image")); err != domain.ErrInvalidFileFormat {
		t.Fatalf("expected invalid format, got %v", err)
	}
}

func TestThumbnailAndConvert(t *testing.T) {
	t.Parallel()
	data := testPNG(t, 200, 150)
	thumb, mime, err := MakeThumbnail(data, 100, 100, "jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if mime != MIMEJPEG {
		t.Fatalf("mime %s", mime)
	}
	info, err := Inspect(thumb)
	if err != nil {
		t.Fatal(err)
	}
	if info.Width != 100 || info.Height != 100 {
		t.Fatalf("thumb size %+v", info)
	}
	webpData, mime, err := Convert(data, "webp")
	if err != nil || mime != MIMEWebP {
		t.Fatalf("webp %s %v", mime, err)
	}
	if _, err := Inspect(webpData); err != nil {
		t.Fatal(err)
	}
}

func TestFormatFromMIME(t *testing.T) {
	t.Parallel()
	if FormatFromMIME(MIMEJPEG) != "jpeg" || ExtForMIME(MIMEPNG) != ".png" {
		t.Fatal("mapping")
	}
	if FormatFromMIME("application/octet-stream") != "" {
		t.Fatal("unknown mime must be empty")
	}
}

func TestInspectRejectsHugeDimensions(t *testing.T) {
	t.Parallel()
	if err := checkPixels(10000, 10000); err != domain.ErrImageTooLarge {
		t.Fatalf("got %v", err)
	}
	if err := checkPixels(0, 10); err != domain.ErrInvalidImage {
		t.Fatalf("got %v", err)
	}
}
