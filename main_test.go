package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// No test may read the real status file of a running agy-foreman.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "gauges-state-")
	if err != nil {
		os.Exit(1)
	}
	os.Setenv("FOREMAN_STATE_DIR", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

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
	if !strings.Contains(string(data), "statusLine") || !strings.Contains(string(data), `"refreshInterval": 1`) {
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

func TestInstallRejectsBadRefresh(t *testing.T) {
	var out, errw bytes.Buffer
	code := run([]string{"install", "--refresh", "0"}, strings.NewReader(""), &out, &errw)
	if code != 2 || !strings.Contains(errw.String(), "--refresh") {
		t.Errorf("code %d, stderr %q", code, errw.String())
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

func snapshotFile(t *testing.T, updated time.Time) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("FOREMAN_STATE_DIR", dir)
	doc := fmt.Sprintf(`{"version":1,"updatedAt":%q,"pid":1,"active":true,
		"plan":{"file":null,"steps":["running"],"done":0,"failed":0,"elapsedSec":5,"tokens":0},
		"jobs":[{"id":"a","step":null,"title":"Solo task","state":"running","phase":"prepare"}]}`,
		updated.UTC().Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(dir, "status.json"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunShowsDelegationLinesFromSnapshot(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	snapshotFile(t, time.Now())
	var out, errw bytes.Buffer
	in := strings.NewReader(`{"model":{"id":"claude-opus-5-5"},"workspace":{"current_dir":"/work/demo"}}`)
	if code := run(nil, in, &out, &errw); code != 0 {
		t.Fatalf("code %d", code)
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	// The spinner glyph depends on the current second, so only the rest of the line is compared.
	if len(lines) != 4 || !strings.HasPrefix(lines[2], "agy-foreman ") ||
		!strings.HasSuffix(lines[2], " 0/1 │ Solo task › preparing worktree") {
		t.Errorf("output:\n%s", out.String())
	}
}

func TestRunIgnoresStaleSnapshot(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	snapshotFile(t, time.Now().Add(-time.Minute))
	var out, errw bytes.Buffer
	in := strings.NewReader(`{"model":{"id":"claude-opus-5-5"},"workspace":{"current_dir":"/work/demo"}}`)
	run(nil, in, &out, &errw)
	if got := strings.Count(strings.TrimRight(out.String(), "\n"), "\n"); got != 1 {
		t.Errorf("a stale snapshot must not add lines:\n%s", out.String())
	}
}
