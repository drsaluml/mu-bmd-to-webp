package main

import (
	"fmt"
	"math"
	"os"
	"mu-bmd-renderer/internal/bmd"
)

func main() {
	path := os.Args[1]
	meshes, bones, err := bmd.Parse(path)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Meshes: %d, Bones: %d\n", len(meshes), len(bones))
	for i, m := range meshes {
		minX, minY, minZ := math.Inf(1), math.Inf(1), math.Inf(1)
		maxX, maxY, maxZ := math.Inf(-1), math.Inf(-1), math.Inf(-1)
		for _, v := range m.Verts {
			x, y, z := float64(v[0]), float64(v[1]), float64(v[2])
			if x < minX { minX = x }
			if y < minY { minY = y }
			if z < minZ { minZ = z }
			if x > maxX { maxX = x }
			if y > maxY { maxY = y }
			if z > maxZ { maxZ = z }
		}
		fmt.Printf("  Mesh[%d]: verts=%d, tris=%d, texture=%q\n", i, len(m.Verts), len(m.Tris), m.TexPath)
		fmt.Printf("    BBox: X[%.1f, %.1f] Y[%.1f, %.1f] Z[%.1f, %.1f]\n", minX, maxX, minY, maxY, minZ, maxZ)
		fmt.Printf("    Size: %.1f x %.1f x %.1f\n", maxX-minX, maxY-minY, maxZ-minZ)

		// Analyze each triangle: direction, area, and coverage
		type faceInfo struct {
			dir  string
			area float64
			zMin float64
			zMax float64
		}
		var faces []faceInfo
		areaByDir := map[string]float64{}
		for _, tri := range m.Tris {
			i0, i1, i2 := int(tri.VI[0]), int(tri.VI[1]), int(tri.VI[2])
			if i0 >= len(m.Verts) || i1 >= len(m.Verts) || i2 >= len(m.Verts) {
				continue
			}
			v0, v1, v2 := m.Verts[i0], m.Verts[i1], m.Verts[i2]
			e1x := float64(v1[0] - v0[0])
			e1y := float64(v1[1] - v0[1])
			e1z := float64(v1[2] - v0[2])
			e2x := float64(v2[0] - v0[0])
			e2y := float64(v2[1] - v0[1])
			e2z := float64(v2[2] - v0[2])
			cx := e1y*e2z - e1z*e2y
			cy := e1z*e2x - e1x*e2z
			cz := e1x*e2y - e1y*e2x
			area := 0.5 * math.Sqrt(cx*cx+cy*cy+cz*cz)
			acx, acy, acz := math.Abs(cx), math.Abs(cy), math.Abs(cz)
			dir := ""
			if acx >= acy && acx >= acz {
				if cx > 0 { dir = "+X(right)" } else { dir = "-X(left)" }
			} else if acy >= acx && acy >= acz {
				if cy > 0 { dir = "+Y(back)" } else { dir = "-Y(front)" }
			} else {
				if cz > 0 { dir = "+Z(top)" } else { dir = "-Z(bottom)" }
			}
			zMin := math.Min(math.Min(float64(v0[2]), float64(v1[2])), float64(v2[2]))
			zMax := math.Max(math.Max(float64(v0[2]), float64(v1[2])), float64(v2[2]))
			faces = append(faces, faceInfo{dir, area, zMin, zMax})
			areaByDir[dir] += area
		}
		fmt.Println("    --- Surface area by direction ---")
		for _, d := range []string{"-Y(front)", "+Y(back)", "+X(right)", "-X(left)", "+Z(top)", "-Z(bottom)"} {
			fmt.Printf("    %s: %.1f sq units\n", d, areaByDir[d])
		}
		fmt.Println("    --- UV mapping: front/back vs left/right ---")
		for fi, tri := range m.Tris {
			f := faces[fi]
			if f.dir == "-Y(front)" || f.dir == "+Y(back)" || f.dir == "+X(right)" || f.dir == "-X(left)" {
				t0, t1, t2 := int(tri.TI[0]), int(tri.TI[1]), int(tri.TI[2])
				uv0, uv1, uv2 := "N/A", "N/A", "N/A"
				if t0 >= 0 && t0 < len(m.UVs) {
					uv0 = fmt.Sprintf("(%.3f,%.3f)", m.UVs[t0][0], m.UVs[t0][1])
				}
				if t1 >= 0 && t1 < len(m.UVs) {
					uv1 = fmt.Sprintf("(%.3f,%.3f)", m.UVs[t1][0], m.UVs[t1][1])
				}
				if t2 >= 0 && t2 < len(m.UVs) {
					uv2 = fmt.Sprintf("(%.3f,%.3f)", m.UVs[t2][0], m.UVs[t2][1])
				}
				fmt.Printf("    tri[%2d] %s Z=[%5.1f..%5.1f] UV: %s %s %s\n", fi, f.dir, f.zMin, f.zMax, uv0, uv1, uv2)
			}
		}
	}
	for i, b := range bones {
		fmt.Printf("  Bone[%d]: parent=%d, pos=(%.2f, %.2f, %.2f), rot=(%.4f, %.4f, %.4f)\n",
			i, b.Parent, b.BindPosition[0], b.BindPosition[1], b.BindPosition[2],
			b.BindRotation[0], b.BindRotation[1], b.BindRotation[2])
	}
}
