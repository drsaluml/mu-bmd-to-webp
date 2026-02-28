package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"mu-bmd-renderer/internal/bmd"
	"mu-bmd-renderer/internal/filter"
	"mu-bmd-renderer/internal/itemlist"
	"mu-bmd-renderer/internal/texture"
	"mu-bmd-renderer/internal/trs"
)

// Item type classification by model file name
var (
	wingRE  = regexp.MustCompile(`(?i)^wing\d+|chaoswing|magic_wing|flamewing|conquerorwing|angel_devil_wing|wingsofpower|ManaBurstWing|SpiritualWorldWing`)
	capeRE  = regexp.MustCompile(`(?i)^darklord|cape_of|cloak|jacquard_|PureWhite|Innocence|Sparkle|Resplendent|Limit_wing`)
	petRE   = regexp.MustCompile(`(?i)_inven|_in[BR]\.|moru|petEgg|Apocal_stone|lightning_(?:stone|anvil)|Ghosthorse`)
	gemRE   = regexp.MustCompile(`(?i)^gem\d+|^jewel|^attjewel|gemmix|SpiritDust|spellstone`)
	seedRE  = regexp.MustCompile(`(?i)^s30_`)
	pentaRE = regexp.MustCompile(`(?i)^penta`)
	scrollRE = regexp.MustCompile(`(?i)scroll|rollofpaper`)
	earRE   = regexp.MustCompile(`(?i)earring|so_neck`)
	ringRE  = regexp.MustCompile(`(?i)^ring\d+`)
	boxRE   = regexp.MustCompile(`(?i)box|gift`)
	bookRE  = regexp.MustCompile(`(?i)^book\d+|strengscroll`)
)

func classifyModel(modelFile string) string {
	stem := strings.TrimSuffix(modelFile, filepath.Ext(modelFile))
	switch {
	case wingRE.MatchString(stem):
		return "wing"
	case capeRE.MatchString(stem):
		return "cape"
	case petRE.MatchString(stem):
		return "pet"
	case gemRE.MatchString(stem):
		return "gem"
	case seedRE.MatchString(stem):
		return "seed"
	case pentaRE.MatchString(stem):
		return "penta"
	case scrollRE.MatchString(stem):
		return "scroll"
	case earRE.MatchString(stem):
		return "earring"
	case ringRE.MatchString(stem):
		return "ring"
	case boxRE.MatchString(stem):
		return "box"
	case bookRE.MatchString(stem):
		return "book"
	default:
		return "misc"
	}
}

