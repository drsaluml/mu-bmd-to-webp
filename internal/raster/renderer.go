package raster

import (
	"image"
	"math"
	"path/filepath"
	"strings"

	"mu-bmd-renderer/internal/bmd"
	"mu-bmd-renderer/internal/filter"
	"mu-bmd-renderer/internal/mathutil"
	"mu-bmd-renderer/internal/skeleton"
	"mu-bmd-renderer/internal/texture"
	"mu-bmd-renderer/internal/trs"
	"mu-bmd-renderer/internal/viewmatrix"
)

// RenderBMD renders parsed BMD meshes to an NRGBA image.
func RenderBMD(
	meshes []bmd.Mesh,
	bones []bmd.Bone,
	entry *trs.Entry,
	texResolver texture.Resolver,
	size int,
	supersample int,
) *image.NRGBA {
	// Pre-filter effect meshes on raw geometry (before bone transforms distort shapes)
	keepAll := entry != nil && entry.KeepAllMeshes
	if !keepAll {
		var nonEffect []bmd.Mesh
		for i := range meshes {
			if !filter.IsEffectMesh(&meshes[i]) {
				nonEffect = append(nonEffect, meshes[i])
			}
		}
		if len(nonEffect) > 0 {
			meshes = nonEffect
		}
	}

	// Bone transforms
	useBones := viewmatrix.ShouldUseBones(entry)
	if useBones {
		skeleton.ApplyTransforms(meshes, bones)
	}

	// Compute view matrix + filter components
	R, bodyMeshes := viewmatrix.ComputeViewMatrix(meshes, entry)
	if len(bodyMeshes) == 0 {
		return image.NewNRGBA(image.Rect(0, 0, size, size))
	}

	renderSize := size * supersample

	// Compute bounding box of all transformed vertices
	var allMin, allMax [3]float64
	allMin = [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)}
	allMax = [3]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for _, m := range bodyMeshes {
		for _, v := range m.Verts {
			tv := R.MulVec3(mathutil.Vec3{float64(v[0]), float64(v[1]), float64(v[2])})
			for k := 0; k < 3; k++ {
				if tv[k] < allMin[k] {
					allMin[k] = tv[k]
				}
				if tv[k] > allMax[k] {
					allMax[k] = tv[k]
				}
			}
		}
	}

	center := [3]float64{
		(allMin[0] + allMax[0]) / 2,
		(allMin[1] + allMax[1]) / 2,
		(allMin[2] + allMax[2]) / 2,
	}
	spanX := allMax[0] - allMin[0]
	spanY := allMax[1] - allMin[1]
	span := spanX
	if spanY > span {
		span = spanY
	}
	if span < 0.001 {
		span = 0.001
	}

	margin := 16 * supersample
	scale := float64(renderSize-2*margin) / span

	// Allocate framebuffer
	fb := NewFrameBuffer(renderSize, renderSize)
	lc := DefaultLightConfig()

	// Split meshes into opaque and additive
	var opaqueMeshes, additiveMeshes []bmd.Mesh
	for _, mesh := range bodyMeshes {
		if isAdditiveTexture(mesh.TexPath) || isBillboardJPEG(&mesh) {
			additiveMeshes = append(additiveMeshes, mesh)
		} else {
			opaqueMeshes = append(opaqueMeshes, mesh)
		}
	}

	// Pass 1: Opaque meshes (normal z-buffer rendering)
	for _, mesh := range opaqueMeshes {
		rasterizeMesh(fb, &mesh, R, center, scale, renderSize, entry, texResolver, &lc, false)
	}

	// Pass 2: Additive meshes (no z-buffer, add colors on top)
	for _, mesh := range additiveMeshes {
		rasterizeMesh(fb, &mesh, R, center, scale, renderSize, entry, texResolver, &lc, true)
	}

	// Convert framebuffer to image
	img := image.NewNRGBA(image.Rect(0, 0, renderSize, renderSize))
	copy(img.Pix, fb.Color)

	return img
}

// isAdditiveTexture returns true if a texture name ends with _R (MU Online convention
// for additive glow/liquid overlays, e.g. "secret_R.jpg", "songko2_R.jpg").
func isAdditiveTexture(texPath string) bool {
	base := filepath.Base(strings.ReplaceAll(texPath, "\\", "/"))
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	return strings.HasSuffix(strings.ToLower(stem), "_r")
}

// isBillboardJPEG returns true if this mesh is a flat billboard quad (≤8 verts, ≤4 tris)
// using a JPEG texture (no alpha). These are glow/wing overlays that the game renders
// with additive blending — black pixels add nothing, bright pixels glow.
func isBillboardJPEG(m *bmd.Mesh) bool {
	if len(m.Verts) > 8 || len(m.Tris) > 4 || len(m.Verts) == 0 {
		return false
	}
	ext := strings.ToLower(filepath.Ext(strings.ReplaceAll(m.TexPath, "\\", "/")))
	return ext == ".jpg" || ext == ".jpeg"
}

func rasterizeMesh(
	fb *FrameBuffer, mesh *bmd.Mesh,
	R mathutil.Mat3, center [3]float64, scale float64, renderSize int,
	entry *trs.Entry, texResolver texture.Resolver, lc *LightConfig,
	additive bool,
) {
	if len(mesh.Verts) == 0 {
		return
	}

	px, py, pz := viewmatrix.ProjectVertices(mesh.Verts, R, center, scale, renderSize, entry)

	var tex *image.NRGBA
	if texResolver != nil {
		tex = texResolver.Resolve(mesh.TexPath)
	}

	var defR, defG, defB, defA uint8 = 160, 160, 170, 255
	if tex != nil {
		defR, defG, defB, defA = averageColor(tex)
	}

	rasterFn := RasterizeTriangle
	if additive {
		rasterFn = RasterizeTriangleAdditive
	}

	for _, tri := range mesh.Tris {
		vi := [3]int{int(tri.VI[0]), int(tri.VI[1]), int(tri.VI[2])}
		ti := [3]int{int(tri.TI[0]), int(tri.TI[1]), int(tri.TI[2])}
		rasterFn(fb, px, py, pz, mesh.UVs, vi, ti, tex, defR, defG, defB, defA, lc)

		if tri.Polygon == 4 {
			vi2 := [3]int{int(tri.VI[0]), int(tri.VI[2]), int(tri.VI[3])}
			ti2 := [3]int{int(tri.TI[0]), int(tri.TI[2]), int(tri.TI[3])}
			rasterFn(fb, px, py, pz, mesh.UVs, vi2, ti2, tex, defR, defG, defB, defA, lc)
		}
	}
}

func averageColor(tex *image.NRGBA) (uint8, uint8, uint8, uint8) {
	b := tex.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return 160, 160, 170, 255
	}

	var sumR, sumG, sumB float64
	total := w * h
	stride := tex.Stride
	for y := 0; y < h; y++ {
		off := y * stride
		for x := 0; x < w; x++ {
			i := off + x*4
			sumR += float64(tex.Pix[i])
			sumG += float64(tex.Pix[i+1])
			sumB += float64(tex.Pix[i+2])
		}
	}
	n := float64(total)
	return uint8(sumR/n + 0.5), uint8(sumG/n + 0.5), uint8(sumB/n + 0.5), 255
}
