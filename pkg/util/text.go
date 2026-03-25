package util

import (
	"encoding/hex"
	"io"
	"os"
	"regexp"

	"lukechampine.com/blake3"
)

// File hashing
//
func HashFiles(paths ...string) string {
	h := blake3.New(32, nil)
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		io.Copy(h, f)
		f.Close()
	}
	return hex.EncodeToString(h.Sum(nil))
}

var mdLinkRegex = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

// Markdown stripping
//
func StripMarkdownLinks(text string) string {
	return mdLinkRegex.ReplaceAllString(text, "$1")
}
