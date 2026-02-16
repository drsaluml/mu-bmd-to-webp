package raster

import "math"

// FrameBuffer holds the rendering target as flat slices for cache locality.
type FrameBuffer struct {
	Width  int
	Height int
	Color  []uint8   // RGBA interleaved, len = W*H*4
	ZBuf   []float64 // depth per pixel, len = W*H, initialized to -inf
}

// NewFrameBuffer allocates a zeroed color buffer and -inf z-buffer.
func NewFrameBuffer(w, h int) *FrameBuffer {
	n := w * h
	zbuf := make([]float64, n)
	for i := range zbuf {
		zbuf[i] = math.Inf(-1)
	}
	return &FrameBuffer{
		Width:  w,
		Height: h,
		Color:  make([]uint8, n*4),
		ZBuf:   zbuf,
	}
}
