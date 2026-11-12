package sitegen

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"portsMaster/pkg/model"
)

// The view types are the JSON contract between the generator and the theme.
// They intentionally contain only what a template needs: no file contents, no
// pointers into the scanner's model, no behaviour.

type SiteView struct {
	TotalPorts        int               `json:"total_ports"`
	BrokenCount       int               `json:"broken_count"`
	UnmaintainedCount int               `json:"unmaintained_count"`
	UpdatedThisWeek   int               `json:"updated_this_week"`
	NewPortsCount     int               `json:"new_ports_count"`
	TotalCommits      int               `json:"total_commits"`
	TotalRecipeLines  int               `json:"total_recipe_lines"`
	Top5LinePercent   float64           `json:"top5_line_percent"`
	Build             BuildView         `json:"build"`
	LicenseStats      map[string]int    `json:"license_stats"`
	Daily             []DailyView       `json:"daily"`
	MaxDailyCommits   int               `json:"max_daily_commits"`
	LastUpdate        time.Time         `json:"last_update"`
	Categories        []CategoryView    `json:"categories"`
	Contributors      []ContributorView `json:"contributors"`
	TopRecipes        []PortRef         `json:"top_recipes"`
	TopSizes          []PortRef         `json:"top_sizes"`
	Recent            []string          `json:"recent"`
}

type BuildView struct {
	Total     int   `json:"total"`
	Success   int   `json:"success"`
	Failed    int   `json:"failed"`
	TotalTime int64 `json:"total_time"`
	AvgTime   int64 `json:"avg_time"`
}

type DailyView struct {
	Date    string `json:"date"`
	Count   int    `json:"count"`
	Updates int    `json:"updates"`
	Builds  int    `json:"builds"`
}

type CategoryView struct {
	Name  string   `json:"name"`
	Count int      `json:"count"`
	Ports []string `json:"ports"`
}

type ContributorView struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Count   int    `json:"count"`
	Tooltip string `json:"tooltip"`
}

type PortRef struct {
	Key         string  `json:"key"`
	Name        string  `json:"name"`
	Category    string  `json:"category"`
	RecipeLines int     `json:"recipe_lines"`
	LinePercent float64 `json:"line_percent"`
	Size        int64   `json:"size"`
}

type PortView struct {
	Key            string        `json:"key"`
	Name           string        `json:"name"`
	DisplayName    string        `json:"display_name,omitempty"`
	Category       string        `json:"category"`
	Description    string        `json:"description"`
	Version        string        `json:"version"`
	Release        string        `json:"release,omitempty"`
	License        string        `json:"license,omitempty"`
	Upstream       string        `json:"upstream,omitempty"`
	Maintainer     string        `json:"maintainer,omitempty"`
	FilePath       string        `json:"file_path"`
	Hash           string        `json:"hash,omitempty"`
	IsBroken       bool          `json:"is_broken"`
	IsUnmaintained bool          `json:"is_unmaintained"`
	RecipeLines    int           `json:"recipe_lines"`
	Provides       []string      `json:"provides"`
	Deps           []DepView     `json:"deps"`
	Packages       []PackageView `json:"packages"`
	Commits        []CommitView  `json:"commits"`
	LastCommit     *CommitView   `json:"last_commit,omitempty"`
	CI             *CIView       `json:"ci,omitempty"`
	Files          []FileView    `json:"files"`
	HasLog         bool          `json:"has_log"`
	LogURL         string        `json:"log_url,omitempty"`
	CommitShare    float64       `json:"commit_share"`
	LineShare      float64       `json:"line_share"`
}

type DepView struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type PackageView struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

type CIView struct {
	Status            string `json:"status"`
	BuildLog          string `json:"build_log,omitempty"`
	BuildDuration     int64  `json:"build_duration"`
	BuildStarted      int64  `json:"build_started"`
	Size              int64  `json:"size"`
	BuilderInfo       string `json:"builder_info,omitempty"`
	InstalledSize     int64  `json:"installed_size"`
	DepsSize          int64  `json:"deps_size"`
	DepsInstalledSize int64  `json:"deps_installed_size"`
	TotalSize         int64  `json:"total_size"`
	TotalInstalled    int64  `json:"total_installed"`
}

type CommitView struct {
	Hash          string    `json:"hash"`
	Author        string    `json:"author"`
	Email         string    `json:"email"`
	Date          time.Time `json:"date"`
	Message       string    `json:"message"`
	AddedFiles    []string  `json:"added_files,omitempty"`
	ModifiedFiles []string  `json:"modified_files,omitempty"`
	DeletedFiles  []string  `json:"deleted_files,omitempty"`
	IsMerge       bool      `json:"is_merge"`
}

