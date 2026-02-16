package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Config holds all configurable paths and render settings.
type Config struct {
	// Paths
	BaseDir     string `json:"base_dir"`
	ItemDir     string `json:"item_dir"`
	ItemListXML string `json:"item_list_xml"`
	TRSBMD      string `json:"trs_bmd"`
	CustomTRS   string `json:"custom_trs_json"`
	OutputDir   string `json:"output_dir"`

	// Render settings
	RenderSize  int `json:"render_size"`
	Supersample int `json:"supersample"`
	WebPQuality int `json:"webp_quality"`
	Workers     int `json:"workers"`
}

// Load reads a JSON config file and returns Config.
// Fields not set in the file keep their zero values.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("config: read %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("config: parse %s: %w", path, err)
	}

	return cfg, nil
}

// Resolve fills in any empty fields with auto-detected defaults.
// CLI flags take priority when non-zero/non-empty.
func (c *Config) Resolve(flags Flags) {
	// CLI flags override config file
	if flags.DataDir != "" {
		c.BaseDir = flags.DataDir
	}
	if flags.OutputDir != "" {
		c.OutputDir = flags.OutputDir
	}
	if flags.Quality > 0 {
		c.WebPQuality = flags.Quality
	}
	if flags.Workers > 0 {
		c.Workers = flags.Workers
	}

	// Auto-detect base dir if still empty
	if c.BaseDir == "" {
		c.BaseDir = detectBaseDir()
	}

	// Resolve relative paths against base dir
	if c.BaseDir != "" {
		if c.ItemDir == "" {
			c.ItemDir = filepath.Join(c.BaseDir, "Data", "Item")
		} else if !filepath.IsAbs(c.ItemDir) {
			c.ItemDir = filepath.Join(c.BaseDir, c.ItemDir)
		}

		if c.ItemListXML == "" {
			c.ItemListXML = findItemListXML(c.BaseDir)
		} else if !filepath.IsAbs(c.ItemListXML) {
			c.ItemListXML = filepath.Join(c.BaseDir, c.ItemListXML)
		}

		if c.TRSBMD == "" {
			c.TRSBMD = filepath.Join(c.BaseDir, "Data", "Local", "itemtrsdata.bmd")
		} else if !filepath.IsAbs(c.TRSBMD) {
			c.TRSBMD = filepath.Join(c.BaseDir, c.TRSBMD)
		}

		if c.CustomTRS == "" {
			c.CustomTRS = filepath.Join(c.BaseDir, "custom_trs.json")
		} else if !filepath.IsAbs(c.CustomTRS) {
			c.CustomTRS = filepath.Join(c.BaseDir, c.CustomTRS)
		}

		if c.OutputDir == "" {
			c.OutputDir = filepath.Join(c.BaseDir, "Data", "Item-renders")
		} else if !filepath.IsAbs(c.OutputDir) {
			c.OutputDir = filepath.Join(c.BaseDir, c.OutputDir)
		}
	}

	// Defaults for render settings
	if c.RenderSize <= 0 {
		c.RenderSize = 256
	}
	if c.Supersample <= 0 {
		c.Supersample = 2
	}
	if c.WebPQuality <= 0 {
		c.WebPQuality = 90
	}
	if c.Workers <= 0 {
		c.Workers = runtime.NumCPU()
	}
}

// Flags holds CLI flag values that override config file settings.
type Flags struct {
	DataDir   string
	OutputDir string
	Quality   int
	Workers   int
}

func detectBaseDir() string {
	// Try relative to executable
	exe, _ := os.Executable()
	if exe != "" {
		dir := filepath.Dir(exe)
		for _, base := range []string{dir, filepath.Dir(dir), filepath.Join(dir, "..", "..")} {
			if _, err := os.Stat(filepath.Join(base, "Data", "Item")); err == nil {
				return base
			}
		}
	}

	// Try current working directory
	cwd, _ := os.Getwd()
	if _, err := os.Stat(filepath.Join(cwd, "Data", "Item")); err == nil {
		return cwd
	}

	// Try parent of cwd (if we're in mu-bmd-renderer/)
	parent := filepath.Dir(cwd)
	if _, err := os.Stat(filepath.Join(parent, "Data", "Item")); err == nil {
		return parent
	}

	return ""
}

func findItemListXML(baseDir string) string {
	candidates := []string{
		filepath.Join(baseDir, "ItemList.xml"),
		filepath.Join(baseDir, "Data", "Xml", "ItemList.xml"),
		filepath.Join(baseDir, "Data", "xml", "ItemList.xml"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return candidates[0]
}
