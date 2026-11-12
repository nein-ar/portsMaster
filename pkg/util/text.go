package util

import "regexp"

var mdLinkRegex = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

// Markdown stripping
func StripMarkdownLinks(text string) string {
	return mdLinkRegex.ReplaceAllString(text, "$1")
}
