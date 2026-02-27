package trs

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"mu-bmd-renderer/internal/crypto"
	"mu-bmd-renderer/internal/itemlist"
)

// Load reads ItemTRSData.bmd and merges custom_trs.json overrides.
func Load(bmdPath, customJSONPath, itemListXMLPath string) (Data, error) {
	data := make(Data)

	// Binary TRS
	if raw, err := os.ReadFile(bmdPath); err == nil && len(raw) >= 4 {
		count := binary.LittleEndian.Uint32(raw[:4])
		off := 4
		for i := 0; i < int(count); i++ {
			if off+32 > len(raw) {
				break
			}
			dec := crypto.DecryptTRS(raw[off : off+32])
			itemID := binary.LittleEndian.Uint32(dec[:4])
			section := int(itemID / 512)
			index := int(itemID % 512)

			entry := &Entry{
				PosX:         float64(math.Float32frombits(binary.LittleEndian.Uint32(dec[4:8]))),
				PosY:         float64(math.Float32frombits(binary.LittleEndian.Uint32(dec[8:12]))),
				PosZ:         float64(math.Float32frombits(binary.LittleEndian.Uint32(dec[12:16]))),
				RotX:         float64(math.Float32frombits(binary.LittleEndian.Uint32(dec[16:20]))),
				RotY:         float64(math.Float32frombits(binary.LittleEndian.Uint32(dec[20:24]))),
				RotZ:         float64(math.Float32frombits(binary.LittleEndian.Uint32(dec[24:28]))),
				Scale:        float64(math.Float32frombits(binary.LittleEndian.Uint32(dec[28:32]))),
				Source:       "binary",
				DisplayAngle: DefaultDisplayAngle,
				FillRatio:    DefaultFillRatio,
				FOV:          DefaultFOV,
			}
			data[[2]int{section, index}] = entry
			off += 32
		}
	}

	// Custom TRS overrides
	mergeCustomTRS(data, customJSONPath, itemListXMLPath)

	return data, nil
}

// customTRSFile matches the JSON schema of custom_trs.json.
type customTRSFile struct {
	Presets  map[string]json.RawMessage `json:"presets"`
	Sections map[string]json.RawMessage `json:"sections"`
	Models   map[string]json.RawMessage `json:"models"`
	Items    map[string]json.RawMessage `json:"items"`
}

type customTRSEntry struct {
	RotX         *float64 `json:"rotX"`
	RotY         *float64 `json:"rotY"`
	RotZ         *float64 `json:"rotZ"`
	Scale        *float64 `json:"scale"`
	Bones        *bool    `json:"bones"`
	Override     *bool    `json:"override"`
	Standardize  *bool    `json:"standardize"`
	DisplayAngle *float64 `json:"display_angle"`
	FillRatio    *float64 `json:"fill_ratio"`
	Flip         *bool    `json:"flip"`
	Camera       *string  `json:"camera"`
	Perspective    *bool    `json:"perspective"`
	FOV            *float64 `json:"fov"`
	KeepAllMeshes    *bool             `json:"keep_all_meshes"`
	FlipCanvas       *bool             `json:"flip_canvas"`
	MirrorPair       *bool             `json:"mirror_pair"`
	AdditiveTextures []string          `json:"additive_textures"`
	Merge            *bool             `json:"merge"`
}

// parseItemKeys parses "section_index" or "section_start-end" into key pairs.
func parseItemKeys(keyStr string) [][2]int {
	parts := strings.SplitN(keyStr, "_", 2)
	if len(parts) != 2 {
		return nil
	}
	sec, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil
	}
	// Range: "72-77"
	if rng := strings.SplitN(parts[1], "-", 2); len(rng) == 2 {
		start, err1 := strconv.Atoi(rng[0])
		end, err2 := strconv.Atoi(rng[1])
		if err1 != nil || err2 != nil || start > end {
			return nil
		}
		keys := make([][2]int, 0, end-start+1)
		for i := start; i <= end; i++ {
			keys = append(keys, [2]int{sec, i})
		}
		return keys
	}
	// Single: "72"
	idx, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil
	}
	return [][2]int{{sec, idx}}
}

func makeEntry(c customTRSEntry) *Entry {
	e := &Entry{
		Source:       "custom",
		DisplayAngle: DefaultDisplayAngle,
		FillRatio:    DefaultFillRatio,
		FOV:          DefaultFOV,
	}
	if c.RotX != nil {
		e.RotX = *c.RotX
	}
	if c.RotY != nil {
		e.RotY = *c.RotY
	}
	if c.RotZ != nil {
		e.RotZ = *c.RotZ
	}
	if c.Scale != nil {
		e.Scale = *c.Scale
	}
	if c.Bones != nil {
		e.UseBones = c.Bones
	}
	if c.Standardize != nil {
		e.Standardize = c.Standardize
	}
	if c.DisplayAngle != nil {
		e.DisplayAngle = *c.DisplayAngle
	}
	if c.FillRatio != nil {
		e.FillRatio = *c.FillRatio
	}
	if c.Flip != nil {
		e.Flip = *c.Flip
	}
	if c.Camera != nil {
		e.Camera = *c.Camera
	}
	if c.Perspective != nil {
		e.Perspective = *c.Perspective
	}
	if c.FOV != nil {
		e.FOV = *c.FOV
	}
	if c.KeepAllMeshes != nil {
		e.KeepAllMeshes = *c.KeepAllMeshes
	}
	if c.FlipCanvas != nil {
		e.FlipCanvas = *c.FlipCanvas
	}
	if c.MirrorPair != nil {
		e.MirrorPair = *c.MirrorPair
	}
	if len(c.AdditiveTextures) > 0 {
		e.AdditiveTextures = c.AdditiveTextures
	}
	return e
}

