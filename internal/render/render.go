// Package render builds the two status lines.
package render

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/mountaser/claude-gauges/internal/gauge"
	"github.com/mountaser/claude-gauges/internal/gitinfo"
	"github.com/mountaser/claude-gauges/internal/session"
)

// Options controls rendering. A nil Git skips the git lookup.
type Options struct {
	Now   time.Time
	Color bool
	Git   func(dir string) (gitinfo.Info, bool)
}

const (
	// 24-bit colors chosen to stay readable on dark terminal themes. The basic
	// 16-color codes map to dark blues and purples on many themes.
	green   = "38;2;126;231;135"
	yellow  = "38;2;242;204;96"
	red     = "38;2;255;123;114"
	blue    = "38;2;121;192;255"
	magenta = "38;2;210;168;255"
	cyan    = "38;2;86;212;221"
	dim     = "38;2;139;148;158"
	bold    = "1"
	orange  = "38;2;255;166;87"
)

type painter struct{ on bool }

func (p painter) c(code, s string) string {
	if !p.on || code == "" || s == "" {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

// Render returns the two status lines joined by a newline.
func Render(s *session.Session, opt Options) string {
	p := painter{opt.Color}
	sep := p.c(dim, " │ ")

	l1 := projectSegments(s, opt, p)
	l1 = append(l1, modelSegment(s, p), contextSegment(s, p))
	l2 := []string{
		limitSegment("5h", s.RateLimits.FiveHour, opt.Now, p),
		limitSegment("7d", s.RateLimits.SevenDay, opt.Now, p),
		cacheSegment(s.PromptCache, opt.Now, p),
	}
	return strings.Join(l1, sep) + "\n" + strings.Join(l2, sep)
}

func projectSegments(s *session.Session, opt Options, p painter) []string {
	dir := s.Dir()
	if dir == "" {
		return nil
	}
	if opt.Git != nil {
		if g, ok := opt.Git(dir); ok {
			segs := []string{p.c(magenta, g.Repo)}
			if g.Branch != "" {
				b := p.c(green, g.Branch)
				if g.Worktree && !strings.Contains(g.Branch, g.Folder) && !strings.Contains(g.Folder, g.Branch) {
					b += " " + p.c(dim, "("+g.Folder+")")
				}
				segs = append(segs, b)
			}
			return segs
		}
	}
	return []string{p.c(magenta, filepath.Base(dir))}
}

func modelSegment(s *session.Session, p painter) string {
	out := p.c(blue, ModelName(s.Model.ID, s.Model.DisplayName))
	if s.Effort != nil && s.Effort.Level != "" {
		out += " " + p.c(effortColor(s.Effort.Level), "· "+s.Effort.Level)
	}
	return out
}

func effortColor(level string) string {
	switch level {
	case "low":
		return dim
	case "medium":
		return cyan
	case "high":
		return magenta
	case "xhigh":
		return orange
	case "max":
		return red
	}
	return ""
}

func contextSegment(s *session.Session, p painter) string {
	label := p.c(dim, "ctx")
	size := int64(s.ContextWindow.Size)
	if size <= 0 {
		return label + " –"
	}
	used := int64(s.ContextWindow.TotalInputTokens)
	f := float64(used) / float64(size)
	col := green
	switch {
	case f >= 0.8:
		col = red
	case f >= 0.5:
		col = yellow
	}
	return label + " " + p.c(col, gauge.Pie(f)) + " " +
		p.c(bold, gauge.Tokens(used)) + p.c(dim, "/"+gauge.Tokens(size))
}

func limitSegment(name string, w *session.Window, now time.Time, p painter) string {
	label := p.c(dim, name)
	if w == nil {
		return label + " –"
	}
	// Round first so the pie, the color and the number shown always agree.
	rem := math.Round(math.Max(0, math.Min(100, 100-float64(w.UsedPercentage))))
	col := red
	switch {
	case rem > 50:
		col = green
	case rem >= 20:
		col = yellow
	}
	out := fmt.Sprintf("%s %s %s", label, p.c(col, gauge.Pie(rem/100)), p.c(bold, fmt.Sprintf("%.0f%%", rem)))
	if w.ResetsAt > 0 {
		left := time.Unix(int64(w.ResetsAt), 0).Sub(now)
		out += " " + p.c(dim, "↺ "+gauge.Countdown(left))
	}
	return out
}

func cacheSegment(c *session.PromptCache, now time.Time, p painter) string {
	label := p.c(dim, "cache")
	if c == nil || !c.CachingObserved {
		return label + " –"
	}
	left := time.Unix(int64(c.ExpiresAt), 0).Sub(now)
	if c.Warm && left > 0 {
		if left < time.Minute {
			return label + " " + p.c(yellow, "◐ "+gauge.CacheCountdown(left))
		}
		return label + " " + p.c(green, "● "+gauge.CacheCountdown(left))
	}
	cold := "▲ cold"
	if c.RecacheTokensIfCold > 0 {
		cold += " · +" + gauge.Tokens(int64(c.RecacheTokensIfCold)) + " tokens to reload"
	}
	return p.c(orange, cold)
}
