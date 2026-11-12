package sitegen

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"portsMaster/pkg/config"
	"portsMaster/pkg/model"
)

// Outputs are machine resources that are not pages. The engine hands them to
// the site generator as ordinary output files.
type Outputs struct {
	SearchIndex []byte
	Commits     []byte
}

// Generator materialises the Djot content tree and its data files.
type Generator struct {
	cfg     *config.Config
	content *Writer
	data    *Writer
}

func New(cfg *config.Config) *Generator {
	return &Generator{
		cfg:     cfg,
		content: NewWriter(cfg.ContentDir, path.Join(cfg.CacheDir, "content-manifest.json")),
		data:    NewWriter(cfg.DataDir, path.Join(cfg.CacheDir, "data-manifest.json")),
	}
}

// Generate writes only changed sources. Calling it when nothing changed writes
// no file at all and returns the same machine outputs as before.
func (g *Generator) Generate(data *model.SiteData) (Outputs, error) {
	site := buildSiteView(data)
	ports := buildPortsView(data)
	portFiles := make(map[string]map[string]string)
	commits := buildCommits(data.RecentCommits)

	if err := g.writeJSON("site.json", site); err != nil {
		return Outputs{}, err
	}
	if err := g.writeJSON("ports.json", ports); err != nil {
		return Outputs{}, err
	}
	if err := g.writeJSON("commits.json", commits); err != nil {
		return Outputs{}, err
	}

	if err := g.page("_index.djot", homeSource(g.cfg)); err != nil {
		return Outputs{}, err
	}

	if err := g.page("categories.djot", pageSource("Categories", "categories", nil)); err != nil {
		return Outputs{}, err
	}
	if err := g.page("commits.djot", pageSource("Recent commits", "commits", nil)); err != nil {
		return Outputs{}, err
	}
	if err := g.page("stats.djot", pageSource("Statistics", "stats", nil)); err != nil {
		return Outputs{}, err
	}
	if err := g.page("search.djot", pageSource("Search", "search", nil)); err != nil {
		return Outputs{}, err
	}

	for _, c := range data.Categories {
		fields := map[string]string{"category": c.Name}
		src := pageSource("Category: "+c.Name, "category", fields)
		if err := g.page(path.Join("categories", c.Name+".djot"), src); err != nil {
			return Outputs{}, err
		}
	}

	for _, p := range data.Ports {
		key := p.Category + "/" + p.Name
		title := p.Name
		if p.DisplayName != "" {
			title = p.DisplayName
		}
		src := pageSource(title, "port", map[string]string{"key": key})
		if err := g.page(path.Join("ports", p.Category, p.Name+".djot"), src); err != nil {
			return Outputs{}, err
		}

		if len(p.FileContents) == 0 {
			continue
		}
		files := make(map[string]string, len(p.FileContents))
		names := make([]string, 0, len(p.FileContents))
		for name, content := range p.FileContents {
			files[name] = content
			names = append(names, name)
		}
		sort.Strings(names)
		portFiles[key] = files

		for _, name := range names {
			viewerURL := fmt.Sprintf("/ports/%s/%s/viewer_%s.html", p.Category, p.Name, name)
			src := viewerSource(key, name, viewerURL, isImage(name))
			if err := g.page(path.Join("ports", p.Category, p.Name, "viewer_"+name+".djot"), src); err != nil {
				return Outputs{}, err
			}
		}
	}

	if err := g.writeJSON("portfiles.json", portFiles); err != nil {
		return Outputs{}, err
	}

	search, err := json.Marshal(buildSearchIndex(data.Ports))
	if err != nil {
		return Outputs{}, err
	}
	commitJSON, err := json.Marshal(commits)
	if err != nil {
		return Outputs{}, err
	}

	return Outputs{SearchIndex: search, Commits: commitJSON}, nil
}

// Finish removes generated sources that are no longer produced.
func (g *Generator) Finish() (int, error) {
	c, err := g.content.Finish()
	if err != nil {
		return c, err
	}
	d, err := g.data.Finish()
	return c + d, err
}

func (g *Generator) page(rel, source string) error {
	return g.content.Write(rel, []byte(source))
}

func (g *Generator) writeJSON(rel string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return g.data.Write(rel, append(data, '\n'))
}

func homeSource(cfg *config.Config) string {
	return "---\n" + kv("title", cfg.Title) + "\n" + kv("layout", "home") + "\n---\n\n"
}

func pageSource(title, layout string, fields map[string]string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(kv("title", title))
	b.WriteString("\n")
	b.WriteString(kv("layout", layout))
	b.WriteString("\n")
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString(kv(k, fields[k]))
		b.WriteString("\n")
	}
	b.WriteString("---\n\n")
	return b.String()
}

func viewerSource(key, filename, url string, image bool) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(kv("title", "File Viewer: "+filename) + "\n")
	b.WriteString(kv("layout", "viewer") + "\n")
	b.WriteString(kv("key", key) + "\n")
	b.WriteString(kv("filename", filename) + "\n")
	b.WriteString("is_image = " + strconv.FormatBool(image) + "\n")
	b.WriteString(kv("url", url) + "\n")
	b.WriteString("---\n\n")
	return b.String()
}

func kv(key, value string) string {
	return key + " = " + strconv.Quote(value)
}

func buildCommits(commits []*model.Commit) []CommitView {
	out := make([]CommitView, 0, len(commits))
	for _, c := range commits {
		out = append(out, CommitView{
			Hash: c.Hash, Author: c.Author, Email: c.Email, Date: c.Date,
			Message: c.Message, AddedFiles: c.AddedFiles, ModifiedFiles: c.ModifiedFiles,
			DeletedFiles: c.DeletedFiles, IsMerge: c.IsMerge,
		})
	}
	return out
}

type searchEntry struct {
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

func buildSearchIndex(ports []*model.Port) []searchEntry {
	sorted := append([]*model.Port(nil), ports...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Category == sorted[j].Category {
			return sorted[i].Name < sorted[j].Name
		}
		return sorted[i].Category < sorted[j].Category
	})
	out := make([]searchEntry, 0, len(sorted))
	for _, p := range sorted {
		deps := make([]string, len(p.Deps))
		for i, d := range p.Deps {
			deps[i] = d.Name
		}
		var date int64
		if p.LastCommit != nil {
			date = p.LastCommit.Date.Unix()
		}
		status := ""
		if p.CI != nil {
			status = p.CI.Status
		}
		out = append(out, searchEntry{
			N: p.Name, C: p.Category, D: p.Description, V: p.Version, L: p.License,
			Ps: p.Provides, Ds: deps, Br: p.IsBroken, Un: p.IsUnmaintained,
			Dt: date, A: p.Maintainer, St: status,
		})
	}
	return out
}
