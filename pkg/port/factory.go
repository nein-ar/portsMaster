package port

import (
	"fmt"
	"portsMaster/pkg/config"
	"portsMaster/pkg/model"
	"portsMaster/pkg/port/spc"
	"portsMaster/pkg/registry"
)

// Scanner factory
//
func NewScanner(cfg *config.Config, reg *registry.Registry) (model.Scanner, error) {
	switch cfg.PackageManager {
	case "spc":
		parser := spc.NewParser(reg)
		return spc.NewScanner(reg, parser), nil
	default:
		return nil, fmt.Errorf("unsupported package manager: %s", cfg.PackageManager)
	}
}

// Parser factory
//
func NewParser(cfg *config.Config, reg *registry.Registry) (model.Parser, error) {
	switch cfg.PackageManager {
	case "spc":
		return spc.NewParser(reg), nil
	default:
		return nil, fmt.Errorf("unsupported package manager: %s", cfg.PackageManager)
	}
}
