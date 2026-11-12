package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Navigation link
type NavLink struct {
	Text string `toml:"text"`
	URL  string `toml:"url"`
}

// Configuration
type Config struct {
	Title          string `toml:"title"`
	Description    string `toml:"description"`
	HeaderTitle    string `toml:"header_title"`
	HeaderPrefix   string `toml:"header_prefix"`
	Subtitle       string `toml:"subtitle"`
	SubtitlePrefix string `toml:"subtitle_prefix"`
	Domain         string `toml:"domain"`
	FooterText     string `toml:"footer_text"`
	FooterURL      string `toml:"footer_url"`
	BaseURL        string `toml:"base_url"`
	SourceCodeURL  string `toml:"source_code_url"`
	Favicon        string `toml:"favicon"`

	NavLinks []NavLink `toml:"nav_links"`

	PortsPath string `toml:"ports_path"`
	OutDir    string `toml:"out_dir"`
	CacheDir  string `toml:"cache_dir"`
	AssetsDir string `toml:"assets_dir"`
	CIStatus  string `toml:"ci_status"`

	// Static site generator
	Theme      string `toml:"theme"`
	ThemesDir  string `toml:"themes_dir"`
	LayoutsDir string `toml:"layouts_dir"`
	ContentDir string `toml:"content_dir"`
	DataDir    string `toml:"data_dir"`
	StaticDir  string `toml:"static_dir"`

	ThemeExplicit bool `toml:"-"`

	Metadata struct {
		PkgsPath string `toml:"pkg_root"`
		LogsPath string `toml:"log_root"`
		CIStatus string `toml:"ci_status"`
	} `toml:"metadata"`

	Port           int    `toml:"port"`
	ServeAddr      string `toml:"serve_addr"`
	Watch          bool   `toml:"watch"`
	Serve          bool   `toml:"serve"`
	PackageManager string `toml:"package_manager"`

	ExtraCSS []string `toml:"extra_css"`
	ExtraJS  []string `toml:"extra_js"`
	Fortunes string   `toml:"fortunes"`
}

// Initialisation
func New() *Config {
	return &Config{
		Title:          "portsMaster",
		Description:    "Universal package repository generator",
		FooterText:     "Powered by portsMaster",
		Port:           1313,
		OutDir:         "public",
		CacheDir:       ".cache",
		PortsPath:      "ports",
		AssetsDir:      "assets",
		PackageManager: "spc",
		Theme:          "shrub",
		ThemesDir:      "themes",
		LayoutsDir:     "layouts",
		ContentDir:     "content",
		DataDir:        "data",
		StaticDir:      "static",
	}
}

// File loading
func (c *Config) LoadFile(path string) error {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("config: %w", err)
	}
	return toml.Unmarshal(data, c)
}

// Flag parsing
func (c *Config) ParseFlags(args []string) (string, error) {
	fs := flag.NewFlagSet("portsMaster", flag.ContinueOnError)

	var configPath string
	fs.StringVar(&configPath, "config", "config.toml", "Path to configuration file")

	title := fs.String("title", "", "Site title")
	desc := fs.String("description", "", "Site description")
	out := fs.String("out", "", "Output directory")
	ports := fs.String("ports", "", "Ports tree directory")
	ci := fs.String("ci-status", "", "Path to ci_status.json")
	theme := fs.String("theme", "", "Theme name (remembered for next runs)")
	themesDir := fs.String("themes", "", "Themes directory")
	layoutsDir := fs.String("layouts", "", "Project layouts directory")
	contentDir := fs.String("content", "", "Generated Djot content directory")
	dataDir := fs.String("data", "", "Generated data directory")
	staticDir := fs.String("static", "", "Static files directory")
	watch := fs.Bool("watch", false, "Watch for changes and rebuild")
	serve := fs.Bool("serve", false, "Start a local web server")
	port := fs.Int("port", 0, "Server port")

	if err := fs.Parse(args); err != nil {
		return "", err
	}

	isSet := func(name string) bool {
		found := false
		fs.Visit(func(f *flag.Flag) {
			if f.Name == name {
				found = true
			}
		})
		return found
	}

	if isSet("title") {
		c.Title = *title
	}
	if isSet("description") {
		c.Description = *desc
	}
	if isSet("out") {
		c.OutDir = *out
	}
	if isSet("ports") {
		c.PortsPath = *ports
	}
	if isSet("ci-status") {
		c.Metadata.CIStatus = *ci
	}
	if isSet("theme") {
		c.Theme = *theme
		c.ThemeExplicit = true
	}
	if isSet("themes") {
		c.ThemesDir = *themesDir
	}
	if isSet("layouts") {
		c.LayoutsDir = *layoutsDir
	}
	if isSet("content") {
		c.ContentDir = *contentDir
	}
	if isSet("data") {
		c.DataDir = *dataDir
	}
	if isSet("static") {
		c.StaticDir = *staticDir
	}
	if isSet("watch") {
		c.Watch = *watch
	}
	if isSet("serve") {
		c.Serve = *serve
	}
	if isSet("port") {
		c.Port = *port
	}

	return configPath, nil
}

