package batch

import (
	"encoding/json"
	"fmt"
	"os"

	"mu-bmd-renderer/internal/itemlist"
)

// ManifestEntry represents one item in the output manifest.
type ManifestEntry struct {
	Section     int    `json:"section"`
	SectionName string `json:"section_name"`
	Index       int    `json:"index"`
	Name        string `json:"name"`
	ModelFile   string `json:"model_file"`
	Image       string `json:"image"`
}

// WriteManifest writes manifest.json to the output directory.
func WriteManifest(path string, items []itemlist.ItemDef) error {
	entries := make([]ManifestEntry, len(items))
	for i, it := range items {
		entries[i] = ManifestEntry{
			Section:     it.Section,
			SectionName: it.SectionName,
			Index:       it.Index,
			Name:        it.Name,
			ModelFile:   it.ModelFile,
			Image:       fmt.Sprintf("%d/%d.webp", it.Section, it.Index),
		}
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
