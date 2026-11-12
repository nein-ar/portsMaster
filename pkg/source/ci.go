package source

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"portsMaster/pkg/cache"
	"portsMaster/pkg/config"
	"portsMaster/pkg/model"
)

type ciMeta struct {
	URL          string    `json:"url"`
	ETag         string    `json:"etag"`
	LastModified string    `json:"last_modified"`
	FetchedAt    time.Time `json:"fetched_at"`
}

// LoadCIStatus reads a local or remote CI status file. The remote path is
// conditional: an unchanged server response costs one 304 and no re-parse.
func LoadCIStatus(path string, cacheDir string) (map[string]*model.CIInfo, error) {
	if !config.IsRemote(path) {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		var data map[string]*model.CIInfo
		if err := json.NewDecoder(f).Decode(&data); err != nil {
			return nil, err
		}
		return data, nil
	}

	dir := filepath.Join(cacheDir, "ci")
	bodyPath := filepath.Join(dir, "status.json")
	metaPath := filepath.Join(dir, "meta.json")

	var meta ciMeta
	if data, err := os.ReadFile(metaPath); err == nil {
		_ = json.Unmarshal(data, &meta)
	}

	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if meta.URL == path {
		if meta.ETag != "" {
			req.Header.Set("If-None-Match", meta.ETag)
		}
		if meta.LastModified != "" {
			req.Header.Set("If-Modified-Since", meta.LastModified)
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return readStaleCI(bodyPath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified && meta.URL == path {
		return readStaleCI(bodyPath, nil)
	}
	if resp.StatusCode != http.StatusOK {
		return readStaleCI(bodyPath, fmt.Errorf("ci status: %s", resp.Status))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return readStaleCI(bodyPath, err)
	}
	var data map[string]*model.CIInfo
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return data, nil
	}
	_ = os.WriteFile(bodyPath, body, 0644)
	meta = ciMeta{
		URL:          path,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		FetchedAt:    time.Now(),
	}
	if metaData, err := json.Marshal(meta); err == nil {
		_ = os.WriteFile(metaPath, metaData, 0644)
	}
	return data, nil
}

func readStaleCI(bodyPath string, cause error) (map[string]*model.CIInfo, error) {
	body, err := os.ReadFile(bodyPath)
	if err != nil {
		if cause != nil {
			return nil, cause
		}
		return nil, err
	}
	var data map[string]*model.CIInfo
	if err := json.Unmarshal(body, &data); err != nil {
		if cause != nil {
			return nil, cause
		}
		return nil, err
	}
	return data, nil
}

// CachedCI loads and caches a local CI file by content hash. Local files are
// usually cheap, but generated JSON still benefits from skipping the decode.
func CachedCI(path string, objectCache *cache.ObjectCache) (map[string]*model.CIInfo, error) {
	if objectCache == nil || config.IsRemote(path) {
		return LoadCIStatus(path, "")
	}
	hash, err := cache.HashFile(path)
	if err != nil {
		return nil, err
	}
	var data map[string]*model.CIInfo
	if objectCache.Get("ci:"+path, hash, &data) {
		return data, nil
	}
	data, err = LoadCIStatus(path, "")
	if err != nil {
		return nil, err
	}
	_ = objectCache.Put("ci:"+path, hash, data)
	return data, nil
}
