package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ObjectCache stores derived JSON values keyed by a path and invalidated by a
// content hash. It is deliberately generic: parsed ports, git history and CI
// payloads use the same mechanism instead of each growing its own format.
type ObjectCache struct {
	dir  string
	mu   sync.Mutex
	used map[string]bool
}

type objectEntry struct {
	Key   string          `json:"key"`
	Hash  string          `json:"hash"`
	Value json.RawMessage `json:"value"`
}

func NewObjectCache(dir string) *ObjectCache {
	return &ObjectCache{dir: dir, used: make(map[string]bool)}
}

func (c *ObjectCache) filename(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(c.dir, hex.EncodeToString(sum[:])+".json")
}

// Get decodes a cached value when its hash matches. A miss, a hash change or
// unreadable data all mean "not cached" rather than an error.
func (c *ObjectCache) Get(key, hash string, value any) bool {
	path := c.filename(key)
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var entry objectEntry
	if err := json.Unmarshal(data, &entry); err != nil || entry.Hash != hash || entry.Key != key {
		return false
	}
	if err := json.Unmarshal(entry.Value, value); err != nil {
		return false
	}
	c.mark(key)
	return true
}

func (c *ObjectCache) Put(key, hash string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	entry := objectEntry{Key: key, Hash: hash, Value: raw}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(c.dir, 0755); err != nil {
		return err
	}
	path := c.filename(key)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	c.mark(key)
	return nil
}

func (c *ObjectCache) mark(key string) {
	c.mu.Lock()
	c.used[key] = true
	c.mu.Unlock()
}

// GC removes objects that were not read or written during this run. It also
// removes temporary files left behind by an interrupted process.
func (c *ObjectCache) GC() error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	c.mu.Lock()
	usedFiles := make(map[string]bool, len(c.used))
	for key := range c.used {
		usedFiles[filepath.Base(c.filename(key))] = true
	}
	c.mu.Unlock()

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".tmp") {
			_ = os.Remove(filepath.Join(c.dir, name))
			continue
		}
		if strings.HasSuffix(name, ".json") && !usedFiles[name] {
			_ = os.Remove(filepath.Join(c.dir, name))
		}
	}
	return nil
}
