package postprocess

import (
	"image"
	"image/color"

	"golang.org/x/image/draw"
)

// MirrorPair takes a rendered image, isolates the largest connected component,
// duplicates it with a horizontal mirror, and places the original and mirror
// side by side to create a pair (e.g. single boot → pair of boots).
// Works with both bones=false (single item) and bones=true (picks one from pair).
// The result is centered on a canvas of the given size.
func MirrorPair(img *image.NRGBA, size int, fillRatio float64) *image.NRGBA {
	// Isolate the largest connected component (picks one boot from walking pair)
	img = keepLargestComponent(img)

	// Crop to non-transparent bounds
	cropped := cropAlpha(img)
	cb := cropped.Bounds()
	cw, ch := cb.Dx(), cb.Dy()
	if cw == 0 || ch == 0 {
		return img
	}

	// Mirror the cropped image horizontally
	mirrored := image.NewNRGBA(image.Rect(0, 0, cw, ch))
	for y := 0; y < ch; y++ {
		for x := 0; x < cw; x++ {
			mirrored.SetNRGBA(x, y, cropped.NRGBAAt(cb.Min.X+cw-1-x, cb.Min.Y+y))
		}
	}

	// Gap between the two items (proportional to item width)
	gap := cw / 8
	if gap < 2 {
		gap = 2
	}

	// Compose: original on left, mirror on right
	pairW := cw*2 + gap
	pairH := ch
	pair := image.NewNRGBA(image.Rect(0, 0, pairW, pairH))
	// Left = original
	draw.Copy(pair, image.Pt(0, 0), cropped, cb, draw.Over, nil)
	// Right = mirrored
	draw.Copy(pair, image.Pt(cw+gap, 0), mirrored, mirrored.Bounds(), draw.Over, nil)

	// Scale and center onto final canvas
	canvas := image.NewNRGBA(image.Rect(0, 0, size, size))
	for i := range canvas.Pix {
		canvas.Pix[i] = 0
	}

	// Fit the pair into canvas with fillRatio
	scaleX := float64(size) * fillRatio / float64(pairW)
	scaleY := float64(size) * fillRatio / float64(pairH)
	sc := scaleX
	if scaleY < sc {
		sc = scaleY
	}

	dstW := int(float64(pairW) * sc)
	dstH := int(float64(pairH) * sc)
	if dstW < 1 {
		dstW = 1
	}
	if dstH < 1 {
		dstH = 1
	}

	offX := (size - dstW) / 2
	offY := (size - dstH) / 2

	dstRect := image.Rect(offX, offY, offX+dstW, offY+dstH)
	draw.CatmullRom.Scale(canvas, dstRect, pair, pair.Bounds(), draw.Over, &draw.Options{
		SrcMask:  &alphaMask{pair},
		SrcMaskP: pair.Bounds().Min,
	})

	// Fill fully transparent pixels with transparent
	_ = color.NRGBA{}

	return canvas
}

// keepLargestComponent finds connected components via 8-connected BFS
// and zeroes out everything except the largest one.
// This isolates one boot from a bone-assembled pair.
func keepLargestComponent(img *image.NRGBA) *image.NRGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	stride := img.Stride

	// Build alpha mask
	alpha := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if img.Pix[y*stride+x*4+3] > 0 {
				alpha[y*w+x] = true
			}
		}
	}

	// BFS to label connected components
	labels := make([]int, w*h)
	for i := range labels {
		labels[i] = -1
	}
	var compSizes []int
	compID := 0

	dx := [8]int{-1, 0, 1, -1, 1, -1, 0, 1}
	dy := [8]int{-1, -1, -1, 0, 0, 1, 1, 1}
	queue := make([]int, 0, 1024)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			if !alpha[idx] || labels[idx] >= 0 {
				continue
			}
			queue = queue[:0]
			queue = append(queue, idx)
			labels[idx] = compID
			size := 0
			for len(queue) > 0 {
				curr := queue[0]
				queue = queue[1:]
				size++
				cy := curr / w
				cx := curr % w
				for d := 0; d < 8; d++ {
					nx := cx + dx[d]
					ny := cy + dy[d]
					if nx < 0 || nx >= w || ny < 0 || ny >= h {
						continue
					}
					ni := ny*w + nx
					if alpha[ni] && labels[ni] < 0 {
						labels[ni] = compID
						queue = append(queue, ni)
					}
				}
			}
			compSizes = append(compSizes, size)
			compID++
		}
	}

	// Only one or zero components — nothing to isolate
	if compID <= 1 {
		return img
	}

	// Find the largest component
	bestID := 0
	for i := 1; i < compID; i++ {
		if compSizes[i] > compSizes[bestID] {
			bestID = i
		}
	}

	// Zero out all other components
	result := image.NewNRGBA(b)
	copy(result.Pix, img.Pix)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			if labels[idx] >= 0 && labels[idx] != bestID {
				i := y*stride + x*4
				result.Pix[i] = 0
				result.Pix[i+1] = 0
				result.Pix[i+2] = 0
				result.Pix[i+3] = 0
			}
		}
	}
	return result
}

// alphaMask implements image.Image using only the alpha channel.
type alphaMask struct {
	src *image.NRGBA
}

func (m *alphaMask) ColorModel() color.Model { return color.AlphaModel }
func (m *alphaMask) Bounds() image.Rectangle { return m.src.Bounds() }
func (m *alphaMask) At(x, y int) color.Color {
	_, _, _, a := m.src.At(x, y).RGBA()
	return color.Alpha{A: uint8(a >> 8)}
}
