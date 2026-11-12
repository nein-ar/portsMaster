package build

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"portsMaster/pkg/cache"
	"portsMaster/pkg/config"
	"portsMaster/pkg/model"
	"portsMaster/pkg/registry"
	"portsMaster/pkg/source"
)

// Collector turns a port tree and its external evidence (CI, git, packages)
// into the two values the rest of the build needs: a database and its view.
type Collector struct {
	cfg     *config.Config
	reg     *registry.Registry
	scanner model.Scanner
	objects *cache.ObjectCache
}

func NewCollector(cfg *config.Config, reg *registry.Registry, scanner model.Scanner, objects *cache.ObjectCache) *Collector {
	return &Collector{cfg: cfg, reg: reg, scanner: scanner, objects: objects}
}

func (c *Collector) Collect(ctx context.Context) (*model.Database, *model.SiteData, error) {
	cats, ports, err := c.scanner.Scan(ctx)
	if err != nil {
		return nil, nil, err
	}

	// The scanner parses in parallel. Sorting here makes every later stage
	// deterministic, which is what allows an unchanged build to be a no-op.
	sort.Slice(cats, func(i, j int) bool { return cats[i].Name < cats[j].Name })
	for _, cat := range cats {
		sort.Slice(cat.Ports, func(i, j int) bool { return cat.Ports[i].Name < cat.Ports[j].Name })
	}
	sort.Slice(ports, func(i, j int) bool {
		if ports[i].Category == ports[j].Category {
			return ports[i].Name < ports[j].Name
		}
		return ports[i].Category < ports[j].Category
	})

	db := &model.Database{Categories: cats, Ports: ports, GeneratedAt: time.Now()}

	if c.reg.PkgsRoot() != "" {
		_ = source.ScanPackages(c.reg, ports)
	}

	ciData := make(map[string]*model.CIInfo)
	ciPath := c.cfg.CIStatus
	if ciPath == "" {
		ciPath = c.cfg.Metadata.CIStatus
	}
	if ciPath != "" {
		var err error
		if config.IsRemote(ciPath) {
			ciData, err = source.LoadCIStatus(ciPath, c.cfg.CacheDir)
		} else {
			ciData, err = source.CachedCI(ciPath, c.objects)
		}
		if err != nil {
			fmt.Printf("warning: ci status: %v\n", err)
			ciData = make(map[string]*model.CIInfo)
		}
	}

	if gp, err := source.NewGitProvider(c.reg.PortsRoot()); err == nil {
		history, recent, stats, err := gp.GetRepositoryDataCached(ports, c.objects)
		if err == nil {
			db.RecentCommits = recent
			db.ContributorStats = stats
			for _, p := range ports {
				if commits, ok := history[p.Category+"/"+p.Name]; ok && len(commits) > 0 {
					p.LastCommit = commits[0]
					p.Commits = commits
				}
			}
		}
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 32)
	for _, p := range ports {
		if ci, ok := ciData[p.Category+"/"+p.Name]; ok {
			p.CI = ci
			wg.Add(1)
			go func(port *model.Port) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				c.processLog(port)
			}(p)
		}
	}
	wg.Wait()

	db.GeneratedAt = latestUpdate(ports)
	return db, PrepareSiteData(db), nil
}

// latestUpdate is the newest observable fact about the tree. It deliberately
// does not use time.Now(): an unchanged repository must produce an unchanged
// site so the next build can prove there is nothing to do.
func latestUpdate(ports []*model.Port) time.Time {
	var latest time.Time
	for _, p := range ports {
		if p.LastCommit != nil && p.LastCommit.Date.After(latest) {
			latest = p.LastCommit.Date
		}
		if p.CI != nil && p.CI.BuildStarted > 0 {
			t := time.Unix(p.CI.BuildStarted, 0)
			if t.After(latest) {
				latest = t
			}
		}
		if stat, err := os.Stat(p.FilePath); err == nil && stat.ModTime().After(latest) {
			latest = stat.ModTime()
		}
	}
	if latest.IsZero() {
		latest = time.Unix(0, 0)
	}
	return latest
}

func (c *Collector) processLog(p *model.Port) {
	if p.CI == nil || p.CI.BuildLog == "" {
		return
	}

	logsRoot := c.cfg.Metadata.LogsPath
	if logsRoot != "" && !config.IsRemote(p.CI.BuildLog) && !strings.HasPrefix(p.CI.BuildLog, "/") {
		p.CI.BuildLog = strings.TrimRight(logsRoot, "/") + "/" + p.CI.BuildLog
	}

	const logName = "ci_log.txt"
	key := "log:" + p.Category + "/" + p.Name
	version := fmt.Sprintf("%d", p.CI.BuildStarted)

	var content string
	if c.objects != nil && p.CI.BuildStarted > 0 && c.objects.Get(key, version, &content) && content != "" {
		c.setLog(p, logName, content)
		return
	}

	var raw []byte
	switch {
	case config.IsRemote(p.CI.BuildLog):
		resp, err := http.Get(p.CI.BuildLog)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				resp.Body.Close()
			}
			return
		}
		raw, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		trimmed := strings.TrimSpace(string(raw))
		if strings.HasPrefix(strings.ToLower(trimmed), "<!doctype") || strings.HasPrefix(strings.ToLower(trimmed), "<html") {
			raw = nil
		}
	default:
		raw, _ = os.ReadFile(p.CI.BuildLog)
	}

	if len(raw) == 0 {
		return
	}
	content = string(raw)
	if c.objects != nil && p.CI.BuildStarted > 0 {
		_ = c.objects.Put(key, version, content)
	}
	c.setLog(p, logName, content)
}

func (c *Collector) setLog(p *model.Port, name, content string) {
	if p.FileContents == nil {
		p.FileContents = make(map[string]string)
	}
	p.FileContents[name] = content
}

