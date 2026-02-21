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
	return e
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
		var c customTRSEntry
		if err := json.Unmarshal(rawEntry, &c); err != nil {
			continue
		}
		override := c.Override != nil && *c.Override
		entry := makeEntry(c)

		for _, item := range items {
			if item.Section != sec {
				continue
			}
			key := [2]int{sec, item.Index}
			if override || data[key] == nil {
				entryCopy := *entry
				data[key] = &entryCopy
			}
		}
	}

	// Model overrides (override sections and binary)
	for modelFile, rawEntry := range file.Models {
		var c customTRSEntry
		if err := json.Unmarshal(rawEntry, &c); err != nil {
			continue
		}
		entry := makeEntry(c)
		for _, item := range items {
			if strings.EqualFold(item.ModelFile, modelFile) {
				key := [2]int{item.Section, item.Index}
				entryCopy := *entry
				data[key] = &entryCopy
			}
		}
	}

	// Per-item overrides (always win)
	for keyStr, rawEntry := range file.Items {
		parts := strings.SplitN(keyStr, "_", 2)
		if len(parts) != 2 {
			continue
		}
		sec, err1 := strconv.Atoi(parts[0])
		idx, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			continue
		}
		var c customTRSEntry
		if err := json.Unmarshal(rawEntry, &c); err != nil {
			continue
		}
		data[[2]int{sec, idx}] = makeEntry(c)
	}
}
