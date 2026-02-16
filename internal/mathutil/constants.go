package mathutil

import "math"

// Precomputed camera and correction matrices matching the Python renderer.
var (
	// ModelFlip converts Z-up (DirectX) to Y-up (OpenGL): Rx(-90°)
	ModelFlip = RotX(math.Pi / -2)

	// MirrorX converts left-handed to right-handed: diag(-1, 1, 1)
	MirrorX = Mat3Diag(-1, 1, 1)

	// ViewFallback is the BMD-viewer reference camera matrix.
	// MIRROR_X @ Rx(-15°) @ Ry(12°) @ MODEL_FLIP
	ViewFallback = Mat3Mul(Mat3Mul(Mat3Mul(MirrorX, RotX(Deg2Rad(-15))), RotY(Deg2Rad(12))), ModelFlip)

	// TRSDefault is the default weapon TRS rotation: Rz(15°) @ Ry(270°) @ Rx(180°)
	TRSDefault = Mat3Mul(Mat3Mul(RotZ(Deg2Rad(15)), RotY(Deg2Rad(270))), RotX(Deg2Rad(180)))

	// TRSCorrection maps game-client TRS rotations to BMD-viewer output.
	// CORRECTION = VIEW_FALLBACK @ inv(TRS_DEFAULT)
	TRSCorrection = Mat3Mul(ViewFallback, TRSDefault.Inverse())

	// NoflipCam is used for items where correction matrix produces edge-on views.
	// MIRROR_X @ Rx(-15°)
	NoflipCam = Mat3Mul(MirrorX, RotX(Deg2Rad(-15)))
)

// AngleDist returns the shortest angular distance between two angles in degrees (0–180).
func AngleDist(a, b float64) float64 {
	d := math.Mod(a-b, 360)
	if d < 0 {
		d += 360
	}
	if d > 180 {
		return 360 - d
	}
	return d
}
