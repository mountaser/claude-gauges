package gauge

import (
	"testing"
	"time"
)

func TestPie(t *testing.T) {
	cases := []struct {
		f    float64
		want string
	}{
		{-0.5, "○"}, {0, "○"}, {0.124, "○"},
		{0.125, "◔"}, {0.374, "◔"},
		{0.375, "◑"}, {0.624, "◑"},
		{0.625, "◕"}, {0.874, "◕"},
		{0.875, "●"}, {1, "●"}, {1.5, "●"},
	}
	for _, c := range cases {
		if got := Pie(c.f); got != c.want {
			t.Errorf("Pie(%v) = %q, want %q", c.f, got, c.want)
		}
	}
}

func TestTokens(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0"}, {999, "999"}, {1000, "1k"}, {12345, "12k"},
		{200000, "200k"}, {420000, "420k"}, {461120, "461k"},
		{999600, "1.0M"}, {1000000, "1M"}, {1500000, "1.5M"}, {2000000, "2M"},
	}
	for _, c := range cases {
		if got := Tokens(c.n); got != c.want {
			t.Errorf("Tokens(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestCountdown(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{-time.Hour, "0h00m"},
		{0, "0h00m"},
		{26 * time.Minute, "0h26m"},
		{2*time.Hour + 14*time.Minute, "2h14m"},
		{23*time.Hour + 59*time.Minute, "23h59m"},
		{24 * time.Hour, "1d 0h"},
		{3*24*time.Hour + 6*time.Hour + 30*time.Minute, "3d 6h"},
	}
	for _, c := range cases {
		if got := Countdown(c.d); got != c.want {
			t.Errorf("Countdown(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestCacheCountdown(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{-time.Second, "0m00s"},
		{48 * time.Second, "0m48s"},
		{3*time.Minute + 12*time.Second, "3m12s"},
		{59*time.Minute + 59*time.Second, "59m59s"},
		{time.Hour + 5*time.Minute, "1h05m"},
	}
	for _, c := range cases {
		if got := CacheCountdown(c.d); got != c.want {
			t.Errorf("CacheCountdown(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}
