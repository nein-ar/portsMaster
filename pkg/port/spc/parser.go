package spc

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"portsMaster/pkg/model"
	"portsMaster/pkg/registry"
	"portsMaster/pkg/util"
	"strings"

	"lukechampine.com/blake3"
)

type Parser struct {
	reg *registry.Registry
}

func NewParser(reg *registry.Registry) *Parser {
	return &Parser{reg: reg}
}

func (pr *Parser) PortDir(category, name string) string {
	return filepath.Join(pr.reg.PortsRoot(), category, name)
}

func (pr *Parser) PortInfoFile(category, name string) string {
	return filepath.Join(pr.PortDir(category, name), "info")
}

func (pr *Parser) PortDepsFile(category, name string) string {
	return filepath.Join(pr.PortDir(category, name), "deps")
}

func (pr *Parser) Type() string {
	return "spc"
}

func (pr *Parser) Parse(category, name string) (*model.Port, error) {
	path := pr.PortDir(category, name)

	p := &model.Port{
		Name:     name,
		Category: category,
		FilePath: path,
	}

	hash, err := calculateDirHash(path)
	if err != nil {
		return nil, err
	}
	p.Hash = hash

	if err := parseInfoFile(p, pr.PortInfoFile(category, name)); err != nil {
		return nil, err
	}

	if err := parseDepsFile(p, pr.PortDepsFile(category, name)); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	p.Upstream = ExpandVariables(p.Upstream, map[string]string{
		"VERSION":     p.Version,
		"NAME":        p.Name,
		"COMMIT":      p.Version, // Often commit is the version
		"RELEASE":     p.Release,
		"RELEASE_TAG": p.Release,
	})

	recipePath := filepath.Join(path, "ndmake.sh")
	if lines, err := countLines(recipePath); err == nil {
		p.RecipeLines = lines
	}

	return p, nil
}

func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	return count, scanner.Err()
}

// ExpandVariables replaces ${VAR} in text with values from vars.
func ExpandVariables(text string, vars map[string]string) string {
	for k, v := range vars {
		text = strings.ReplaceAll(text, "${"+k+"}", v)
	}
	return text
}

func parseInfoFile(p *model.Port, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "version":
			p.Version = val
		case "release":
			p.Release = val
		case "description":
			p.Description = val
		case "license":
			p.License = val
		case "upstream":
			p.Upstream = val
		case "maintainer":
			val = strings.TrimSpace(val)
			if val == "-" {
				p.IsUnmaintained = true
				p.Maintainer = ""
			} else {
				p.Maintainer = util.StripMarkdownLinks(val)
			}
		case "provides":
			vals := strings.Split(val, ",")
			for _, v := range vals {
				p.Provides = append(p.Provides, strings.TrimSpace(v))
			}
		}
	}
	return scanner.Err()
}

func parseDepsFile(p *model.Port, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		var dep model.Dependency

		switch {
		case strings.HasPrefix(line, "."):
			dep.Type = model.DepBuild
			dep.Name = line[1:]
		case strings.HasPrefix(line, ">"):
			dep.Type = model.DepRun
			dep.Name = line[1:]
		case strings.HasPrefix(line, "/"):
			dep.Type = model.DepLink
			dep.Name = line[1:]
		default:
			dep.Type = model.DepLink
			dep.Name = line
		}

		dep.Name = strings.TrimSpace(dep.Name)
		p.Deps = append(p.Deps, dep)
	}
	return scanner.Err()
}

func calculateDirHash(dir string) (string, error) {
	h := blake3.New(32, nil)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		fmt.Fprintf(h, "%s|%d|%d;", rel, info.Size(), info.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
