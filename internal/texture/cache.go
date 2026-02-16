package texture

import (
	"image"
	"sync"
)

// Resolver resolves a texture name to a decoded RGBA image.
type Resolver interface {
	Resolve(texName string) *image.NRGBA
}

// Cache is a concurrency-safe texture cache.
type Cache struct {
	mu    sync.RWMutex
	items map[string]*cacheEntry
	index *Index
}

type cacheEntry struct {
	img    *image.NRGBA
	loaded bool // true if we've attempted to load (img may still be nil)
}

// NewCache creates a new texture cache backed by the given index.
func NewCache(index *Index) *Cache {
	return &Cache{
		items: make(map[string]*cacheEntry),
		index: index,
	}
}

// Resolve loads and caches a texture by name. Returns nil if not found.
func (c *Cache) Resolve(texName string) *image.NRGBA {
	path, ok := c.index.ResolvePath(texName)
	if !ok {
		return nil
	}

	// Fast path: read lock
	c.mu.RLock()
	if entry, exists := c.items[path]; exists {
		c.mu.RUnlock()
		return entry.img
	}
	c.mu.RUnlock()

	// Slow path: load from disk
	img, _ := LoadTexture(path)

	// Write lock with double-check
	c.mu.Lock()
	if entry, exists := c.items[path]; exists {
		c.mu.Unlock()
		return entry.img
	}
	c.items[path] = &cacheEntry{img: img, loaded: true}
	c.mu.Unlock()

	return img
}
