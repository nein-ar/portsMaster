package source

import (
	"os"
	"path/filepath"
	"strings"

	"portsMaster/pkg/model"
	"portsMaster/pkg/registry"
)

// ScanPackages walks the Registry's package root and augments ports with binary package info.
// Structure: category/package/package.spc.<fmt>
func ScanPackages(reg *registry.Registry, ports []*model.Port) error {
	pkgsRoot := reg.PkgsRoot()
	if pkgsRoot == "" {
		return nil
	}

	// Check if PkgsRoot is remote (starts with http). If so, we can't scan it with os.ReadDir.
	if len(pkgsRoot) > 4 && (pkgsRoot[:4] == "http" || pkgsRoot[:4] == "ftp:") {
		return nil
	}

	portMap := make(map[string]*model.Port, len(ports))
	for _, p := range ports {
		portMap[p.Category+"/"+p.Name] = p
	}

	entries, err := os.ReadDir(pkgsRoot)
	if err != nil {
		return err
	}

	for _, catEntry := range entries {
		if !catEntry.IsDir() {
			continue
		}
		catName := catEntry.Name()
		catDir := filepath.Join(pkgsRoot, catName)

		pkgEntries, err := os.ReadDir(catDir)
		if err != nil {
			continue
		}

		for _, pkgEntry := range pkgEntries {
			if !pkgEntry.IsDir() {
				continue
			}
			pkgName := pkgEntry.Name()

			portKey := catName + "/" + pkgName
			p, exists := portMap[portKey]
			if !exists {
				continue
			}

			pkgDir := filepath.Join(pkgsRoot, catName, pkgName)
			files, err := os.ReadDir(pkgDir)
			if err != nil {
				continue
			}

			for _, f := range files {
				if f.IsDir() {
					continue
				}
				name := f.Name()
				if strings.Contains(name, ".spc.") {
					info := model.PackageInfo{
						Filename: name,
						Path:     filepath.Join(pkgDir, name),
						Size:     0,
					}
					if stat, err := os.Stat(info.Path); err == nil {
						info.Size = stat.Size()
					}
					p.Packages = append(p.Packages, info)
				}
			}
		}
	}
	return nil
}
