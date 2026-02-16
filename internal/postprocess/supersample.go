package postprocess

import (
	"image"

	"golang.org/x/image/draw"
)

// Downsample reduces image size with premultiplied-alpha-aware Lanczos filtering.
// This prevents dark halo artifacts at transparent edges.
func Downsample(img *image.NRGBA, targetSize int) *image.NRGBA {
	b := img.Bounds()
	if b.Dx() <= targetSize && b.Dy() <= targetSize {
		return img
	}

	// Premultiply alpha
	premul := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			si := img.PixOffset(x, y)
			di := premul.PixOffset(x, y)
			a := float64(img.Pix[si+3]) / 255.0
			premul.Pix[di] = uint8(float64(img.Pix[si])*a + 0.5)
			premul.Pix[di+1] = uint8(float64(img.Pix[si+1])*a + 0.5)
			premul.Pix[di+2] = uint8(float64(img.Pix[si+2])*a + 0.5)
			premul.Pix[di+3] = img.Pix[si+3]
		}
	}

	// Downsample with CatmullRom (approximates Lanczos)
	dst := image.NewRGBA(image.Rect(0, 0, targetSize, targetSize))
	draw.CatmullRom.Scale(dst, dst.Bounds(), premul, premul.Bounds(), draw.Src, nil)

	// Unpremultiply alpha
	result := image.NewNRGBA(dst.Bounds())
	for y := 0; y < targetSize; y++ {
		for x := 0; x < targetSize; x++ {
			si := dst.PixOffset(x, y)
			di := result.PixOffset(x, y)
			a := float64(dst.Pix[si+3])
			if a > 1 {
				inv := 255.0 / a
				result.Pix[di] = clamp8(float64(dst.Pix[si]) * inv)
				result.Pix[di+1] = clamp8(float64(dst.Pix[si+1]) * inv)
				result.Pix[di+2] = clamp8(float64(dst.Pix[si+2]) * inv)
			}
			result.Pix[di+3] = dst.Pix[si+3]
		}
	}

	return result
}

func clamp8(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v + 0.5)
}
