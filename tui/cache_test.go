package tui

import (
	"testing"
	"time"
)

func TestDirCache(t *testing.T) {
	cache := newDirCache(100 * time.Millisecond)

	// Empty cache returns nil
	if entries := cache.Get("/test"); entries != nil {
		t.Fatal("expected nil for empty cache")
	}

	// Set and get
	testEntries := []FileEntry{{Name: "file1"}, {Name: "dir1", IsDir: true}}
	cache.Set("/test", testEntries)

	got := cache.Get("/test")
	if got == nil {
		t.Fatal("expected cached entries")
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}

	// TTL expiry
	time.Sleep(150 * time.Millisecond)
	if entries := cache.Get("/test"); entries != nil {
		t.Fatal("expected nil after TTL expiry")
	}

	// Invalidate
	cache.Set("/test2", testEntries)
	cache.Invalidate("/test2")
	if entries := cache.Get("/test2"); entries != nil {
		t.Fatal("expected nil after invalidate")
	}

	// InvalidateAll
	cache.Set("/a", testEntries)
	cache.Set("/b", testEntries)
	cache.InvalidateAll()
	if cache.Get("/a") != nil || cache.Get("/b") != nil {
		t.Fatal("expected nil after invalidateAll")
	}
}
