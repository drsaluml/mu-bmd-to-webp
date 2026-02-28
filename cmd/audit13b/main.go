package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"mu-bmd-renderer/internal/bmd"
	"mu-bmd-renderer/internal/filter"
	"mu-bmd-renderer/internal/texture"
)

type auditItem struct {
	section   int
	index     int
	name      string
	modelFile string
}

func texStem(texPath string) string {
	base := filepath.Base(strings.ReplaceAll(texPath, "\\", "/"))
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func texExt(texPath string) string {
	return strings.ToLower(filepath.Ext(strings.ReplaceAll(texPath, "\\", "/")))
}

func main() {
	itemDir := "Data/Item"

	// Build texture index
	idx := texture.BuildIndex(itemDir)
	cache := texture.NewCache(idx)

	// Items to audit (from ItemList.xml section 13)
	items := []auditItem{
		{13, 50, "Illusion Sorcerer Covenant", "oath.bmd"},
		{13, 65, "Spirit of Guardian", "maria.bmd"},
		{13, 129, "Goat Figurine", "SheepStatue.bmd"},
		{13, 189, "Pure Crimson Wing Core (Type 1)", "Wing511_core1.bmd"},
	}

	for _, it := range items {
		fmt.Printf("============================================================\n")
		fmt.Printf("ITEM: %d_%d  %s\n", it.section, it.index, it.name)
		fmt.Printf("Model: %s\n", it.modelFile)
		fmt.Printf("============================================================\n")

		bmdPath := filepath.Join(itemDir, it.modelFile)
		if _, err := os.Stat(bmdPath); os.IsNotExist(err) {
			fmt.Printf("  *** BMD FILE NOT FOUND: %s ***\n\n", bmdPath)
			continue
		}

		meshes, bones, err := bmd.Parse(bmdPath)
		if err != nil {
			fmt.Printf("  *** PARSE ERROR: %v ***\n\n", err)
			continue
		}

		fmt.Printf("Mesh count: %d\n", len(meshes))
		fmt.Printf("Bone count: %d\n", len(bones))

		// ---- Per-mesh detail ----
		fmt.Printf("\n--- Meshes ---\n")
		for mi, m := range meshes {
			stem := texStem(m.TexPath)
			ext := texExt(m.TexPath)

			// Bounding box
			var minV, maxV [3]float32
			if len(m.Verts) > 0 {
				minV = m.Verts[0]
				maxV = m.Verts[0]
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
			}
			bboxStr := fmt.Sprintf("min=(%.1f, %.1f, %.1f) max=(%.1f, %.1f, %.1f) size=(%.1f, %.1f, %.1f)",
				minV[0], minV[1], minV[2],
				maxV[0], maxV[1], maxV[2],
				maxV[0]-minV[0], maxV[1]-minV[1], maxV[2]-minV[2])

			// Texture resolution
			tex := cache.Resolve(m.TexPath)
			texInfo := "MISSING"
			if tex != nil {
				b := tex.Bounds()
				texInfo = fmt.Sprintf("%dx%d", b.Dx(), b.Dy())
			}
			_, texResolved := idx.ResolvePath(m.TexPath)

			// Filter checks
			isEffect := filter.IsEffectMesh(&m)
			isBody := filter.IsBodyMesh(&m)

			// Bone usage
			boneSet := map[int16]bool{}
			for _, n := range m.Nodes {
				boneSet[n] = true
			}

			fmt.Printf("\n  Mesh[%d]:\n", mi)
			fmt.Printf("    Texture:  %q (stem=%q, ext=%s)\n", m.TexPath, stem, ext)
			fmt.Printf("    TexSize:  %s  (resolved=%v)\n", texInfo, texResolved)
			fmt.Printf("    Vertices: %d\n", len(m.Verts))
			fmt.Printf("    Triangles:%d\n", len(m.Tris))
			fmt.Printf("    Normals:  %d\n", len(m.Normals))
			fmt.Printf("    UVs:      %d\n", len(m.UVs))
			fmt.Printf("    BBox:     %s\n", bboxStr)
			fmt.Printf("    BoneRefs: %d unique bones %v\n", len(boneSet), sortedKeys(boneSet))
			fmt.Printf("    IsEffect: %v\n", isEffect)
			fmt.Printf("    IsBody:   %v\n", isBody)

			// Check for quads
			quads := 0
			for _, t := range m.Tris {
				if t.Polygon == 4 {
					quads++
				}
			}
			if quads > 0 {
				fmt.Printf("    Quads:    %d (of %d polys)\n", quads, len(m.Tris))
			}
		}

		// ---- Bone hierarchy ----
		if len(bones) > 0 {
			fmt.Printf("\n--- Bone Hierarchy (%d bones) ---\n", len(bones))
			for bi, b := range bones {
				dummy := ""
				if b.IsDummy {
					dummy = " [DUMMY]"
				}
				pos := b.BindPosition
				rot := b.BindRotation
				rotDeg := [3]float64{
					rot[0] * 180 / math.Pi,
					rot[1] * 180 / math.Pi,
					rot[2] * 180 / math.Pi,
				}
				fmt.Printf("  Bone[%d]: parent=%d%s\n", bi, b.Parent, dummy)
				fmt.Printf("    pos=(%.2f, %.2f, %.2f)  rot=(%.1f°, %.1f°, %.1f°)\n",
					pos[0], pos[1], pos[2], rotDeg[0], rotDeg[1], rotDeg[2])
			}

			// Print parent chain for each leaf bone
			fmt.Printf("\n--- Bone Parent Chains ---\n")
			// Find leaf bones (not parent of any other bone)
			isParent := map[int]bool{}
			for _, b := range bones {
				if b.Parent >= 0 {
					isParent[b.Parent] = true
				}
			}
			for bi := range bones {
				if !isParent[bi] {
					chain := []int{bi}
					cur := bi
					for bones[cur].Parent >= 0 && bones[cur].Parent != cur {
						cur = bones[cur].Parent
						chain = append(chain, cur)
					}
					fmt.Printf("  Leaf bone %d chain: %v\n", bi, chain)
				}
			}
		}

		// ---- Overall BBox ----
		var allMin, allMax [3]float32
		first := true
		for _, m := range meshes {
			for _, v := range m.Verts {
				if first {
					allMin = v
					allMax = v
					first = false
				} else {
					for k := 0; k < 3; k++ {
						if v[k] < allMin[k] {
							allMin[k] = v[k]
						}
						if v[k] > allMax[k] {
							allMax[k] = v[k]
						}
					}
				}
			}
		}
		if !first {
			sx := allMax[0] - allMin[0]
			sy := allMax[1] - allMin[1]
			sz := allMax[2] - allMin[2]
			maxDim := float64(sx)
			if float64(sy) > maxDim {
				maxDim = float64(sy)
			}
			if float64(sz) > maxDim {
				maxDim = float64(sz)
			}
			minDim := float64(sx)
			if float64(sy) < minDim {
				minDim = float64(sy)
			}
			if float64(sz) < minDim {
				minDim = float64(sz)
			}
			flatRatio := 0.0
			if maxDim > 0 {
				flatRatio = minDim / maxDim
			}
			fmt.Printf("\n--- Overall BBox ---\n")
			fmt.Printf("  min=(%.1f, %.1f, %.1f)  max=(%.1f, %.1f, %.1f)\n",
				allMin[0], allMin[1], allMin[2], allMax[0], allMax[1], allMax[2])
			fmt.Printf("  size=(%.1f, %.1f, %.1f)  flatRatio=%.3f\n", sx, sy, sz, flatRatio)
		}

		fmt.Println()
	}
}

func sortedKeys(m map[int16]bool) []int {
	var keys []int
	for k := range m {
		keys = append(keys, int(k))
	}
	// Simple insertion sort (small sets)
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	return keys
}
