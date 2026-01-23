package util

import (
	"fmt"
	"portsMaster/pkg/model"
	"strings"
	"time"
)

func FormatContributorTooltip(c *model.Contributor) string {
	var validOthers []string
	seen := make(map[string]bool)
	for _, n := range c.OtherNames {
		n = strings.TrimSpace(n)
		if n != "" && n != c.Name && !seen[n] {
			validOthers = append(validOthers, n)
			seen[n] = true
		}
	}
	if len(validOthers) == 0 {
		return c.Email
	}
	return fmt.Sprintf("Previously: %s | %s", strings.Join(validOthers, ", "), c.Email)
}

func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func FormatDuration(sec int64) string {
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	if sec < 3600 {
		return fmt.Sprintf("%dm %ds", sec/60, sec%60)
	}
	return fmt.Sprintf("%dh %dm", sec/3600, (sec%3600)/60)
}

func FormatUnix(ts int64) string {
	if ts == 0 {
		return "-"
	}
	return time.Unix(ts, 0).Format("2006-01-02 15:04")
}

func FormatFinishTime(start, dur int64) string {
	if start == 0 {
		return "-"
	}
	return time.Unix(start+dur, 0).Format("2006-01-02 15:04")
}

func FormatTimeAgo(t time.Time) string {
	d := time.Since(t)
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	days := int(d.Hours()) / 24
	if days < 30 {
		return fmt.Sprintf("%dd ago", days)
	}
	return t.Format("2006-01-02")
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Plural(n int, unit string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, unit)
	}
	return fmt.Sprintf("%d %ss", n, unit)
}
