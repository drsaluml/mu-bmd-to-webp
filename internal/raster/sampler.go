package raster

import "image"

// SampleTexture performs bilinear filtering with UV wrapping.
// Returns RGBA as uint8. Accesses tex.Pix directly for performance.
func SampleTexture(tex *image.NRGBA, u, v float64) (r, g, b, a uint8) {
	w := tex.Rect.Dx()
	h := tex.Rect.Dy()

	// Wrap UVs
	u = u - float64(int(u))
	if u < 0 {
		u += 1.0
	}
	v = v - float64(int(v))
	if v < 0 {
		v += 1.0
	}

	fx := u * float64(w-1)
	fy := v * float64(h-1)
	x0 := int(fx)
	y0 := int(fy)
	x1 := (x0 + 1) % w
	y1 := (y0 + 1) % h
	dx := fx - float64(x0)
	dy := fy - float64(y0)

	stride := tex.Stride
	pix := tex.Pix

	// Four texels
	i00 := y0*stride + x0*4
	i10 := y0*stride + x1*4
	i01 := y1*stride + x0*4
	i11 := y1*stride + x1*4

	w00 := (1 - dx) * (1 - dy)
	w10 := dx * (1 - dy)
	w01 := (1 - dx) * dy
	w11 := dx * dy

	fr := float64(pix[i00])*w00 + float64(pix[i10])*w10 + float64(pix[i01])*w01 + float64(pix[i11])*w11
	fg := float64(pix[i00+1])*w00 + float64(pix[i10+1])*w10 + float64(pix[i01+1])*w01 + float64(pix[i11+1])*w11
	fb := float64(pix[i00+2])*w00 + float64(pix[i10+2])*w10 + float64(pix[i01+2])*w01 + float64(pix[i11+2])*w11
	fa := float64(pix[i00+3])*w00 + float64(pix[i10+3])*w10 + float64(pix[i01+3])*w01 + float64(pix[i11+3])*w11

	return uint8(fr + 0.5), uint8(fg + 0.5), uint8(fb + 0.5), uint8(fa + 0.5)
}
