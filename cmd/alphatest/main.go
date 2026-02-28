package main

import (
	"fmt"
	"image"

	"mu-bmd-renderer/internal/texture"
)

func main() {
	// Build texture index
	idx := texture.BuildIndex("Data/Item")

	// Create cache
	cache := texture.NewCache(idx)

	// Resolve shield03.jpg (used by shield05.bmd = item 6_4)
	tex := cache.Resolve("shield03.jpg")
	if tex == nil {
		fmt.Println("Failed to resolve shield03.jpg")
		return
	}

	fmt.Printf("Texture: %dx%d\n", tex.Bounds().Dx(), tex.Bounds().Dy())

	// Check alpha values
	b := tex.Bounds()
	w, h := b.Dx(), b.Dy()
	var minA, maxA uint8 = 255, 0
	total := 0
	sumA := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := tex.Pix[y*tex.Stride+x*4+3]
			total++
			sumA += int(a)
			if a < minA {
				minA = a
			}
			if a > maxA {
				maxA = a
			}
		}
	}
	fmt.Printf("Alpha: min=%d, max=%d, avg=%.0f, all_255=%v\n",
		minA, maxA, float64(sumA)/float64(total), minA == 255)

	// Also check a few specific pixels
	for _, p := range [][2]int{{0, 0}, {32, 32}, {63, 63}} {
		x, y := p[0], p[1]
		if x < w && y < h {
			i := y*tex.Stride + x*4
			fmt.Printf("  Pixel(%d,%d): R=%d G=%d B=%d A=%d\n",
				x, y, tex.Pix[i], tex.Pix[i+1], tex.Pix[i+2], tex.Pix[i+3])
		}
	}

	// Check what Go's JPEG decoder returns for this file
	fmt.Println("\n--- Direct load test ---")
	tex2, err := texture.LoadTexture("Data/Item/texture/shield03.ozj")
	if err != nil {
		fmt.Printf("LoadTexture error: %v\n", err)
		return
	}
	checkNRGBAAlpha(tex2, "shield03.ozj")

	// Also check shield07.tga (used by shield04.bmd)
	tex3 := cache.Resolve("shield07.tga")
	if tex3 != nil {
		checkNRGBAAlpha(tex3, "shield07.tga")
	}
}

func checkNRGBAAlpha(tex *image.NRGBA, name string) {
	b := tex.Bounds()
	w, h := b.Dx(), b.Dy()
	var minA, maxA uint8 = 255, 0
	total := 0
	opaque := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := tex.Pix[y*tex.Stride+x*4+3]
			total++
			if a < minA {
				minA = a
			}
			if a > maxA {
				maxA = a
			}
			if a == 255 {
				opaque++
			}
		}
	}
	fmt.Printf("%s: %dx%d, alpha: min=%d max=%d opaque=%d/%d (%.0f%%)\n",
		name, w, h, minA, maxA, opaque, total, 100*float64(opaque)/float64(total))
}
