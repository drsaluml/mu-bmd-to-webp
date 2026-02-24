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

	// Filter glow layer pairs (before bone transforms change geometry).
	// Detects JPEG+TGA pairs with same (verts, tris) count, and standalone
	// bright JPEG glow layers. The game composites these with special blending;
	// without it, their colored backgrounds create visible auras.
	if !keepAll && texResolver != nil && len(meshes) > 1 {
		meshes = filterGlowLayers(meshes, texResolver)
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

// isBillboardJPEG returns true if this mesh is a flat billboard (≤16 verts, ≤12 tris)
// using a JPEG texture (no alpha). These are glow/wing/energy overlays that the game
// renders with additive blending — black pixels add nothing, bright pixels glow.
// Covers single quads (4v/2t), double quads (8v/4t), cross-shaped billboards (12v/6t),
// and small diamond/octahedron shapes (16v/12t).
func isBillboardJPEG(m *bmd.Mesh) bool {
	if len(m.Verts) > 16 || len(m.Tris) > 12 || len(m.Verts) == 0 {
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

// filterGlowLayers removes glow layer meshes from the body mesh list.
// Detects two patterns:
// 1. Geometry pairs: meshes with same (verts, tris) count where one uses JPEG
//    and another uses TGA — the game composites these with special blending.
// 2. Standalone bright JPEGs: very bright, low-saturation JPEG textures that
//    are shimmer/glow overlays.
func filterGlowLayers(meshes []bmd.Mesh, texResolver texture.Resolver) []bmd.Mesh {
	type meshKey struct {
		verts, tris int
	}

	// Group by geometry (vertex count, triangle count)
	groups := make(map[meshKey][]int)
	for i := range meshes {
		k := meshKey{len(meshes[i].Verts), len(meshes[i].Tris)}
		groups[k] = append(groups[k], i)
	}

	remove := make(map[int]bool)

	// Pattern 1: JPEG+TGA pairs with same geometry
	for _, indices := range groups {
		if len(indices) < 2 {
			continue
		}
		hasJPG := false
		hasTGA := false
		for _, i := range indices {
			ext := strings.ToLower(filepath.Ext(strings.ReplaceAll(meshes[i].TexPath, "\\", "/")))
			if ext == ".jpg" || ext == ".jpeg" {
				hasJPG = true
			} else if ext == ".tga" {
				hasTGA = true
			}
		}
		if hasJPG && hasTGA {
			for _, i := range indices {
				remove[i] = true
			}
		}
	}

	// Pattern 2: Standalone bright JPEG glow layers.
	// Skip the mesh with the most triangles — that's the primary body mesh
	// and should never be classified as a glow overlay, even if bright.
	maxTris := 0
	maxTrisIdx := -1
	for i := range meshes {
		if remove[i] {
			continue
		}
		if len(meshes[i].Tris) > maxTris {
			maxTris = len(meshes[i].Tris)
			maxTrisIdx = i
		}
	}
	for i := range meshes {
		if remove[i] || i == maxTrisIdx {
			continue
		}
		if isBrightGlowJPEG(&meshes[i], texResolver) {
			remove[i] = true
		}
	}

	if len(remove) == 0 {
		return meshes
	}

	var result []bmd.Mesh
	for i := range meshes {
		if !remove[i] {
			result = append(result, meshes[i])
		}
	}
	if len(result) == 0 {
		return meshes // don't filter everything
	}
	return result
}

// isBrightGlowJPEG returns true if a mesh uses a very bright, desaturated
// JPEG texture — typically a white shimmer/glow overlay that the game renders
// with additive blending. Without special handling these appear as opaque gray.
func isBrightGlowJPEG(m *bmd.Mesh, texResolver texture.Resolver) bool {
	ext := strings.ToLower(filepath.Ext(strings.ReplaceAll(m.TexPath, "\\", "/")))
	if ext != ".jpg" && ext != ".jpeg" {
		return false
	}
	tex := texResolver.Resolve(m.TexPath)
	if tex == nil {
		return false
	}
	r, g, b, _ := averageColor(tex)
	fr, fg, fb := float64(r), float64(g), float64(b)
	brightness := (fr + fg + fb) / 3
	maxC := fr
	if fg > maxC {
		maxC = fg
	}
	if fb > maxC {
		maxC = fb
	}
	minC := fr
	if fg < minC {
		minC = fg
	}
	if fb < minC {
		minC = fb
	}
	saturation := 0.0
	if maxC > 0 {
		saturation = (maxC - minC) / maxC
	}
	return brightness > 180 && saturation < 0.25
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
