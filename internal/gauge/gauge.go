// Package gauge formats the small visual pieces of the status line.
package gauge

import (
	"fmt"
	"time"
)

// Pie returns a pie glyph that is filled in proportion to f (0 to 1).
func Pie(f float64) string {
	switch {
	case f < 0.125:
		return "\u25cb"
	case f < 0.375:
		return "\u25d4"
	case f < 0.625:
		return "\u25d1"
	case f < 0.875:
		return "\u25d5"
	default:
		return "\u25cf"
	}
}

// Tokens formats a token count: 999, 420k, 1M, 1.5M.
func Tokens(n int64) string {
	switch {
	case n >= 999_500:
		if n%1_000_000 == 0 {
			return fmt.Sprintf("%dM", n/1_000_000)
		}
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1000:
		return fmt.Sprintf("%dk", (n+500)/1000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// Countdown formats the time until a limit resets: 2h14m or 3d 6h.
func Countdown(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	m := int(d.Minutes())
	days, hours, mins := m/1440, (m%1440)/60, m%60
	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	return fmt.Sprintf("%dh%02dm", hours, mins)
}

// CacheCountdown formats the time until the prompt cache expires: 3m12s or 1h05m.
func CacheCountdown(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	s := int(d.Seconds())
	if s >= 3600 {
		return fmt.Sprintf("%dh%02dm", s/3600, (s%3600)/60)
	}
	return fmt.Sprintf("%dm%02ds", s/60, s%60)
}
