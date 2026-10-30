# claude-gauges Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `claude-gauges`, a Go status line for Claude Code that prints project, branch, model with effort, context usage, 5-hour and 7-day limits with reset countdowns, and a prompt cache timer, plus `install` and `uninstall` commands.

**Architecture:** A small `main` package dispatches commands. Rendering is a pure function from a parsed session plus the current time to a two-line string, built from five focused packages under `internal/`. `install` edits `settings.json` through a separate package that never starts other programs.

**Tech Stack:** Go (standard library only), `go test`, local `go build`.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-10-30-claude-gauges-design.md`. Behavior there is authoritative.
- Module path: `github.com/mountaser/claude-gauges`; `go 1.22` in `go.mod`.
- Standard library only. No network access. Never start `git` or any other process.
- Rendering exits with status 0 in every case, including invalid input.
- Output is UTF-8. No escape sequences when `NO_COLOR` is set.
- Pie glyphs: `○` `◔` `◑` `◕` `●`, thresholds 0.125 / 0.375 / 0.625 / 0.875.
- Limits show remaining percentage (full = untouched). Context shows tokens used and counts up.
- `install` writes `{"type":"command","command":"<path>","refreshInterval":10}` with a forward-slash path, quoted when it contains a space.
- License: MIT. `NOTICE.md` credits moo-statusline for the worktree approach and reproduces its notice.
- Commit after every task. Do not push.

## File Structure

```
go.mod
main.go                         command dispatch, render command, install/uninstall commands
main_test.go
LICENSE
NOTICE.md
README.md
.gitignore
internal/session/session.go     parse Claude Code's JSON input
internal/session/session_test.go
internal/gauge/gauge.go         pie glyph, token and duration formatting
internal/gauge/gauge_test.go
internal/gitinfo/gitinfo.go     branch and worktree detection from .git files
internal/gitinfo/gitinfo_test.go
internal/render/render.go       the two-line output
internal/render/model.go        model id to display name
internal/render/render_test.go
internal/install/install.go     settings.json editing, install, uninstall
internal/install/install_test.go
```

---

### Task 1: Module scaffold and session parsing

**Files:**
- Create: `go.mod`, `.gitignore`, `LICENSE`, `NOTICE.md`
- Create: `internal/session/session.go`
- Test: `internal/session/session_test.go`

**Interfaces:**
- Produces: package `session` with
  - `type Session struct { Cwd string; Model Model; Workspace Workspace; Effort *Effort; ContextWindow ContextWindow; RateLimits RateLimits; PromptCache *PromptCache }`
  - `type Model struct { ID, DisplayName string }`
  - `type Workspace struct { CurrentDir string }`
  - `type Effort struct { Level string }`
  - `type ContextWindow struct { TotalInputTokens, Size int64 }`
  - `type Window struct { UsedPercentage float64; ResetsAt int64 }`
  - `type RateLimits struct { FiveHour, SevenDay *Window }`
  - `type PromptCache struct { Warm, CachingObserved bool; TTL string; ExpiresAt, RecacheTokensIfCold int64 }`
  - `func Parse(r io.Reader) (*Session, error)`
  - `func (s *Session) Dir() string` returns `Workspace.CurrentDir`, else `Cwd`, else `""`.

- [ ] **Step 1: Create the module files**

`go.mod`:
```
module github.com/mountaser/claude-gauges

go 1.22
```

`.gitignore`:
```
claude-gauges
claude-gauges.exe
dist/
```

`LICENSE`:
```
MIT License

Copyright (c) 2026 mountaser

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

`NOTICE.md`:
````markdown
# Third-party notices

