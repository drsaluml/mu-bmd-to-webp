package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"mu-bmd-renderer/internal/batch"
	"mu-bmd-renderer/internal/config"
	"mu-bmd-renderer/internal/itemlist"
	"mu-bmd-renderer/internal/texture"
	"mu-bmd-renderer/internal/trs"
)

func main() {
	// CLI flags
	configFile := flag.String("config", "", "Path to config.json file")
	testN := flag.Int("test", 0, "Render only first N items for testing")
	section := flag.Int("section", -1, "Render only items from this section")
	index := flag.Int("index", -1, "Render only item with this index (requires -section)")
	workers := flag.Int("workers", 0, "Number of worker goroutines (default: NumCPU)")
	dataDir := flag.String("data", "", "Path to base directory (default: auto-detect)")
	outputDir := flag.String("output", "", "Output directory (default: Data/Item-renders)")
	quality := flag.Int("quality", 0, "WebP quality 1-100 (default: 90)")

	flag.Parse()

	// Load config
	var cfg config.Config
	if *configFile != "" {
		var err error
		cfg, err = config.Load(*configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}
	}

	// CLI flags override config file
	cfg.Resolve(config.Flags{
		DataDir:   *dataDir,
		OutputDir: *outputDir,
		Quality:   *quality,
		Workers:   *workers,
	})

	if cfg.BaseDir == "" {
		fmt.Fprintln(os.Stderr, "Error: cannot find Data directory. Use -data flag or config.json.")
		os.Exit(1)
	}

	// Load item list
	items, err := itemlist.Parse(cfg.ItemListXML)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading ItemList.xml: %v\n", err)
		os.Exit(1)
	}

	// Filter by section/index
	if *section >= 0 {
		var filtered []itemlist.ItemDef
		for _, it := range items {
			if it.Section != *section {
				continue
			}
			if *index >= 0 && it.Index != *index {
				continue
			}
			filtered = append(filtered, it)
		}
		items = filtered
	}

	// Limit for testing
	if *testN > 0 && *testN < len(items) {
		items = items[:*testN]
	}

	if len(items) == 0 {
		fmt.Println("No items to render.")
		os.Exit(0)
	}

	// Load TRS data
	trsData, err := trs.Load(cfg.TRSBMD, cfg.CustomTRS, cfg.ItemListXML)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: TRS load: %v\n", err)
	}
	fmt.Printf("TRS data: %d items loaded\n", len(trsData))

	// Build texture index
	texIndex := texture.BuildIndex(cfg.ItemDir)
	texCache := texture.NewCache(texIndex)
	fmt.Printf("Textures: %d indexed\n", texIndex.Len())

	// Print summary
	mode := ""
	if *section >= 0 {
		mode = fmt.Sprintf(" (Section %d)", *section)
	} else if *testN > 0 {
		mode = fmt.Sprintf(" (TEST: first %d)", *testN)
	}

	fmt.Printf("MU Online BMD 3D Renderer → WebP%s\n", mode)
	fmt.Printf("Items: %d, Workers: %d\n", len(items), cfg.Workers)
	fmt.Printf("Output: %s\n", cfg.OutputDir)
	fmt.Println("------------------------------------------------------------")

	start := time.Now()

	// Run batch
	batchCfg := batch.Config{
		ItemDir:     cfg.ItemDir,
		OutputDir:   cfg.OutputDir,
		TexResolver: texCache,
		TRSData:     trsData,
		RenderSize:  cfg.RenderSize,
		WebPQuality: cfg.WebPQuality,
		Supersample: cfg.Supersample,
		Workers:     cfg.Workers,
	}

	results := batch.Run(batchCfg, items)

	elapsed := time.Since(start)
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("Done in %.1fs\n", elapsed.Seconds())

	// Count results
	success, failed := 0, 0
	var errors []Result
	for _, r := range results {
		if r.Success {
			success++
		} else {
			failed++
			errors = append(errors, r)
		}
	}

	fmt.Printf("Rendered: %d/%d\n", success, len(items))

	if len(errors) > 0 {
		fmt.Printf("\nFailed (%d):\n", failed)
		limit := 20
		if len(errors) < limit {
			limit = len(errors)
		}
		for _, e := range errors[:limit] {
			fmt.Printf("  %s: %s\n", e.Name, e.Error)
		}
	}

	// Write manifest
	manifestPath := filepath.Join(cfg.OutputDir, "manifest.json")
	os.MkdirAll(cfg.OutputDir, 0755)
	if err := batch.WriteManifest(manifestPath, items); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: manifest write failed: %v\n", err)
	} else {
		fmt.Printf("Manifest: %s\n", manifestPath)
	}

	if failed > 0 {
		os.Exit(1)
	}
}

type Result = batch.Result
