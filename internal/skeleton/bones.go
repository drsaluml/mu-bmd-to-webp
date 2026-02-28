package skeleton

import (
	"mu-bmd-renderer/internal/bmd"
	"mu-bmd-renderer/internal/mathutil"
	"math"
)

// BuildWorldMatrices computes the world transform for each bone using bind pose (frame 0, action 0).
// If boneFlip is true, root bone matrices are prefixed with Rx(-90°) to match
// BMD-viewer's Three.js group inheritance (group.rotation.x = -PI/2).
func BuildWorldMatrices(bones []bmd.Bone, boneFlip bool) []mathutil.Mat4 {
	worlds := make([]mathutil.Mat4, len(bones))
	for i := range worlds {
		worlds[i] = mathutil.Mat4Identity()
	}

	// Rx(-90°) as Mat4 for root bone prefix
	var rx90 mathutil.Mat4
	if boneFlip {
		rx90 = mathutil.FromMat3Translation(mathutil.RotX(-math.Pi/2), mathutil.Vec3{0, 0, 0})
	}

	for i, bone := range bones {
		if bone.IsDummy {
			continue
		}

		// Local transform: rotation from Euler + translation
		q := mathutil.EulerToQuat(bone.BindRotation[0], bone.BindRotation[1], bone.BindRotation[2])
		rot := mathutil.QuatToMat3(q)
		pos := mathutil.Vec3{bone.BindPosition[0], bone.BindPosition[1], bone.BindPosition[2]}
		local := mathutil.FromMat3Translation(rot, pos)

		// Chain with parent
		if bone.Parent >= 0 && bone.Parent < i {
			worlds[i] = mathutil.Mat4Mul(worlds[bone.Parent], local)
		} else {
			// Root bone
			if boneFlip {
				worlds[i] = mathutil.Mat4Mul(rx90, local)
			} else {
				worlds[i] = local
			}
		}
	}

	return worlds
}

// ApplyTransforms modifies mesh vertex positions in-place using bone world matrices.
// Rigid skinning: 1 bone per vertex, weight = 1.0.
func ApplyTransforms(meshes []bmd.Mesh, bones []bmd.Bone, boneFlip bool) {
	if len(bones) == 0 {
		return
	}

	worlds := BuildWorldMatrices(bones, boneFlip)

	// Check if all matrices are identity (skip if so)
	allIdentity := true
	for _, w := range worlds {
		if !w.IsIdentity() {
			allIdentity = false
			break
		}
	}
	if allIdentity {
		return
	}

	for mi := range meshes {
		mesh := &meshes[mi]
		for vi := range mesh.Verts {
			boneIdx := int(mesh.Nodes[vi])
			if boneIdx < 0 || boneIdx >= len(worlds) {
				continue
			}
			v := mathutil.Vec3{
				float64(mesh.Verts[vi][0]),
				float64(mesh.Verts[vi][1]),
				float64(mesh.Verts[vi][2]),
			}
			t := worlds[boneIdx].MulPoint(v)
			mesh.Verts[vi] = [3]float32{float32(t[0]), float32(t[1]), float32(t[2])}
		}
	}
}
