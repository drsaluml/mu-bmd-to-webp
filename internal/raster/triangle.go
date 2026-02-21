package raster

import (
	"image"
	"math"
)

// RasterizeTriangle rasterizes a single triangle with texture mapping, z-buffer,
// sRGB color space, lighting, and ACES tone mapping.
//
// This is the HOT PATH — designed for zero allocation in the inner loop.
// All lighting is flat-shaded (per-face, not per-pixel).
func RasterizeTriangle(
	fb *FrameBuffer,
	px, py, pz []float64,
	uvs [][2]float32,
	vi [3]int, ti [3]int,
	tex *image.NRGBA,
	defaultR, defaultG, defaultB, defaultA uint8,
	lc *LightConfig,
) {
	nv := len(px)
	nuv := len(uvs)

	// Caller controls winding order via vi (renderer.go handles swap)
	idx := [3]int{vi[0], vi[1], vi[2]}

	// Bounds check
	for _, i := range idx {
		if i < 0 || i >= nv {
			return
		}
	}

	x0, y0, z0 := px[idx[0]], py[idx[0]], pz[idx[0]]
	x1, y1, z1 := px[idx[1]], py[idx[1]], pz[idx[1]]
	x2, y2, z2 := px[idx[2]], py[idx[2]], pz[idx[2]]

	uvIdx := [3]int{ti[0], ti[1], ti[2]}
	hasUV := tex != nil
	for _, i := range uvIdx {
		if i < 0 || i >= nuv {
			hasUV = false
			break
		}
	}

	var u0, v0uv, u1, v1uv, u2, v2uv float64
	if hasUV {
		u0, v0uv = float64(uvs[uvIdx[0]][0]), float64(uvs[uvIdx[0]][1])
		u1, v1uv = float64(uvs[uvIdx[1]][0]), float64(uvs[uvIdx[1]][1])
		u2, v2uv = float64(uvs[uvIdx[2]][0]), float64(uvs[uvIdx[2]][1])
	}

	// Face normal for flat shading
	e1x, e1y, e1z := x1-x0, y1-y0, z1-z0
	e2x, e2y, e2z := x2-x0, y2-y0, z2-z0
	nx := e1y*e2z - e1z*e2y
	ny := e1z*e2x - e1x*e2z
	nz := e1x*e2y - e1y*e2x
	nl := math.Sqrt(nx*nx + ny*ny + nz*nz)
	if nl < 1e-8 {
		return
	}
	invNL := 1.0 / nl
	nx *= invNL
	ny *= invNL
	nz *= invNL

	// Compute shade using lighting config
	ndlMain := math.Abs(nx*lc.LightDir[0] + ny*lc.LightDir[1] + nz*lc.LightDir[2])
	ndlRim := math.Abs(nx*lc.RimDir[0] + ny*lc.RimDir[1] + nz*lc.RimDir[2])
	hemi := (1.0-math.Abs(ny))*0.5 + 0.5
	hemiLight := hemi * lc.Hemi
	ndh := nx*lc.HalfMain[0] + ny*lc.HalfMain[1] + nz*lc.HalfMain[2]
	if ndh < 0 {
		ndh = 0
	}
	spec := math.Pow(ndh, lc.SpecPow) * lc.SpecInt
	shade := lc.Ambient + hemiLight + ndlMain*lc.Direct + ndlRim*lc.Rim + spec

	// Bounding box
	size := fb.Width
	minX := int(math.Min(math.Min(x0, x1), x2))
	maxX := int(math.Max(math.Max(x0, x1), x2)) + 1
	minY := int(math.Min(math.Min(y0, y1), y2))
	maxY := int(math.Max(math.Max(y0, y1), y2)) + 1

	if minX < 0 {
		minX = 0
	}
	if maxX >= size {
		maxX = size - 1
	}
	if minY < 0 {
		minY = 0
	}
	if maxY >= size {
		maxY = size - 1
	}
	if minX >= maxX || minY >= maxY {
		return
	}

	// Barycentric setup
	det := (y1-y2)*(x0-x2) + (x2-x1)*(y0-y2)
	if det > -1e-8 && det < 1e-8 {
		return
	}
	invDet := 1.0 / det

	// Precompute edge deltas
	dy12 := y1 - y2
	dx21 := x2 - x1
	dy20 := y2 - y0
	dx02 := x0 - x2

	exposure := lc.Exposure
	invGamma := lc.InvGamma

	// Pixel loop — zero allocations
	for sy := minY; sy <= maxY; sy++ {
		dsy := float64(sy) - y2
		rowOff := sy * size
		for sx := minX; sx <= maxX; sx++ {
			dsx := float64(sx) - x2
			w0 := (dy12*dsx + dx21*dsy) * invDet
			w1 := (dy20*dsx + dx02*dsy) * invDet
			w2 := 1.0 - w0 - w1

			if w0 < -0.001 || w1 < -0.001 || w2 < -0.001 {
				continue
			}

			z := w0*z0 + w1*z1 + w2*z2
			zIdx := rowOff + sx
			if z <= fb.ZBuf[zIdx] {
				continue
			}

			var cr, cg, cb, ca uint8
			if hasUV {
				u := w0*u0 + w1*u1 + w2*u2
				v := w0*v0uv + w1*v1uv + w2*v2uv
				cr, cg, cb, ca = SampleTexture(tex, u, v)
			} else {
				cr, cg, cb, ca = defaultR, defaultG, defaultB, defaultA
			}

			// Skip transparent texels
			if ca < 8 {
				continue
			}
			fb.ZBuf[zIdx] = z

			// sRGB decode → linear (LUT)
			lr := srgbToLinear[cr]
			lg := srgbToLinear[cg]
			lb := srgbToLinear[cb]

			// Apply shading + ACES tone mapping
			sr := lr * shade * exposure
			sg := lg * shade * exposure
			sb := lb * shade * exposure

			tr := ACESTonemap(sr)
			tg := ACESTonemap(sg)
			tb := ACESTonemap(sb)

			// Linear → sRGB encode
			fr := math.Pow(tr, invGamma)
			fg := math.Pow(tg, invGamma)
			ffb := math.Pow(tb, invGamma)

			pxIdx := zIdx * 4
			fb.Color[pxIdx] = clamp255(fr * 255)
			fb.Color[pxIdx+1] = clamp255(fg * 255)
			fb.Color[pxIdx+2] = clamp255(ffb * 255)
			fb.Color[pxIdx+3] = ca
		}
	}
}

