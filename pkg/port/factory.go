package port

import (
	"fmt"
	"portsMaster/pkg/cache"
	"portsMaster/pkg/config"
	"portsMaster/pkg/model"
	"portsMaster/pkg/port/spc"
	"portsMaster/pkg/registry"
)

// Scanner factory
func NewScanner(cfg *config.Config, reg *registry.Registry, cache *cache.ObjectCache) (model.Scanner, error) {
	switch cfg.PackageManager {
	case "spc":
		parser := spc.NewParser(reg)
		if cache != nil {
			parser = spc.NewCachedParser(reg, cache)
		}
		return spc.NewScanner(reg, parser), nil
	default:
		return nil, fmt.Errorf("unsupported package manager: %s", cfg.PackageManager)
	}
}

// Parser factory
func NewParser(cfg *config.Config, reg *registry.Registry, cache *cache.ObjectCache) (model.Parser, error) {
	switch cfg.PackageManager {
	case "spc":
		if cache != nil {
			return spc.NewCachedParser(reg, cache), nil
		}
		return spc.NewParser(reg), nil
	default:
		return nil, fmt.Errorf("unsupported package manager: %s", cfg.PackageManager)
	}
}
