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
		cb := filepath.Base(filepath.Clean(common))
		if cb == ".git" {
			info.Repo = filepath.Base(filepath.Dir(filepath.Clean(common)))
		} else {
			info.Repo = strings.TrimSuffix(cb, ".git") // bare repository
		}
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
