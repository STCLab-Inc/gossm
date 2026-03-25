package tui

import (
	"sync"
	"time"
)

// dirCache caches remote directory listings to avoid repeated SSM calls.
type dirCache struct {
	mu      sync.RWMutex
	entries map[string]cachedDir
	ttl     time.Duration
}

type cachedDir struct {
	entries   []FileEntry
	fetchedAt time.Time
}

func newDirCache(ttl time.Duration) *dirCache {
	return &dirCache{
		entries: make(map[string]cachedDir),
		ttl:     ttl,
	}
}

// Get returns cached entries if fresh, or nil if stale/missing.
func (c *dirCache) Get(path string) []FileEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cached, ok := c.entries[path]
	if !ok {
		return nil
	}
	if time.Since(cached.fetchedAt) > c.ttl {
		return nil
	}
	return cached.entries
}

// Set stores directory entries in the cache.
func (c *dirCache) Set(path string, entries []FileEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[path] = cachedDir{entries: entries, fetchedAt: time.Now()}
}

// Invalidate removes a specific path from cache (e.g., after transfer).
func (c *dirCache) Invalidate(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, path)
}

// InvalidateAll clears entire cache.
func (c *dirCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]cachedDir)
}
