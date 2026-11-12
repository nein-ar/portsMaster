package sitegen

import (
	"testing"
	"time"

	"portsMaster/pkg/model"
)

func TestViewsAreDeterministic(t *testing.T) {
	commit := &model.Commit{Hash: "abc", Date: time.Unix(1700000000, 0), Message: "one"}
	ports := []*model.Port{
		{Name: "b", Category: "z", Description: "second", Version: "1", Files: []string{"info"}},
		{Name: "a", Category: "z", Description: "first", Version: "1", Files: []string{"info"}, LastCommit: commit},
	}
	data := &model.SiteData{
		Ports:           ports,
		TotalPorts:      2,
		TotalCommits:    3,
		RecentCommits:   []*model.Commit{commit},
		LicenseStats:    map[string]int{"MIT": 2},
		LastUpdate:      commit.Date,
		Categories:      []*model.Category{{Name: "z", Ports: ports}},
		TopContributors: []*model.Contributor{{Name: "dev", Email: "dev@example", Count: 3}},
	}

	view := buildPortsView(data)
	if len(view.List) != 2 || view.List[0] != "z/a" || view.List[1] != "z/b" {
		t.Fatalf("port list is not sorted: %v", view.List)
	}
	if view.ByName["a"] != "z/a" || view.ByName["b"] != "z/b" {
		t.Fatalf("name index is wrong: %v", view.ByName)
	}

	search := buildSearchIndex(ports)
	if len(search) != 2 || search[0].N != "a" || search[1].N != "b" {
		t.Fatalf("search index is not sorted: %+v", search)
	}

	site := buildSiteView(data)
	if len(site.Recent) != 2 || site.Recent[0] != "z/a" {
		t.Fatalf("recent list is wrong: %v", site.Recent)
	}
	if site.Categories[0].Name != "z" || site.Categories[0].Count != 2 {
		t.Fatalf("category view is wrong: %+v", site.Categories[0])
	}
}
