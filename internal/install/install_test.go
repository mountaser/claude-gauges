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
	if !strings.HasSuffix(strings.Trim(cmd, `"`), "/"+ExeName()) || strings.Contains(cmd, `\`) {
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

func TestInstallKeepsHookTextReadable(t *testing.T) {
	o, _ := setup(t)
	writeFile(t, o.SettingsPath, `{"hooks":{"Stop":[{"command":"a && b > out"}]}}`)
	if _, err := Install(o); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(o.SettingsPath)
	if !strings.Contains(string(data), "a && b > out") {
		t.Errorf("hook text was escaped:\n%s", data)
	}
}

func TestInstallMakesRelativeDestAbsolute(t *testing.T) {
	o, dir := setup(t)
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })
	o.DestDir = "rel-bin"
	cmd, err := Install(o)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(strings.Trim(cmd, `"`)) {
		t.Errorf("command should be absolute: %s", cmd)
	}
}

func TestInstallWritesThroughSymlink(t *testing.T) {
	o, dir := setup(t)
	real := filepath.Join(dir, "real-settings.json")
	writeFile(t, real, `{"theme":"dark"}`)
	if err := os.Symlink(real, o.SettingsPath); err != nil {
		t.Skipf("symlinks not available: %v", err)
	}
	if _, err := Install(o); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Lstat(o.SettingsPath); err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Error("settings.json should still be a symlink")
	}
	if readJSON(t, real)["statusLine"] == nil {
		t.Error("the symlink target was not updated")
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

func TestInstallAcceptsByteOrderMark(t *testing.T) {
	o, _ := setup(t)
	writeFile(t, o.SettingsPath, "\xef\xbb\xbf"+`{"theme":"dark"}`)
	if _, err := Install(o); err != nil {
		t.Fatalf("a settings file with a byte order mark must be accepted: %v", err)
	}
	m := readJSON(t, o.SettingsPath)
	if m["theme"] != "dark" || m["statusLine"] == nil {
		t.Errorf("settings = %v", m)
	}
}
