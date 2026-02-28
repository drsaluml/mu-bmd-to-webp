package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"mu-bmd-renderer/internal/bmd"
	"mu-bmd-renderer/internal/filter"
	"mu-bmd-renderer/internal/texture"
)

func main() {
	path := os.Args[1]
	meshes, bones, err := bmd.Parse(path)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	idx := texture.BuildIndex("Data/Item")
	cache := texture.NewCache(idx)

	fmt.Printf("Meshes: %d, Bones: %d\n", len(meshes), len(bones))
	for i, m := range meshes {
		isEffect := filter.IsEffectMesh(&m)
		ext := strings.ToLower(filepath.Ext(strings.ReplaceAll(m.TexPath, "\\", "/")))
		tex := cache.Resolve(m.TexPath)
		texInfo := "NOT FOUND"
		if tex != nil {
			b := tex.Bounds()
			visible, opaque := 0, 0
			for y := 0; y < b.Dy(); y++ {
				for x := 0; x < b.Dx(); x++ {
					a := tex.Pix[y*tex.Stride+x*4+3]
					if a > 0 {
						visible++
					}
					if a == 255 {
						opaque++
					}
				}
			}
			pct := 0.0
			if visible > 0 {
				pct = 100 * float64(opaque) / float64(visible)
			}
			texInfo = fmt.Sprintf("%dx%d, opaque=%.0f%% (%d/%d)", b.Dx(), b.Dy(), pct, opaque, visible)
		}
		fmt.Printf("  Mesh[%d]: verts=%d tris=%d tex=%q ext=%s isEffect=%v\n",
			i, len(m.Verts), len(m.Tris), m.TexPath, ext, isEffect)
		fmt.Printf("    Texture: %s\n", texInfo)

		// Dump texture as PNG if requested with --dump flag
		if len(os.Args) > 2 && os.Args[2] == "--dump" && tex != nil {
			stem := strings.TrimSuffix(filepath.Base(strings.ReplaceAll(m.TexPath, "\\", "/")), ext)
			outPath := fmt.Sprintf("/tmp/%s_dump.png", stem)
			f, _ := os.Create(outPath)
			png.Encode(f, tex)
			f.Close()
			fmt.Printf("    Dumped: %s\n", outPath)
		}
	}
}
