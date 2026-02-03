package build

import (
	"context"
	"sort"
	"strings"
	"time"

	"portsMaster/pkg/config"
	"portsMaster/pkg/model"
	"portsMaster/pkg/registry"
	"portsMaster/pkg/source"
)

// Collector gathers data from various sources to build the site database.
type Collector struct {
	cfg     *config.Config
	reg     *registry.Registry
	scanner model.Scanner
}

// NewCollector creates a new data collector.
func NewCollector(cfg *config.Config, reg *registry.Registry, scanner model.Scanner) *Collector {
	return &Collector{cfg: cfg, reg: reg, scanner: scanner}
}

// Stream scans the ports and streams results through channels.
func (c *Collector) Stream(ctx context.Context, portChan chan<- *model.Port, metaChan chan<- *model.Database) error {
	defer close(portChan)
	defer close(metaChan)

	cats, ports, err := c.scanner.Scan(ctx)
	if err != nil {
		return err
	}

	db := &model.Database{
		Categories:  cats,
		Ports:       ports,
		GeneratedAt: time.Now(),
	}

	if c.reg.PkgsRoot() != "" {
		source.ScanPackages(c.reg, ports)
	}

	ciData := make(map[string]*model.CIInfo)
	ciPath := c.cfg.CIStatus
	if ciPath == "" {
		ciPath = c.cfg.Metadata.CIStatus
	}

	if ciPath != "" {
		if data, err := source.LoadCIStatus(ciPath); err == nil {
			ciData = data
		}
	}

	if gp, err := source.NewGitProvider(c.reg.PortsRoot()); err == nil {
		history, recent, stats, _ := gp.GetRepositoryDataCached(ports, c.cfg.CacheDir)
		db.RecentCommits = recent
		db.ContributorStats = stats

		for _, p := range ports {
			if commits, ok := history[p.Category+"/"+p.Name]; ok && len(commits) > 0 {
				p.LastCommit = commits[0]
				p.Commits = commits
			}
		}
	}

	for _, p := range ports {
		if ci, ok := ciData[p.Category+"/"+p.Name]; ok {
			p.CI = ci
			// Prefix BuildLog with LogsPath if set.
			// We check if BuildLog is already a remote URL or absolute path.
			logsRoot := c.cfg.Metadata.LogsPath
			if logsRoot != "" && p.CI.BuildLog != "" && !config.IsRemote(p.CI.BuildLog) && !strings.HasPrefix(p.CI.BuildLog, "/") {
				p.CI.BuildLog = strings.TrimRight(logsRoot, "/") + "/" + p.CI.BuildLog
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case portChan <- p:
		}
	}

	metaChan <- db
	return nil
}

// PrepareSiteData processes the database into a format suitable for view rendering.
func (c *Collector) PrepareSiteData(db *model.Database) *model.SiteData {
	data := &model.SiteData{
		Categories:       db.Categories,
		Ports:            db.Ports,
		PortMap:          make(map[string]*model.Port),
		SimplePortMap:    make(map[string]*model.Port),
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
		data.PortMap[p.Category+"/"+p.Name] = p
		if _, exists := data.SimplePortMap[p.Name]; !exists {
			data.SimplePortMap[p.Name] = p
		}
		for _, prov := range p.Provides {
			if _, exists := data.SimplePortMap[prov]; !exists {
				data.SimplePortMap[prov] = p
			}
		}

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

		// A port is considered "new" if its oldest commit is within the last 30 days
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

	c.finalizeContributorStats(data)
	c.finalizeRecipeStats(data)
	c.finalizeSizeStats(data)
	c.finalizeActivityStats(data, activity, updates, builds)

	return data
}

func (c *Collector) finalizeSizeStats(data *model.SiteData) {
	for _, p := range data.Ports {
		if p.CI != nil && p.CI.Size > 0 {
			data.TopSizes = append(data.TopSizes, p)
		}
	}
	sort.Slice(data.TopSizes, func(i, j int) bool {
		return data.TopSizes[i].CI.Size > data.TopSizes[j].CI.Size
	})
	if len(data.TopSizes) > 10 {
		data.TopSizes = data.TopSizes[:10]
	}
}

func (c *Collector) finalizeContributorStats(data *model.SiteData) {
	for _, v := range data.ContributorStats {
		data.AllAuthors = append(data.AllAuthors, v.Name)
		data.TopContributors = append(data.TopContributors, v)
		data.TotalCommits += v.Count
	}
	sort.Strings(data.AllAuthors)
	sort.Slice(data.TopContributors, func(i, j int) bool {
		return data.TopContributors[i].Count > data.TopContributors[j].Count
	})
}

func (c *Collector) finalizeRecipeStats(data *model.SiteData) {
	data.TopRecipes = make([]*model.Port, len(data.Ports))
	copy(data.TopRecipes, data.Ports)
	sort.Slice(data.TopRecipes, func(i, j int) bool {
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

func (c *Collector) finalizeActivityStats(data *model.SiteData, activity, updates, builds map[string]int) {
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
			Date:    dateStr,
			Count:   commitCount,
			Updates: updateCount,
			Builds:  buildCount,
		})
	}
}
