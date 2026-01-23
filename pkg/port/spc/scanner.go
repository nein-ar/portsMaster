package spc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"portsMaster/pkg/model"
	"portsMaster/pkg/registry"
)

// Scanner implements model.Scanner for the SPC format.
type Scanner struct {
	reg    *registry.Registry
	parser model.Parser
}

// NewScanner creates a new SPC scanner.
func NewScanner(reg *registry.Registry, parser model.Parser) *Scanner {
	return &Scanner{reg: reg, parser: parser}
}

// Type returns the scanner format type.
func (s *Scanner) Type() string {
	return "spc"
}

// Scan traverses the ports directory to discover categories and ports.
func (s *Scanner) Scan(ctx context.Context) ([]*model.Category, []*model.Port, error) {
	entries, err := os.ReadDir(s.reg.PortsRoot())
	if err != nil {
		return nil, nil, err
	}

	var (
		categories []*model.Category
		allPorts   []*model.Port
		mu         sync.Mutex
		wg         sync.WaitGroup
	)

	for _, e := range entries {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}

		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || e.Name() == "bundles" {
			continue
		}

		cat := &model.Category{Name: e.Name()}
		mu.Lock()
		categories = append(categories, cat)
		mu.Unlock()

		portEntries, err := os.ReadDir(filepath.Join(s.reg.PortsRoot(), cat.Name))
		if err != nil {
			continue
		}

		for _, pe := range portEntries {
			if !pe.IsDir() {
				continue
			}

			wg.Add(1)
			go func(catName, portName string) {
				defer wg.Done()
				p, err := s.parser.Parse(catName, portName)
				if err != nil {
					return
				}

				// Check for BROKEN file override
				if _, err := os.Stat(filepath.Join(p.FilePath, "BROKEN")); err == nil {
					p.IsBroken = true
				}

				mu.Lock()
				cat.Ports = append(cat.Ports, p)
				allPorts = append(allPorts, p)
				mu.Unlock()
			}(cat.Name, pe.Name())
		}
	}
	wg.Wait()

	return filterEmpty(categories), allPorts, nil
}

func filterEmpty(cats []*model.Category) []*model.Category {
	var filtered []*model.Category
	for _, c := range cats {
		if len(c.Ports) > 0 {
			filtered = append(filtered, c)
		}
	}
	return filtered
}