// PrepareSiteData derives every statistic the theme can display. Keeping it
// here means templates remain pure presentation and the data is reusable by
// any theme, including one written later.
func PrepareSiteData(db *model.Database) *model.SiteData {
	data := &model.SiteData{
		Categories:       db.Categories,
		Ports:            db.Ports,
		RecentCommits:    db.RecentCommits,
		TotalPorts:       len(db.Ports),
		LastUpdate:       db.GeneratedAt,
		ContributorStats: db.ContributorStats,
		LicenseStats:     make(map[string]int),
	}

	activity := make(map[string]int)
	updates := make(map[string]int)
	builds := make(map[string]int)

	for _, c := range db.RecentCommits {
		activity[c.Date.Format("2006-01-02")]++
	}

	weekAgo := time.Now().AddDate(0, 0, -7)
	monthAgo := time.Now().AddDate(0, 0, -30)
	for _, p := range db.Ports {
		data.TotalRecipeLines += p.RecipeLines

		if p.IsBroken || (p.CI != nil && p.CI.Status == "failed") {
			data.BrokenCount++
		}
		if p.IsUnmaintained {
			data.UnmaintainedCount++
		}
		if p.LastCommit != nil {
			updates[p.LastCommit.Date.Format("2006-01-02")]++
			if p.LastCommit.Date.After(weekAgo) {
				data.UpdatedThisWeek++
			}
		}

		if len(p.Commits) > 0 {
			oldest := p.Commits[len(p.Commits)-1].Date
			if oldest.After(monthAgo) {
				data.NewPortsCount++
			}
		}

		if p.License != "" {
			for _, l := range strings.Split(p.License, ",") {
				data.LicenseStats[strings.TrimSpace(l)]++
			}
		}
		if p.CI != nil {
			data.BuildStats.Total++
			if p.CI.Status == "success" {
				data.BuildStats.Success++
				data.BuildStats.TotalTime += p.CI.BuildDuration
			} else {
				data.BuildStats.Failed++
			}
			if p.CI.BuildStarted > 0 {
				buildDate := time.Unix(p.CI.BuildStarted, 0).Format("2006-01-02")
				builds[buildDate]++
			}
		}
	}

	if data.BuildStats.Success > 0 {
		data.BuildStats.AvgTime = data.BuildStats.TotalTime / int64(data.BuildStats.Success)
	}

	finalizeContributorStats(data)
	finalizeRecipeStats(data)
	finalizeSizeStats(data)
	finalizeActivityStats(data, activity, updates, builds)
	return data
}

func finalizeSizeStats(data *model.SiteData) {
	for _, p := range data.Ports {
		if p.CI != nil && p.CI.Size > 0 {
			data.TopSizes = append(data.TopSizes, p)
		}
	}
	sort.Slice(data.TopSizes, func(i, j int) bool {
		if data.TopSizes[i].CI.Size == data.TopSizes[j].CI.Size {
			return data.TopSizes[i].Category+"/"+data.TopSizes[i].Name < data.TopSizes[j].Category+"/"+data.TopSizes[j].Name
		}
		return data.TopSizes[i].CI.Size > data.TopSizes[j].CI.Size
	})
	if len(data.TopSizes) > 10 {
		data.TopSizes = data.TopSizes[:10]
	}
}

func finalizeContributorStats(data *model.SiteData) {
	for _, v := range data.ContributorStats {
		data.TopContributors = append(data.TopContributors, v)
		data.TotalCommits += v.Count
	}
	sort.Slice(data.TopContributors, func(i, j int) bool {
		if data.TopContributors[i].Count == data.TopContributors[j].Count {
			return data.TopContributors[i].Name < data.TopContributors[j].Name
		}
		return data.TopContributors[i].Count > data.TopContributors[j].Count
	})
}

func finalizeRecipeStats(data *model.SiteData) {
	data.TopRecipes = make([]*model.Port, len(data.Ports))
	copy(data.TopRecipes, data.Ports)
	sort.Slice(data.TopRecipes, func(i, j int) bool {
		if data.TopRecipes[i].RecipeLines == data.TopRecipes[j].RecipeLines {
			return data.TopRecipes[i].Category+"/"+data.TopRecipes[i].Name < data.TopRecipes[j].Category+"/"+data.TopRecipes[j].Name
		}
		return data.TopRecipes[i].RecipeLines > data.TopRecipes[j].RecipeLines
	})

	top5Sum := 0
	for i := 0; i < 5 && i < len(data.TopRecipes); i++ {
		top5Sum += data.TopRecipes[i].RecipeLines
	}
	if data.TotalRecipeLines > 0 {
		data.Top5LinePercentage = float64(top5Sum) / float64(data.TotalRecipeLines) * 100
	}

	if len(data.TopRecipes) > 10 {
		data.TopRecipes = data.TopRecipes[:10]
	}
}

func finalizeActivityStats(data *model.SiteData, activity, updates, builds map[string]int) {
	data.MaxDailyCommits = 1
	start := time.Now().AddDate(0, 0, -60)
	end := time.Now()

	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		commitCount := activity[dateStr]
		updateCount := updates[dateStr]
		buildCount := builds[dateStr]

		if commitCount > data.MaxDailyCommits {
			data.MaxDailyCommits = commitCount
		}
		if updateCount > data.MaxDailyCommits {
			data.MaxDailyCommits = updateCount
		}
		if buildCount > data.MaxDailyCommits {
			data.MaxDailyCommits = buildCount
		}

		data.DailyStats = append(data.DailyStats, model.DailyStat{
			Date: dateStr, Count: commitCount, Updates: updateCount, Builds: buildCount,
		})
	}
}