type FileView struct {
	Name    string `json:"name"`
	URL     string `json:"url,omitempty"`
	IsImage bool   `json:"is_image"`
}

type PortsView struct {
	ByKey  map[string]*PortView `json:"by_key"`
	ByName map[string]string    `json:"by_name"`
	List   []string             `json:"list"`
}

func buildSiteView(data *model.SiteData) *SiteView {
	s := &SiteView{
		Categories:        []CategoryView{},
		Contributors:      []ContributorView{},
		TopRecipes:        []PortRef{},
		TopSizes:          []PortRef{},
		Daily:             []DailyView{},
		Recent:            []string{},
		LicenseStats:      map[string]int{},
		TotalPorts:        data.TotalPorts,
		BrokenCount:       data.BrokenCount,
		UnmaintainedCount: data.UnmaintainedCount,
		UpdatedThisWeek:   data.UpdatedThisWeek,
		NewPortsCount:     data.NewPortsCount,
		TotalCommits:      data.TotalCommits,
		TotalRecipeLines:  data.TotalRecipeLines,
		Top5LinePercent:   data.Top5LinePercentage,
		Build: BuildView{
			Total:     data.BuildStats.Total,
			Success:   data.BuildStats.Success,
			Failed:    data.BuildStats.Failed,
			TotalTime: data.BuildStats.TotalTime,
			AvgTime:   data.BuildStats.AvgTime,
		},
		MaxDailyCommits: data.MaxDailyCommits,
		LastUpdate:      data.LastUpdate.UTC(),
	}
	for license, count := range data.LicenseStats {
		s.LicenseStats[license] = count
	}

	for _, c := range data.Categories {
		keys := make([]string, 0, len(c.Ports))
		for _, p := range c.Ports {
			keys = append(keys, p.Category+"/"+p.Name)
		}
		sort.Strings(keys)
		s.Categories = append(s.Categories, CategoryView{Name: c.Name, Count: len(c.Ports), Ports: keys})
	}
	sort.Slice(s.Categories, func(i, j int) bool { return s.Categories[i].Name < s.Categories[j].Name })

	for _, c := range data.TopContributors {
		s.Contributors = append(s.Contributors, ContributorView{
			Name:    c.Name,
			Email:   c.Email,
			Count:   c.Count,
			Tooltip: contributorTooltip(c),
		})
	}
	for _, d := range data.DailyStats {
		s.Daily = append(s.Daily, DailyView{Date: d.Date, Count: d.Count, Updates: d.Updates, Builds: d.Builds})
	}
	for _, p := range data.TopRecipes {
		s.TopRecipes = append(s.TopRecipes, portRef(p, data.TotalRecipeLines))
	}
	for _, p := range data.TopSizes {
		s.TopSizes = append(s.TopSizes, portRef(p, data.TotalRecipeLines))
	}
	for _, p := range recentPorts(data.Ports) {
		s.Recent = append(s.Recent, p.Category+"/"+p.Name)
	}
	return s
}

func contributorTooltip(c *model.Contributor) string {
	var others []string
	seen := make(map[string]bool)
	for _, n := range c.OtherNames {
		n = strings.TrimSpace(n)
		if n != "" && n != c.Name && !seen[n] {
			others = append(others, n)
			seen[n] = true
		}
	}
	if len(others) == 0 {
		return c.Email
	}
	return fmt.Sprintf("aka: %s | %s", strings.Join(others, ", "), c.Email)
}

func portRef(p *model.Port, totalLines int) PortRef {
	ref := PortRef{
		Key:         p.Category + "/" + p.Name,
		Name:        p.Name,
		Category:    p.Category,
		RecipeLines: p.RecipeLines,
	}
	if totalLines > 0 {
		ref.LinePercent = float64(p.RecipeLines) / float64(totalLines) * 100
	}
	if p.CI != nil {
		ref.Size = p.CI.Size
	}
	return ref
}

func recentPorts(ports []*model.Port) []*model.Port {
	out := make([]*model.Port, len(ports))
	copy(out, ports)
	sort.Slice(out, func(i, j int) bool {
		var a, b time.Time
		if out[i].LastCommit != nil {
			a = out[i].LastCommit.Date
		}
		if out[j].LastCommit != nil {
			b = out[j].LastCommit.Date
		}
		if a.Equal(b) {
			return out[i].Name < out[j].Name
		}
		return a.After(b)
	})
	if len(out) > 12 {
		out = out[:12]
	}
	return out
}

