package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"mu-bmd-renderer/internal/bmd"
	"mu-bmd-renderer/internal/filter"
	"mu-bmd-renderer/internal/itemlist"
	"mu-bmd-renderer/internal/texture"
)

// Body/skin texture patterns — textures from character models, not equipment
var bodyTextureRE = regexp.MustCompile(`(?i)^(?:` +
	`hqskin(?:2)?(?:_)?class\d+` + // HQSkinClass313, HQskin2Class314, HQskin_Class109
	`|skin_(?:barbarian|warrior|class)` + // skin_barbarian_01, skin_warrior_01, skin_Class107
	`|level_man\d+` + // level_man01, level_man022, level_man033
	`|hair_r` + // hair glow overlay (missing)
	`)`)

func texStem(texPath string) string {
	base := filepath.Base(strings.ReplaceAll(texPath, "\\", "/"))
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func main() {
	itemDir := "Data/Item"
	xmlPath := "Data/Xml/ItemList.xml"

	items, err := itemlist.Parse(xmlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing ItemList: %v\n", err)
		os.Exit(1)
	}

	idx := texture.BuildIndex(itemDir)
	cache := texture.NewCache(idx)

	// Filter section 7 only
	var sec7 []itemlist.ItemDef
	for _, it := range items {
		if it.Section == 7 {
			sec7 = append(sec7, it)
		}
	}
	fmt.Printf("Section 7: %d items\n\n", len(sec7))

	// Track unique models already processed
	processedModels := map[string]bool{}

	// Stats
	totalItems := 0
	itemsWithBodyMesh := 0
	itemsWithMissingTex := 0
	itemsWithEffectMesh := 0
	totalMeshes := 0
	bodyMeshCount := 0
	missingTexCount := 0
	effectMeshCount := 0
	additiveRcount := 0
	billboardCount := 0

	// Collect problem items for summary
	type problemItem struct {
		section, index int
		name, model    string
		issues         []string
	}
	var problems []problemItem

	for _, it := range sec7 {
		totalItems++
		bmdPath := filepath.Join(itemDir, it.SubDir, it.ModelFile)
		if _, err := os.Stat(bmdPath); os.IsNotExist(err) {
			problems = append(problems, problemItem{it.Section, it.Index, it.Name, it.ModelFile, []string{"BMD NOT FOUND"}})
			continue
		}

		meshes, _, err := bmd.Parse(bmdPath)
		if err != nil {
			problems = append(problems, problemItem{it.Section, it.Index, it.Name, it.ModelFile, []string{fmt.Sprintf("PARSE ERROR: %v", err)}})
			continue
		}

		if len(meshes) == 0 {
			problems = append(problems, problemItem{it.Section, it.Index, it.Name, it.ModelFile, []string{"NO MESHES"}})
			continue
		}

		// Only print details for first occurrence of each model
		isNewModel := !processedModels[strings.ToLower(it.ModelFile)]
		processedModels[strings.ToLower(it.ModelFile)] = true

		hasBody := false
		hasMissing := false
		hasEffect := false
		var itemIssues []string

		for mi, m := range meshes {
			totalMeshes++
			stem := texStem(m.TexPath)
			stemLower := strings.ToLower(stem)
			ext := strings.ToLower(filepath.Ext(strings.ReplaceAll(m.TexPath, "\\", "/")))
			tex := cache.Resolve(m.TexPath)
			resolved := tex != nil
			isEffect := filter.IsEffectMesh(&m)
			isBody := !resolved && bodyTextureRE.MatchString(stemLower)
			isAddR := strings.HasSuffix(stemLower, "_r")
			isBillboard := len(m.Verts) <= 16 && len(m.Tris) <= 12 && len(m.Verts) > 0 && (ext == ".jpg" || ext == ".jpeg")

			if isBody {
				bodyMeshCount++
				hasBody = true
			}
			if !resolved {
				missingTexCount++
				hasMissing = true
			}
			if isEffect {
				effectMeshCount++
				hasEffect = true
			}
			if isAddR {
				additiveRcount++
			}
			if isBillboard {
				billboardCount++
			}

			// Print details for new models or problem meshes
			if isNewModel && (!resolved || isBody || isEffect || isAddR) {
				texSize := "N/A"
				if tex != nil {
					b := tex.Bounds()
					texSize = fmt.Sprintf("%dx%d", b.Dx(), b.Dy())
				}
				flags := ""
				if isBody {
					flags += " [BODY]"
				}
				if !resolved {
					flags += " [MISSING_TEX]"
				}
				if isEffect {
					flags += " [EFFECT]"
				}
				if isAddR {
					flags += " [ADDITIVE_R]"
				}
				if isBillboard {
					flags += " [BILLBOARD]"
				}
				fmt.Printf("  7_%d %s (%s)\n", it.Index, it.Name, it.ModelFile)
				fmt.Printf("    Mesh[%d]: v=%d t=%d tex=%q (%s) resolved=%v%s\n",
					mi, len(m.Verts), len(m.Tris), stem, texSize, resolved, flags)
			}
		}

		if hasBody {
			itemsWithBodyMesh++
			itemIssues = append(itemIssues, "BODY_MESH")
		}
		if hasMissing {
			itemsWithMissingTex++
		}
		if hasEffect {
			itemsWithEffectMesh++
			itemIssues = append(itemIssues, "EFFECT_MESH")
		}
		if len(itemIssues) > 0 && isNewModel {
			problems = append(problems, problemItem{it.Section, it.Index, it.Name, it.ModelFile, itemIssues})
		}
	}

	// Summary
	fmt.Printf("\n=== SUMMARY ===\n")
	fmt.Printf("Items scanned:      %d\n", totalItems)
	fmt.Printf("Unique models:      %d\n", len(processedModels))
	fmt.Printf("Total meshes:       %d\n", totalMeshes)
	fmt.Printf("\n")
	fmt.Printf("Items with body mesh (gray):  %d (%.1f%%)\n", itemsWithBodyMesh, 100*float64(itemsWithBodyMesh)/float64(totalItems))
	fmt.Printf("Items with missing texture:   %d\n", itemsWithMissingTex)
	fmt.Printf("Items with effect mesh:       %d\n", itemsWithEffectMesh)
	fmt.Printf("\n")
	fmt.Printf("Body meshes:        %d\n", bodyMeshCount)
	fmt.Printf("Missing tex meshes: %d\n", missingTexCount)
	fmt.Printf("Effect meshes:      %d\n", effectMeshCount)
	fmt.Printf("Additive _R:        %d\n", additiveRcount)
	fmt.Printf("Billboard JPEG:     %d\n", billboardCount)

	// Problem items list
	if len(problems) > 0 {
		fmt.Printf("\n=== PROBLEM ITEMS (%d) ===\n", len(problems))
		for _, p := range problems {
			fmt.Printf("  7_%d %-40s %-30s %v\n", p.index, p.name, p.model, p.issues)
		}
	}
}
