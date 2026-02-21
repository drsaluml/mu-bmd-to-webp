package raster

import (
	"image"
	"math"

	"mu-bmd-renderer/internal/bmd"
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
	// Decide bone transforms
	useBones := viewmatrix.ShouldUseBones(entry)
	if useBones {
		skeleton.ApplyTransforms(meshes, bones)
	}

	// Compute view matrix + filter meshes
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

	// Rasterize each mesh
	for _, mesh := range bodyMeshes {
		if len(mesh.Verts) == 0 {
			continue
		}

		px, py, pz := viewmatrix.ProjectVertices(mesh.Verts, R, center, scale, renderSize, entry)

		// Load texture
		var tex *image.NRGBA
		if texResolver != nil {
			tex = texResolver.Resolve(mesh.TexPath)
		}

		// Compute default color (average of texture)
		var defR, defG, defB, defA uint8 = 160, 160, 170, 255
		if tex != nil {
			defR, defG, defB, defA = averageColor(tex)
		}

		for _, tri := range mesh.Tris {
			vi := [3]int{int(tri.VI[0]), int(tri.VI[1]), int(tri.VI[2])}
			ti := [3]int{int(tri.TI[0]), int(tri.TI[1]), int(tri.TI[2])}
			RasterizeTriangle(fb, px, py, pz, mesh.UVs, vi, ti, tex, defR, defG, defB, defA, &lc)

			// Quad: second triangle
			if tri.Polygon == 4 {
				vi2 := [3]int{int(tri.VI[0]), int(tri.VI[2]), int(tri.VI[3])}
				ti2 := [3]int{int(tri.TI[0]), int(tri.TI[2]), int(tri.TI[3])}
				RasterizeTriangle(fb, px, py, pz, mesh.UVs, vi2, ti2, tex, defR, defG, defB, defA, &lc)
			}
		}
	}

	// Convert framebuffer to image
	img := image.NewNRGBA(image.Rect(0, 0, renderSize, renderSize))
	copy(img.Pix, fb.Color)

	return img
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
