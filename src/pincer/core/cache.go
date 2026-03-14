package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Cache provides simple file-based caching for selector data.
type Cache struct {
	dir string
}

// NewCache creates a cache rooted at ~/.pincer/state/<namespace>/.
func NewCache(namespace string) (*Cache, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".pincer", "state", namespace)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Cache{dir: dir}, nil
}

type cacheEntry struct {
	Value     json.RawMessage `json:"value"`
	ExpiresAt time.Time       `json:"expires_at"`
}

// Get retrieves a cached value. Returns nil if not found or expired.
func (c *Cache) Get(key string) json.RawMessage {
	data, err := os.ReadFile(filepath.Join(c.dir, key+".json"))
	if err != nil {
		return nil
	}
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil
	}
	if time.Now().After(entry.ExpiresAt) {
		_ = c.Delete(key)
		return nil
	}
	return entry.Value
}

// Set stores a value with a TTL.
func (c *Cache) Set(key string, value any, ttl time.Duration) error {
	v, err := json.Marshal(value)
	if err != nil {
		return err
	}
	entry := cacheEntry{
		Value:     v,
		ExpiresAt: time.Now().Add(ttl),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.dir, key+".json"), data, 0o644)
}

// Delete removes a cached value.
func (c *Cache) Delete(key string) error {
	return os.Remove(filepath.Join(c.dir, key+".json"))
}

// Clear removes all cached values.
func (c *Cache) Clear() error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		_ = os.Remove(filepath.Join(c.dir, e.Name()))
	}
	return nil
}
