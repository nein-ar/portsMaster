package views

import (
	"path/filepath"
	"strings"

	"github.com/a-h/templ"
)

// Href calculates the relative path from the current page to the target.
// target should be an absolute path from the site root, starting with "/".
func Href(current, target string) templ.SafeURL {
	if strings.HasPrefix(target, "http") || strings.HasPrefix(target, "//") {
		return templ.SafeURL(target)
	}

	if !strings.HasPrefix(target, "/") {
		return templ.SafeURL(target)
	}

	// Split path and query
	parts := strings.SplitN(target, "?", 2)
	path := strings.TrimPrefix(parts[0], "/")
	query := ""
	if len(parts) > 1 {
		query = "?" + parts[1]
	}

	baseDir := filepath.Dir(current)
	if baseDir == "." {
		baseDir = ""
	}

	rel, err := filepath.Rel(baseDir, path)
	if err != nil {
		return templ.SafeURL("/" + path + query)
	}

	rel = filepath.ToSlash(rel)
	return templ.SafeURL(rel + query)
}
