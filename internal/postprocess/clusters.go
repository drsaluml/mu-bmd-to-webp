package postprocess

import "image"

// RemoveSmallClusters zeroes out small disconnected pixel groups.
// minRatio is the minimum fraction of total non-transparent pixels to keep.
func RemoveSmallClusters(img *image.NRGBA, minRatio float64) *image.NRGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	stride := img.Stride

	// Find non-transparent pixels
	alpha := make([]bool, w*h)
	totalAlpha := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if img.Pix[y*stride+x*4+3] > 0 {
				alpha[y*w+x] = true
				totalAlpha++
			}
		}
	}

	if totalAlpha == 0 {
		return img
	}

	// 8-connected flood fill BFS
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

			// BFS from this pixel
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

	if compID <= 1 {
		return img
	}

	minSize := int(float64(totalAlpha) * minRatio)

	// Zero out small components
	result := image.NewNRGBA(b)
	copy(result.Pix, img.Pix)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			if labels[idx] >= 0 && compSizes[labels[idx]] < minSize {
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
