package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const ManifestVersion = "v2"

// Manifest
//
type Manifest struct {
	Version string            `json:"version"`
	Hashes  map[string]string `json:"hashes"`
	mu      sync.RWMutex
	used    map[string]bool
}

// Manifest loading
//
func LoadManifest(path string) *Manifest {
	m := &Manifest{
		Version: ManifestVersion,
		Hashes:  make(map[string]string),
		used:    make(map[string]bool),
	}
	f, err := os.Open(path)
	if err == nil {
		defer f.Close()
		var loaded Manifest
		if err := json.NewDecoder(f).Decode(&loaded); err == nil {
			if loaded.Version == ManifestVersion {
				m.Hashes = loaded.Hashes
			} else {
				// Start fresh on version mismatch
				//
				m.Hashes = make(map[string]string)
			}
		}
	}
	return m
}

// Manifest persistence
//
func (m *Manifest) Save(path string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}

// Change detection
//
func (m *Manifest) HasChanged(path, hash string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Hashes[path] != hash
}

// Manifest update
//
func (m *Manifest) Update(path, hash string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Hashes[path] = hash
	if m.used == nil {
		m.used = make(map[string]bool)
	}
	m.used[path] = true
}

// Usage tracking
//
func (m *Manifest) MarkUsed(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.used == nil {
		m.used = make(map[string]bool)
	}
	m.used[path] = true
}

// Usage check
//
func (m *Manifest) IsUsed(path string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.used[path]
}
