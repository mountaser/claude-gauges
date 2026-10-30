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
