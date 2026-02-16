package batch

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"mu-bmd-renderer/internal/bmd"
	"mu-bmd-renderer/internal/itemlist"
	"mu-bmd-renderer/internal/postprocess"
	"mu-bmd-renderer/internal/raster"
	"mu-bmd-renderer/internal/texture"
	"mu-bmd-renderer/internal/trs"

	"github.com/HugoSmits86/nativewebp"
)

// Config holds all shared resources for a batch run.
type Config struct {
	ItemDir     string
	OutputDir   string
	TexResolver texture.Resolver
	TRSData     trs.Data
	RenderSize  int
	WebPQuality int
	Supersample int
	Workers     int
}

// Result holds the outcome of processing one item.
type Result struct {
	Name    string
	Section int
	Index   int
	Success bool
	Error   string
}

// Run processes all items using a worker pool.
func Run(cfg Config, items []itemlist.ItemDef) []Result {
	total := len(items)
	results := make([]Result, total)
	var processed atomic.Int64

	start := time.Now()

	// Progress reporter
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				p := processed.Load()
				if p > 0 {
					elapsed := time.Since(start).Seconds()
					rate := float64(p) / elapsed
					fmt.Printf("  [%d/%d] %.1f items/sec\n", p, total, rate)
				}
			}
		}
	}()

	// Worker pool
	itemChan := make(chan int, cfg.Workers*2)
	var wg sync.WaitGroup

	for w := 0; w < cfg.Workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range itemChan {
				results[idx] = processItem(cfg, items[idx])
				processed.Add(1)
			}
		}()
	}

	// Send work
	for i := range items {
		itemChan <- i
	}
	close(itemChan)

	wg.Wait()
	close(done)

	return results
}

func processItem(cfg Config, item itemlist.ItemDef) Result {
	bmdPath := filepath.Join(cfg.ItemDir, item.ModelFile)
	if _, err := os.Stat(bmdPath); os.IsNotExist(err) {
		return Result{
			Name:    item.Name,
			Section: item.Section,
			Index:   item.Index,
			Error:   fmt.Sprintf("BMD not found: %s", item.ModelFile),
		}
	}

	meshes, bones, err := bmd.Parse(bmdPath)
	if err != nil {
		return Result{
			Name:    item.Name,
			Section: item.Section,
			Index:   item.Index,
			Error:   err.Error(),
		}
	}

	if len(meshes) == 0 {
		return Result{
			Name:    item.Name,
			Section: item.Section,
			Index:   item.Index,
			Error:   "No meshes in BMD",
		}
	}

	entry := cfg.TRSData[[2]int{item.Section, item.Index}]

	img := raster.RenderBMD(meshes, bones, entry, cfg.TexResolver, cfg.RenderSize, cfg.Supersample)

	// Post-processing: supersample downsample
	if cfg.Supersample > 1 {
		img = postprocess.Downsample(img, cfg.RenderSize)
	}

	// Remove small clusters
	img = postprocess.RemoveSmallClusters(img, 0.02)

	// Standardize (PCA rotation + scale + center)
	displayAngle := trs.DefaultDisplayAngle
	fillRatio := trs.DefaultFillRatio
	forceFlip := false
	if entry != nil {
		displayAngle = entry.DisplayAngle
		fillRatio = entry.FillRatio
		forceFlip = entry.Flip
	}
	img = postprocess.StandardizeImage(img, cfg.RenderSize, displayAngle, fillRatio, forceFlip)

	// Save as WebP
	outPath := filepath.Join(cfg.OutputDir, fmt.Sprintf("%d", item.Section), fmt.Sprintf("%d.webp", item.Index))
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return Result{
			Name:    item.Name,
			Section: item.Section,
			Index:   item.Index,
			Error:   err.Error(),
		}
	}

	f, err := os.Create(outPath)
	if err != nil {
		return Result{
			Name:    item.Name,
			Section: item.Section,
			Index:   item.Index,
			Error:   err.Error(),
		}
	}
	defer f.Close()

	if err := nativewebp.Encode(f, img, nil); err != nil {
		return Result{
			Name:    item.Name,
			Section: item.Section,
			Index:   item.Index,
			Error:   fmt.Sprintf("WebP encode: %v", err),
		}
	}

	return Result{
		Name:    item.Name,
		Section: item.Section,
		Index:   item.Index,
		Success: true,
	}
}
