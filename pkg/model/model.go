package model

import (
	"time"
)

type DepType string

const (
	DepBuild DepType = "build"
	DepRun   DepType = "run"
	DepLink  DepType = "link"
)

type Dependency struct {
	Name string  `cbor:"name" json:"name"`
	Type DepType `cbor:"type" json:"type"`
}

type PackageInfo struct {
	Filename string `cbor:"filename" json:"filename"`
	Path     string `cbor:"path" json:"path"`
	Size     int64  `cbor:"size" json:"size"`
}

type Port struct {
	Name           string        `cbor:"name" json:"name"`
	Category       string        `cbor:"category" json:"category"`
	Description    string        `cbor:"description" json:"description"`
	Version        string        `cbor:"version" json:"version"`
	Release        string        `cbor:"release" json:"release"`
	License        string        `cbor:"license" json:"license"`
	Upstream       string        `cbor:"upstream" json:"upstream"`
	Maintainer     string        `cbor:"maintainer" json:"maintainer"`
	FilePath       string        `cbor:"file_path" json:"file_path"`
	Hash           string        `cbor:"hash,omitempty" json:"hash,omitempty"`
	LastCommit     *Commit       `cbor:"last_commit,omitempty" json:"last_commit,omitempty"`
	Commits        []*Commit     `cbor:"commits,omitempty" json:"commits,omitempty"`
	Deps           []Dependency  `cbor:"deps,omitempty" json:"deps,omitempty"`
	Provides       []string      `cbor:"provides,omitempty" json:"provides,omitempty"`
	Packages       []PackageInfo `cbor:"packages,omitempty" json:"packages,omitempty"`
	IsBroken       bool          `cbor:"is_broken" json:"is_broken"`
	IsUnmaintained bool          `cbor:"is_unmaintained" json:"is_unmaintained"`
	CI             *CIInfo       `cbor:"ci,omitempty" json:"ci,omitempty"`
	RecipeLines    int           `cbor:"recipe_lines" json:"recipe_lines"`
}

type CIInfo struct {
	Status            string `cbor:"status" json:"status"`
	BuildLog          string `json:"build_log"`
	BuildDuration     int64  `json:"build_duration"`
	BuildStarted      int64  `json:"build_started"`
	Size              int64  `json:"size"`
	BuilderInfo       string `json:"builder_info"`
	InstalledSize     int64  `json:"installed_size"`
	DepsSize          int64  `json:"deps_size"`
	DepsInstalledSize int64  `json:"deps_installed_size"`
}

type Category struct {
	Name        string  `cbor:"name" json:"name"`
	Description string  `cbor:"description" json:"description"`
	Ports       []*Port `cbor:"ports,omitempty" json:"ports,omitempty"`
}

type Commit struct {
	Hash          string    `cbor:"hash" json:"hash"`
	Author        string    `cbor:"author" json:"author"`
	Email         string    `cbor:"email" json:"email"`
	Date          time.Time `cbor:"date" json:"date"`
	Message       string    `cbor:"message" json:"message"`
	AddedFiles    []string  `cbor:"added_files,omitempty" json:"added_files,omitempty"`
	ModifiedFiles []string  `cbor:"modified_files,omitempty" json:"modified_files,omitempty"`
	DeletedFiles  []string  `cbor:"deleted_files,omitempty" json:"deleted_files,omitempty"`
	IsMerge       bool      `cbor:"is_merge" json:"is_merge"`
}

type Database struct {
	Categories       []*Category             `cbor:"categories" json:"categories"`
	Ports            []*Port                 `cbor:"ports" json:"ports"`
	RecentCommits    []*Commit               `cbor:"recent_commits" json:"recent_commits"`
	ContributorStats map[string]*Contributor `cbor:"contributor_stats" json:"contributor_stats"`
	GeneratedAt      time.Time               `cbor:"generated_at" json:"generated_at"`
}

type SiteData struct {
	Categories        []*Category
	Ports             []*Port
	PortMap           map[string]*Port
	SimplePortMap     map[string]*Port
	RecentCommits     []*Commit
	TotalPorts        int
	BrokenCount       int
	UnmaintainedCount int
	UpdatedThisWeek   int
	AllAuthors        []string
	AllTimeframes     []string

	CommitActivity   map[string]int
	DailyStats       []DailyStat
	ContributorStats map[string]*Contributor
	TopContributors  []*Contributor
	LicenseStats     map[string]int
	BuildStats       BuildStats
	MaxDailyCommits  int
	LastUpdate       time.Time

	TopRecipes         []*Port
	TotalRecipeLines   int
	Top5LinePercentage float64
	TotalCommits       int
	TopSizes           []*Port
}

type Contributor struct {
	Name       string   `cbor:"name" json:"name"`
	Email      string   `cbor:"email" json:"email"`
	Count      int      `cbor:"count" json:"count"`
	OtherNames []string `cbor:"other_names" json:"other_names"`
}

type BuildStats struct {
	Total     int
	Success   int
	Failed    int
	TotalTime int64
	AvgTime   int64
}

type DailyStat struct {
	Date    string
	Count   int
	Updates int
	Builds  int
}