func buildPortsView(data *model.SiteData) *PortsView {
	v := &PortsView{
		ByKey:  make(map[string]*PortView, len(data.Ports)),
		ByName: make(map[string]string),
	}
	for _, p := range data.Ports {
		key := p.Category + "/" + p.Name
		pv := buildPortView(p, data)
		v.ByKey[key] = pv
		v.List = append(v.List, key)
		if _, exists := v.ByName[p.Name]; !exists {
			v.ByName[p.Name] = key
		}
		for _, prov := range p.Provides {
			if _, exists := v.ByName[prov]; !exists {
				v.ByName[prov] = key
			}
		}
	}
	sort.Strings(v.List)
	return v
}

func buildPortView(p *model.Port, data *model.SiteData) *PortView {
	v := &PortView{
		Key:            p.Category + "/" + p.Name,
		Name:           p.Name,
		DisplayName:    p.DisplayName,
		Category:       p.Category,
		Description:    p.Description,
		Version:        p.Version,
		Release:        p.Release,
		License:        p.License,
		Upstream:       p.Upstream,
		Maintainer:     p.Maintainer,
		FilePath:       p.FilePath,
		Hash:           p.Hash,
		IsBroken:       p.IsBroken,
		IsUnmaintained: p.IsUnmaintained,
		RecipeLines:    p.RecipeLines,
		Provides:       append([]string{}, p.Provides...),
		Deps:           []DepView{},
		Packages:       []PackageView{},
		Commits:        []CommitView{},
		Files:          []FileView{},
	}
	if data.TotalCommits > 0 {
		v.CommitShare = float64(len(p.Commits)) / float64(data.TotalCommits) * 100
	}
	if data.TotalRecipeLines > 0 {
		v.LineShare = float64(p.RecipeLines) / float64(data.TotalRecipeLines) * 100
	}
	for _, d := range p.Deps {
		v.Deps = append(v.Deps, DepView{Name: d.Name, Type: string(d.Type)})
	}
	for _, pkg := range p.Packages {
		v.Packages = append(v.Packages, PackageView{Filename: pkg.Filename, Size: pkg.Size})
	}
	for _, c := range p.Commits {
		v.Commits = append(v.Commits, CommitView{
			Hash:          c.Hash,
			Author:        c.Author,
			Email:         c.Email,
			Date:          c.Date,
			Message:       c.Message,
			AddedFiles:    c.AddedFiles,
			ModifiedFiles: c.ModifiedFiles,
			DeletedFiles:  c.DeletedFiles,
			IsMerge:       c.IsMerge,
		})
	}
	if p.LastCommit != nil {
		cv := CommitView{
			Hash: p.LastCommit.Hash, Author: p.LastCommit.Author, Email: p.LastCommit.Email,
			Date: p.LastCommit.Date, Message: p.LastCommit.Message,
		}
		v.LastCommit = &cv
	}
	if p.CI != nil {
		v.CI = &CIView{
			Status: p.CI.Status, BuildLog: p.CI.BuildLog, BuildDuration: p.CI.BuildDuration,
			BuildStarted: p.CI.BuildStarted, Size: p.CI.Size, BuilderInfo: p.CI.BuilderInfo,
			InstalledSize: p.CI.InstalledSize, DepsSize: p.CI.DepsSize,
			DepsInstalledSize: p.CI.DepsInstalledSize,
			TotalSize:         p.CI.Size + p.CI.DepsSize,
			TotalInstalled:    p.CI.InstalledSize + p.CI.DepsInstalledSize,
		}
		v.HasLog = p.CI.BuildLog != ""
		if _, ok := p.FileContents["ci_log.txt"]; ok {
			v.LogURL = fmt.Sprintf("/ports/%s/%s/viewer_ci_log.txt.html", p.Category, p.Name)
		}
	}
	files := append([]string(nil), p.Files...)
	sort.Strings(files)
	for _, name := range files {
		_, embedded := p.FileContents[name]
		f := FileView{Name: name, IsImage: isImage(name)}
		if embedded {
			f.URL = fmt.Sprintf("/ports/%s/%s/viewer_%s.html", p.Category, p.Name, name)
		}
		v.Files = append(v.Files, f)
	}
	return v
}

func isImage(name string) bool {
	switch strings.ToLower(name[strings.LastIndex(name, ".")+1:]) {
	case "png", "jpg", "jpeg", "gif", "bmp", "svg", "webp":
		return true
	}
	return false
}
