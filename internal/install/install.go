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

// absPath makes p absolute so the command written to settings.json works
// from any working directory.
func absPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
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
	o.SettingsPath, o.DestDir = absPath(o.SettingsPath), absPath(o.DestDir)
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
	settingsPath, destDir = absPath(settingsPath), absPath(destDir)
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
	os.Remove(dest + ".old")
	err = os.Remove(dest)
	return err == nil || errors.Is(err, os.ErrNotExist), nil
}

func readSettings(path string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	// Windows PowerShell and some editors save JSON with a UTF-8 byte order mark.
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
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
	// Keep hook commands readable: do not escape &, < and > as \u0026 etc.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return err
	}
	// Write through a symlink to the real file and keep its permissions.
	if target, err := filepath.EvalSymlinks(path); err == nil {
		path = target
	}
	mode := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), mode); err != nil {
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
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		// Windows cannot overwrite a running program, but it can rename it away.
		old := dest + ".old"
		os.Remove(old)
		if os.Rename(dest, old) != nil {
			os.Remove(tmp)
			return fmt.Errorf("could not replace %s (close your Claude Code sessions and try again): %w", dest, err)
		}
		if err2 := os.Rename(tmp, dest); err2 != nil {
			os.Rename(old, dest)
			os.Remove(tmp)
			return fmt.Errorf("could not replace %s: %w", dest, err2)
		}
		os.Remove(old) // best effort; fails while the old copy is still running
	}
	return nil
}
