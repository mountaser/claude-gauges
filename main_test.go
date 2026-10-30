package main

import (
	"bytes"
	"os"
	"path/filepath"
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
