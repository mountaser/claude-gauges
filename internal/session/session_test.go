package session

import (
	"strings"
	"testing"
)

const full = `{
  "cwd": "/work/app",
  "model": {"id": "claude-opus-5-5", "display_name": "Opus"},
  "workspace": {"current_dir": "/work/app/sub"},
  "effort": {"level": "high"},
  "context_window": {"total_input_tokens": 420000, "context_window_size": 1000000},
  "rate_limits": {
    "five_hour": {"used_percentage": 23.5, "resets_at": 1738425600},
    "seven_day": {"used_percentage": 41.2, "resets_at": 1738857600}
  },
  "prompt_cache": {"warm": true, "caching_observed": true,
    "expires_at": 1738429200, "recache_tokens_if_cold": 45000}
}`

func TestParseFull(t *testing.T) {
	s, err := Parse(strings.NewReader(full))
	if err != nil {
		t.Fatal(err)
	}
	if s.Model.ID != "claude-opus-5-5" || s.Model.DisplayName != "Opus" {
		t.Errorf("model = %+v", s.Model)
	}
	if s.Effort == nil || s.Effort.Level != "high" {
		t.Errorf("effort = %+v", s.Effort)
	}
	if s.ContextWindow.TotalInputTokens != 420000 || s.ContextWindow.Size != 1000000 {
		t.Errorf("context = %+v", s.ContextWindow)
	}
	if s.RateLimits.FiveHour == nil || s.RateLimits.FiveHour.UsedPercentage != 23.5 ||
		s.RateLimits.FiveHour.ResetsAt != 1738425600 {
		t.Errorf("five_hour = %+v", s.RateLimits.FiveHour)
	}
	if s.RateLimits.SevenDay == nil || s.RateLimits.SevenDay.ResetsAt != 1738857600 {
		t.Errorf("seven_day = %+v", s.RateLimits.SevenDay)
	}
	c := s.PromptCache
	if c == nil || !c.Warm || !c.CachingObserved ||
		c.ExpiresAt != 1738429200 || c.RecacheTokensIfCold != 45000 {
		t.Errorf("prompt_cache = %+v", c)
	}
}

func TestParseEmptyObject(t *testing.T) {
	s, err := Parse(strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.Effort != nil || s.PromptCache != nil ||
		s.RateLimits.FiveHour != nil || s.RateLimits.SevenDay != nil {
		t.Errorf("optional fields should be nil: %+v", s)
	}
}

func TestParseLenientNumbers(t *testing.T) {
	in := `{
	  "context_window": {"total_input_tokens": "123", "context_window_size": 1e6},
	  "rate_limits": {"five_hour": {"used_percentage": "12.5", "resets_at": 1738425600.0},
	                  "seven_day": {"used_percentage": null, "resets_at": "2026-01-01T00:00:00Z"}},
	  "prompt_cache": {"expires_at": {"unexpected": true}}
	}`
	s, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("lenient parse failed: %v", err)
	}
	if s.ContextWindow.TotalInputTokens != 123 || s.ContextWindow.Size != 1000000 {
		t.Errorf("context = %+v", s.ContextWindow)
	}
	if f := s.RateLimits.FiveHour; f.UsedPercentage != 12.5 || f.ResetsAt != 1738425600 {
		t.Errorf("five_hour = %+v", f)
	}
	if d := s.RateLimits.SevenDay; d.UsedPercentage != 0 || d.ResetsAt != 0 {
		t.Errorf("seven_day should read as zero: %+v", d)
	}
	if s.PromptCache.ExpiresAt != 0 {
		t.Errorf("prompt_cache = %+v", s.PromptCache)
	}
}

func TestParseInvalid(t *testing.T) {
	if _, err := Parse(strings.NewReader(`not json`)); err == nil {
		t.Fatal("expected an error")
	}
}

func TestDir(t *testing.T) {
	cases := []struct{ json, want string }{
		{`{"cwd":"/a","workspace":{"current_dir":"/b"}}`, "/b"},
		{`{"cwd":"/a"}`, "/a"},
		{`{}`, ""},
	}
	for _, c := range cases {
		s, err := Parse(strings.NewReader(c.json))
		if err != nil {
			t.Fatal(err)
		}
		if got := s.Dir(); got != c.want {
			t.Errorf("Dir(%s) = %q, want %q", c.json, got, c.want)
		}
	}
}
