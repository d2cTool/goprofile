package imageutil

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"strings"

	"github.com/HugoSmits86/nativewebp"
	"github.com/disintegration/imaging"
	xwebp "golang.org/x/image/webp"

	"github.com/d2cTool/goprofile/internal/domain"
)

const (
	MIMEJPEG = "image/jpeg"
	MIMEPNG  = "image/png"
	MIMEWebP = "image/webp"
)

func init() {
	image.RegisterFormat("webp", "RIFF????WEBP", xwebp.Decode, xwebp.DecodeConfig)
}

type Info struct {
	MIME   string
	Ext    string
	Width  int
	Height int
}

func DetectMIME(data []byte) (string, error) {
	if len(data) < 12 {
		return "", domain.ErrInvalidFileFormat
	}
	switch {
	case data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return MIMEJPEG, nil
	case bytes.Equal(data[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}):
		return MIMEPNG, nil
	case string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return MIMEWebP, nil
	default:
		return "", domain.ErrInvalidFileFormat
	}
}

func ExtForMIME(mime string) string {
	switch mime {
	case MIMEJPEG:
		return ".jpg"
	case MIMEPNG:
		return ".png"
	case MIMEWebP:
		return ".webp"
	default:
		return ""
	}
}

func Inspect(data []byte) (Info, error) {
	mime, err := DetectMIME(data)
	if err != nil {
		return Info{}, err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Info{}, domain.ErrInvalidImage
	}
	if err := checkPixels(cfg.Width, cfg.Height); err != nil {
		return Info{}, err
	}
	return Info{
		MIME:   mime,
		Ext:    ExtForMIME(mime),
		Width:  cfg.Width,
		Height: cfg.Height,
	}, nil
}

func Decode(data []byte) (image.Image, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidImage, err)
	}
	if err := checkPixels(cfg.Width, cfg.Height); err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidImage, err)
	}
	return img, nil
}

func checkPixels(w, h int) error {
	if w <= 0 || h <= 0 {
		return domain.ErrInvalidImage
	}
	if w > domain.MaxPixels || h > domain.MaxPixels || w*h > domain.MaxPixels {
		return domain.ErrImageTooLarge
	}
	return nil
}

func Resize(src image.Image, width, height int) image.Image {
	return imaging.Fill(src, width, height, imaging.Center, imaging.Lanczos)
}

func Encode(img image.Image, format string) ([]byte, string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "jpg" {
		format = "jpeg"
	}
	var buf bytes.Buffer
	switch format {
	case "jpeg", "":
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), MIMEJPEG, nil
	case "png":
		if err := png.Encode(&buf, img); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), MIMEPNG, nil
	case "webp":
		if err := nativewebp.Encode(&buf, img, nil); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), MIMEWebP, nil
	default:
		return nil, "", domain.ErrInvalidFormatParam
	}
}

func Convert(data []byte, format string) ([]byte, string, error) {
	img, err := Decode(data)
	if err != nil {
		return nil, "", err
	}
	return Encode(img, format)
}

func MakeThumbnail(data []byte, width, height int, format string) ([]byte, string, error) {
	img, err := Decode(data)
	if err != nil {
		return nil, "", err
	}
	return Encode(Resize(img, width, height), format)
}

func FormatFromMIME(mime string) string {
	switch mime {
	case MIMEJPEG:
		return "jpeg"
	case MIMEPNG:
		return "png"
	case MIMEWebP:
		return "webp"
	default:
		return ""
	}
}