// Remote check
func IsRemote(path string) bool {
	return len(path) > 4 && (path[:4] == "http" || path[:4] == "ftp:")
}

// Finalisation
func (c *Config) Finalize() {
	if c.BaseURL == "/" {
		c.BaseURL = ""
	}
	if c.Serve && c.ServeAddr == "" {
		c.ServeAddr = fmt.Sprintf(":%d", c.Port)
	}

	expand := func(p string) string {
		if IsRemote(p) {
			return p
		}
		if len(p) > 0 && p[0] == '~' {
			if home, err := os.UserHomeDir(); err == nil {
				return filepath.Join(home, p[1:])
			}
		}
		return p
	}

	c.PortsPath = expand(c.PortsPath)
	c.Metadata.PkgsPath = expand(c.Metadata.PkgsPath)
	c.Metadata.LogsPath = expand(c.Metadata.LogsPath)
	c.Metadata.CIStatus = expand(c.Metadata.CIStatus)
	c.CIStatus = expand(c.CIStatus)
	c.OutDir = expand(c.OutDir)
	c.CacheDir = expand(c.CacheDir)
	c.AssetsDir = expand(c.AssetsDir)
	c.ThemesDir = expand(c.ThemesDir)
	c.LayoutsDir = expand(c.LayoutsDir)
	c.ContentDir = expand(c.ContentDir)
	c.DataDir = expand(c.DataDir)
	c.StaticDir = expand(c.StaticDir)
}

// ResolveTheme applies the remembered theme unless the user chose one on this
// run, in which case the choice becomes the new remembered preference.
func normalizeTheme(name string) string {
	switch name {
	case "derive", "base":
		return "shrub"
	case "classic":
		return "xplshn"
	case "":
		return "shrub"
	}
	return name
}

func (c *Config) ResolveTheme() error {
	file := filepath.Join(c.CacheDir, "theme")
	c.Theme = normalizeTheme(c.Theme)
	if c.ThemeExplicit {
		if err := os.MkdirAll(c.CacheDir, 0755); err != nil {
			return err
		}
		return os.WriteFile(file, []byte(c.Theme+"\n"), 0644)
	}
	data, err := os.ReadFile(file)
	if err == nil {
		if theme := strings.TrimSpace(string(data)); theme != "" {
			c.Theme = normalizeTheme(theme)
		}
	}
	return nil
}

// Asset URL
func (c *Config) AssetURL(path string) string {
	if IsRemote(path) || (len(path) > 0 && path[0] == '/') {
		return path
	}
	if IsRemote(c.AssetsDir) {
		return fmt.Sprintf("%s/%s", strings.TrimRight(c.AssetsDir, "/"), path)
	}
	return fmt.Sprintf("%s/assets/%s", c.BaseURL, path)
}
