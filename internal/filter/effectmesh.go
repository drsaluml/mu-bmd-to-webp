package filter

import (
	"path/filepath"
	"regexp"
	"strings"

	"mu-bmd-renderer/internal/bmd"
)

var gradientEffectRE = regexp.MustCompile(`^(?:mini_|hangul)?gra(?:\d|_|$)`)

var effectPatterns = []string{
	"glow", "flare", "chrome", "effect",
	"aura", "shiny", "spark", "fire", "blur",
	"elec_light", "arrowlight", "lighting_mega", "pin_star",
	"lightmarks", "light_blue", "light_red",
	"energy", "plasma", "shine", "halo", "trail",
	"gradation", "sdblight", "alpha_line", "4x4", "damage",
	"ground_wind", "ground_star", "line_of_big",
	"force", "runeset",
	"shockwave", "swordeff",
	"cursorpin", "empact", "circle_shield",
	"arrowbom", "raypiece",
}

// effectPrefixPatterns must match at the START of the texture stem only.
// "flame" is prefix-only to avoid false positives like "requitalbox_flame_wood"
// (metal frame of reward box, not a fire effect).
var effectPrefixPatterns = []string{"flame"}

// bodyTextureRE matches character body/skin/hair texture stems that are NOT part of
// equipment geometry. These appear in helmet/armor BMDs as the character model
// underneath the equipment piece. When unresolvable they render as gray blobs;
// when resolved (e.g. HQhair_R) they create unwanted body/hair overlays.
var bodyTextureRE = regexp.MustCompile(`(?i)^(?:` +
	`hqskin(?:2)?(?:_)?class\d+` + // HQSkinClass313, HQskin2Class314, HQskin_Class109
	`|skinclass\d+head` + // Skinclass206head_N (face mesh); excludes Skinclass206_headhelmet (underscore before "head")
	`|nude_` + // nude_* body/skin underlays (nude_Item1161, nude_Armor, nude_class206_head, etc.)
	`|item\d+_head` + // Item3002_Head (face), Item3002_headhair (hair) — character head in equipment BMDs
	`|skin_(?:barbarian|warrior|class)` + // skin_barbarian_01, skin_warrior_01, skin_Class107
	`|level_man\d+` + // level_man01, level_man022, level_man033
	`|(?:hq)?hair_r` + // hair glow overlay: hair_R (missing) and HQhair_R (resolved)
	`|cobraset_hair` + // wizard beard (HDK_HelmMale02.bmd)
	`|tknight_hair` + // knight hair (HelmMale172/177_fighter.bmd)
	`)`)

// IsBodyMesh returns true if this mesh is a character body/skin/hair mesh.
// These appear in helmet/armor BMDs as the character model underneath the
// equipment piece. Detected by texture name pattern — these names are specific
// to character models (HQSkinClass*, skin_barbarian*, level_man*, hair_R)
// and never used for equipment textures.
func IsBodyMesh(m *bmd.Mesh) bool {
	tex := strings.ToLower(m.TexPath)
	stem := strings.TrimSuffix(filepath.Base(strings.ReplaceAll(tex, "\\", "/")), filepath.Ext(tex))
	return bodyTextureRE.MatchString(stem)
}

// IsEffectMesh returns true if this mesh is an aura/glow/effect overlay.
func IsEffectMesh(m *bmd.Mesh) bool {
	tex := strings.ToLower(m.TexPath)
	stem := strings.TrimSuffix(filepath.Base(strings.ReplaceAll(tex, "\\", "/")), filepath.Ext(tex))

	// Texture-based detection
	if gradientEffectRE.MatchString(stem) {
		return true
	}
	for _, p := range effectPatterns {
		if strings.Contains(stem, p) {
			return true
		}
	}
	for _, p := range effectPrefixPatterns {
		if strings.HasPrefix(stem, p) {
			return true
		}
	}

	// Small geometry heuristic — but keep large quads (e.g. blade decals)
	nv := len(m.Verts)
	nt := len(m.Tris)
	if nv <= 8 && nt <= 4 && nv > 0 {
		var minV, maxV [3]float32
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
		span := float64(0)
		for k := 0; k < 3; k++ {
			d := float64(maxV[k] - minV[k])
			if d > span {
				span = d
			}
		}
		if span > 20 {
			return false // significant visual area
		}
		return true
	}

	return false
}

