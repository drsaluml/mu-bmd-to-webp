package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	ozjHeaderSize = 24
	oztHeaderSize = 4
)

type texFile struct {
	srcPath string
	dstName string
	format  string // "ozj" or "ozt"
}

func dumpTexture(base string, f texFile) error {
	src := filepath.Join(base, "Data", "Item", "texture", f.srcPath)
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}

	var headerSize int
	var dataType string
	switch f.format {
	case "ozj":
		headerSize = ozjHeaderSize
		dataType = "JPEG"
	case "ozt":
		headerSize = oztHeaderSize
		dataType = "TGA"
	default:
		return fmt.Errorf("unknown format %q", f.format)
	}

	if len(data) <= headerSize {
		return fmt.Errorf("%s: file too small (%d bytes, need > %d)", src, len(data), headerSize)
	}

	payload := data[headerSize:]
	if err := os.WriteFile(f.dstName, payload, 0644); err != nil {
		return fmt.Errorf("write %s: %w", f.dstName, err)
	}
	fmt.Printf("OK  %s -> %s  (%d-byte header skipped, %d bytes %s written)\n",
		f.srcPath, f.dstName, headerSize, len(payload), dataType)
	return nil
}

func main() {
	base := "."
	if len(os.Args) > 1 {
		base = os.Args[1]
	}

	files := []texFile{
		{srcPath: "rewardbox01.OZJ", dstName: "rewardbox01_dump.jpg", format: "ozj"},
		{srcPath: "itembox_gold.ozj", dstName: "itembox_gold_dump.jpg", format: "ozj"},
		{srcPath: "itembox_lock.ozj", dstName: "itembox_lock_dump.jpg", format: "ozj"},
		{srcPath: "itembox_wood_gold.ozj", dstName: "itembox_wood_gold_dump.jpg", format: "ozj"},
		{srcPath: "itembox_chain.ozt", dstName: "itembox_chain_dump.tga", format: "ozt"},
	}

	errors := 0
	for _, f := range files {
		if err := dumpTexture(base, f); err != nil {
			fmt.Fprintf(os.Stderr, "ERR %v\n", err)
			errors++
		}
	}
	if errors > 0 {
		fmt.Printf("\nDone with %d error(s).\n", errors)
		os.Exit(1)
	}
	fmt.Println("\nDone. All textures extracted.")
}
