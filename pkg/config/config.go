package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Config holds all site generation and server settings.
type Config struct {
	Title         string `toml:"title"`
	Description   string `toml:"description"`
	Domain        string `toml:"domain"`
	FooterText    string `toml:"footer_text"`
	FooterURL     string `toml:"footer_url"`
	BaseURL       string `toml:"base_url"`
	SourceCodeURL string `toml:"source_code_url"`
	Favicon       string `toml:"favicon"`

	PortsPath string `toml:"ports_path"`
	OutDir    string `toml:"out_dir"`
	CacheDir  string `toml:"cache_dir"`
	AssetsDir string `toml:"assets_dir"`

	Metadata struct {
		PkgsPath string `toml:"pkg_root"`
		LogsPath string `toml:"log_root"`
		CIStatus string `toml:"ci_status"`
	} `toml:"metadata"`

	Port           int    `toml:"port"`
	ServeAddr      string `toml:"serve_addr"`
	Verbose        bool   `toml:"verbose"`
	Watch          bool   `toml:"watch"`
	Serve          bool   `toml:"serve"`
	PackageManager string `toml:"package_manager"`

	ExtraCSS []string `toml:"extra_css"`
	ExtraJS  []string `toml:"extra_js"`
	Fortunes string   `toml:"fortunes"`
}

// New returns a configuration with sensible defaults.
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
	}
}

// LoadFile parses a TOML configuration file.
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

// ParseFlags updates configuration from command-line flags.
func (c *Config) ParseFlags(args []string) (string, error) {
	fs := flag.NewFlagSet("portsMaster", flag.ContinueOnError)

	var configPath string
	fs.StringVar(&configPath, "config", "config.toml", "Path to configuration file")

	title := fs.String("title", "", "Site title")
	desc := fs.String("description", "", "Site description")
	out := fs.String("out", "", "Output directory")
	ports := fs.String("ports", "", "Ports tree directory")
	ci := fs.String("ci-status", "", "Path to ci_status.json")
	verbose := fs.Bool("verbose", false, "Enable verbose logging")
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
	if isSet("verbose") {
		c.Verbose = *verbose
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

// Finalize resolves paths and sets derived values.
func (c *Config) Finalize() {
	if c.BaseURL == "/" {
		c.BaseURL = ""
	}
	if c.Serve && c.ServeAddr == "" {
		c.ServeAddr = fmt.Sprintf(":%d", c.Port)
	}

	expand := func(p string) string {
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
	c.OutDir = expand(c.OutDir)
	c.CacheDir = expand(c.CacheDir)
	c.AssetsDir = expand(c.AssetsDir)
}

// AssetURL returns a path relative to the site root for the given asset.
func (c *Config) AssetURL(path string) string {
	if len(path) > 0 && (path[0] == '/' || (len(path) > 4 && path[:4] == "http")) {
		return path
	}
	return fmt.Sprintf("%s/assets/%s", c.BaseURL, path)
}
