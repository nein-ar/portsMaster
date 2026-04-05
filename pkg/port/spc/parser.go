package spc

import (
	"bufio"
	"encoding/base64"
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

// Parser
//
type Parser struct {
	reg *registry.Registry
}

func NewParser(reg *registry.Registry) *Parser {
	return &Parser{reg: reg}
}

// Port directory
//
func (pr *Parser) PortDir(category, name string) string {
	return filepath.Join(pr.reg.PortsRoot(), category, name)
}

// Port info file
//
func (pr *Parser) PortInfoFile(category, name string) string {
	return filepath.Join(pr.PortDir(category, name), "info")
}

// Port deps file
//
func (pr *Parser) PortDepsFile(category, name string) string {
	return filepath.Join(pr.PortDir(category, name), "deps")
}

// Port type
//
func (pr *Parser) Type() string {
	return "spc"
}

// Port parsing
//
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
		"COMMIT":      p.Version, 
		"RELEASE":     p.Release,
		"RELEASE_TAG": p.Release,
	})

	recipePath := filepath.Join(path, "ndmake.sh")
	if lines, err := countLines(recipePath); err == nil {
		p.RecipeLines = lines
	}

	files, _ := filepath.Glob(filepath.Join(path, "*"))
	p.FileContents = make(map[string]string)
	for _, f := range files {
		info, err := os.Stat(f)
		if err == nil && !info.IsDir() {
			name := filepath.Base(f)
			p.Files = append(p.Files, name)

			// File embedding
			//
			if info.Size() < 5*1024*1024 {
				content, err := os.ReadFile(f)
				if err == nil {
					if IsImage(name) {
						ext := strings.TrimPrefix(filepath.Ext(name), ".")
						if ext == "jpg" { ext = "jpeg" }
						encoded := base64.StdEncoding.EncodeToString(content)
						p.FileContents[name] = fmt.Sprintf("data:image/%s;base64,%s", ext, encoded)
					} else {
						p.FileContents[name] = string(content)
					}
				}
			}
		}
	}

	return p, nil
}

// Image check
//
func IsImage(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".svg", ".webp":
		return true
	default:
		return false
	}
}

// Line count
//
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

// Variable expansion
//
func ExpandVariables(text string, vars map[string]string) string {
	for k, v := range vars {
		text = strings.ReplaceAll(text, "${"+k+"}", v)
	}
	return text
}

// Info file parsing
//
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
		case "name":
			p.DisplayName = val
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

// Deps file parsing
//
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

// Directory hash calculation
//
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
