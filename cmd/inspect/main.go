package main

import (
	"fmt"
	"math"
	"os"

	"mu-bmd-renderer/internal/bmd"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: inspect <file.bmd>\n")
		os.Exit(1)
	}

	meshes, bones, err := bmd.Parse(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Bones: %d\n", len(bones))
	fmt.Printf("Meshes: %d\n\n", len(meshes))

	for i, m := range meshes {
		var minX, minY, minZ float32 = math.MaxFloat32, math.MaxFloat32, math.MaxFloat32
		var maxX, maxY, maxZ float32 = -math.MaxFloat32, -math.MaxFloat32, -math.MaxFloat32
		for _, v := range m.Verts {
			if v[0] < minX { minX = v[0] }
			if v[1] < minY { minY = v[1] }
			if v[2] < minZ { minZ = v[2] }
			if v[0] > maxX { maxX = v[0] }
			if v[1] > maxY { maxY = v[1] }
			if v[2] > maxZ { maxZ = v[2] }
		}

		triCount := 0
		for _, t := range m.Tris {
			if t.Polygon == 4 {
				triCount += 2
			} else {
				triCount++
			}
		}

		fmt.Printf("Mesh %d: tex=%q\n", i, m.TexPath)
		fmt.Printf("  Verts: %d, Tris: %d (polys: %d)\n", len(m.Verts), triCount, len(m.Tris))
		fmt.Printf("  Bounds X: [%.2f, %.2f] (%.2f)\n", minX, maxX, maxX-minX)
		fmt.Printf("  Bounds Y: [%.2f, %.2f] (%.2f)\n", minY, maxY, maxY-minY)
		fmt.Printf("  Bounds Z: [%.2f, %.2f] (%.2f)\n", minZ, maxZ, maxZ-minZ)

		// Print first few vertices
		limit := len(m.Verts)
		if limit > 10 { limit = 10 }
		for j := 0; j < limit; j++ {
			v := m.Verts[j]
			fmt.Printf("  v[%d] = (%.3f, %.3f, %.3f) bone=%d\n", j, v[0], v[1], v[2], m.Nodes[j])
		}
		if len(m.Verts) > 10 {
			fmt.Printf("  ... (%d more vertices)\n", len(m.Verts)-10)
		}
		fmt.Println()
	}

	// Print bone hierarchy
	if len(bones) > 0 {
		fmt.Println("Bone hierarchy:")
		for i, b := range bones {
			fmt.Printf("  Bone %d: parent=%d dummy=%v pos=(%.3f,%.3f,%.3f) rot=(%.3f,%.3f,%.3f)\n",
				i, b.Parent, b.IsDummy,
				b.BindPosition[0], b.BindPosition[1], b.BindPosition[2],
				b.BindRotation[0], b.BindRotation[1], b.BindRotation[2])
		}
	}
}
