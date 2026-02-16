package texture

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"os"
	"strings"

	"github.com/ftrvxmtrx/tga"
)

// LoadTexture reads an OZJ or OZT file and returns an NRGBA image.
func LoadTexture(path string) (*image.NRGBA, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("texture: read %s: %w", path, err)
	}

	ext := strings.ToLower(path[len(path)-4:])
	var img image.Image

	switch ext {
	case ".ozj":
		// OZJ: 24-byte header + JPEG data
		if len(raw) <= 24 {
			return nil, fmt.Errorf("texture: OZJ too short: %s", path)
		}
		var err error
		img, err = jpeg.Decode(bytes.NewReader(raw[24:]))
		if err != nil {
			return nil, fmt.Errorf("texture: decode OZJ %s: %w", path, err)
		}
	case ".ozt":
		// OZT: 4-byte header + TGA data
		if len(raw) <= 4 {
			return nil, fmt.Errorf("texture: OZT too short: %s", path)
		}
		var err error
		img, err = tga.Decode(bytes.NewReader(raw[4:]))
		if err != nil {
			return nil, fmt.Errorf("texture: decode OZT %s: %w", path, err)
		}
	default:
		return nil, fmt.Errorf("texture: unknown extension: %s", ext)
	}

	return toNRGBA(img), nil
}

// toNRGBA converts any image to NRGBA format.
func toNRGBA(src image.Image) *image.NRGBA {
	if n, ok := src.(*image.NRGBA); ok {
		return n
	}
	b := src.Bounds()
	dst := image.NewNRGBA(b)
	draw.Draw(dst, b, src, b.Min, draw.Src)
	// Ensure alpha=255 for opaque formats (JPEG/YCbCr/Gray)
	switch src.(type) {
	case *image.YCbCr, *image.Gray:
		for i := 3; i < len(dst.Pix); i += 4 {
			dst.Pix[i] = 255
		}
	}
	return dst
}
