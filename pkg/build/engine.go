package build

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"portsMaster/pkg/cache"
	"portsMaster/pkg/config"
	"portsMaster/pkg/model"
	"portsMaster/pkg/port"
	"portsMaster/pkg/registry"
	"portsMaster/views"

	"github.com/a-h/templ"
	"github.com/tdewolff/minify/v2"
	minjson "github.com/tdewolff/minify/v2/json"
)

type Engine struct {
	cfg      *config.Config
	reg      *registry.Registry
	scanner  model.Scanner
	manifest *cache.Manifest
	touched  map[string]bool
	mu       sync.Mutex
	Ready    chan struct{}
}

func New(cfg *config.Config) (*Engine, error) {
	reg := registry.New(cfg.PortsPath, cfg.Metadata.PkgsPath, cfg.Metadata.LogsPath, cfg.OutDir, cfg.AssetsDir)
	scanner, err := port.NewScanner(cfg, reg)
	if err != nil {
		return nil, err
	}

	return &Engine{
		cfg:      cfg,
		reg:      reg,
		scanner:  scanner,
		manifest: cache.LoadManifest(filepath.Join(cfg.CacheDir, "manifest.json")),
		touched:  make(map[string]bool),
		Ready:    make(chan struct{}),
	}, nil
}

func (e *Engine) Run(ctx context.Context) error {
	col := NewCollector(e.cfg, e.reg, e.scanner)
	portChan := make(chan *model.Port, 100)
	metaChan := make(chan *model.Database, 1)
	errChan := make(chan error, 1)

	go func() { errChan <- col.Stream(ctx, portChan, metaChan) }()

	var db *model.Database
	var siteData *model.SiteData

	globalHash := e.computeGlobalHash()
	e.syncAssets(globalHash)
	e.renderFortunes(globalHash)

	for portChan != nil || metaChan != nil {
		select {
		case _, ok := <-portChan:
			if !ok {
				portChan = nil
			}
		case d, ok := <-metaChan:
			if !ok {
				metaChan = nil
			} else {
				db = d
				siteData = col.PrepareSiteData(db)
			}
		case err := <-errChan:
			if err != nil {
				return fmt.Errorf("collection failed: %w", err)
			}
			errChan = nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if siteData == nil {
		return fmt.Errorf("no data collected")
	}

	dataHash := e.computeDataHash(siteData)

	e.renderCorePages(siteData, db, globalHash, dataHash)
	e.renderCategories(siteData, db, globalHash, dataHash)
	e.renderPorts(siteData, db, globalHash, dataHash)

	e.exportJSON("ports.json", e.buildSearchIndex(db.Ports), globalHash)
	e.exportJSON("commits.json", db.RecentCommits, globalHash)

	select {
	case e.Ready <- struct{}{}:
	default:
	}

	e.cleanup()
	return e.manifest.Save(filepath.Join(e.cfg.CacheDir, "manifest.json"))
}

func (e *Engine) renderCorePages(data *model.SiteData, db *model.Database, globalHash, dataHash string) {
	pages := []struct {
		path string
		comp templ.Component
	}{
		{"index.html", views.Home(data, e.getRecentUpdates(db.Ports), e.cfg, "index.html")},
		{"categories/index.html", views.CategoryList(data, e.cfg, "categories/index.html")},
		{"commits/index.html", views.Commits(data, e.cfg, "commits/index.html")},
		{"stats/index.html", views.Stats(data, e.cfg, "stats/index.html")},
		{"search/index.html", views.Search(data, e.cfg, "search/index.html")},
	}

	for _, p := range pages {
		e.render(p.path, p.comp, cache.HashString(globalHash+dataHash+p.path))
	}
}

func (e *Engine) renderCategories(data *model.SiteData, db *model.Database, globalHash, dataHash string) {
	for _, c := range db.Categories {
		path := fmt.Sprintf("categories/%s/index.html", c.Name)
		e.render(path, views.CategoryDetail(data, c, e.cfg, path), e.computeCategoryHash(c, globalHash, dataHash))
	}
}

func (e *Engine) renderPorts(data *model.SiteData, db *model.Database, globalHash, dataHash string) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 32)
	var processed uint64
	total := uint64(len(db.Ports))

	for _, p := range db.Ports {
		wg.Add(1)
		go func(p *model.Port) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			path := fmt.Sprintf("ports/%s/%s/index.html", p.Category, p.Name)
			e.render(path, views.PortDetail(data, p, e.cfg, path), e.computePortHash(p, globalHash, dataHash, data.SimplePortMap))

			if atomic.AddUint64(&processed, 1) == total/10 {
				select {
				case e.Ready <- struct{}{}:
				default:
				}
			}
		}(p)
	}
	wg.Wait()
}

