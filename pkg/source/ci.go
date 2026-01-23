package source

import (
	"encoding/json"
	"os"
	"portsMaster/pkg/model"
)

// LoadCIStatus reads a JSON file mapping "category/name" to CIInfo.
func LoadCIStatus(path string) (map[string]*model.CIInfo, error) {
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
