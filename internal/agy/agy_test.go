package agy

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var now = time.Date(2026, 10, 30, 14, 30, 0, 0, time.UTC)

const tmpl = `{
  "version": %d, "updatedAt": %q, "pid": 42, "active": %t,
  "plan": %s,
  "jobs": [{"id": "c", "step": 4, "title": "Render the two lines", "state": "running", "phase": "run",
            "activity": {"kind": "editing", "target": "render.go", "at": "2026-10-30T14:29:56.000Z"}}],
  "quota": {"fiveHour": {"remaining": 94, "resetAt": "2026-10-30T17:42:00Z"},
            "weekly": {"remaining": 70, "resetAt": "2026-11-03T00:00:00Z"},
            "fetchedAt": "2026-10-30T14:29:00.000Z"}
}`

const goodPlan = `{"file": "docs/p.md", "steps": ["passed", "passed", "running", "pending", "pending"],
  "done": 2, "failed": 0, "elapsedSec": 651, "tokens": 424000,
  "diff": {"files": 2, "added": 217, "deleted": 0}}`

func doc(version int, updated time.Time, active bool, plan string) []byte {
	return []byte(fmt.Sprintf(tmpl, version, updated.Format(time.RFC3339Nano), active, plan))
}

func TestParseValid(t *testing.T) {
	s, ok := Parse(doc(1, now.Add(-2*time.Second), true, goodPlan), now)
	if !ok {
		t.Fatal("expected a usable snapshot")
	}
	if s.Pid != 42 || !s.Active {
		t.Errorf("header = %+v", s)
	}
	p := s.Plan
	if p.Done != 2 || len(p.Steps) != 5 || p.Steps[2] != "running" ||
		p.ElapsedSec != 651 || p.Diff == nil || p.Diff.Files != 2 || p.Diff.Added != 217 {
		t.Errorf("plan = %+v", p)
	}
	if p.File == nil || *p.File != "docs/p.md" {
		t.Errorf("plan file = %v", p.File)
	}
	if len(s.Jobs) != 1 || s.Jobs[0].ID != "c" || s.Jobs[0].Step == nil || *s.Jobs[0].Step != 4 ||
		s.Jobs[0].Activity == nil || s.Jobs[0].Activity.Kind != "editing" ||
		s.Jobs[0].Activity.Target != "render.go" {
		t.Errorf("jobs = %+v", s.Jobs)
	}
	if s.Quota == nil || s.Quota.FiveHour == nil || s.Quota.FiveHour.Remaining != 94 ||
		s.Quota.Weekly == nil || s.Quota.Weekly.ResetAt != "2026-11-03T00:00:00Z" {
		t.Errorf("quota = %+v", s.Quota)
	}
}

func TestParseFreshness(t *testing.T) {
	if _, ok := Parse(doc(1, now.Add(-15*time.Second), true, goodPlan), now); !ok {
		t.Error("15 seconds old is still fresh")
	}
	if _, ok := Parse(doc(1, now.Add(-16*time.Second), true, goodPlan), now); ok {
		t.Error("16 seconds old is stale")
	}
}

func TestParseRejects(t *testing.T) {
	fresh := now.Add(-time.Second)
	cases := map[string][]byte{
		"inactive":      doc(1, fresh, false, goodPlan),
		"other version": doc(2, fresh, true, goodPlan),
		"no plan":       doc(1, fresh, true, "null"),
		"no steps":      doc(1, fresh, true, `{"steps": []}`),
		"not json":      []byte("not json"),
		"bad time":      []byte(`{"version":1,"updatedAt":"yesterday","active":true,"plan":{"steps":["running"]}}`),
	}
	for name, data := range cases {
		if _, ok := Parse(data, now); ok {
			t.Errorf("%s: expected the snapshot to be rejected", name)
		}
	}
}

func TestParseIgnoresUnknownFields(t *testing.T) {
	data := []byte(`{"version":1,"updatedAt":"2026-10-30T14:29:59Z","active":true,
		"plan":{"steps":["running"],"done":0,"failed":0,"elapsedSec":1,"tokens":0},
		"jobs":[],"future":{"x":1}}`)
	if _, ok := Parse(data, now); !ok {
		t.Error("unknown fields must be ignored")
	}
}

func TestRead(t *testing.T) {
	dir := t.TempDir()
	if _, ok := Read(filepath.Join(dir, "missing.json"), now); ok {
		t.Error("a missing file is not a snapshot")
	}
	path := filepath.Join(dir, "status.json")
	if err := os.WriteFile(path, doc(1, now.Add(-time.Second), true, goodPlan), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := Read(path, now); !ok {
		t.Error("expected the written snapshot to be read")
	}
}

func TestDefaultPath(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	cases := []struct {
		goos, home string
		env        map[string]string
		want       string
	}{
		{"windows", `C:\Users\a`, map[string]string{"LOCALAPPDATA": `C:\Users\a\AppData\Local`},
			`C:\Users\a\AppData\Local\agy-foreman\status.json`},
		{"windows", `C:\Users\a`, nil, `C:\Users\a\AppData\Local\agy-foreman\status.json`},
		{"darwin", "/Users/a", nil, "/Users/a/Library/Application Support/agy-foreman/status.json"},
		{"linux", "/home/a", map[string]string{"XDG_STATE_HOME": "/s"}, "/s/agy-foreman/status.json"},
		{"linux", "/home/a", nil, "/home/a/.local/state/agy-foreman/status.json"},
		{"linux", "/home/a", map[string]string{"FOREMAN_STATE_DIR": "/x"}, "/x/status.json"},
		{"windows", `C:\Users\a`, map[string]string{"FOREMAN_STATE_DIR": `D:\state`}, `D:\state\status.json`},
	}
	for _, c := range cases {
		if got := DefaultPath(c.goos, c.home, env(c.env)); got != c.want {
			t.Errorf("DefaultPath(%s) = %q, want %q", c.goos, got, c.want)
		}
	}
}