func (e *Engine) render(path string, comp templ.Component, hash string) {
	e.mu.Lock()
	e.touched[path] = true
	e.mu.Unlock()

	if !e.manifest.HasChanged(path, hash) {
		return
	}

	full := e.reg.PublicPage(path)
	os.MkdirAll(filepath.Dir(full), 0755)
	f, err := os.Create(full)
	if err != nil {
		return
	}
	defer f.Close()

	comp.Render(context.Background(), f)
	e.manifest.Update(path, hash)
}

func (e *Engine) exportJSON(path string, data interface{}, global string) {
	e.mu.Lock()
	e.touched[path] = true
	e.mu.Unlock()

	b, _ := json.Marshal(data)
	h := cache.HashString(global + string(b))
	if !e.manifest.HasChanged(path, h) {
		return
	}

	full := e.reg.PublicPage(path)
	os.MkdirAll(filepath.Dir(full), 0755)
	f, err := os.Create(full)
	if err != nil {
		return
	}
	defer f.Close()

	m := minify.New()
	m.AddFunc("application/json", minjson.Minify)
	mw := m.Writer("application/json", f)
	json.NewEncoder(mw).Encode(data)
	mw.Close()
	e.manifest.Update(path, h)
}

func (e *Engine) syncAssets(global string) {
	entries, _ := os.ReadDir(e.cfg.AssetsDir)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		src := e.reg.AssetSource(entry.Name())
		fh, _ := cache.HashFile(src)
		h := cache.HashString(global + fh)
		path := filepath.Join("assets", entry.Name())

		e.mu.Lock()
		e.touched[path] = true
		e.mu.Unlock()

		if e.manifest.HasChanged(path, h) {
			copyFile(src, e.reg.PublicAsset(entry.Name()))
			e.manifest.Update(path, h)
		}
	}

	publicAssetsDir := filepath.Join(e.cfg.OutDir, "assets")
	pEntries, _ := os.ReadDir(publicAssetsDir)
	for _, entry := range pEntries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join("assets", entry.Name())

		e.mu.Lock()
		isTouched := e.touched[path]
		e.mu.Unlock()

		if !isTouched {
			os.Remove(filepath.Join(publicAssetsDir, entry.Name()))
			e.mu.Lock()
			delete(e.manifest.Hashes, path)
			e.mu.Unlock()
		}
	}
}

func (e *Engine) renderFortunes(global string) {
	if e.cfg.Fortunes == "" {
		return
	}

	path := e.cfg.Fortunes
	if !filepath.IsAbs(path) {
		path = filepath.Join(e.cfg.AssetsDir, path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("warning: could not read fortunes file %s: %v\n", path, err)
		return
	}

	parts := strings.Split(string(content), "!---")
	var fortunes []string
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			fortunes = append(fortunes, t)
		}
	}
	if len(fortunes) == 0 {
		return
	}

	h := cache.HashString(global + string(content))
	e.render("assets/fortunes.js", views.FortunesScript(fortunes), h)
}