// mergeEntryFields merges only non-nil fields from customTRSEntry into an existing Entry.
// Used by section "merge" mode to override specific fields while keeping binary TRS values.
func mergeEntryFields(existing *Entry, c customTRSEntry) {
	if c.RotX != nil {
		existing.RotX = *c.RotX
	}
	if c.RotY != nil {
		existing.RotY = *c.RotY
	}
	if c.RotZ != nil {
		existing.RotZ = *c.RotZ
	}
	if c.Scale != nil {
		existing.Scale = *c.Scale
	}
	if c.Bones != nil {
		existing.UseBones = c.Bones
	}
	if c.Standardize != nil {
		existing.Standardize = c.Standardize
	}
	if c.DisplayAngle != nil {
		existing.DisplayAngle = *c.DisplayAngle
	}
	if c.FillRatio != nil {
		existing.FillRatio = *c.FillRatio
	}
	if c.Flip != nil {
		existing.Flip = *c.Flip
	}
	if c.Camera != nil {
		existing.Camera = *c.Camera
	}
	if c.Perspective != nil {
		existing.Perspective = *c.Perspective
	}
	if c.FOV != nil {
		existing.FOV = *c.FOV
	}
	if c.KeepAllMeshes != nil {
		existing.KeepAllMeshes = *c.KeepAllMeshes
	}
	if c.FlipCanvas != nil {
		existing.FlipCanvas = *c.FlipCanvas
	}
	if c.MirrorPair != nil {
		existing.MirrorPair = *c.MirrorPair
	}
	if len(c.AdditiveTextures) > 0 {
		existing.AdditiveTextures = c.AdditiveTextures
	}
}

// resolveEntry resolves a json.RawMessage that is either a preset name (string)
// or an inline config object into a customTRSEntry.
func resolveEntry(raw json.RawMessage, presets map[string]json.RawMessage) (*customTRSEntry, error) {
	// Try string first (preset reference)
	var name string
	if err := json.Unmarshal(raw, &name); err == nil {
		presetRaw, ok := presets[name]
		if !ok {
			return nil, fmt.Errorf("preset %q not found", name)
		}
		raw = presetRaw
	}
	var c customTRSEntry
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func mergeCustomTRS(data Data, jsonPath, xmlPath string) {
	raw, err := os.ReadFile(jsonPath)
	if err != nil {
		return
	}

	var file customTRSFile
	if err := json.Unmarshal(raw, &file); err != nil {
		fmt.Printf("Warning: custom_trs.json parse error: %v\n", err)
		return
	}

	// Parse itemlist once for sections and models lookups
	var items []itemlist.ItemDef
	if len(file.Sections) > 0 || len(file.Models) > 0 {
		items, _ = itemlist.Parse(xmlPath)
	}

	// Section defaults/overrides
	for secStr, rawEntry := range file.Sections {
		sec, err := strconv.Atoi(secStr)
		if err != nil {
			continue
		}
		c, err := resolveEntry(rawEntry, file.Presets)
		if err != nil {
			continue
		}
		override := c.Override != nil && *c.Override
		merge := c.Merge != nil && *c.Merge
		entry := makeEntry(*c)

		for _, item := range items {
			if item.Section != sec {
				continue
			}
			key := [2]int{sec, item.Index}
			existing := data[key]

			if existing == nil {
				// No binary TRS: create from section config
				entryCopy := *entry
				data[key] = &entryCopy
			} else if merge {
				// Merge: only override specified (non-nil) fields
				mergeEntryFields(existing, *c)
			} else if override {
				// Override: replace entirely
				entryCopy := *entry
				data[key] = &entryCopy
			}
		}
	}

	// Model overrides (override sections and binary)
	// Supports two formats:
	//   "model.bmd": "preset"           — single model → preset/inline config
	//   "preset": ["a.bmd", "b.bmd"]   — preset name → array of model files
	for key, rawEntry := range file.Models {
		// Try array format: key=preset, value=["model1.bmd", "model2.bmd"]
		var modelFiles []string
		if json.Unmarshal(rawEntry, &modelFiles) == nil && len(modelFiles) > 0 {
			presetRaw, ok := file.Presets[key]
			if !ok {
				continue
			}
			var c customTRSEntry
			if json.Unmarshal(presetRaw, &c) != nil {
				continue
			}
			entry := makeEntry(c)
			for _, mf := range modelFiles {
				for _, item := range items {
					if strings.EqualFold(item.ModelFile, mf) {
						entryCopy := *entry
						data[[2]int{item.Section, item.Index}] = &entryCopy
					}
				}
			}
			continue
		}

		// Normal format: key=model filename, value=preset or inline config
		c, err := resolveEntry(rawEntry, file.Presets)
		if err != nil {
			continue
		}
		entry := makeEntry(*c)
		for _, item := range items {
			if strings.EqualFold(item.ModelFile, key) {
				entryCopy := *entry
				data[[2]int{item.Section, item.Index}] = &entryCopy
			}
		}
	}

	// Per-item overrides (always win)
	// Keys support range syntax: "14_72-77" expands to 14_72, 14_73, ... 14_77
	for keyStr, rawEntry := range file.Items {
		keys := parseItemKeys(keyStr)
		if keys == nil {
			continue
		}
		c, err := resolveEntry(rawEntry, file.Presets)
		if err != nil {
			continue
		}
		for _, key := range keys {
			data[key] = makeEntry(*c)
		}
	}
}
