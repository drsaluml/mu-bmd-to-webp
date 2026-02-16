package texture

import (
	"os"
	"path/filepath"
	"strings"
)

// Index maps lowercase texture stems to filesystem paths.
// OZT files take priority over OZJ for the same stem (alpha channel).
type Index struct {
	entries map[string]string // stem.lower() → full path
}

// BuildIndex scans itemDir/texture/ and subdirectories for OZJ/OZT files.
func BuildIndex(itemDir string) *Index {
	idx := &Index{entries: make(map[string]string)}

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

			existing, exists := idx.entries[stem]
			if !exists {
				idx.entries[stem] = path
			} else if ext == ".ozt" && strings.ToLower(filepath.Ext(existing)) == ".ozj" {
				// OZT wins over OZJ (has alpha channel)
				idx.entries[stem] = path
			}
			return nil
		})
	}

	return idx
}

// ResolvePath returns the filesystem path for a texture name, or ("", false).
func (idx *Index) ResolvePath(texName string) (string, bool) {
	// Strip path prefix (e.g., "Monsters\\texture\\foo.jpg" → "foo")
	texName = strings.ReplaceAll(texName, "\\", "/")
	base := filepath.Base(texName)
	stem := strings.ToLower(strings.TrimSuffix(base, filepath.Ext(base)))

	path, ok := idx.entries[stem]
	return path, ok
}

// Len returns the number of indexed textures.
func (idx *Index) Len() int {
	return len(idx.entries)
}
