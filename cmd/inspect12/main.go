package main

import (
	"fmt"
	"math"
	"os"

	"mu-bmd-renderer/internal/bmd"
	"mu-bmd-renderer/internal/filter"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <bmd-file> [<bmd-file> ...]\n", os.Args[0])
		os.Exit(1)
	}

	for i, path := range os.Args[1:] {
		if i > 0 {
			fmt.Println()
			fmt.Println("================================================================")
			fmt.Println()
		}
		inspectBMD(path)
	}
}

func inspectBMD(path string) {
	fmt.Printf("=== %s ===\n\n", path)

	meshes, bones, err := bmd.Parse(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR parsing %s: %v\n", path, err)
		return
	}

	fmt.Printf("Total meshes: %d\n", len(meshes))
	fmt.Printf("Total bones:  %d\n", len(bones))
	fmt.Println()

	// Print bone summary
	if len(bones) > 0 {
		fmt.Println("--- Bones ---")
		for bi, b := range bones {
			if b.IsDummy {
				fmt.Printf("  Bone %2d: DUMMY\n", bi)
			} else {
				fmt.Printf("  Bone %2d: parent=%d  pos=(%.2f, %.2f, %.2f)  rot=(%.4f, %.4f, %.4f)\n",
					bi, b.Parent,
					b.BindPosition[0], b.BindPosition[1], b.BindPosition[2],
					b.BindRotation[0], b.BindRotation[1], b.BindRotation[2])
			}
		}
		fmt.Println()
	}

	for mi := range meshes {
		m := &meshes[mi]
		fmt.Printf("--- Mesh %d ---\n", mi)
		fmt.Printf("  TexPath:    %s\n", m.TexPath)
		fmt.Printf("  Vertices:   %d\n", len(m.Verts))
		fmt.Printf("  Normals:    %d\n", len(m.Normals))
		fmt.Printf("  UVs:        %d\n", len(m.UVs))
		fmt.Printf("  Triangles:  %d\n", len(m.Tris))

		// Count actual triangles (quads count as 2)
		actualTris := 0
		for _, tri := range m.Tris {
			if tri.Polygon == 4 {
				actualTris += 2
			} else {
				actualTris++
			}
		}
		fmt.Printf("  Actual tris (with quads expanded): %d\n", actualTris)

		// BBox
		if len(m.Verts) > 0 {
			minV := m.Verts[0]
			maxV := m.Verts[0]
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
			sizeX := maxV[0] - minV[0]
			sizeY := maxV[1] - minV[1]
			sizeZ := maxV[2] - minV[2]
			fmt.Printf("  BBox min:   (%.2f, %.2f, %.2f)\n", minV[0], minV[1], minV[2])
			fmt.Printf("  BBox max:   (%.2f, %.2f, %.2f)\n", maxV[0], maxV[1], maxV[2])
			fmt.Printf("  BBox size:  (%.2f, %.2f, %.2f)\n", sizeX, sizeY, sizeZ)
			diag := math.Sqrt(float64(sizeX*sizeX + sizeY*sizeY + sizeZ*sizeZ))
			fmt.Printf("  BBox diag:  %.2f\n", diag)
		}

		// Bone usage
		boneUsage := map[int16]int{}
		for _, n := range m.Nodes {
			boneUsage[n]++
		}
		fmt.Printf("  Bone refs:  %d unique bones", len(boneUsage))
		if len(boneUsage) <= 8 {
			fmt.Print(" →")
			for bn, cnt := range boneUsage {
				fmt.Printf(" [bone %d: %d verts]", bn, cnt)
			}
		}
		fmt.Println()

		// Filter checks
		isEffect := filter.IsEffectMesh(m)
		isBody := filter.IsBodyMesh(m)
		fmt.Printf("  IsEffectMesh: %v\n", isEffect)
		fmt.Printf("  IsBodyMesh:   %v\n", isBody)
		fmt.Println()
	}
}
