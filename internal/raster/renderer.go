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

	// Split meshes into opaque, alpha-blend, additive, and force-additive (unlit)
	var opaqueMeshes, alphaBlendMeshes, additiveMeshes, forceAdditiveMeshes []bmd.Mesh
	for i, mesh := range bodyMeshes {
		// Check per-item additive_textures override first
		if isForceAdditive(mesh.TexPath, entry) {
			forceAdditiveMeshes = append(forceAdditiveMeshes, mesh)
			continue
		}
		// Only classify as billboard when there are other body meshes —
		// a single-mesh model can't be an "overlay" on nothing.
		// Also skip billboard classification if this mesh has a _R additive
		// counterpart with the same geometry — it's the base layer, not glow.
		billboard := len(bodyMeshes) > 1 && isBillboardJPEG(&mesh) && !hasAdditiveCounterpart(bodyMeshes, i)
		if isAdditiveTexture(mesh.TexPath) || billboard || isDuplicateGeometryOverlay(bodyMeshes, i) {
			additiveMeshes = append(additiveMeshes, mesh)
		} else if isTGAPairedGlowJPEG(bodyMeshes, i, texResolver) {
			additiveMeshes = append(additiveMeshes, mesh)
		} else if isAlphaOverlay(bodyMeshes, i, texResolver) {
			alphaBlendMeshes = append(alphaBlendMeshes, mesh)
		} else {
			opaqueMeshes = append(opaqueMeshes, mesh)
		}
	}

	// Safety: if no opaque mesh exists, promote from additive/alpha to avoid
	// rendering everything with luminance-based alpha onto an empty canvas.
	if len(opaqueMeshes) == 0 && (len(additiveMeshes) > 0 || len(alphaBlendMeshes) > 0) {
		if len(additiveMeshes) > 0 {
			opaqueMeshes = append(opaqueMeshes, additiveMeshes[0])
			additiveMeshes = additiveMeshes[1:]
		} else {
			opaqueMeshes = append(opaqueMeshes, alphaBlendMeshes[0])
			alphaBlendMeshes = alphaBlendMeshes[1:]
		}
	}

	// Pass 1: Opaque meshes (normal z-buffer rendering)
	for _, mesh := range opaqueMeshes {
		rasterizeMesh(fb, &mesh, R, center, scale, renderSize, entry, texResolver, &lc, blendOpaque)
	}

	// Pass 2: Alpha-blend meshes (z-read but no z-write, alpha composite)
	for _, mesh := range alphaBlendMeshes {
		rasterizeMesh(fb, &mesh, R, center, scale, renderSize, entry, texResolver, &lc, blendAlpha)
	}

	// Pass 3: Additive meshes (no z-buffer, add colors on top)
	for _, mesh := range additiveMeshes {
		rasterizeMesh(fb, &mesh, R, center, scale, renderSize, entry, texResolver, &lc, blendAdditive)
	}

	// Pass 4: Force-additive meshes — rendered to a separate background
	// framebuffer then composited UNDER the main content ("dst-over").
	// The body mesh stays at full brightness; the overlay only shows
	// through transparent areas where the body doesn't cover.
	if len(forceAdditiveMeshes) > 0 {
		bgFB := NewFrameBuffer(renderSize, renderSize)
		for _, mesh := range forceAdditiveMeshes {
			rasterizeMesh(bgFB, &mesh, R, center, scale, renderSize, entry, texResolver, &lc, blendOpaque)
		}
		// Remove dark pixels — only keep blue/bright areas as background.
		// Dark texels (brightness < threshold) become transparent to avoid black aura.
		for i := 0; i < len(bgFB.Color); i += 4 {
			if bgFB.Color[i+3] == 0 {
				continue
			}
			r, g, b := int(bgFB.Color[i]), int(bgFB.Color[i+1]), int(bgFB.Color[i+2])
			if (r+g+b)/3 < 60 {
				bgFB.Color[i+3] = 0
			}
		}
		compositeUnder(fb, bgFB)
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

// isBillboardJPEG returns true if this mesh is a small JPEG-textured overlay (≤16 verts, ≤12 tris)
// that the game renders with additive blending — black pixels add nothing, bright pixels glow.
// Covers single quads (4v/2t), double quads (8v/4t), cross-shaped billboards (12v/6t),
// and small diamond/octahedron shapes (16v/12t).
// Threshold kept conservative to avoid misclassifying real small mesh parts (e.g. shield bosses).
func isBillboardJPEG(m *bmd.Mesh) bool {
	if len(m.Verts) > 16 || len(m.Tris) > 12 || len(m.Verts) == 0 {
		return false
	}
	ext := strings.ToLower(filepath.Ext(strings.ReplaceAll(m.TexPath, "\\", "/")))
	return ext == ".jpg" || ext == ".jpeg"
}

// Blend mode constants for rasterizeMesh.
const (
	blendOpaque   = 0
	blendAlpha    = 1
	blendAdditive = 2
)

// isAlphaOverlay returns true if this TGA-textured mesh should use alpha blending
// because the model also has a JPEG mesh with similar geometry, indicating this TGA
// is a decorative overlay layer (e.g. Crossbow17.bmd: TGA 154v/166t + JPEG 134v/146t).
// Only triggers when a JPEG mesh has similar complexity (both verts and tris within 2×),
// which distinguishes overlay pairs from models where TGA is the main body
// (e.g. CW_Bow.bmd: TGA 182v/310t vs JPEG 103v/150t — tri ratio 2.07× exceeds 2×).
// Skips JPEG meshes with tiny textures (≤32×32) as those are glow fills, not bodies.
func isAlphaOverlay(meshes []bmd.Mesh, idx int, texResolver texture.Resolver) bool {
	ext := strings.ToLower(filepath.Ext(strings.ReplaceAll(meshes[idx].TexPath, "\\", "/")))
	if ext != ".tga" {
		return false
	}
	tgaV := len(meshes[idx].Verts)
	tgaT := len(meshes[idx].Tris)
	for i := range meshes {
		if i == idx {
			continue
		}
		e := strings.ToLower(filepath.Ext(strings.ReplaceAll(meshes[i].TexPath, "\\", "/")))
		if e != ".jpg" && e != ".jpeg" {
			continue
		}
		jpgV := len(meshes[i].Verts)
		jpgT := len(meshes[i].Tris)
		if jpgV == 0 || jpgT == 0 {
			continue
		}
		// Skip JPEG meshes with tiny textures — those are glow fills, not body meshes
		if texResolver != nil {
			tex := texResolver.Resolve(meshes[i].TexPath)
			if tex != nil {
				b := tex.Bounds()
				if b.Dx() <= 32 && b.Dy() <= 32 {
					continue
				}
			}
		}
		// Similar geometry: both vert and tri counts within 2× of each other (both directions)
		if tgaV <= jpgV*2 && jpgV <= tgaV*2 && tgaT <= jpgT*2 && jpgT <= tgaT*2 {
			return true
		}
	}
	return false
}

// isDuplicateGeometryOverlay returns true if meshes[idx] has the same vertex count,
// triangle count, and bounding box as an earlier mesh — indicating it's a glow/effect
// overlay layer (MU Online pattern: same geometry + sequential textures like xx00/xx01).
func isDuplicateGeometryOverlay(meshes []bmd.Mesh, idx int) bool {
	if idx == 0 {
		return false
	}
	m := &meshes[idx]
	nv, nt := len(m.Verts), len(m.Tris)
	for j := 0; j < idx; j++ {
		prev := &meshes[j]
		if len(prev.Verts) == nv && len(prev.Tris) == nt {
			return true
		}
	}
	return false
}

// hasAdditiveCounterpart returns true if another mesh in the model has the _R suffix
// texture with the same vertex/triangle count — indicating meshes[idx] is the base
// layer (not a billboard) and the _R mesh is its additive glow overlay.
func hasAdditiveCounterpart(meshes []bmd.Mesh, idx int) bool {
	nv, nt := len(meshes[idx].Verts), len(meshes[idx].Tris)
	for j := range meshes {
		if j == idx {
			continue
		}
		if isAdditiveTexture(meshes[j].TexPath) &&
			len(meshes[j].Verts) == nv && len(meshes[j].Tris) == nt {
			return true
		}
	}
	return false
}

func rasterizeMesh(
	fb *FrameBuffer, mesh *bmd.Mesh,
	R mathutil.Mat3, center [3]float64, scale float64, renderSize int,
	entry *trs.Entry, texResolver texture.Resolver, lc *LightConfig,
	blendMode int,
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

	type rasterFunc func(*FrameBuffer, []float64, []float64, []float64, [][2]float32, [3]int, [3]int, *image.NRGBA, uint8, uint8, uint8, uint8, *LightConfig)
	var rasterFn rasterFunc
	switch blendMode {
	case blendAdditive:
		rasterFn = RasterizeTriangleAdditive
	case blendAlpha:
		rasterFn = RasterizeTriangleAlphaBlend
	default:
		rasterFn = RasterizeTriangle
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

// isTGAPairedGlowJPEG returns true if a JPEG mesh in a model with TGA meshes
// has a tiny texture (≤32px in both dimensions) — indicating it's a glow/gradient
// fill overlay, not a real body texture (e.g. staff20.bmd: 16×16 cyan "a_2.jpg").
// The game renders these with additive blending where black = transparent.
func isTGAPairedGlowJPEG(meshes []bmd.Mesh, idx int, texResolver texture.Resolver) bool {
	if texResolver == nil {
		return false
	}
	ext := strings.ToLower(filepath.Ext(strings.ReplaceAll(meshes[idx].TexPath, "\\", "/")))
	if ext != ".jpg" && ext != ".jpeg" {
		return false
	}
	// Only applies when model also has a TGA mesh
	hasTGA := false
	for i := range meshes {
		if i == idx {
			continue
		}
		e := strings.ToLower(filepath.Ext(strings.ReplaceAll(meshes[i].TexPath, "\\", "/")))
		if e == ".tga" {
			hasTGA = true
			break
		}
	}
	if !hasTGA {
		return false
	}
	tex := texResolver.Resolve(meshes[idx].TexPath)
	if tex == nil {
		return false
	}
	b := tex.Bounds()
	w, h := b.Dx(), b.Dy()
	// Tiny textures (≤32×32) are gradient/glow fills, not body textures
	return w <= 32 && h <= 32
}

// compositeUnder composites bg UNDER dst ("dst-over" in Porter-Duff).
// The background shows through transparent areas of the main buffer.
// Where main is fully opaque, background is hidden.
func compositeUnder(dst, bg *FrameBuffer) {
	for i := 0; i < len(dst.Color); i += 4 {
		bgA := bg.Color[i+3]
		if bgA == 0 {
			continue
		}
		dstA := float64(dst.Color[i+3]) / 255.0
		bgAlpha := float64(bgA) / 255.0 * (1.0 - dstA)
		if bgAlpha < 1.0/255.0 {
			continue
		}
		dst.Color[i] = clamp255(float64(dst.Color[i]) + float64(bg.Color[i])*bgAlpha)
		dst.Color[i+1] = clamp255(float64(dst.Color[i+1]) + float64(bg.Color[i+1])*bgAlpha)
		dst.Color[i+2] = clamp255(float64(dst.Color[i+2]) + float64(bg.Color[i+2])*bgAlpha)
		newA := dstA + bgAlpha*(1.0-dstA)
		if newA > 1.0 {
			newA = 1.0
		}
		dst.Color[i+3] = uint8(newA*255.0 + 0.5)
	}
}

// isForceAdditive returns true if the mesh's texture stem matches one of the
// per-item additive_textures overrides (case-insensitive stem match).
func isForceAdditive(texPath string, entry *trs.Entry) bool {
	if entry == nil || len(entry.AdditiveTextures) == 0 {
		return false
	}
	base := filepath.Base(strings.ReplaceAll(texPath, "\\", "/"))
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	stemLower := strings.ToLower(stem)
	for _, s := range entry.AdditiveTextures {
		if strings.ToLower(s) == stemLower {
			return true
		}
	}
	return false
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
