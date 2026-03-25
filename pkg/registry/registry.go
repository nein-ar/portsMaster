package registry

import (
	"path/filepath"
)

// Registry
//
type Registry struct {
	portsRoot  string
	pkgsRoot   string
	logsRoot   string
	outputRoot string
	assetsRoot string
}

// Initialisation
//
func New(ports, pkgs, logs, output, assets string) *Registry {
	clean := func(p string) string {
		if len(p) > 4 && (p[:4] == "http" || p[:4] == "ftp:") {
			return p
		}
		return filepath.Clean(p)
	}
	return &Registry{
		portsRoot:  clean(ports),
		pkgsRoot:   clean(pkgs),
		logsRoot:   clean(logs),
		outputRoot: clean(output),
		assetsRoot: clean(assets),
	}
}

func (r *Registry) PortsRoot() string  { return r.portsRoot }
func (r *Registry) PkgsRoot() string   { return r.pkgsRoot }
func (r *Registry) LogsRoot() string   { return r.logsRoot }
func (r *Registry) OutputRoot() string { return r.outputRoot }
func (r *Registry) AssetsRoot() string { return r.assetsRoot }

// Public asset path
//
func (r *Registry) PublicAsset(filename string) string {
	return filepath.Join(r.outputRoot, "assets", filename)
}

// Public page path
//
func (r *Registry) PublicPage(path string) string {
	return filepath.Join(r.outputRoot, path)
}

// Asset source path
//
func (r *Registry) AssetSource(filename string) string {
	return filepath.Join(r.assetsRoot, filename)
}
