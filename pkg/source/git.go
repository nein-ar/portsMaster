package source

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"portsMaster/pkg/model"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type GitProvider struct {
	repo *git.Repository
}

func NewGitProvider(path string) (*GitProvider, error) {
	r, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open git repo at %s: %w", path, err)
	}
	return &GitProvider{repo: r}, nil
}

type cachedGitData struct {
	HeadHash         string                        `json:"head"`
	PortCommits      map[string][]*model.Commit    `json:"port_commits"`
	RecentCommits    []*model.Commit               `json:"recent"`
	ContributorStats map[string]*model.Contributor `json:"contributors"`
}

func (g *GitProvider) GetRepositoryDataCached(ports []*model.Port, cacheDir string) (map[string][]*model.Commit, []*model.Commit, map[string]*model.Contributor, error) {
	ref, err := g.repo.Head()
	if err != nil {
		return nil, nil, nil, err
	}
	currentHead := ref.Hash().String()

	cachePath := filepath.Join(cacheDir, "git_history.json")
	if f, err := os.Open(cachePath); err == nil {
		var cache cachedGitData
		if err := json.NewDecoder(f).Decode(&cache); err == nil {
			f.Close()
			if cache.HeadHash == currentHead {
				return cache.PortCommits, cache.RecentCommits, cache.ContributorStats, nil
			}
		} else {
			f.Close()
		}
	}

	m, r, stats, err := g.GetRepositoryData(ports)
	if err != nil {
		return nil, nil, nil, err
	}

	_ = os.MkdirAll(cacheDir, 0755)
	if f, err := os.Create(cachePath); err == nil {
		cache := cachedGitData{
			HeadHash:         currentHead,
			PortCommits:      m,
			RecentCommits:    r,
			ContributorStats: stats,
		}
		_ = json.NewEncoder(f).Encode(cache)
		f.Close()
	}

	return m, r, stats, nil
}

func (g *GitProvider) GetRepositoryData(ports []*model.Port) (map[string][]*model.Commit, []*model.Commit, map[string]*model.Contributor, error) {
	ref, err := g.repo.Head()
	if err != nil {
		return nil, nil, nil, err
	}

	cIter, err := g.repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return nil, nil, nil, err
	}

	portCommits := make(map[string][]*model.Commit)
	contributorStats := make(map[string]*model.Contributor)
	var recentCommits []*model.Commit

	interested := make(map[string]bool)
	for _, p := range ports {
		interested[p.Category+"/"+p.Name] = true
	}

	const recentLimit = 100
	count := 0

	err = cIter.ForEach(func(c *object.Commit) error {
		                email := strings.ToLower(strings.TrimSpace(c.Author.Email))
		                if email == "" {
		                        email = "unknown"
		                }
		
		                stats, ok := contributorStats[email]
		                if !ok {
		                        stats = &model.Contributor{Name: c.Author.Name, Email: email}
		                        contributorStats[email] = stats
		                }
		                stats.Count++
		                if c.Author.Name != stats.Name {
		                        found := false
		                        for _, n := range stats.OtherNames {
		                                if n == c.Author.Name {
		                                        found = true
		                                        break
		                                }
		                        }
		                        if !found {
		                                stats.OtherNames = append(stats.OtherNames, c.Author.Name)
		                        }
		                }
		mc := &model.Commit{
			Hash:    c.Hash.String(),
			Author:  c.Author.Name,
			Email:   c.Author.Email,
			Date:    c.Author.When,
			Message: strings.TrimSpace(c.Message),
			IsMerge: c.NumParents() > 1,
		}

		if count < recentLimit {
			recentCommits = append(recentCommits, mc)
		}

		parent, _ := c.Parent(0)
		var changes object.Changes
		if parent != nil {
			pTree, _ := parent.Tree()
			cTree, _ := c.Tree()
			changes, _ = pTree.Diff(cTree)
		} else {
			cTree, _ := c.Tree()
			_ = cTree.Files().ForEach(func(f *object.File) error {
				changes = append(changes, &object.Change{To: object.ChangeEntry{Name: f.Name}})
				return nil
			})
		}

		seenInCommit := make(map[string]bool)
		for _, ch := range changes {
			file := ch.To.Name
			if file == "" {
				file = ch.From.Name
			}

			if count < recentLimit {
				action, _ := ch.Action()
				switch action.String() {
				case "Insert":
					mc.AddedFiles = append(mc.AddedFiles, file)
				case "Delete":
					mc.DeletedFiles = append(mc.DeletedFiles, file)
				case "Modify":
					mc.ModifiedFiles = append(mc.ModifiedFiles, file)
				}
			}

			parts := strings.Split(file, "/")
			if len(parts) >= 2 {
				key := parts[0] + "/" + parts[1]
				if interested[key] && !seenInCommit[key] {
					seenInCommit[key] = true
					portCommits[key] = append(portCommits[key], mc)
				}
			}
		}

		count++
		return nil
	})

	if err != nil && err.Error() != "done" {
		return nil, nil, nil, err
	}

	return portCommits, recentCommits, contributorStats, nil
}
