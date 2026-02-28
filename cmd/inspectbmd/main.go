package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"mu-bmd-renderer/internal/bmd"
	"mu-bmd-renderer/internal/filter"
	"mu-bmd-renderer/internal/mathutil"
	"mu-bmd-renderer/internal/skeleton"
	"mu-bmd-renderer/internal/texture"
)

func main() {
	idx := texture.BuildIndex("Data/Item")
	cache := texture.NewCache(idx)

	for _, arg := range os.Args[1:] {
		meshes, bones, err := bmd.Parse(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Parse error %s: %v\n", arg, err)
			continue
		}
		fmt.Printf("\n=== %s (meshes=%d bones=%d) ===\n", arg, len(meshes), len(bones))

		// Show raw geometry
		fmt.Println("--- RAW (before bones) ---")
		printMeshes(meshes, cache)

		// Apply bone transforms
		skeleton.ApplyTransforms(meshes, bones, false)
		fmt.Println("--- AFTER BONES ---")
		printMeshes(meshes, cache)

		// Show transformed coordinates (after ModelFlip = MirrorX @ RotX(-90))
		R := mathutil.Mat3Mul(mathutil.MirrorX, mathutil.ModelFlip)
		fmt.Println("--- AFTER ModelFlip (screen space: X=right, Y=up, Z=depth) ---")
		for i, m := range meshes {
			if len(m.Verts) == 0 {
				continue
			}
			stem := strings.TrimSuffix(filepath.Base(strings.ReplaceAll(m.TexPath, "\\", "/")), filepath.Ext(m.TexPath))
			var tMin, tMax [3]float64
			tMin = [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)}
			tMax = [3]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
			for _, v := range m.Verts {
				tv := R.MulVec3(mathutil.Vec3{float64(v[0]), float64(v[1]), float64(v[2])})
				for k := 0; k < 3; k++ {
					if tv[k] < tMin[k] {
						tMin[k] = tv[k]
					}
					if tv[k] > tMax[k] {
						tMax[k] = tv[k]
					}
				}
			}
			fmt.Printf("  Mesh[%d] %s: screenX=[%.0f..%.0f] screenY=[%.0f..%.0f] depth=[%.0f..%.0f]\n",
				i, stem,
				tMin[0], tMax[0], tMin[1], tMax[1], tMin[2], tMax[2])
		}
	}
}

func printMeshes(meshes []bmd.Mesh, cache *texture.Cache) {
	for i, m := range meshes {
		stem := strings.TrimSuffix(filepath.Base(strings.ReplaceAll(m.TexPath, "\\", "/")), filepath.Ext(m.TexPath))
		ext := strings.ToLower(filepath.Ext(strings.ReplaceAll(m.TexPath, "\\", "/")))
		tex := cache.Resolve(m.TexPath)
		texInfo := "MISSING"
		bright := 0.0
		if tex != nil {
			b := tex.Bounds()
			texInfo = fmt.Sprintf("%dx%d", b.Dx(), b.Dy())
			total := 0.0
			count := len(tex.Pix) / 4
			for j := 0; j < len(tex.Pix); j += 4 {
				total += float64(int(tex.Pix[j])+int(tex.Pix[j+1])+int(tex.Pix[j+2])) / 3.0
			}
			if count > 0 {
				bright = total / float64(count)
			}
		}
		flags := ""
		if filter.IsEffectMesh(&m) {
			flags += " [EFFECT]"
		}
		if filter.IsBodyMesh(&m) {
			flags += " [BODY]"
		}

		if len(m.Verts) > 0 {
			minV, maxV := m.Verts[0], m.Verts[0]
			for _, v := range m.Verts[1:] {
				for k := 0; k < 3; k++ {
					if v[k] < minV[k] {
						minV[k] = v[k]
					}
					if v[k] > maxV[k] {
						maxV[k] = v[k]
					}
				}
			}
			sx := maxV[0] - minV[0]
			sy := maxV[1] - minV[1]
			sz := maxV[2] - minV[2]
			fmt.Printf("  Mesh[%d]: v=%d t=%d tex=%q%s (%s) bright=%.0f bbox=(%.0f,%.0f,%.0f) min=(%.0f,%.0f,%.0f) max=(%.0f,%.0f,%.0f)%s\n",
				i, len(m.Verts), len(m.Tris), stem, ext, texInfo, bright,
				math.Abs(float64(sx)), math.Abs(float64(sy)), math.Abs(float64(sz)),
				float64(minV[0]), float64(minV[1]), float64(minV[2]),
				float64(maxV[0]), float64(maxV[1]), float64(maxV[2]),
				flags)
		}
	}
}
