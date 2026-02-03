package source

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"portsMaster/pkg/model"
)

// LoadCIStatus reads a JSON file mapping "category/name" to CIInfo.
func LoadCIStatus(path string) (map[string]*model.CIInfo, error) {
	var rc io.ReadCloser
	if len(path) > 4 && path[:4] == "http" {
		resp, err := http.Get(path)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to fetch CI status: %s", resp.Status)
		}
		rc = resp.Body
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		rc = f
	}
	defer rc.Close()

	var data map[string]*model.CIInfo
	if err := json.NewDecoder(rc).Decode(&data); err != nil {
		return nil, err
	}
	return data, nil
}
