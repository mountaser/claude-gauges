// Package agy reads the status snapshot that agy-foreman writes while it delegates work.
package agy

import (
	"encoding/json"
	"os"
	"time"
)

const (
	// Version is the snapshot format this package understands.
	Version = 1
	// MaxAge is how old a snapshot may be before it is ignored.
	MaxAge = 15 * time.Second
)

type Activity struct {
	Kind   string `json:"kind"`
	Target string `json:"target"`
	At     string `json:"at"`
}

type DiffStat struct {
	Files   int `json:"files"`
	Added   int `json:"added"`
	Deleted int `json:"deleted"`
}

type Job struct {
	ID       string    `json:"id"`
	Step     *int      `json:"step"`
	Title    string    `json:"title"`
	State    string    `json:"state"`
	Phase    string    `json:"phase"`
	Activity *Activity `json:"activity"`
}

type Plan struct {
	File       *string   `json:"file"`
	Steps      []string  `json:"steps"`
	Done       int       `json:"done"`
	Failed     int       `json:"failed"`
	ElapsedSec float64   `json:"elapsedSec"`
	Diff       *DiffStat `json:"diff"`
}

type Window struct {
	Remaining float64 `json:"remaining"`
	ResetAt   string  `json:"resetAt"`
}

type Quota struct {
	FiveHour  *Window `json:"fiveHour"`
	Weekly    *Window `json:"weekly"`
	FetchedAt string  `json:"fetchedAt"`
}

type Snapshot struct {
	Version   int    `json:"version"`
	UpdatedAt string `json:"updatedAt"`
	Pid       int    `json:"pid"`
	Active    bool   `json:"active"`
	Plan      *Plan  `json:"plan"`
	Jobs      []Job  `json:"jobs"`
	Quota     *Quota `json:"quota"`
}

// Parse decodes a snapshot and reports whether it is worth showing: a known version, active, with
// a plan that has steps, and fresh.
func Parse(data []byte, now time.Time) (*Snapshot, bool) {
	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, false
	}
	if s.Version != Version || !s.Active || s.Plan == nil || len(s.Plan.Steps) == 0 {
		return nil, false
	}
	updated, err := time.Parse(time.RFC3339, s.UpdatedAt)
	if err != nil || now.Sub(updated) > MaxAge {
		return nil, false
	}
	return &s, true
}

// Read loads and validates the snapshot file. A missing or unreadable file gives false.
func Read(path string, now time.Time) (*Snapshot, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return Parse(data, now)
}

func stateDir(goos, home string, getenv func(string) string) string {
	if d := getenv("FOREMAN_STATE_DIR"); d != "" {
		return d
	}
	switch goos {
	case "windows":
		base := getenv("LOCALAPPDATA")
		if base == "" {
			base = home + `\AppData\Local`
		}
		return base + `\agy-foreman`
	case "darwin":
		return home + "/Library/Application Support/agy-foreman"
	default:
		base := getenv("XDG_STATE_HOME")
		if base == "" {
			base = home + "/.local/state"
		}
		return base + "/agy-foreman"
	}
}

// DefaultPath is where agy-foreman writes status.json on the given platform.
func DefaultPath(goos, home string, getenv func(string) string) string {
	dir := stateDir(goos, home, getenv)
	if goos == "windows" {
		return dir + `\status.json`
	}
	return dir + "/status.json"
}
