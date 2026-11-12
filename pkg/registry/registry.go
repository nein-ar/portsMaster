package registry

import "path/filepath"

// Registry is the set of source roots a port tree refers to. Output and theme
// paths belong to the site generator, so they are not here.
type Registry struct {
	portsRoot string
	pkgsRoot  string
	logsRoot  string
}

func New(ports, pkgs, logs string) *Registry {
	clean := func(p string) string {
		if len(p) > 4 && (p[:4] == "http" || p[:4] == "ftp:") {
			return p
		}
		return filepath.Clean(p)
	}
	return &Registry{
		portsRoot: clean(ports),
		pkgsRoot:  clean(pkgs),
		logsRoot:  clean(logs),
	}
}

func (r *Registry) PortsRoot() string { return r.portsRoot }
func (r *Registry) PkgsRoot() string  { return r.pkgsRoot }
func (r *Registry) LogsRoot() string  { return r.logsRoot }
