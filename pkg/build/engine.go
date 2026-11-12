package build

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"portsMaster/pkg/cache"
	"portsMaster/pkg/config"
	"portsMaster/pkg/model"
	"portsMaster/pkg/port"
	"portsMaster/pkg/registry"
	"portsMaster/pkg/sitegen"

	"github.com/nein-ar/dejot/ssg"
)

// Engine owns one build: collect, materialise Djot sources, and hand them to
// the site generator. Each stage has its own cache and each stage can prove it
// has nothing to do without doing it.
type Engine struct {
	cfg     *config.Config
	reg     *registry.Registry
	scanner model.Scanner
	objects *cache.ObjectCache
	Ready   chan struct{}
}

// New prepares the engine. No files are read yet.
func New(cfg *config.Config) (*Engine, error) {
	reg := registry.New(cfg.PortsPath, cfg.Metadata.PkgsPath, cfg.Metadata.LogsPath)
	objects := cache.NewObjectCache(filepath.Join(cfg.CacheDir, "objects"))
	scanner, err := port.NewScanner(cfg, reg, objects)
	if err != nil {
		return nil, err
	}
	return &Engine{
		cfg:     cfg,
		reg:     reg,
		scanner: scanner,
		objects: objects,
		Ready:   make(chan struct{}),
	}, nil
}

// Run executes one build. The Ready channel is signalled when the first
// complete output is available.
func (e *Engine) Run(ctx context.Context) error {
	col := NewCollector(e.cfg, e.reg, e.scanner, e.objects)
	_, data, err := col.Collect(ctx)
	if err != nil {
		return fmt.Errorf("collection failed: %w", err)
	}

	gen := sitegen.New(e.cfg)
	outputs, err := gen.Generate(data)
	if err != nil {
		return fmt.Errorf("source generation failed: %w", err)
	}
	if _, err := gen.Finish(); err != nil {
		return fmt.Errorf("source cleanup failed: %w", err)
	}

	siteCfg := siteConfig(e.cfg)
	site, err := ssg.Load(siteCfg)
	if err != nil {
		return fmt.Errorf("content load failed: %w", err)
	}
	site.AddOutput("ports.json", outputs.SearchIndex)
	site.AddOutput("commits.json", outputs.Commits)
	if fortunes := e.fortunes(); fortunes != nil {
		site.AddOutput("assets/fortunes.js", fortunes)
	}

	result, err := site.Build()
	if err != nil {
		return fmt.Errorf("site build failed: %w", err)
	}
	if result.Skipped {
		fmt.Println("site: up to date")
	} else {
		fmt.Printf("site: %d pages, %d written, %d removed\n", result.Pages, result.Written, result.Removed)
	}

	if err := e.objects.GC(); err != nil {
		return fmt.Errorf("cache cleanup failed: %w", err)
	}

	select {
	case e.Ready <- struct{}{}:
	default:
	}
	return nil
}

func siteConfig(cfg *config.Config) *ssg.Config {
	sc := ssg.NewConfig()
	sc.Title = cfg.Title
	sc.Description = cfg.Description
	sc.BaseURL = cfg.BaseURL
	sc.Theme = cfg.Theme
	sc.ThemesDir = cfg.ThemesDir
	sc.LayoutsDir = cfg.LayoutsDir
	sc.ContentDir = cfg.ContentDir
	sc.DataDir = cfg.DataDir
	sc.StaticDir = cfg.StaticDir
	sc.OutputDir = cfg.OutDir
	sc.CacheDir = cfg.CacheDir

	nav := make([]map[string]string, 0, len(cfg.NavLinks))
	for _, link := range cfg.NavLinks {
		nav = append(nav, map[string]string{"text": link.Text, "url": link.URL})
	}

	defaultSkin := "shrub"
	if cfg.Theme == "xplshn" {
		defaultSkin = "xplshn"
	}

	sc.Params = map[string]any{
		"default_theme":   defaultSkin,
		"domain":          cfg.Domain,
		"header_title":    cfg.HeaderTitle,
		"header_prefix":   cfg.HeaderPrefix,
		"subtitle":        cfg.Subtitle,
		"subtitle_prefix": cfg.SubtitlePrefix,
		"footer_text":     cfg.FooterText,
		"footer_url":      cfg.FooterURL,
		"source_code_url": cfg.SourceCodeURL,
		"favicon":         cfg.Favicon,
		"extra_css":       cfg.ExtraCSS,
		"extra_js":        cfg.ExtraJS,
		"nav_links":       nav,
	}

	if config.IsRemote(cfg.AssetsDir) {
		sc.Params["assets_url"] = strings.TrimRight(cfg.AssetsDir, "/")
	} else if stat, err := os.Stat(cfg.AssetsDir); err == nil && stat.IsDir() {
		sc.StaticDirs = append(sc.StaticDirs, cfg.AssetsDir)
	}
	return sc
}

func (e *Engine) fortunes() []byte {
	if e.cfg.Fortunes == "" {
		return nil
	}
	content, err := readFortunes(e.cfg)
	if err != nil {
		fmt.Printf("warning: fortunes: %v\n", err)
		return nil
	}
	var fortunes []string
	for _, part := range strings.Split(string(content), "!---") {
		if text := strings.TrimSpace(part); text != "" {
			fortunes = append(fortunes, text)
		}
	}
	if len(fortunes) == 0 {
		return nil
	}
	list, _ := json.Marshal(fortunes)
	return []byte("document.addEventListener('DOMContentLoaded', () => {\n" +
		"\tconst fortunes = " + string(list) + ";\n" +
		"\tconst el = document.getElementById('fortune-text');\n" +
		"\tif (el && fortunes.length > 0) {\n" +
		"\t\tel.innerText = fortunes[Math.floor(Math.random() * fortunes.length)];\n" +
		"\t}\n" +
		"});\n")
}

func readFortunes(cfg *config.Config) ([]byte, error) {
	path := cfg.Fortunes
	if config.IsRemote(path) {
		resp, err := http.Get(path)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s", resp.Status)
		}
		return io.ReadAll(resp.Body)
	}
	if filepath.IsAbs(path) {
		return os.ReadFile(path)
	}
	candidates := []string{path}
	if cfg.AssetsDir != "" {
		candidates = append(candidates, filepath.Join(cfg.AssetsDir, path))
	}
	for _, themeDir := range siteConfig(cfg).ThemeChain() {
		candidates = append(candidates,
			filepath.Join(themeDir, "static", "assets", path),
			filepath.Join(themeDir, "static", path),
		)
	}
	for _, candidate := range candidates {
		if data, err := os.ReadFile(candidate); err == nil {
			return data, nil
		}
	}
	return nil, os.ErrNotExist
}
