// Package session parses the JSON that Claude Code sends to a status line.
package session

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"
)

// Num is a number that tolerates the shapes Claude Code might send: a JSON
// number or a numeric string. Anything else reads as 0 and is not an error,
// so one odd field never breaks the whole status line.
type Num float64

func (n *Num) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch x := v.(type) {
	case float64:
		*n = Num(x)
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(x), 64); err == nil {
			*n = Num(f)
		}
	}
	return nil
}

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
	TotalInputTokens Num `json:"total_input_tokens"`
	Size             Num `json:"context_window_size"`
}

type Window struct {
	UsedPercentage Num `json:"used_percentage"`
	ResetsAt       Num `json:"resets_at"`
}

type RateLimits struct {
	FiveHour *Window `json:"five_hour"`
	SevenDay *Window `json:"seven_day"`
}

type PromptCache struct {
	Warm                bool `json:"warm"`
	CachingObserved     bool `json:"caching_observed"`
	ExpiresAt           Num  `json:"expires_at"`
	RecacheTokensIfCold Num  `json:"recache_tokens_if_cold"`
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
