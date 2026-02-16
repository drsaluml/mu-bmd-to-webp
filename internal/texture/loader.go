package texture

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"os"
	"strings"

	_ "github.com/ftrvxmtrx/tga"
)

// LoadTexture reads an OZJ or OZT file and returns an NRGBA image.
func LoadTexture(path string) (*image.NRGBA, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("texture: read %s: %w", path, err)
	}

	ext := strings.ToLower(path[len(path)-4:])
	var imgData []byte

	switch ext {
	case ".ozj":
		// OZJ: 24-byte header + JPEG data
		if len(raw) <= 24 {
			return nil, fmt.Errorf("texture: OZJ too short: %s", path)
		}
		imgData = raw[24:]
	case ".ozt":
		// OZT: 4-byte header + TGA data
		if len(raw) <= 4 {
			return nil, fmt.Errorf("texture: OZT too short: %s", path)
		}
		imgData = raw[4:]
	default:
		return nil, fmt.Errorf("texture: unknown extension: %s", ext)
	}

	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return nil, fmt.Errorf("texture: decode %s: %w", path, err)
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
	// Check if source has alpha
	switch src.(type) {
	case *image.YCbCr, *image.Gray:
		// No alpha — draw and set alpha to 255
		draw.Draw(dst, b, src, b.Min, draw.Src)
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				i := dst.PixOffset(x, y)
				dst.Pix[i+3] = 255
			}
		}
	default:
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				c := src.At(x, y)
				r, g, b_, a := color.NRGBAModel.Convert(c).(color.NRGBA).R,
					color.NRGBAModel.Convert(c).(color.NRGBA).G,
					color.NRGBAModel.Convert(c).(color.NRGBA).B,
					color.NRGBAModel.Convert(c).(color.NRGBA).A
				i := dst.PixOffset(x, y)
				dst.Pix[i] = r
				dst.Pix[i+1] = g
				dst.Pix[i+2] = b_
				dst.Pix[i+3] = a
			}
		}
	}
	return dst
}