The git worktree handling in `internal/gitinfo` follows the approach of
[moo-statusline](https://github.com/moogento/moo-statusline), which is
distributed under the following license.

```
MIT License

Copyright (c) 2025

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
````

- [ ] **Step 2: Write the failing test**

`internal/session/session_test.go`:
```go
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
  "prompt_cache": {"warm": true, "caching_observed": true, "ttl": "1h",
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
	if c == nil || !c.Warm || !c.CachingObserved || c.TTL != "1h" ||
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
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/session/`
Expected: FAIL (build error: `undefined: Parse`)

- [ ] **Step 4: Write the implementation**

`internal/session/session.go`:
```go
// Package session parses the JSON that Claude Code sends to a status line.
package session

import (
	"encoding/json"
	"io"
)

type Model struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type Workspace struct {
	CurrentDir string `json:"current_dir"`
}

type Effort struct {
	Level string `json:"level"`
}

type ContextWindow struct {
	TotalInputTokens int64 `json:"total_input_tokens"`
	Size             int64 `json:"context_window_size"`
}

type Window struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       int64   `json:"resets_at"`
}

type RateLimits struct {
	FiveHour *Window `json:"five_hour"`
	SevenDay *Window `json:"seven_day"`
}

type PromptCache struct {
	Warm                bool   `json:"warm"`
	CachingObserved     bool   `json:"caching_observed"`
	TTL                 string `json:"ttl"`
	ExpiresAt           int64  `json:"expires_at"`
	RecacheTokensIfCold int64  `json:"recache_tokens_if_cold"`
}

type Session struct {
	Cwd           string        `json:"cwd"`
	Model         Model         `json:"model"`
	Workspace     Workspace     `json:"workspace"`
	Effort        *Effort       `json:"effort"`
	ContextWindow ContextWindow `json:"context_window"`
	RateLimits    RateLimits    `json:"rate_limits"`
	PromptCache   *PromptCache  `json:"prompt_cache"`
}

// Parse decodes one session JSON document. Unknown fields are ignored.
func Parse(r io.Reader) (*Session, error) {
	var s Session
	if err := json.NewDecoder(r).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Dir returns the directory the session is working in, or "" if unknown.
func (s *Session) Dir() string {
	if s.Workspace.CurrentDir != "" {
		return s.Workspace.CurrentDir
	}
	return s.Cwd
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/session/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add go.mod .gitignore LICENSE NOTICE.md internal/session
git commit -m "feat: parse Claude Code session JSON"
```

---

### Task 2: Gauge formatting

**Files:**
- Create: `internal/gauge/gauge.go`
- Test: `internal/gauge/gauge_test.go`

**Interfaces:**
- Produces: package `gauge` with
  - `func Pie(f float64) string` returns one of `○ ◔ ◑ ◕ ●` for fraction filled `f`
  - `func Tokens(n int64) string`
  - `func Countdown(d time.Duration) string` reset countdown
  - `func CacheCountdown(d time.Duration) string`

- [ ] **Step 1: Write the failing test**

`internal/gauge/gauge_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gauge/`
Expected: FAIL (build error: `undefined: Pie`)

- [ ] **Step 3: Write the implementation**

`internal/gauge/gauge.go`:
```go
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
		return "○"
	case f < 0.375:
		return "◔"
	case f < 0.625:
		return "◑"
	case f < 0.875:
		return "◕"
	default:
		return "●"
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gauge/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/gauge
git commit -m "feat: pie glyph and number formatting"
```

---

### Task 3: Git branch and worktree detection

**Files:**
- Create: `internal/gitinfo/gitinfo.go`
- Test: `internal/gitinfo/gitinfo_test.go`

**Interfaces:**
- Produces: package `gitinfo` with
  - `type Info struct { Repo, Branch, Folder string; Worktree bool }`
  - `func Read(dir string) (Info, bool)`: `Repo` is the repository folder name (the main repository's name inside a worktree), `Branch` is the branch or the first 7 characters of the commit when detached, `Folder` is the name of the folder that contains `.git`, `Worktree` is true in a linked worktree. The bool is false when `dir` is empty or not inside a repository.

- [ ] **Step 1: Write the failing test**

`internal/gitinfo/gitinfo_test.go`:
```go
package gitinfo

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadRegularRepo(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "screentether")
	write(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/master\n")
	sub := filepath.Join(repo, "app", "src")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	info, ok := Read(sub)
	want := Info{Repo: "screentether", Branch: "master", Folder: "screentether"}
	if !ok || info != want {
		t.Errorf("Read = %+v, %v; want %+v", info, ok, want)
	}
}

func TestReadDetachedHead(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "app")
	write(t, filepath.Join(repo, ".git", "HEAD"), "0123456789abcdef0123456789abcdef01234567\n")
	info, ok := Read(repo)
	if !ok || info.Branch != "0123456" {
		t.Errorf("Read = %+v, %v", info, ok)
	}
}

func TestReadWorktree(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "screentether")
	wt := filepath.Join(root, "onboarding-wifi")
	write(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/master\n")
	gitdir := filepath.Join(repo, ".git", "worktrees", "onboarding-wifi")
	write(t, filepath.Join(gitdir, "HEAD"), "ref: refs/heads/feat/onboarding-wifi\n")
	write(t, filepath.Join(gitdir, "commondir"), "../..\n")
	write(t, filepath.Join(wt, ".git"), "gitdir: "+filepath.ToSlash(gitdir)+"\n")

	info, ok := Read(wt)
	want := Info{Repo: "screentether", Branch: "feat/onboarding-wifi", Folder: "onboarding-wifi", Worktree: true}
	if !ok || info != want {
		t.Errorf("Read = %+v, %v; want %+v", info, ok, want)
	}
}

func TestReadNotARepo(t *testing.T) {
	if _, ok := Read(t.TempDir()); ok {
		t.Error("expected ok = false outside a repository")
	}
	if _, ok := Read(""); ok {
		t.Error("expected ok = false for an empty directory")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gitinfo/`
Expected: FAIL (build error: `undefined: Read`)

- [ ] **Step 3: Write the implementation**

`internal/gitinfo/gitinfo.go`:
```go
// Package gitinfo reads the branch and worktree from .git files.
// It never starts the git program.
package gitinfo

import (
	"os"
	"path/filepath"
	"strings"
)

type Info struct {
	Repo     string
	Branch   string
	Folder   string
	Worktree bool
}

// Read looks for a repository at dir or in a parent folder.
func Read(dir string) (Info, bool) {
	if dir == "" {
		return Info{}, false
	}
	root, gitPath := findGit(dir)
	if root == "" {
		return Info{}, false
	}
	st, err := os.Stat(gitPath)
	if err != nil {
		return Info{}, false
	}

	gitDir := gitPath
	common := ""
	if !st.IsDir() {
		// A linked worktree or submodule: .git is a file "gitdir: <path>".
		data, err := os.ReadFile(gitPath)
		if err != nil {
			return Info{}, false
		}
		line := strings.TrimSpace(string(data))
		if !strings.HasPrefix(line, "gitdir:") {
			return Info{}, false
		}
		gitDir = strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
		if !filepath.IsAbs(gitDir) {
			gitDir = filepath.Join(root, gitDir)
		}
		if cd, err := os.ReadFile(filepath.Join(gitDir, "commondir")); err == nil {
			common = strings.TrimSpace(string(cd))
			if !filepath.IsAbs(common) {
				common = filepath.Join(gitDir, common)
			}
		}
	}

	head, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return Info{}, false
	}
	branch := ""
	switch s := strings.TrimSpace(string(head)); {
	case strings.HasPrefix(s, "ref: refs/heads/"):
		branch = strings.TrimPrefix(s, "ref: refs/heads/")
	case len(s) >= 7:
		branch = s[:7]
	}

	info := Info{Repo: filepath.Base(root), Branch: branch, Folder: filepath.Base(root)}
	if common != "" {
		info.Worktree = true
		info.Repo = filepath.Base(filepath.Dir(filepath.Clean(common)))
	}
	return info, true
}

func findGit(dir string) (root, gitPath string) {
	dir = filepath.Clean(dir)
	for {
		p := filepath.Join(dir, ".git")
		if _, err := os.Stat(p); err == nil {
			return dir, p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ""
		}
		dir = parent
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gitinfo/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/gitinfo
git commit -m "feat: read branch and worktree from .git files"
```

---

### Task 4: Render the two lines

**Files:**
- Create: `internal/render/model.go`, `internal/render/render.go`
- Test: `internal/render/render_test.go`

**Interfaces:**
- Consumes: `session.Session` and its field types (Task 1); `gauge.Pie`, `gauge.Tokens`, `gauge.Countdown`, `gauge.CacheCountdown` (Task 2); `gitinfo.Info` (Task 3).
- Produces: package `render` with
  - `type Options struct { Now time.Time; Color bool; Git func(dir string) (gitinfo.Info, bool) }`; a nil `Git` means no git lookup.
  - `func Render(s *session.Session, opt Options) string` returns the two lines joined by `\n`, with no trailing newline.
  - `func ModelName(id, display string) string`

- [ ] **Step 1: Write the failing test**

`internal/render/render_test.go`:
```go
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
	if !strings.Contains(colored, "\x1b[31m○\x1b[0m") {
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
		{"", "Opus", "Opus"},
		{"gibberish", "Custom", "Gibberish"},
	}
	for _, c := range cases {
		if got := ModelName(c.id, c.display); got != c.want {
			t.Errorf("ModelName(%q, %q) = %q, want %q", c.id, c.display, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/render/`
Expected: FAIL (build error: `undefined: Render`)

- [ ] **Step 3: Write the model name implementation**

`internal/render/model.go`:
```go
package render

import "strings"

// ModelName turns a model id such as "claude-opus-5-5" into "Opus 5.5".
// Context suffixes like "[1m]" and date segments are ignored. It falls back
// to display when no family can be found in the id.
func ModelName(id, display string) string {
	core := id
	if i := strings.Index(core, "["); i >= 0 {
		core = core[:i]
	}
	core = strings.TrimPrefix(core, "claude-")

	family := ""
	var nums []string
	for _, p := range strings.Split(core, "-") {
		switch {
		case p == "":
		case isDigits(p):
			if len(p) <= 2 {
				nums = append(nums, p)
			}
		case family == "":
			family = p
		}
	}
	if family == "" {
		return display
	}
	name := strings.ToUpper(family[:1]) + family[1:]
	if len(nums) > 2 {
		nums = nums[:2]
	}
	if len(nums) > 0 {
		name += " " + strings.Join(nums, ".")
	}
	return name
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
```

- [ ] **Step 4: Write the render implementation**

`internal/render/render.go`:
```go
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
	green   = "32"
	yellow  = "33"
	red     = "31"
	blue    = "34"
	magenta = "35"
	cyan    = "36"
	dim     = "90"
	bold    = "1"
	orange  = "38;5;208"
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
	size := s.ContextWindow.Size
	if size <= 0 {
		return label + " –"
	}
	used := s.ContextWindow.TotalInputTokens
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
	rem := math.Max(0, math.Min(100, 100-w.UsedPercentage))
	col := red
	switch {
	case rem > 50:
		col = green
	case rem >= 20:
		col = yellow
	}
	out := fmt.Sprintf("%s %s %s", label, p.c(col, gauge.Pie(rem/100)), p.c(bold, fmt.Sprintf("%.0f%%", rem)))
	if w.ResetsAt > 0 {
		left := time.Unix(w.ResetsAt, 0).Sub(now)
		out += " " + p.c(dim, "↺ "+gauge.Countdown(left))
	}
	return out
}

func cacheSegment(c *session.PromptCache, now time.Time, p painter) string {
	label := p.c(dim, "cache")
	if c == nil || !c.CachingObserved {
		return label + " –"
	}
	left := time.Unix(c.ExpiresAt, 0).Sub(now)
	if c.Warm && left > 0 {
		if left < time.Minute {
			return label + " " + p.c(yellow, "◐ "+gauge.CacheCountdown(left))
		}
		return label + " " + p.c(green, "● "+gauge.CacheCountdown(left))
	}
	cold := "▲ cold"
	if c.RecacheTokensIfCold > 0 {
		cold += " · +" + gauge.Tokens(c.RecacheTokensIfCold) + " tokens to reload"
	}
	return p.c(orange, cold)
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/render/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/render
git commit -m "feat: render the two status lines"
```

---

### Task 5: Command entry point and render command

**Files:**
- Create: `main.go`
- Test: `main_test.go`

**Interfaces:**
- Consumes: `session.Parse`, `render.Render`, `render.Options`, `gitinfo.Read`.
- Produces: in package `main`
  - `var version = "dev"`
  - `func run(args []string, in io.Reader, out, errw io.Writer) int`
  - `func renderCmd(in io.Reader, out io.Writer) int`
  - `installCmd` and `uninstallCmd` are added in Task 7; until then `run` answers those two commands with "not available yet" and exit code 2 (replaced in Task 7).

- [ ] **Step 1: Write the failing test**

`main_test.go`:
```go
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunRender(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	in := strings.NewReader(`{"model":{"id":"claude-opus-5-5"},"workspace":{"current_dir":"/work/demo"}}`)
	var out, errw bytes.Buffer
	code := run(nil, in, &out, &errw)
	want := "demo │ Opus 5.5 │ ctx –\n5h – │ 7d – │ cache –\n"
	if code != 0 || out.String() != want {
		t.Errorf("code %d, output:\n%s\nwant:\n%s", code, out.String(), want)
	}
}

func TestRunRenderBadInput(t *testing.T) {
	var out, errw bytes.Buffer
	code := run(nil, strings.NewReader("not json"), &out, &errw)
	if code != 0 || !strings.Contains(out.String(), "could not read session data") {
		t.Errorf("code %d, output %q", code, out.String())
	}
}

func TestRunVersionAndHelp(t *testing.T) {
	var out, errw bytes.Buffer
	if code := run([]string{"version"}, strings.NewReader(""), &out, &errw); code != 0 || strings.TrimSpace(out.String()) != version {
		t.Errorf("version: code %d, output %q", code, out.String())
	}
	out.Reset()
	if code := run([]string{"help"}, strings.NewReader(""), &out, &errw); code != 0 || !strings.Contains(out.String(), "install") {
		t.Errorf("help: code %d, output %q", code, out.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var out, errw bytes.Buffer
	if code := run([]string{"bogus"}, strings.NewReader(""), &out, &errw); code != 2 {
		t.Errorf("code %d, want 2", code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test .`
Expected: FAIL (build error: `undefined: run`)

- [ ] **Step 3: Write the implementation**

`main.go`:
```go
// Command claude-gauges prints a status line for Claude Code.
package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mountaser/claude-gauges/internal/gitinfo"
	"github.com/mountaser/claude-gauges/internal/render"
	"github.com/mountaser/claude-gauges/internal/session"
)

var version = "dev"

const usage = `claude-gauges: status line for Claude Code

Usage:
  claude-gauges             read session JSON on stdin and print the status line
  claude-gauges install     install and register as the Claude Code status line
  claude-gauges uninstall   restore the previous status line
  claude-gauges version     print the version
  claude-gauges help        print this text
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, in io.Reader, out, errw io.Writer) int {
	if len(args) == 0 {
		return renderCmd(in, out)
	}
	switch args[0] {
	case "version":
		fmt.Fprintln(out, version)
		return 0
	case "help", "-h", "--help":
		fmt.Fprint(out, usage)
		return 0
	case "install", "uninstall":
		fmt.Fprintln(errw, "claude-gauges:", args[0], "is not available yet")
		return 2
	default:
		fmt.Fprintf(errw, "claude-gauges: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

func renderCmd(in io.Reader, out io.Writer) int {
	s, err := session.Parse(in)
	if err != nil {
		fmt.Fprintln(out, "claude-gauges: could not read session data")
		return 0
	}
	_, noColor := os.LookupEnv("NO_COLOR")
	fmt.Fprintln(out, render.Render(s, render.Options{
		Now:   time.Now(),
		Color: !noColor,
		Git:   gitinfo.Read,
	}))
	return 0
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test .`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add main.go main_test.go
git commit -m "feat: command entry point and render command"
```

---

### Task 6: Install and uninstall logic

**Files:**
- Create: `internal/install/install.go`
- Test: `internal/install/install_test.go`

**Interfaces:**
- Produces: package `install` with
  - `type Options struct { SettingsPath, SourceExe, DestDir string; Force bool; RefreshSeconds int }`
  - `func ExeName() string` returns `claude-gauges.exe` on Windows, else `claude-gauges`
  - `func Install(o Options) (command string, err error)` returns the command string written to `settings.json`
  - `func Uninstall(settingsPath, destDir string) (exeRemoved bool, err error)`

- [ ] **Step 1: Write the failing test**

`internal/install/install_test.go`:
```go
package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setup(t *testing.T) (Options, string) {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "src-bin")
	if err := os.WriteFile(src, []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	return Options{
		SettingsPath:   filepath.Join(dir, "settings.json"),
		SourceExe:      src,
		DestDir:        filepath.Join(dir, "dest"),
		RefreshSeconds: 10,
	}, dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("%s is not valid JSON: %v\n%s", path, err, data)
	}
	return m
}

func TestInstallKeepsOtherSettings(t *testing.T) {
	o, _ := setup(t)
	original := `{"theme":"dark","hooks":{"Stop":[{"command":"x"}]}}`
	writeFile(t, o.SettingsPath, original)

	cmd, err := Install(o)
	if err != nil {
		t.Fatal(err)
	}
	m := readJSON(t, o.SettingsPath)
	if m["theme"] != "dark" || m["hooks"] == nil {
		t.Errorf("other settings were lost: %v", m)
	}
	sl, _ := m["statusLine"].(map[string]any)
	if sl["type"] != "command" || sl["command"] != cmd || sl["refreshInterval"] != float64(10) {
		t.Errorf("statusLine = %v", sl)
	}
	if !strings.HasSuffix(cmd, "/"+ExeName()) || strings.Contains(cmd, `\`) {
		t.Errorf("command should use forward slashes and end with the program name: %s", cmd)
	}
	bak, err := os.ReadFile(o.SettingsPath + ".bak")
	if err != nil || string(bak) != original {
		t.Errorf("backup missing or different: %v %q", err, bak)
	}
	exe, err := os.ReadFile(filepath.Join(o.DestDir, ExeName()))
	if err != nil || string(exe) != "bin" {
		t.Errorf("program was not copied: %v %q", err, exe)
	}
}

func TestInstallCreatesMissingSettings(t *testing.T) {
	o, _ := setup(t)
	if _, err := Install(o); err != nil {
		t.Fatal(err)
	}
	if readJSON(t, o.SettingsPath)["statusLine"] == nil {
		t.Error("statusLine missing")
	}
}

func TestInstallQuotesPathWithSpaces(t *testing.T) {
	o, dir := setup(t)
	o.DestDir = filepath.Join(dir, "my dir")
	cmd, err := Install(o)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cmd, `"`) || !strings.HasSuffix(cmd, `"`) {
		t.Errorf("command should be quoted: %s", cmd)
	}
}

func TestInstallRefusesExistingStatusLine(t *testing.T) {
	o, _ := setup(t)
	original := `{"statusLine":{"type":"command","command":"other.sh"}}`
	writeFile(t, o.SettingsPath, original)

	if _, err := Install(o); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected an error mentioning --force, got %v", err)
	}
	if got, _ := os.ReadFile(o.SettingsPath); string(got) != original {
		t.Errorf("settings changed: %s", got)
	}
	if _, err := os.Stat(filepath.Join(o.DestDir, ExeName())); !os.IsNotExist(err) {
		t.Error("program must not be copied when the install is refused")
	}
}

func TestForceThenUninstallRestores(t *testing.T) {
	o, _ := setup(t)
	writeFile(t, o.SettingsPath, `{"theme":"dark","statusLine":{"type":"command","command":"other.sh"}}`)
	o.Force = true
	if _, err := Install(o); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(o.DestDir, previousFile)); err != nil {
		t.Fatalf("previous status line was not saved: %v", err)
	}

	removed, err := Uninstall(o.SettingsPath, o.DestDir)
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Error("expected the program to be removed")
	}
	m := readJSON(t, o.SettingsPath)
	sl, _ := m["statusLine"].(map[string]any)
	if sl["command"] != "other.sh" || m["theme"] != "dark" {
		t.Errorf("not restored: %v", m)
	}
	if _, err := os.Stat(filepath.Join(o.DestDir, previousFile)); !os.IsNotExist(err) {
		t.Error("previous file should be deleted")
	}
	if _, err := os.Stat(filepath.Join(o.DestDir, ExeName())); !os.IsNotExist(err) {
		t.Error("program should be deleted")
	}
}

func TestUninstallWithoutPreviousRemovesKey(t *testing.T) {
	o, _ := setup(t)
	writeFile(t, o.SettingsPath, `{"theme":"dark"}`)
	if _, err := Install(o); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(o.SettingsPath, o.DestDir); err != nil {
		t.Fatal(err)
	}
	m := readJSON(t, o.SettingsPath)
	if _, has := m["statusLine"]; has || m["theme"] != "dark" {
		t.Errorf("settings = %v", m)
	}
}

func TestUninstallLeavesForeignStatusLine(t *testing.T) {
	o, _ := setup(t)
	original := `{"statusLine":{"type":"command","command":"other.sh"}}`
	writeFile(t, o.SettingsPath, original)
	if _, err := Uninstall(o.SettingsPath, o.DestDir); err == nil {
		t.Fatal("expected an error")
	}
	if got, _ := os.ReadFile(o.SettingsPath); string(got) != original {
		t.Errorf("settings changed: %s", got)
	}
}

func TestInstallRejectsInvalidJSON(t *testing.T) {
	o, _ := setup(t)
	writeFile(t, o.SettingsPath, "{not json")
	if _, err := Install(o); err == nil {
		t.Fatal("expected an error")
	}
	if got, _ := os.ReadFile(o.SettingsPath); string(got) != "{not json" {
		t.Errorf("settings changed: %s", got)
	}
}

func TestInstallTwiceIsFine(t *testing.T) {
	o, _ := setup(t)
	if _, err := Install(o); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(o); err != nil {
		t.Fatalf("second install failed: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/install/`
Expected: FAIL (build error: `undefined: Options`)

- [ ] **Step 3: Write the implementation**

`internal/install/install.go`:
```go
// Package install registers claude-gauges as the Claude Code status line by
// editing settings.json.
package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const previousFile = "claude-gauges.previous.json"

type Options struct {
	SettingsPath   string
	SourceExe      string
	DestDir        string
	Force          bool
	RefreshSeconds int
}

// ExeName is the file name of the installed program.
func ExeName() string {
	if runtime.GOOS == "windows" {
		return "claude-gauges.exe"
	}
	return "claude-gauges"
}

func commandFor(dest string) string {
	p := filepath.ToSlash(dest)
	if strings.Contains(p, " ") {
		p = `"` + p + `"`
	}
	return p
}

func isOurs(raw json.RawMessage, cmd string) bool {
	var sl struct {
		Command string `json:"command"`
	}
	return json.Unmarshal(raw, &sl) == nil && sl.Command == cmd
}

// Install copies the program into o.DestDir and sets statusLine in settings.json.
// It returns the command string it wrote.
func Install(o Options) (string, error) {
	settings, err := readSettings(o.SettingsPath)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(o.DestDir, ExeName())
	cmd := commandFor(dest)

	prev, has := settings["statusLine"]
	saveOld := has && !isOurs(prev, cmd)
	if saveOld && !o.Force {
		return "", fmt.Errorf("a status line is already set in %s; run again with --force to replace it (the old one is saved and uninstall restores it)", o.SettingsPath)
	}

	if err := os.MkdirAll(o.DestDir, 0o755); err != nil {
		return "", err
	}
	if saveOld {
		if err := os.WriteFile(filepath.Join(o.DestDir, previousFile), prev, 0o644); err != nil {
			return "", err
		}
	}
	if err := copyExe(o.SourceExe, dest); err != nil {
		return "", err
	}
	if err := backup(o.SettingsPath); err != nil {
		return "", err
	}
	entry, err := json.Marshal(map[string]any{
		"type":            "command",
		"command":         cmd,
		"refreshInterval": o.RefreshSeconds,
	})
	if err != nil {
		return "", err
	}
	settings["statusLine"] = entry
	if err := writeSettings(o.SettingsPath, settings); err != nil {
		return "", err
	}
	return cmd, nil
}

// Uninstall restores the status line that was set before Install, or removes
// the statusLine key when there was none. It reports whether the installed
// program could be deleted (it cannot while it is running on Windows).
func Uninstall(settingsPath, destDir string) (bool, error) {
	settings, err := readSettings(settingsPath)
	if err != nil {
		return false, err
	}
	dest := filepath.Join(destDir, ExeName())
	cur, has := settings["statusLine"]
	if !has || !isOurs(cur, commandFor(dest)) {
		return false, fmt.Errorf("the status line in %s is not claude-gauges; nothing was changed", settingsPath)
	}
	if err := backup(settingsPath); err != nil {
		return false, err
	}
	prevPath := filepath.Join(destDir, previousFile)
	if prev, err := os.ReadFile(prevPath); err == nil {
		settings["statusLine"] = json.RawMessage(prev)
	} else {
		delete(settings, "statusLine")
	}
	if err := writeSettings(settingsPath, settings); err != nil {
		return false, err
	}
	os.Remove(prevPath)
	return os.Remove(dest) == nil, nil
}

func readSettings(path string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) || (err == nil && len(bytes.TrimSpace(data)) == 0) {
		return map[string]json.RawMessage{}, nil
	}
	if err != nil {
		return nil, err
	}
	m := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON, nothing was changed: %w", path, err)
	}
	if m == nil {
		m = map[string]json.RawMessage{}
	}
	return m, nil
}

func writeSettings(path string, m map[string]json.RawMessage) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func backup(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return os.WriteFile(path+".bak", data, 0o644)
}

func copyExe(src, dest string) error {
	if a, err := os.Stat(src); err == nil {
		if b, err := os.Stat(dest); err == nil && os.SameFile(a, b) {
			return nil
		}
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dest + ".new"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("could not replace %s (is Claude Code running it? close your Claude Code sessions and try again): %w", dest, err)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/install/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/install
git commit -m "feat: install and uninstall by editing settings.json"
```

---

### Task 7: Install and uninstall commands

**Files:**
- Modify: `main.go` (the `case "install", "uninstall":` branch of `run`, and the imports)
- Modify: `main_test.go` (append one test)

**Interfaces:**
- Consumes: `install.Install`, `install.Uninstall`, `install.Options`, `install.ExeName` (Task 6).
- Produces: `func installCmd(args []string, out, errw io.Writer) int` and `func uninstallCmd(args []string, out, errw io.Writer) int`. Both accept `--settings <path>` and `--dest <dir>`; `installCmd` also accepts `--force` and `--refresh <seconds>` (default 10). Defaults are `~/.claude/settings.json` and `~/.claude`.

- [ ] **Step 1: Append the failing test to `main_test.go`**

Add `"os"`, `"path/filepath"` to the import block, then append:
```go
func TestInstallAndUninstallCommands(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "settings.json")
	dest := filepath.Join(dir, "bin")
	if err := os.WriteFile(settings, []byte(`{"theme":"dark"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errw bytes.Buffer
	code := run([]string{"install", "--settings", settings, "--dest", dest}, strings.NewReader(""), &out, &errw)
	if code != 0 {
		t.Fatalf("install: code %d, stderr %q", code, errw.String())
	}
	data, _ := os.ReadFile(settings)
	if !strings.Contains(string(data), "statusLine") || !strings.Contains(string(data), `"refreshInterval": 10`) {
		t.Errorf("settings after install: %s", data)
	}

	out.Reset()
	errw.Reset()
	code = run([]string{"uninstall", "--settings", settings, "--dest", dest}, strings.NewReader(""), &out, &errw)
	if code != 0 {
		t.Fatalf("uninstall: code %d, stderr %q", code, errw.String())
	}
	data, _ = os.ReadFile(settings)
	if strings.Contains(string(data), "statusLine") || !strings.Contains(string(data), "dark") {
		t.Errorf("settings after uninstall: %s", data)
	}
}

func TestInstallRefusedWithoutForce(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(settings, []byte(`{"statusLine":{"command":"other.sh"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errw bytes.Buffer
	code := run([]string{"install", "--settings", settings, "--dest", filepath.Join(dir, "bin")}, strings.NewReader(""), &out, &errw)
	if code != 1 || !strings.Contains(errw.String(), "--force") {
		t.Errorf("code %d, stderr %q", code, errw.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test . -run 'Install'`
Expected: FAIL (`install: code 2` because the commands are not available yet)

- [ ] **Step 3: Implement the commands**

In `main.go`, change the import block to:
```go
import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mountaser/claude-gauges/internal/gitinfo"
	"github.com/mountaser/claude-gauges/internal/install"
	"github.com/mountaser/claude-gauges/internal/render"
	"github.com/mountaser/claude-gauges/internal/session"
)
```

Replace the `case "install", "uninstall":` branch in `run` with:
```go
	case "install":
		return installCmd(args[1:], out, errw)
	case "uninstall":
		return uninstallCmd(args[1:], out, errw)
```

Append to `main.go`:
```go
func defaultPaths(settings, dest string) (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil && (settings == "" || dest == "") {
		return "", "", err
	}
	if dest == "" {
		dest = filepath.Join(home, ".claude")
	}
	if settings == "" {
		settings = filepath.Join(home, ".claude", "settings.json")
	}
	return settings, dest, nil
}

func installCmd(args []string, out, errw io.Writer) int {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(errw)
	force := fs.Bool("force", false, "replace an existing status line (it is saved for uninstall)")
	settings := fs.String("settings", "", "settings.json to edit (default ~/.claude/settings.json)")
	dest := fs.String("dest", "", "folder to install the program into (default ~/.claude)")
	refresh := fs.Int("refresh", 10, "seconds between status line refreshes")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	sp, dp, err := defaultPaths(*settings, *dest)
	if err != nil {
		fmt.Fprintln(errw, "claude-gauges:", err)
		return 1
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(errw, "claude-gauges:", err)
		return 1
	}
	cmd, err := install.Install(install.Options{
		SettingsPath:   sp,
		SourceExe:      exe,
		DestDir:        dp,
		Force:          *force,
		RefreshSeconds: *refresh,
	})
	if err != nil {
		fmt.Fprintln(errw, "claude-gauges:", err)
		return 1
	}
	fmt.Fprintf(out, "Installed.\n  program:  %s\n  settings: %s\nRestart Claude Code to see the new status line.\n", cmd, sp)
	return 0
}

func uninstallCmd(args []string, out, errw io.Writer) int {
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	fs.SetOutput(errw)
	settings := fs.String("settings", "", "settings.json to edit (default ~/.claude/settings.json)")
	dest := fs.String("dest", "", "folder the program was installed into (default ~/.claude)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	sp, dp, err := defaultPaths(*settings, *dest)
	if err != nil {
		fmt.Fprintln(errw, "claude-gauges:", err)
		return 1
	}
	removed, err := install.Uninstall(sp, dp)
	if err != nil {
		fmt.Fprintln(errw, "claude-gauges:", err)
		return 1
	}
	fmt.Fprintf(out, "Status line restored in %s.\n", sp)
	if !removed {
		fmt.Fprintf(out, "The program file in %s was left in place (it may be in use). Delete it by hand.\n", dp)
	}
	return 0
}
```

- [ ] **Step 4: Run all tests**

Run: `go vet ./... && go test ./...`
Expected: PASS for every package

- [ ] **Step 5: Commit**

```bash
git add main.go main_test.go
git commit -m "feat: install and uninstall commands"
```

---

### Task 8: README, build, and verification

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write the README**

`README.md`:
````markdown
# claude-gauges

A status line for the Claude Code CLI. Two lines under your prompt:

```
screentether │ master │ Opus 5.5 · high │ ctx ◑ 420k/1M
5h ◑ 42% ↺ 2h14m │ 7d ◕ 69% ↺ 3d 6h │ cache ● 3m12s
```

- **Project, branch**: the repository and current branch. In a git worktree, the main repository's name.
- **Model · effort**: the model and the effort level that is set.
- **ctx**: tokens in the context window out of its size. Counts up; the pie fills.
- **5h, 7d**: how much of each usage limit is left (100% is untouched) and the time until it resets. The pie empties as you use it.
- **cache**: time until the prompt cache expires. When it has expired it shows `▲ cold · +461k tokens to reload`, the tokens the next message has to re-read.

A missing value shows `–`. Limits and cache data arrive from Claude Code after the first reply in a session.

Pies go green, yellow and red as limits run out or the context fills. Set `NO_COLOR` to turn colors off.

## Install

Download or build `claude-gauges`, then run it once:

```
claude-gauges install
```

This copies the program to `~/.claude/` and sets `statusLine` in `~/.claude/settings.json`, keeping your other settings and saving a backup as `settings.json.bak`. Restart Claude Code.

If a status line is already configured, `install` stops. Run `claude-gauges install --force` to replace it; the old one is saved and `uninstall` restores it.

Options: `--settings <path>`, `--dest <dir>`, `--refresh <seconds>` (default 10).

## Uninstall

```
claude-gauges uninstall
```

## Build

Requires Go 1.22 or newer.

```
go test ./...
go build -ldflags "-s -w -X main.version=0.1.0" -o claude-gauges.exe .
```

Cross-compile with `GOOS` and `GOARCH`, for example `GOOS=linux GOARCH=amd64 go build -o claude-gauges .`

The pie glyphs need a terminal font that includes the Geometric Shapes block.

## License

MIT. See `LICENSE` and `NOTICE.md`.
````

- [ ] **Step 2: Run the full test suite and vet**

Run: `go vet ./... && go test ./...`
Expected: PASS for every package

- [ ] **Step 3: Build the Windows executable**

Run: `go build -ldflags "-s -w -X main.version=0.1.0" -o claude-gauges.exe .`
Expected: no output; `claude-gauges.exe` exists

- [ ] **Step 4: Run it against sample input**

Run (bash):
```bash
now=$(date +%s)
printf '{"workspace":{"current_dir":"%s"},"model":{"id":"claude-opus-5-5"},"effort":{"level":"high"},"context_window":{"total_input_tokens":420000,"context_window_size":1000000},"rate_limits":{"five_hour":{"used_percentage":58,"resets_at":%d},"seven_day":{"used_percentage":31,"resets_at":%d}},"prompt_cache":{"warm":true,"caching_observed":true,"expires_at":%d,"recache_tokens_if_cold":461120}}' "$PWD" $((now+8040)) $((now+281000)) $((now+192)) | ./claude-gauges.exe
```
Expected: two lines like
```
claude-gauges │ <branch> │ Opus 5.5 · high │ ctx ◑ 420k/1M
5h ◑ 42% ↺ 2h13m │ 7d ◕ 69% ↺ 3d 6h │ cache ● 3m11s
```
(the exact minutes and seconds differ by a moment; the project name is the folder, the branch is the current branch).

- [ ] **Step 5: Check the other platforms compile**

Run:
```bash
mkdir -p dist
GOOS=linux GOARCH=amd64 go build -o dist/claude-gauges-linux-amd64 .
GOOS=linux GOARCH=arm64 go build -o dist/claude-gauges-linux-arm64 .
GOOS=darwin GOARCH=amd64 go build -o dist/claude-gauges-darwin-amd64 .
GOOS=darwin GOARCH=arm64 go build -o dist/claude-gauges-darwin-arm64 .
ls dist
```
Expected: four files listed. These are compile checks only; they are not run.

- [ ] **Step 6: Try install and uninstall against a scratch settings file**

Run:
```bash
tmp=$(mktemp -d)
echo '{"theme":"dark"}' > "$tmp/settings.json"
./claude-gauges.exe install --settings "$tmp/settings.json" --dest "$tmp/bin"
cat "$tmp/settings.json"
./claude-gauges.exe uninstall --settings "$tmp/settings.json" --dest "$tmp/bin"
cat "$tmp/settings.json"
```
Expected: after install the file contains `theme` and a `statusLine` with a forward-slash `command` ending in `/claude-gauges.exe` and `"refreshInterval": 10`; after uninstall only `theme` remains.

- [ ] **Step 7: Commit**

```bash
git add README.md
git commit -m "docs: README with usage, install and build"
```

- [ ] **Step 8: Install for real and check in Claude Code**

Run: `./claude-gauges.exe install` (add `--force` only if it refuses because another status line exists; the old one is saved and `claude-gauges uninstall` restores it).
Then restart Claude Code and confirm the two lines appear, the countdowns tick about every 10 seconds, and the pies look right in the terminal font. If a glyph renders as an empty box, the terminal font lacks the Geometric Shapes block; switch the terminal font.
