package render

import (
	"strings"
	"testing"
	"time"

	"github.com/mountaser/claude-gauges/internal/agy"
)

func ip(n int) *int { return &n }

func act(kind, target string, ago time.Duration) *agy.Activity {
	return &agy.Activity{Kind: kind, Target: target, At: now.Add(-ago).Format(time.RFC3339)}
}

func at(d time.Duration) string { return now.Add(d).Format(time.RFC3339) }

func typical() *agy.Snapshot {
	return &agy.Snapshot{
		Version: 1,
		Active:  true,
		Plan: &agy.Plan{
			Steps:      []string{"passed", "passed", "running", "pending", "pending"},
			Done:       2,
			ElapsedSec: 651,
			Diff:       &agy.DiffStat{Files: 2, Added: 217},
		},
		Jobs: []agy.Job{{
			ID: "c", Step: ip(4), Title: "Render the two lines", State: "running", Phase: "run",
			Activity: act("editing", "render.go", 4*time.Second),
		}},
		Quota: &agy.Quota{
			FiveHour: &agy.Window{Remaining: 94, ResetAt: at(3*time.Hour + 12*time.Minute)},
			Weekly:   &agy.Window{Remaining: 70, ResetAt: at(4 * 24 * time.Hour)},
		},
	}
}

func plainLines(s *agy.Snapshot, columns int) (string, string) {
	return AgyLines(s, now, columns, false)
}

func TestAgyTypical(t *testing.T) {
	a, b := plainLines(typical(), 0)
	wantA := "agy-foreman ●●◐○○ 2/5 │ #4 Render the two lines › editing render.go"
	wantB := "5h ● 94% ↺ 3h12m │ 7d ◕ 70% ↺ 4d 0h │ plan 10m51s │ diff 2 files +217"
	if a != wantA {
		t.Errorf("line 3:\n%s\nwant:\n%s", a, wantA)
	}
	if b != wantB {
		t.Errorf("line 4:\n%s\nwant:\n%s", b, wantB)
	}
}

func TestAgyParallelJobs(t *testing.T) {
	s := typical()
	s.Plan.Steps = []string{"passed", "running", "running", "running", "pending"}
	s.Plan.Done = 1
	s.Jobs = []agy.Job{
		{ID: "a", Step: ip(2), Title: "Gauge formatting", State: "running", Phase: "run", Activity: act("editing", "gauge.go", time.Second)},
		{ID: "b", Step: ip(3), Title: "Git detection", State: "running", Phase: "run", Activity: act("testing", "", time.Second)},
		{ID: "c", Step: ip(6), Title: "Install logic", State: "running", Phase: "run", Activity: act("thinking", "", 8*time.Second)},
	}
	a, _ := plainLines(s, 0)
	want := "agy-foreman ●◐◐◐○ 1/5 · 3 running │ #2 editing gauge.go · #3 running tests · #6 thinking 8s"
	if a != want {
		t.Errorf("got:\n%s\nwant:\n%s", a, want)
	}
}

func TestAgyFailedStep(t *testing.T) {
	s := typical()
	s.Plan.Steps = []string{"passed", "passed", "passed", "failed", "running"}
	s.Plan.Done, s.Plan.Failed = 3, 1
	s.Jobs = []agy.Job{{ID: "c", Step: ip(6), Title: "Install logic", State: "running", Phase: "run",
		Activity: act("reading", "brief.md", time.Second)}}
	a, _ := plainLines(s, 0)
	want := "agy-foreman ●●●●◐ 3/5 · 1 failed │ #6 Install logic › reading brief.md"
	if a != want {
		t.Errorf("got:\n%s\nwant:\n%s", a, want)
	}
}

func TestAgyActivityWording(t *testing.T) {
	cases := []struct {
		phase string
		act   *agy.Activity
		want  string
	}{
		{"queued", nil, "queued"},
		{"awaiting_approval", nil, "waiting for approval"},
		{"prepare", nil, "preparing worktree"},
		{"contract", nil, "checking contract tests"},
		{"brief", nil, "writing brief"},
		{"verify", nil, "verifying"},
		{"report", nil, "committing"},
		{"run", nil, "working"},
		{"run", act("reading", "", time.Second), "reading"},
		{"run", act("testing", "", time.Second), "running tests"},
		{"run", act("git", "", time.Second), "checking changes"},
		{"run", act("command", "", time.Second), "running a command"},
		{"run", act("thinking", "", 2*time.Second), "thinking"},
		{"run", act("thinking", "", 5*time.Second), "thinking 5s"},
	}
	for _, c := range cases {
		s := typical()
		s.Jobs = []agy.Job{{ID: "x", Title: "Gauge formatting", State: "running", Phase: c.phase, Activity: c.act}}
		a, _ := plainLines(s, 0)
		want := "agy-foreman ●●◐○○ 2/5 │ Gauge formatting › " + c.want
		if a != want {
			t.Errorf("%s/%v:\n%s\nwant:\n%s", c.phase, c.act, a, want)
		}
	}
}