func (e *Engine) cleanup() {
	for path := range e.manifest.Hashes {
		if !e.touched[path] {
			os.Remove(e.reg.PublicPage(path))
			delete(e.manifest.Hashes, path)
		}
	}
}

func (e *Engine) computeGlobalHash() string {
	h := cache.NewHasher()
	h.Add(cache.ManifestVersion)
	b, _ := json.Marshal(e.cfg)
	h.AddBytes(b)
	filepath.Walk("views", func(p string, info os.FileInfo, err error) error {
		if err == nil && strings.HasSuffix(p, ".templ") {
			if fh, err := cache.HashFile(p); err == nil {
				h.Add(fh)
			}
		}
		return nil
	})
	return h.Sum()
}

func (e *Engine) computeDataHash(d *model.SiteData) string {
	return cache.HashString(fmt.Sprintf("%d-%d-%d", d.TotalPorts, d.BrokenCount, d.TotalCommits))
}

func (e *Engine) computePortHash(p *model.Port, global, data string, pm map[string]*model.Port) string {
	h := cache.NewHasher()
	h.Add(global + data + p.Hash)
	for _, d := range p.Deps {
		if dp, ok := pm[d.Name]; ok {
			h.Add(dp.Hash)
		}
	}
	if p.LastCommit != nil {
		h.Add(p.LastCommit.Hash)
	}
	return h.Sum()
}

func (e *Engine) computeCategoryHash(c *model.Category, global, data string) string {
	h := cache.NewHasher()
	h.Add(global + data + c.Name)
	for _, p := range c.Ports {
		h.Add(p.Hash)
	}
	return h.Sum()
}

func (e *Engine) getRecentUpdates(ports []*model.Port) []*model.Port {
	r := make([]*model.Port, len(ports))
	copy(r, ports)
	sort.Slice(r, func(i, j int) bool {
		var t1, t2 time.Time
		if r[i].LastCommit != nil {
			t1 = r[i].LastCommit.Date
		}
		if r[j].LastCommit != nil {
			t2 = r[j].LastCommit.Date
		}
		return t1.After(t2)
	})
	if len(r) > 12 {
		return r[:12]
	}
	return r
}

func (e *Engine) buildSearchIndex(ports []*model.Port) interface{} {
	        type Entry struct {
	                N  string   `json:"n"`
	                C  string   `json:"c"`
	                D  string   `json:"d"`
	                V  string   `json:"v"`
	                L  string   `json:"l,omitempty"`
	                Ps []string `json:"pds,omitempty"`
	                Ds []string `json:"dps,omitempty"`
	                Br bool     `json:"br,omitempty"`
	                Un bool     `json:"un,omitempty"`
	                Dt int64    `json:"dt,omitempty"`
	                A  string   `json:"a,omitempty"`
	                St string   `json:"st,omitempty"`
	        }
	        out := make([]Entry, 0, len(ports))
	        for _, p := range ports {
	                ds := make([]string, len(p.Deps))
	                for i, d := range p.Deps {
	                        ds[i] = d.Name
	                }
	                dt := int64(0)
	                if p.LastCommit != nil {
	                        dt = p.LastCommit.Date.Unix()
	                }
	                st := ""
	                if p.CI != nil {
	                        st = p.CI.Status
	                }
	                                        out = append(out, Entry{
	                                                N: p.Name, C: p.Category, D: p.Description, V: p.Version,
	                                                L: p.License, Ps: p.Provides, Ds: ds, Br: p.IsBroken,
	                                                Un: p.IsUnmaintained, Dt: dt, A: p.Maintainer, St: st,
	                                        })
	                                }
	                        return out
	                }

func copyFile(src, dst string) {
	in, _ := os.Open(src)
	defer in.Close()
	os.MkdirAll(filepath.Dir(dst), 0755)
	out, _ := os.Create(dst)
	defer out.Close()
	io.Copy(out, in)
}
