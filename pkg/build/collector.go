package build

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

// Data collector
//
type Collector struct {
	cfg      *config.Config
	reg      *registry.Registry
	scanner  model.Scanner
	manifest *cache.Manifest
}

// Initialisation
//
func NewCollector(cfg *config.Config, reg *registry.Registry, scanner model.Scanner, manifest *cache.Manifest) *Collector {
	return &Collector{cfg: cfg, reg: reg, scanner: scanner, manifest: manifest}
}

// Data streaming
//
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

	// Package scanning
	//
	if c.reg.PkgsRoot() != "" {
		source.ScanPackages(c.reg, ports)
	}

	// CI status loading
	//
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

	// Git data loading
	//
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

	// Parallel log processing
	//
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

	// Port streaming
	//
	for _, p := range ports {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case portChan <- p:
		}
	}

	metaChan <- db
	return nil
}

// Log processing
//
func (c *Collector) processLog(p *model.Port) {
	if p.CI == nil || p.CI.BuildLog == "" {
		return
	}

	// remote path detection
	//
	logsRoot := c.cfg.Metadata.LogsPath
	if logsRoot != "" && !config.IsRemote(p.CI.BuildLog) && !strings.HasPrefix(p.CI.BuildLog, "/") {
		p.CI.BuildLog = strings.TrimRight(logsRoot, "/") + "/" + p.CI.BuildLog
	}

	logName := "ci_log.txt"
	cachePath := filepath.Join(c.cfg.CacheDir, "logs", p.Category, p.Name, logName)
	cacheKey := fmt.Sprintf("log:%s:%d", p.Category+"/"+p.Name, p.CI.BuildStarted)

	var logContent []byte
	// Cache invalidation based on build timestamp
	//
	if !c.manifest.HasChanged(cacheKey, cacheKey) {
		logContent, _ = os.ReadFile(cachePath)
		if len(logContent) > 0 {
			c.manifest.MarkUsed(cacheKey)
		}
	}

	// fetch remote logs
	//
	if len(logContent) == 0 && config.IsRemote(p.CI.BuildLog) {
		resp, err := http.Get(p.CI.BuildLog)
		if err == nil && resp.StatusCode == http.StatusOK {
			logContent, _ = io.ReadAll(resp.Body)
			resp.Body.Close()

			// skip HTML error pages
			//
			trimmed := strings.TrimSpace(string(logContent))
			if strings.HasPrefix(strings.ToLower(trimmed), "<!doctype") || strings.HasPrefix(strings.ToLower(trimmed), "<html") {
				logContent = nil
			} else {
				os.MkdirAll(filepath.Dir(cachePath), 0755)
				os.WriteFile(cachePath, logContent, 0644)
				c.manifest.Update(cacheKey, cacheKey)
			}
		}
	} else if len(logContent) == 0 {
		// local file reading
		//
		logContent, _ = os.ReadFile(p.CI.BuildLog)
	}

	// data embedding
	//
	if len(logContent) > 0 {
		if p.FileContents == nil {
			p.FileContents = make(map[string]string)
		}
		p.FileContents[logName] = string(logContent)
	}
}

// Site data preparation
//
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

		// new port detection
		//
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

// Size statistics
//
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

// Contributor statistics
//
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

// Recipe statistics
//
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

// Activity statistics
//
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
