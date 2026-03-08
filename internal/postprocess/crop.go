package postprocess

import "image"

// CropBottom clears the bottom fraction of the image to transparent.
// ratio is 0-1: 0.4 means clear the bottom 40% of opaque content.
// It finds the bounding box of non-transparent pixels, then clears
// pixels below (top + height*(1-ratio)).
func CropBottom(img *image.NRGBA, ratio float64) {
	b := img.Bounds()
	// Find bounding box of non-transparent pixels
	minY, maxY := b.Max.Y, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			off := (y-b.Min.Y)*img.Stride + (x-b.Min.X)*4
			if img.Pix[off+3] > 0 {
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if minY >= maxY {
		return
	}
	contentH := float64(maxY - minY + 1)
	cutY := minY + int(contentH*(1-ratio))

	// Clear everything below cutY to transparent
	for y := cutY; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			off := (y-b.Min.Y)*img.Stride + (x-b.Min.X)*4
			img.Pix[off] = 0
			img.Pix[off+1] = 0
			img.Pix[off+2] = 0
			img.Pix[off+3] = 0
		}
	}
}
