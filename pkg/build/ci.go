package build

import (
	"encoding/json"
	"os"
	"path/filepath"

	"portsMaster/pkg/model"
)

func (e *Engine) GenerateCIData(ports []*model.Port) error {
	ciData := make(map[string]*model.CIInfo)

	for _, p := range ports {
		path := filepath.Join(e.cfg.OutDir, "ports", p.Category, p.Name, "index.html")
		status := "success"
		if _, err := os.Stat(path); os.IsNotExist(err) {
			status = "failed"
		} else if p.IsBroken {
			status = "broken"
		}

		ciData[p.Category+"/"+p.Name] = &model.CIInfo{
			Status: status,
		}
	}

	f, err := os.Create(filepath.Join(e.cfg.OutDir, "ci_status.json"))
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(ciData)
}