func TestAgyDiffAndMissingParts(t *testing.T) {
	cases := []struct {
		diff *agy.DiffStat
		want string
	}{
		{nil, "diff –"},
		{&agy.DiffStat{}, "diff –"},
		{&agy.DiffStat{Files: 1, Added: 63}, "diff 1 file +63"},
		{&agy.DiffStat{Files: 2, Added: 217, Deleted: 12}, "diff 2 files +217 −12"},
	}
	for _, c := range cases {
		s := typical()
		s.Plan.Diff = c.diff
		_, b := plainLines(s, 0)
		if !strings.HasSuffix(b, c.want) {
			t.Errorf("line 4 %q should end with %q", b, c.want)
		}
	}

	s := typical()
	s.Quota = nil
	if _, b := plainLines(s, 0); b != "plan 10m51s │ diff 2 files +217" {
		t.Errorf("without quota: %q", b)
	}

	s = typical()
	s.Quota.FiveHour = nil
	if _, b := plainLines(s, 0); !strings.HasPrefix(b, "7d ◕ 70% ↺ 4d 0h │ plan") {
		t.Errorf("without the 5h limit: %q", b)
	}

	s = typical()
	s.Quota.FiveHour.Remaining = 8
	if _, b := plainLines(s, 0); !strings.HasPrefix(b, "5h ○ 8% ↺ 3h12m │") {
		t.Errorf("nearly empty limit: %q", b)
	}

	s = typical()
	s.Plan.ElapsedSec = 4
	if _, b := plainLines(s, 0); !strings.Contains(b, "plan 0m04s │ diff") {
		t.Errorf("fresh plan: %q", b)
	}
}

func TestAgyFitsNarrowTerminals(t *testing.T) {
	fullB := "5h ● 94% ↺ 3h12m │ 7d ◕ 70% ↺ 4d 0h │ plan 10m51s │ diff 2 files +217"
	noCountdown := "5h ● 94% │ 7d ◕ 70% │ plan 10m51s │ diff 2 files +217"
	cases := []struct {
		columns int
		want    string
	}{
		{0, fullB}, {69, fullB}, {68, noCountdown}, {53, noCountdown}, {40, noCountdown},
	}
	for _, c := range cases {
		if _, b := plainLines(typical(), c.columns); b != c.want {
			t.Errorf("columns %d:\n%s\nwant:\n%s", c.columns, b, c.want)
		}
	}

	a, _ := plainLines(typical(), 60)
	want := "agy-foreman ●●◐○○ 2/5 │ #4 Render the t… › editing render.go"
	if a != want {
		t.Errorf("shortened title:\n%s\nwant:\n%s", a, want)
	}
	if len([]rune(a)) != 60 {
		t.Errorf("line 3 should be exactly 60 wide, got %d", len([]rune(a)))
	}
	if a, _ := plainLines(typical(), 30); a != "agy-foreman ●●◐○○ 2/5 │ #4 Render the two lines › editing render.go" {
		t.Errorf("a title that cannot keep 8 characters stays whole: %q", a)
	}
}

func TestAgyColors(t *testing.T) {
	s := typical()
	s.Plan.Steps = []string{"passed", "failed", "running", "pending", "pending"}
	s.Plan.Failed = 1
	a, b := AgyLines(s, now, 0, true)
	if !strings.Contains(a, "\x1b["+red+"m●\x1b[0m") {
		t.Errorf("expected a red failed step in %q", a)
	}
	if !strings.Contains(a, "\x1b["+green+"m●\x1b[0m") {
		t.Errorf("expected a green passed step in %q", a)
	}
	if !strings.Contains(b, "\x1b["+green+"m+217\x1b[0m") {
		t.Errorf("expected green added lines in %q", b)
	}
	if plainA, plainB := plainLines(s, 0); strings.Contains(plainA+plainB, "\x1b") {
		t.Error("plain output must not contain escape sequences")
	}
}
