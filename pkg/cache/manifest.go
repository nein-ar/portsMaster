package cache

import (
	"encoding/json"
	"os"
	"sync"
)

const ManifestVersion = "v2" // Incremented when cache format or logic changes

type Manifest struct {
	Version string            `json:"version"`
	Hashes  map[string]string `json:"hashes"`
	mu      sync.RWMutex
}

func LoadManifest(path string) *Manifest {
	m := &Manifest{
		Version: ManifestVersion,
		Hashes:  make(map[string]string),
	}
	f, err := os.Open(path)
	if err == nil {
		defer f.Close()
		var loaded Manifest
		if err := json.NewDecoder(f).Decode(&loaded); err == nil {
			if loaded.Version == ManifestVersion {
				m.Hashes = loaded.Hashes
			} else {
				// Version mismatch, start fresh
				m.Hashes = make(map[string]string)
			}
		}
	}
	return m
}

func (m *Manifest) Save(path string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}

func (m *Manifest) HasChanged(path, hash string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Hashes[path] != hash
}

func (m *Manifest) Update(path, hash string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Hashes[path] = hash
}
