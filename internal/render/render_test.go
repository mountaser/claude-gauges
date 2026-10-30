package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mountaser/claude-gauges/internal/gitinfo"
	"github.com/mountaser/claude-gauges/internal/session"
)

var now = time.Unix(1_700_000_000, 0)

func base() map[string]any {
	return map[string]any{
		"workspace":      map[string]any{"current_dir": "/work/screentether"},
		"model":          map[string]any{"id": "claude-opus-5-5", "display_name": "Opus"},
		"effort":         map[string]any{"level": "high"},
		"context_window": map[string]any{"total_input_tokens": 420000, "context_window_size": 1000000},
		"rate_limits": map[string]any{
			"five_hour": map[string]any{"used_percentage": 58.0, "resets_at": now.Add(2*time.Hour + 14*time.Minute).Unix()},
			"seven_day": map[string]any{"used_percentage": 31.0, "resets_at": now.Add(3*24*time.Hour + 6*time.Hour).Unix()},
		},
		"prompt_cache": map[string]any{
			"warm": true, "caching_observed": true, "ttl": "1h",
			"expires_at":             now.Add(3*time.Minute + 12*time.Second).Unix(),
			"recache_tokens_if_cold": 461120,
		},
	}
}

func build(t *testing.T, mutate func(m map[string]any)) *session.Session {
	t.Helper()
	m := base()
	if mutate != nil {
		mutate(m)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	s, err := session.Parse(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func plain(s *session.Session) string { return Render(s, Options{Now: now}) }

func TestRenderTypical(t *testing.T) {
	want := "screentether │ Opus 5.5 · high │ ctx ◑ 420k/1M\n" +
		"5h ◑ 42% ↺ 2h14m │ 7d ◕ 69% ↺ 3d 6h │ cache ● 3m12s"
	if got := plain(build(t, nil)); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderWithGit(t *testing.T) {
	git := func(string) (gitinfo.Info, bool) {
		return gitinfo.Info{Repo: "screentether", Branch: "master", Folder: "screentether"}, true
	}
	got := Render(build(t, nil), Options{Now: now, Git: git})
	if !strings.HasPrefix(got, "screentether │ master │ Opus 5.5") {
		t.Errorf("got %q", got)
	}
}

func TestRenderWorktreeFolder(t *testing.T) {
	cases := []struct {
		info gitinfo.Info
		want string
	}{
		{gitinfo.Info{Repo: "screentether", Branch: "feat/onboarding", Folder: "wifi-setup", Worktree: true},
			"screentether │ feat/onboarding (wifi-setup) │ "},
		{gitinfo.Info{Repo: "screentether", Branch: "fix-wifi", Folder: "fix-wifi", Worktree: true},
			"screentether │ fix-wifi │ "},
	}
	for _, c := range cases {
		info := c.info
		git := func(string) (gitinfo.Info, bool) { return info, true }
		got := Render(build(t, nil), Options{Now: now, Git: git})
		if !strings.HasPrefix(got, c.want) {
			t.Errorf("got %q, want prefix %q", got, c.want)
		}
	}
}

func TestRenderHeavyUsage(t *testing.T) {
	s := build(t, func(m map[string]any) {
		m["effort"] = map[string]any{"level": "max"}
		m["context_window"] = map[string]any{"total_input_tokens": 910000, "context_window_size": 1000000}
		rl := m["rate_limits"].(map[string]any)
		rl["five_hour"].(map[string]any)["used_percentage"] = 92.0
		rl["seven_day"].(map[string]any)["used_percentage"] = 74.0
	})
	want := "screentether │ Opus 5.5 · max │ ctx ● 910k/1M\n" +
		"5h ○ 8% ↺ 2h14m │ 7d ◔ 26% ↺ 3d 6h │ cache ● 3m12s"
	if got := plain(s); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderFreshSession(t *testing.T) {
	s := build(t, func(m map[string]any) {
		m["context_window"] = map[string]any{"total_input_tokens": 0, "context_window_size": 200000}
		delete(m, "rate_limits")
		delete(m, "prompt_cache")
		delete(m, "effort")
	})
	want := "screentether │ Opus 5.5 │ ctx ○ 0/200k\n5h – │ 7d – │ cache –"
	if got := plain(s); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderUnknownContextSize(t *testing.T) {
	s := build(t, func(m map[string]any) {
		m["context_window"] = map[string]any{"total_input_tokens": 5, "context_window_size": 0}
	})
	if got := plain(s); !strings.Contains(got, "ctx –") {
		t.Errorf("got %q", got)
	}
}

func TestRenderLimitWithoutResetTime(t *testing.T) {
	s := build(t, func(m map[string]any) {
		m["rate_limits"].(map[string]any)["five_hour"] = map[string]any{"used_percentage": 58.0}
	})
	if got := plain(s); !strings.Contains(got, "5h ◑ 42% │ 7d") {
		t.Errorf("got %q", got)
	}
}

func TestRenderCacheStates(t *testing.T) {
	cases := []struct {
		name  string
		cache map[string]any
		want  string
	}{
		{"expiring", map[string]any{"warm": true, "caching_observed": true,
			"expires_at": now.Add(48 * time.Second).Unix()}, "cache ◐ 0m48s"},
		{"cold with cost", map[string]any{"warm": false, "caching_observed": true,
			"expires_at": now.Add(-time.Minute).Unix(), "recache_tokens_if_cold": 461120},
			"▲ cold · +461k tokens to reload"},
		{"cold without cost", map[string]any{"warm": false, "caching_observed": true,
			"expires_at": now.Add(-time.Minute).Unix()}, "│ ▲ cold"},
		{"expired while flagged warm", map[string]any{"warm": true, "caching_observed": true,
			"expires_at": now.Add(-time.Second).Unix()}, "▲ cold"},
		{"not observed", map[string]any{"caching_observed": false}, "cache –"},
	}
	for _, c := range cases {
		s := build(t, func(m map[string]any) { m["prompt_cache"] = c.cache })
		if got := plain(s); !strings.Contains(got, c.want) {
			t.Errorf("%s: got %q, want it to contain %q", c.name, got, c.want)
		}
	}
}

func TestRenderColor(t *testing.T) {
	s := build(t, func(m map[string]any) {
		m["rate_limits"].(map[string]any)["five_hour"].(map[string]any)["used_percentage"] = 92.0
	})
	colored := Render(s, Options{Now: now, Color: true})
	if !strings.Contains(colored, "\x1b["+red+"m○\x1b[0m") {
		t.Errorf("expected a red empty pie in %q", colored)
	}
	if strings.Contains(plain(s), "\x1b") {
		t.Error("plain output must not contain escape sequences")
	}
}

func TestModelName(t *testing.T) {
	cases := []struct{ id, display, want string }{
		{"claude-opus-5-5", "Opus", "Opus 5.5"},
		{"claude-sonnet-5-5", "Sonnet", "Sonnet 5.5"},
		{"claude-haiku-5-5", "Haiku", "Haiku 5.5"},
		{"claude-opus-5-20260115", "Opus", "Opus 5"},
		{"claude-sonnet-4-5-20250929", "Sonnet", "Sonnet 4.5"},
		{"claude-3-5-sonnet-20241022", "Sonnet", "Sonnet 3.5"},
		{"claude-opus-5-5[1m]", "Opus", "Opus 5.5"},
		{"us.anthropic.claude-sonnet-4-5-20250929-v1:0", "Sonnet", "Sonnet 4.5"},
		{"claude-opus-4-1@20250805", "Opus", "Opus 4.1"},
		{"", "Opus", "Opus"},
		{"gibberish", "Custom", "Custom"},
	}
	for _, c := range cases {
		if got := ModelName(c.id, c.display); got != c.want {
			t.Errorf("ModelName(%q, %q) = %q, want %q", c.id, c.display, got, c.want)
		}
	}
}

func TestRenderAppendsDelegationLines(t *testing.T) {
	s := build(t, nil)
	out := Render(s, Options{Now: now, Agy: typical()})
	lines := strings.Split(out, "\n")
	if len(lines) != 4 {
		t.Fatalf("expected four lines, got %d:\n%s", len(lines), out)
	}
	if !strings.HasPrefix(lines[2], "agy-foreman ●●◐○○ 2/5") || !strings.HasPrefix(lines[3], "5h ● 94%") {
		t.Errorf("delegation lines:\n%s\n%s", lines[2], lines[3])
	}
	if plainOnly := plain(s); strings.Count(plainOnly, "\n") != 1 {
		t.Errorf("without a snapshot there must be two lines:\n%s", plainOnly)
	}
}