func texStem(texPath string) string {
	base := filepath.Base(strings.ReplaceAll(texPath, "\\", "/"))
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func main() {
	itemDir := "Data/Item"
	xmlPath := "Data/Xml/ItemList.xml"
	bmdPath := "Data/Local/itemtrsdata.bmd"
	customPath := "custom_trs.json"

	items, err := itemlist.Parse(xmlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing ItemList: %v\n", err)
		os.Exit(1)
	}

	idx := texture.BuildIndex(itemDir)
	cache := texture.NewCache(idx)

	trsData, err := trs.Load(bmdPath, customPath, xmlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: TRS load error: %v\n", err)
	}

	// Filter section 12
	var sec12 []itemlist.ItemDef
	for _, it := range items {
		if it.Section == 12 {
			sec12 = append(sec12, it)
		}
	}
	sort.Slice(sec12, func(i, j int) bool { return sec12[i].Index < sec12[j].Index })
	fmt.Printf("Section 12: %d items\n\n", len(sec12))

	processedModels := map[string]bool{}

	// Stats by type
	typeCounts := map[string]int{}
	typeItems := map[string][]int{}

	// Problem tracking
	type problemItem struct {
		index  int
		name   string
		model  string
		issues []string
	}
	var problems []problemItem

	// Stats
	totalItems := 0
	bmdNotFound := 0
	parseErrors := 0
	missingTexItems := 0
	effectMeshItems := 0
	bodyMeshItems := 0

	// TRS stats
	trsHas := 0
	trsNone := 0
	trsCameras := map[string]int{}

	for _, it := range sec12 {
		totalItems++
		cat := classifyModel(it.ModelFile)
		typeCounts[cat]++
		typeItems[cat] = append(typeItems[cat], it.Index)

		bmdFile := filepath.Join(itemDir, it.SubDir, it.ModelFile)
		if _, err := os.Stat(bmdFile); os.IsNotExist(err) {
			problems = append(problems, problemItem{it.Index, it.Name, it.ModelFile, []string{"BMD_NOT_FOUND"}})
			bmdNotFound++
			continue
		}

		meshes, bones, err := bmd.Parse(bmdFile)
		if err != nil {
			problems = append(problems, problemItem{it.Index, it.Name, it.ModelFile, []string{fmt.Sprintf("PARSE_ERROR: %v", err)}})
			parseErrors++
			continue
		}

		// TRS info
		entry := trsData[[2]int{12, it.Index}]
		if entry != nil {
			trsHas++
			cam := entry.Camera
			if cam == "" {
				cam = "auto"
			}
			trsCameras[cam]++
		} else {
			trsNone++
		}

		isNewModel := !processedModels[strings.ToLower(it.ModelFile)]
		processedModels[strings.ToLower(it.ModelFile)] = true

		var itemIssues []string
		hasMissingTex := false
		hasEffect := false
		hasBody := false

		// Compute model BBox across all meshes
		var allMinV, allMaxV [3]float32
		firstVert := true

		for mi, m := range meshes {
			stem := texStem(m.TexPath)
			tex := cache.Resolve(m.TexPath)
			isEffect := filter.IsEffectMesh(&m)
			isBody := filter.IsBodyMesh(&m)

			if tex == nil {
				hasMissingTex = true
			}
			if isEffect {
				hasEffect = true
			}
			if isBody {
				hasBody = true
			}

			// BBox
			for _, v := range m.Verts {
				if firstVert {
					allMinV = v
					allMaxV = v
					firstVert = false
				} else {
					for k := 0; k < 3; k++ {
						if v[k] < allMinV[k] {
							allMinV[k] = v[k]
						}
						if v[k] > allMaxV[k] {
							allMaxV[k] = v[k]
						}
					}
				}
			}

			// Print detailed mesh info for new models that have issues
			if isNewModel && (tex == nil || isEffect || isBody) {
				texSize := "N/A"
				if tex != nil {
					b := tex.Bounds()
					texSize = fmt.Sprintf("%dx%d", b.Dx(), b.Dy())
				}
				flags := ""
				if tex == nil {
					flags += " [MISSING_TEX]"
				}
				if isEffect {
					flags += " [EFFECT]"
				}
				if isBody {
					flags += " [BODY]"
				}
				fmt.Printf("  12_%d %s (%s)\n", it.Index, it.Name, it.ModelFile)
				fmt.Printf("    Mesh[%d]: v=%d t=%d tex=%q (%s)%s\n",
					mi, len(m.Verts), len(m.Tris), stem, texSize, flags)
			}
		}

		if hasMissingTex {
			missingTexItems++
			itemIssues = append(itemIssues, "MISSING_TEX")
		}
		if hasEffect {
			effectMeshItems++
		}
		if hasBody {
			bodyMeshItems++
			itemIssues = append(itemIssues, "BODY_MESH")
		}

		// BBox shape analysis
		if !firstVert {
			sx := float64(allMaxV[0] - allMinV[0])
			sy := float64(allMaxV[1] - allMinV[1])
			sz := float64(allMaxV[2] - allMinV[2])
			maxDim := math.Max(sx, math.Max(sy, sz))
			minDim := math.Min(sx, math.Min(sy, sz))
			if maxDim > 0 && minDim/maxDim < 0.05 {
				shape := fmt.Sprintf("FLAT(%.0f×%.0f×%.0f)", sx, sy, sz)
				if isNewModel {
					itemIssues = append(itemIssues, shape)
				}
			}
		}

		// Print model summary for new models
		if isNewModel && len(meshes) > 0 {
			sx := allMaxV[0] - allMinV[0]
			sy := allMaxV[1] - allMinV[1]
			sz := allMaxV[2] - allMinV[2]
			cam := "none"
			if entry != nil {
				cam = entry.Camera
				if cam == "" {
					cam = "auto"
				}
			}
			_ = bones // suppress unused
			fmt.Printf("  [%s] 12_%d %-35s %-25s meshes=%d bones=%d bbox=(%.0f,%.0f,%.0f) cam=%s\n",
				cat, it.Index, it.Name, it.ModelFile, len(meshes), len(bones), sx, sy, sz, cam)
		}

		if len(itemIssues) > 0 && isNewModel {
			problems = append(problems, problemItem{it.Index, it.Name, it.ModelFile, itemIssues})
		}
	}

	// Summary
	fmt.Printf("\n=== SUMMARY ===\n")
	fmt.Printf("Total items:      %d\n", totalItems)
	fmt.Printf("Unique models:    %d\n", len(processedModels))
	fmt.Printf("BMD not found:    %d\n", bmdNotFound)
	fmt.Printf("Parse errors:     %d\n", parseErrors)
	fmt.Printf("Missing textures: %d items\n", missingTexItems)
	fmt.Printf("Effect meshes:    %d items\n", effectMeshItems)
	fmt.Printf("Body meshes:      %d items\n", bodyMeshItems)

	fmt.Printf("\n=== BY TYPE ===\n")
	typeOrder := []string{"wing", "cape", "pet", "gem", "seed", "penta", "scroll", "earring", "ring", "box", "book", "misc"}
	for _, t := range typeOrder {
		if c := typeCounts[t]; c > 0 {
			fmt.Printf("  %-10s %3d items\n", t, c)
		}
	}

	fmt.Printf("\n=== TRS CONFIG ===\n")
	fmt.Printf("Has TRS:  %d\n", trsHas)
	fmt.Printf("No TRS:   %d\n", trsNone)
	fmt.Printf("Cameras:  ")
	for cam, cnt := range trsCameras {
		fmt.Printf("%s=%d ", cam, cnt)
	}
	fmt.Println()

	// Items without TRS (should be 0 with override mode)
	if trsNone > 0 {
		fmt.Printf("\n=== ITEMS WITHOUT TRS (%d) ===\n", trsNone)
		for _, it := range sec12 {
			if trsData[[2]int{12, it.Index}] == nil {
				fmt.Printf("  12_%d %s (%s)\n", it.Index, it.Name, it.ModelFile)
			}
		}
	}

	// Problem items
	if len(problems) > 0 {
		fmt.Printf("\n=== PROBLEM ITEMS (%d) ===\n", len(problems))
		for _, p := range problems {
			fmt.Printf("  12_%-4d %-35s %-25s %v\n", p.index, p.name, p.model, p.issues)
		}
	}

	// Wing/Cape items without per-item TRS override (using section default)
	fmt.Printf("\n=== WING/CAPE ITEMS — TRS SOURCE CHECK ===\n")
	wingCapeNoOverride := 0
	for _, it := range sec12 {
		cat := classifyModel(it.ModelFile)
		if cat != "wing" && cat != "cape" {
			continue
		}
		entry := trsData[[2]int{12, it.Index}]
		if entry == nil {
			fmt.Printf("  12_%d [%s] %s (%s) — NO TRS\n", it.Index, cat, it.Name, it.ModelFile)
			wingCapeNoOverride++
		} else if entry.Camera == "fallback" && entry.RotX == 0 && entry.RotY == 0 {
			// Using section default (no specific wing/cape rotation)
			fmt.Printf("  12_%d [%s] %s (%s) — SECTION DEFAULT (cam=%s rotX=%.0f rotY=%.0f)\n",
				it.Index, cat, it.Name, it.ModelFile, entry.Camera, entry.RotX, entry.RotY)
			wingCapeNoOverride++
		}
	}
	if wingCapeNoOverride == 0 {
		fmt.Println("  All wing/cape items have per-item TRS overrides.")
	} else {
		fmt.Printf("  %d wing/cape items using section default (may need wing12/cape12 preset)\n", wingCapeNoOverride)
	}
}
