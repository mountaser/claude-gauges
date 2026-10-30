package render

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mountaser/claude-gauges/internal/agy"
	"github.com/mountaser/claude-gauges/internal/gauge"
)

var agySpinner = []string{"◐", "◓", "◑", "◒"}

// AgyLines returns the two delegation lines. columns 0 means no width limit.
func AgyLines(s *agy.Snapshot, now time.Time, columns int, color bool) (string, string) {
	return agyLineA(s, now, columns, color), agyLineB(s, now, columns, color)
}

func agyStrip(steps []string, now time.Time, p painter) (string, int) {
	frame := agySpinner[int(((now.Unix()%4)+4)%4)]
	var b strings.Builder
	running := 0
	for _, st := range steps {
		switch st {
		case "passed":
			b.WriteString(p.c(green, "●"))
		case "running":
			running++
			b.WriteString(p.c(cyan, frame))
		case "failed":
			b.WriteString(p.c(red, "●"))
		default:
			b.WriteString(p.c(dim, "○"))
		}
	}
	return b.String(), running
}

func agyVerb(p painter, verb, target string) string {
	if target == "" {
		return p.c(cyan, verb)
	}
	return p.c(cyan, verb) + " " + p.c(bold, target)
}

func agyActivity(j agy.Job, now time.Time, p painter) string {
	if j.Phase != "run" {
		switch j.Phase {
		case "queued":
			return p.c(dim, "queued")
		case "awaiting_approval":
			return p.c(yellow, "waiting for approval")
		case "prepare":
			return p.c(cyan, "preparing worktree")
		case "contract":
			return p.c(cyan, "checking contract tests")
		case "brief":
			return p.c(cyan, "writing brief")
		case "verify":
			return p.c(cyan, "verifying")
		case "report":
			return p.c(cyan, "committing")
		}
		return p.c(cyan, j.Phase)
	}
	a := j.Activity
	if a == nil {
		return p.c(cyan, "working")
	}
	switch a.Kind {
	case "reading":
		return agyVerb(p, "reading", a.Target)
	case "editing":
		return agyVerb(p, "editing", a.Target)
	case "testing":
		return p.c(cyan, "running tests")
	case "git":
		return p.c(cyan, "checking changes")
	case "command":
		return p.c(cyan, "running a command")
	case "thinking":
		secs := 0
		if t, err := time.Parse(time.RFC3339, a.At); err == nil {
			secs = int(now.Sub(t).Seconds())
		}
		if secs >= 5 {
			return p.c(cyan, fmt.Sprintf("thinking %ds", secs))
		}
		return p.c(cyan, "thinking")
	}
	return p.c(cyan, "working")
}

func agyJobs(jobs []agy.Job, now time.Time, titleMax int, p painter) string {
	if len(jobs) == 0 {
		return ""
	}
	if len(jobs) == 1 {
		j := jobs[0]
		out := ""
		if j.Step != nil {
			out += p.c(dim, fmt.Sprintf("#%d", *j.Step)) + " "
		}
		title := j.Title
		if titleMax > 0 && utf8.RuneCountInString(title) > titleMax {
			title = string([]rune(title)[:titleMax-1]) + "…"
		}
		return out + p.c(bold, title) + " " + p.c(dim, "›") + " " + agyActivity(j, now, p)
	}
	parts := make([]string, 0, len(jobs))
	for _, j := range jobs {
		s := ""
		if j.Step != nil {
			s = p.c(dim, fmt.Sprintf("#%d", *j.Step)) + " "
		}
		parts = append(parts, s+agyActivity(j, now, p))
	}
	return strings.Join(parts, p.c(dim, " · "))
}

func agyLineA(s *agy.Snapshot, now time.Time, columns int, color bool) string {
	build := func(p painter, titleMax int) string {
		strip, running := agyStrip(s.Plan.Steps, now, p)
		head := p.c(magenta, "agy-foreman") + " " + strip + " " +
			p.c(bold, fmt.Sprintf("%d/%d", s.Plan.Done, len(s.Plan.Steps)))
		if running > 1 {
			head += " " + p.c(dim, fmt.Sprintf("· %d running", running))
		}
		if s.Plan.Failed > 0 {
			head += " " + p.c(red, fmt.Sprintf("· %d failed", s.Plan.Failed))
		}
		if jobs := agyJobs(s.Jobs, now, titleMax, p); jobs != "" {
			head += p.c(dim, " │ ") + jobs
		}
		return head
	}
	if columns > 0 && len(s.Jobs) == 1 {
		width := utf8.RuneCountInString(build(painter{false}, 0))
		if width > columns {
			keep := utf8.RuneCountInString(s.Jobs[0].Title) - (width - columns)
			if keep >= 8 {
				return build(painter{color}, keep)
			}
		}
	}
	return build(painter{color}, 0)
}

func limitColor(rem float64) string {
	switch {
	case rem > 50:
		return green
	case rem >= 20:
		return yellow
	}
	return red
}

func agyLimit(name string, w *agy.Window, now time.Time, p painter, countdown bool) string {
	rem := math.Round(math.Max(0, math.Min(100, w.Remaining)))
	out := p.c(dim, name) + " " + p.c(limitColor(rem), gauge.Pie(rem/100)) + " " +
		p.c(bold, fmt.Sprintf("%.0f%%", rem))
	if countdown {
		if t, err := time.Parse(time.RFC3339, w.ResetAt); err == nil {
			out += " " + p.c(dim, "↺ "+gauge.Countdown(t.Sub(now)))
		}
	}
	return out
}

func agyDiff(d *agy.DiffStat, p painter) string {
	label := p.c(dim, "diff")
	if d == nil || d.Files == 0 {
		return label + " " + p.c(dim, "–")
	}
	unit := "files"
	if d.Files == 1 {
		unit = "file"
	}
	out := label + " " + p.c(bold, fmt.Sprint(d.Files)) + " " + p.c(dim, unit) + " " +
		p.c(green, fmt.Sprintf("+%d", d.Added))
	if d.Deleted > 0 {
		out += " " + p.c(red, fmt.Sprintf("−%d", d.Deleted))
	}
	return out
}

func agyLineBWith(s *agy.Snapshot, now time.Time, p painter, countdown, tok bool) string {
	var segs []string
	if q := s.Quota; q != nil {
		if q.FiveHour != nil {
			segs = append(segs, agyLimit("5h", q.FiveHour, now, p, countdown))
		}
		if q.Weekly != nil {
			segs = append(segs, agyLimit("7d", q.Weekly, now, p, countdown))
		}
	}
	elapsed := time.Duration(s.Plan.ElapsedSec * float64(time.Second))
	segs = append(segs, p.c(dim, "plan")+" "+p.c(bold, gauge.CacheCountdown(elapsed)))
	if tok {
		segs = append(segs, p.c(dim, "tok")+" "+p.c(bold, gauge.Tokens(s.Plan.Tokens)))
	}
	segs = append(segs, agyDiff(s.Plan.Diff, p))
	return strings.Join(segs, p.c(dim, " │ "))
}

func agyLineB(s *agy.Snapshot, now time.Time, columns int, color bool) string {
	variants := [][2]bool{{true, true}, {false, true}, {false, false}}
	for _, v := range variants {
		plain := agyLineBWith(s, now, painter{false}, v[0], v[1])
		if columns <= 0 || utf8.RuneCountInString(plain) <= columns {
			return agyLineBWith(s, now, painter{color}, v[0], v[1])
		}
	}
	last := variants[len(variants)-1]
	return agyLineBWith(s, now, painter{color}, last[0], last[1])
}
