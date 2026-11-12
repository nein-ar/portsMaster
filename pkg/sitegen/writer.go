package sitegen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"portsMaster/pkg/cache"
)

// Writer writes generated source files only when their bytes changed. It also
// removes files from the previous run that are no longer produced, so switching
// a theme or deleting a port never leaves stale sources behind.
type Writer struct {
	root     string
	manifest string
	mu       sync.Mutex
	old      map[string]string
	next     map[string]string
}

type writerManifest struct {
	Files map[string]string `json:"files"`
}

func NewWriter(root, manifest string) *Writer {
	w := &Writer{root: root, manifest: manifest, next: make(map[string]string)}
	if data, err := os.ReadFile(manifest); err == nil {
		var m writerManifest
		if json.Unmarshal(data, &m) == nil && m.Files != nil {
			w.old = m.Files
		}
	}
	if w.old == nil {
		w.old = make(map[string]string)
	}
	return w
}

func (w *Writer) Write(rel string, data []byte) error {
	rel = filepath.ToSlash(filepath.Clean(rel))
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || strings.HasPrefix(rel, "../") {
		return nil
	}
	sum := cache.HashBytes(data)

	w.mu.Lock()
	w.next[rel] = sum
	old := w.old[rel]
	w.mu.Unlock()

	full := filepath.Join(w.root, filepath.FromSlash(rel))
	if old == sum {
		if _, err := os.Stat(full); err == nil {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	tmp := full + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, full)
}

func (w *Writer) Finish() (int, error) {
	removed := 0
	for rel := range w.old {
		if _, ok := w.next[rel]; ok {
			continue
		}
		if err := os.Remove(filepath.Join(w.root, filepath.FromSlash(rel))); err == nil {
			removed++
		}
	}
	if err := os.MkdirAll(filepath.Dir(w.manifest), 0755); err != nil {
		return removed, err
	}
	data, err := json.MarshalIndent(writerManifest{Files: w.next}, "", "  ")
	if err != nil {
		return removed, err
	}
	if err := os.WriteFile(w.manifest+".tmp", append(data, '\n'), 0644); err != nil {
		return removed, err
	}
	if err := os.Rename(w.manifest+".tmp", w.manifest); err != nil {
		return removed, err
	}
	pruneEmpty(w.root)
	return removed, nil
}

func pruneEmpty(root string) {
	var dirs []string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() && p != root {
			dirs = append(dirs, p)
		}
		return nil
	})
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err == nil && len(entries) == 0 {
			_ = os.Remove(dir)
		}
	}
}