// RasterizeTriangleAdditive renders a triangle with additive blending.
// No z-buffer check/write — colors are ADDED to existing framebuffer values.
// Used for inner glow meshes (liquid in bottles, _R suffix textures).
func RasterizeTriangleAdditive(
	fb *FrameBuffer,
	px, py, pz []float64,
	uvs [][2]float32,
	vi [3]int, ti [3]int,
	tex *image.NRGBA,
	defaultR, defaultG, defaultB, defaultA uint8,
	lc *LightConfig,
) {
	nv := len(px)
	nuv := len(uvs)

	idx := [3]int{vi[0], vi[1], vi[2]}
	for _, i := range idx {
		if i < 0 || i >= nv {
			return
		}
	}

	x0, y0 := px[idx[0]], py[idx[0]]
	x1, y1 := px[idx[1]], py[idx[1]]
	x2, y2 := px[idx[2]], py[idx[2]]

	uvIdx := [3]int{ti[0], ti[1], ti[2]}
	hasUV := tex != nil
	for _, i := range uvIdx {
		if i < 0 || i >= nuv {
			hasUV = false
			break
		}
	}

	var u0, v0uv, u1, v1uv, u2, v2uv float64
	if hasUV {
		u0, v0uv = float64(uvs[uvIdx[0]][0]), float64(uvs[uvIdx[0]][1])
		u1, v1uv = float64(uvs[uvIdx[1]][0]), float64(uvs[uvIdx[1]][1])
		u2, v2uv = float64(uvs[uvIdx[2]][0]), float64(uvs[uvIdx[2]][1])
	}

	// Flat shading
	e1x, e1y, e1z := x1-x0, y1-y0, pz[idx[1]]-pz[idx[0]]
	e2x, e2y, e2z := x2-x0, y2-y0, pz[idx[2]]-pz[idx[0]]
	nx := e1y*e2z - e1z*e2y
	ny := e1z*e2x - e1x*e2z
	nz := e1x*e2y - e1y*e2x
	nl := math.Sqrt(nx*nx + ny*ny + nz*nz)
	if nl < 1e-8 {
		return
	}
	invNL := 1.0 / nl
	nx *= invNL
	ny *= invNL
	nz *= invNL

	ndlMain := math.Abs(nx*lc.LightDir[0] + ny*lc.LightDir[1] + nz*lc.LightDir[2])
	ndlRim := math.Abs(nx*lc.RimDir[0] + ny*lc.RimDir[1] + nz*lc.RimDir[2])
	hemi := (1.0-math.Abs(ny))*0.5 + 0.5
	hemiLight := hemi * lc.Hemi
	ndh := nx*lc.HalfMain[0] + ny*lc.HalfMain[1] + nz*lc.HalfMain[2]
	if ndh < 0 {
		ndh = 0
	}
	spec := math.Pow(ndh, lc.SpecPow) * lc.SpecInt
	shade := lc.Ambient + hemiLight + ndlMain*lc.Direct + ndlRim*lc.Rim + spec

	size := fb.Width
	minX := int(math.Min(math.Min(x0, x1), x2))
	maxX := int(math.Max(math.Max(x0, x1), x2)) + 1
	minY := int(math.Min(math.Min(y0, y1), y2))
	maxY := int(math.Max(math.Max(y0, y1), y2)) + 1

	if minX < 0 {
		minX = 0
	}
	if maxX >= size {
		maxX = size - 1
	}
	if minY < 0 {
		minY = 0
	}
	if maxY >= size {
		maxY = size - 1
	}
	if minX >= maxX || minY >= maxY {
		return
	}

	det := (y1-y2)*(x0-x2) + (x2-x1)*(y0-y2)
	if det > -1e-8 && det < 1e-8 {
		return
	}
	invDet := 1.0 / det

	dy12 := y1 - y2
	dx21 := x2 - x1
	dy20 := y2 - y0
	dx02 := x0 - x2

	exposure := lc.Exposure
	invGamma := lc.InvGamma

	for sy := minY; sy <= maxY; sy++ {
		dsy := float64(sy) - y2
		rowOff := sy * size
		for sx := minX; sx <= maxX; sx++ {
			dsx := float64(sx) - x2
			w0 := (dy12*dsx + dx21*dsy) * invDet
			w1 := (dy20*dsx + dx02*dsy) * invDet
			w2 := 1.0 - w0 - w1

			if w0 < -0.001 || w1 < -0.001 || w2 < -0.001 {
				continue
			}

			var cr, cg, cb, ca uint8
			if hasUV {
				u := w0*u0 + w1*u1 + w2*u2
				v := w0*v0uv + w1*v1uv + w2*v2uv
				cr, cg, cb, ca = SampleTexture(tex, u, v)
			} else {
				cr, cg, cb, ca = defaultR, defaultG, defaultB, defaultA
			}

			if ca < 8 {
				continue
			}

			// No z-buffer check/write — additive blending

			lr := srgbToLinear[cr]
			lg := srgbToLinear[cg]
			lb := srgbToLinear[cb]

			sr := lr * shade * exposure
			sg := lg * shade * exposure
			sb := lb * shade * exposure

			tr := ACESTonemap(sr)
			tg := ACESTonemap(sg)
			tb := ACESTonemap(sb)

			fr := math.Pow(tr, invGamma) * 255
			fg := math.Pow(tg, invGamma) * 255
			ffb := math.Pow(tb, invGamma) * 255

			pxIdx := (rowOff + sx) * 4
			// Additive: add to existing pixel, clamp to 255
			fb.Color[pxIdx] = clamp255(float64(fb.Color[pxIdx]) + fr)
			fb.Color[pxIdx+1] = clamp255(float64(fb.Color[pxIdx+1]) + fg)
			fb.Color[pxIdx+2] = clamp255(float64(fb.Color[pxIdx+2]) + ffb)
			// Alpha: use brightness of added color (dark pixels stay transparent)
			lum := fr*0.299 + fg*0.587 + ffb*0.114
			addAlpha := clamp255(lum)
			if addAlpha > fb.Color[pxIdx+3] {
				fb.Color[pxIdx+3] = addAlpha
			}
		}
	}
}

func clamp255(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v + 0.5)
}
