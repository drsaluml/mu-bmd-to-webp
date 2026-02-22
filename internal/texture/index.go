package texture

import (
	"os"
	"path/filepath"
	"strings"
)

// texEntry stores both OZJ and OZT paths for a texture stem.
type texEntry struct {
	ozj string // OZJ (JPEG, no alpha) path
	ozt string // OZT (TGA, has alpha) path
}

// Index maps lowercase texture stems to filesystem paths.
// When both OZJ and OZT exist for a stem, the caller's requested extension
// determines which to use: .jpg/.jpeg requests prefer OZJ, .tga requests prefer OZT.
type Index struct {
	entries map[string]*texEntry // stem.lower() → paths
}

// BuildIndex scans itemDir/texture/ and subdirectories for OZJ/OZT files.
func BuildIndex(itemDir string) *Index {
	idx := &Index{entries: make(map[string]*texEntry)}

	// Scan main texture dir and all subdirectory texture dirs
	searchDirs := []string{filepath.Join(itemDir, "texture")}

	// Also scan subdirectory textures (e.g., Jewel/Texture, partCharge1/Texture)
	entries, _ := os.ReadDir(itemDir)
	for _, e := range entries {
		if e.IsDir() {
			subTex := filepath.Join(itemDir, e.Name(), "Texture")
			if info, err := os.Stat(subTex); err == nil && info.IsDir() {
				searchDirs = append(searchDirs, subTex)
			}
			subTexLower := filepath.Join(itemDir, e.Name(), "texture")
			if subTexLower != subTex {
				if info, err := os.Stat(subTexLower); err == nil && info.IsDir() {
					searchDirs = append(searchDirs, subTexLower)
				}
			}
		}
	}

	for _, dir := range searchDirs {
		filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".ozj" && ext != ".ozt" {
				return nil
			}
			stem := strings.ToLower(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))

			entry := idx.entries[stem]
			if entry == nil {
				entry = &texEntry{}
				idx.entries[stem] = entry
			}
			if ext == ".ozj" && entry.ozj == "" {
				entry.ozj = path
			} else if ext == ".ozt" && entry.ozt == "" {
				entry.ozt = path
			}
			return nil
		})
	}

	return idx
}

// ResolvePath returns the filesystem path for a texture name, or ("", false).
// When both OZJ and OZT exist for the same stem, the requested extension
// determines priority: .jpg/.jpeg → OZJ first, .tga → OZT first.
// This ensures JPEG-referencing meshes get opaque textures and TGA-referencing
// meshes get alpha-channel textures, matching the model designer's intent.
func (idx *Index) ResolvePath(texName string) (string, bool) {
	// Strip path prefix (e.g., "Monsters\\texture\\foo.jpg" → "foo")
	texName = strings.ReplaceAll(texName, "\\", "/")
	base := filepath.Base(texName)
	reqExt := strings.ToLower(filepath.Ext(base))
	stem := strings.ToLower(strings.TrimSuffix(base, filepath.Ext(base)))

	entry, ok := idx.entries[stem]
	if !ok {
		return "", false
	}

	// Choose based on requested extension
	switch reqExt {
	case ".jpg", ".jpeg":
		if entry.ozj != "" {
			return entry.ozj, true
		}
		if entry.ozt != "" {
			return entry.ozt, true
		}
	case ".tga":
		if entry.ozt != "" {
			return entry.ozt, true
		}
		if entry.ozj != "" {
			return entry.ozj, true
		}
	default:
		// Unknown extension: prefer OZT (has alpha)
		if entry.ozt != "" {
			return entry.ozt, true
		}
		if entry.ozj != "" {
			return entry.ozj, true
		}
	}

	return "", false
}

// Len returns the number of indexed textures.
func (idx *Index) Len() int {
	return len(idx.entries)
}