// FilterComponents removes small disconnected components from a mesh.
// Returns a new mesh with filtered triangles (shares underlying vertex data).
func FilterComponents(m *bmd.Mesh, minVerts int) bmd.Mesh {
	if len(m.Verts) == 0 || len(m.Tris) == 0 {
		return *m
	}
	// Very small meshes (e.g. billboard wings, simple quads) are unlikely
	// to contain junk components — skip filtering to avoid removing
	// symmetric pairs like bat wings (helper02.bmd: 2 quads, 8 verts).
	if len(m.Verts) <= 2*minVerts {
		return *m
	}

	// Build adjacency
	adj := make(map[int][]int)
	for _, tri := range m.Tris {
		n := 3
		if tri.Polygon == 4 {
			n = 4
		}
		for a := 0; a < n; a++ {
			for b := a + 1; b < n; b++ {
				va, vb := int(tri.VI[a]), int(tri.VI[b])
				if va < 0 || va >= len(m.Verts) || vb < 0 || vb >= len(m.Verts) {
					continue
				}
				adj[va] = append(adj[va], vb)
				adj[vb] = append(adj[vb], va)
			}
		}
	}

	// BFS connected components
	visited := make([]bool, len(m.Verts))
	var components [][]int
	for v := range m.Verts {
		if visited[v] || len(adj[v]) == 0 {
			continue
		}
		comp := []int{}
		stack := []int{v}
		for len(stack) > 0 {
			curr := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if visited[curr] {
				continue
			}
			visited[curr] = true
			comp = append(comp, curr)
			for _, nb := range adj[curr] {
				if !visited[nb] {
					stack = append(stack, nb)
				}
			}
		}
		components = append(components, comp)
	}

	if len(components) <= 1 {
		return *m
	}

	// Find largest component
	largestIdx := 0
	for i, c := range components {
		if len(c) > len(components[largestIdx]) {
			largestIdx = i
		}
	}
	largest := components[largestIdx]

	// Compute largest component center and span
	var lCenter [3]float64
	var lMin, lMax [3]float32
	lMin = m.Verts[largest[0]]
	lMax = m.Verts[largest[0]]
	for _, vi := range largest {
		v := m.Verts[vi]
		for k := 0; k < 3; k++ {
			lCenter[k] += float64(v[k])
			if v[k] < lMin[k] {
				lMin[k] = v[k]
			}
			if v[k] > lMax[k] {
				lMax[k] = v[k]
			}
		}
	}
	for k := 0; k < 3; k++ {
		lCenter[k] /= float64(len(largest))
	}
	var lSpan float64
	for k := 0; k < 3; k++ {
		d := float64(lMax[k] - lMin[k])
		if d > lSpan {
			lSpan = d
		}
	}

	// Decide which vertices to keep
	keepVerts := make(map[int]bool)
	for _, vi := range largest {
		keepVerts[vi] = true
	}
	for i, comp := range components {
		if i == largestIdx {
			continue
		}
		if len(comp) >= minVerts {
			for _, vi := range comp {
				keepVerts[vi] = true
			}
		} else {
			// Keep if close to largest component bounding box
			var cCenter [3]float64
			for _, vi := range comp {
				for k := 0; k < 3; k++ {
					cCenter[k] += float64(m.Verts[vi][k])
				}
			}
			for k := 0; k < 3; k++ {
				cCenter[k] /= float64(len(comp))
			}
			// Distance to nearest point on largest component's bbox
			var distSq float64
			for k := 0; k < 3; k++ {
				lo := float64(lMin[k])
				hi := float64(lMax[k])
				c := cCenter[k]
				if c < lo {
					d := lo - c
					distSq += d * d
				} else if c > hi {
					d := c - hi
					distSq += d * d
				}
			}
			if distSq < lSpan*lSpan*0.16 { // 0.4² = 0.16
				for _, vi := range comp {
					keepVerts[vi] = true
				}
			}
		}
	}

	// Filter triangles
	var filteredTris []bmd.Triangle
	for _, tri := range m.Tris {
		if keepVerts[int(tri.VI[0])] && keepVerts[int(tri.VI[1])] && keepVerts[int(tri.VI[2])] {
			filteredTris = append(filteredTris, tri)
		}
	}

	result := *m
	result.Tris = filteredTris
	return result
}
