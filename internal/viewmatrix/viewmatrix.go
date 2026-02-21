package viewmatrix

import (
	"math"

	"mu-bmd-renderer/internal/bmd"
	"mu-bmd-renderer/internal/filter"
	"mu-bmd-renderer/internal/mathutil"
	"mu-bmd-renderer/internal/trs"
)

// TRSViewMatrix builds a 3×3 view matrix from a TRS entry.
// Uses 3-tier hybrid routing based on rotY.
func TRSViewMatrix(e *trs.Entry) mathutil.Mat3 {
	rx := mathutil.Deg2Rad(e.RotX)
	ry := mathutil.Deg2Rad(e.RotY)
	rz := mathutil.Deg2Rad(e.RotZ)
	trsRot := mathutil.Mat3Mul(mathutil.Mat3Mul(mathutil.RotZ(rz), mathutil.RotY(ry)), mathutil.RotX(rx))

	// Custom camera override
	switch e.Camera {
	case "noflip":
		return mathutil.Mat3Mul(mathutil.NoflipCam, trsRot)
	case "correction":
		return mathutil.Mat3Mul(mathutil.TRSCorrection, trsRot)
	case "fallback":
		return mathutil.ViewFallback
	}

	// Auto-routing by rotY
	rotY := e.RotY
	if mathutil.AngleDist(rotY, 270) <= 45 {
		return mathutil.Mat3Mul(mathutil.TRSCorrection, trsRot)
	} else if mathutil.AngleDist(rotY, 90) <= 45 {
		return mathutil.ViewFallback
	}
	return mathutil.Mat3Mul(mathutil.NoflipCam, trsRot)
}

// IsFallbackPath returns true if this TRS entry routes to VIEW_FALLBACK.
func IsFallbackPath(e *trs.Entry) bool {
	if e.Camera == "fallback" {
		return true
	}
	if e.Camera != "" {
		return false
	}
	return mathutil.AngleDist(e.RotY, 90) <= 45 && mathutil.AngleDist(e.RotY, 270) > 45
}

// ComputeViewMatrix applies component filtering and returns the view matrix + filtered body meshes.
// Effect mesh filtering is done earlier in the pipeline (before bone transforms).
func ComputeViewMatrix(meshes []bmd.Mesh, entry *trs.Entry) (mathutil.Mat3, []bmd.Mesh) {
	var bodyMeshes []bmd.Mesh
	for i := range meshes {
		filtered := filter.FilterComponents(&meshes[i], 6)
		bodyMeshes = append(bodyMeshes, filtered)
	}

	if len(bodyMeshes) == 0 {
		return mathutil.Mat3Identity(), nil
	}

	if entry != nil {
		return TRSViewMatrix(entry), bodyMeshes
	}
	return mathutil.ViewFallback, bodyMeshes
}

// ShouldUseBones determines whether bone transforms should be applied.
func ShouldUseBones(entry *trs.Entry) bool {
	if entry == nil {
		return true
	}
	// Explicit override
	if entry.UseBones != nil {
		return *entry.UseBones
	}
	// Binary TRS items skip bones (calibrated for raw mesh)
	if entry.Source == "binary" {
		// Exception: rotY≈90° binary TRS items use VIEW_FALLBACK
		// which matches BMD-viewer (always uses bones)
		rotY := entry.RotY
		if mathutil.AngleDist(rotY, 90) <= 45 && mathutil.AngleDist(rotY, 270) > 45 {
			return true
		}
		return false
	}
	return true
}

// ProjectVertices transforms 3D vertices to 2D screen coordinates.
// Returns px, py, pz slices (screen X, screen Y, depth).
func ProjectVertices(verts [][3]float32, R mathutil.Mat3, center [3]float64, scale float64, renderSize int, entry *trs.Entry) ([]float64, []float64, []float64) {
	n := len(verts)
	px := make([]float64, n)
	py := make([]float64, n)
	pz := make([]float64, n)

	half := float64(renderSize) / 2

	// Perspective setup
	usePersp := entry != nil && entry.Perspective
	var perspCamDist, perspZCenter float64
	if usePersp {
		fov := entry.FOV
		if fov == 0 {
			fov = trs.DefaultFOV
		}
		halfFOV := mathutil.Deg2Rad(fov / 2)

		// Compute z range and xy half-extent from ALL transformed verts
		var zMin, zMax, xyMax float64
		zMin = math.Inf(1)
		zMax = math.Inf(-1)
		for i := range verts {
			v := mathutil.Vec3{float64(verts[i][0]), float64(verts[i][1]), float64(verts[i][2])}
			t := R.MulVec3(v)
			if t[2] < zMin {
				zMin = t[2]
			}
			if t[2] > zMax {
				zMax = t[2]
			}
			for k := 0; k < 2; k++ {
				d := math.Abs(t[k] - center[k])
				if d > xyMax {
					xyMax = d
				}
			}
		}
		perspZCenter = (zMin + zMax) / 2
		if xyMax < 0.001 {
			xyMax = 0.001
		}
		perspCamDist = xyMax / math.Tan(halfFOV)
	}

	for i := range verts {
		v := mathutil.Vec3{float64(verts[i][0]), float64(verts[i][1]), float64(verts[i][2])}
		t := R.MulVec3(v)

		if usePersp {
			zOff := t[2] - perspZCenter
			depth := math.Max(perspCamDist-zOff, 0.1)
			factor := perspCamDist / depth
			t[0] *= factor
			t[1] *= factor
		}

		px[i] = (t[0]-center[0])*scale + half
		py[i] = -(t[1]-center[1])*scale + half
		pz[i] = t[2]
	}

	return px, py, pz
}
