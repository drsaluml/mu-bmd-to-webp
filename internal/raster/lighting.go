package raster

import (
	"math"

	"mu-bmd-renderer/internal/mathutil"
)

// LightConfig holds precomputed lighting parameters.
type LightConfig struct {
	LightDir  mathutil.Vec3
	RimDir    mathutil.Vec3
	ViewDir   mathutil.Vec3
	HalfMain  mathutil.Vec3 // precomputed half-vector for Blinn-Phong
	Ambient   float64
	Hemi      float64
	Direct    float64
	Rim       float64
	SpecInt   float64
	SpecPow   float64
	Exposure  float64
	SRGBGamma float64
	InvGamma  float64
}

// DefaultLightConfig returns the standard lighting matching the Python renderer.
func DefaultLightConfig() LightConfig {
	lightDir := mathutil.Vec3{180, 260, 140}.Normalize()
	rimDir := mathutil.Vec3{-160, 130, -210}.Normalize()
	viewDir := mathutil.Vec3{0, -110, -400}.Normalize()

	halfMain := lightDir.Sub(viewDir).Normalize()

	return LightConfig{
		LightDir:  lightDir,
		RimDir:    rimDir,
		ViewDir:   viewDir,
		HalfMain:  halfMain,
		Ambient:   0.55,
		Hemi:      0.50,
		Direct:    1.50,
		Rim:       0.60,
		SpecInt:   0.45,
		SpecPow:   12.0,
		Exposure:  1.05,
		SRGBGamma: 2.2,
		InvGamma:  1.0 / 2.2,
	}
}

// ComputeShade returns the combined lighting scalar for a face normal.
func (lc *LightConfig) ComputeShade(normal mathutil.Vec3) float64 {
	// Lambertian (abs for double-sided)
	ndlMain := math.Abs(normal.Dot(lc.LightDir))
	ndlRim := math.Abs(normal.Dot(lc.RimDir))

	// Hemisphere fill
	hemi := (1.0-math.Abs(normal[1]))*0.5 + 0.5
	hemiLight := hemi * lc.Hemi

	// Blinn-Phong specular
	ndh := normal.Dot(lc.HalfMain)
	if ndh < 0 {
		ndh = 0
	}
	spec := math.Pow(ndh, lc.SpecPow) * lc.SpecInt

	return lc.Ambient + hemiLight + ndlMain*lc.Direct + ndlRim*lc.Rim + spec
}

// Precomputed sRGB-to-linear lookup table (256 entries).
var srgbToLinear [256]float64

func init() {
	for i := 0; i < 256; i++ {
		srgbToLinear[i] = math.Pow(float64(i)/255.0, 2.2)
	}
}

// ACESTonemap applies ACES Filmic tone mapping to a linear value.
func ACESTonemap(x float64) float64 {
	return (x * (2.51*x + 0.03)) / (x*(2.43*x+0.59) + 0.14)
}
