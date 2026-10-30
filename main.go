// Command claude-gauges prints a status line for Claude Code.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/mountaser/claude-gauges/internal/agy"
	"github.com/mountaser/claude-gauges/internal/gitinfo"
	"github.com/mountaser/claude-gauges/internal/install"
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
	case "install":
		return installCmd(args[1:], out, errw)
	case "uninstall":
		return uninstallCmd(args[1:], out, errw)
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
	var snap *agy.Snapshot
	home, _ := os.UserHomeDir()
	if sn, ok := agy.Read(agy.DefaultPath(runtime.GOOS, home, os.Getenv), time.Now()); ok {
		snap = sn
	}
	cols, _ := strconv.Atoi(os.Getenv("COLUMNS"))
	fmt.Fprintln(out, render.Render(s, render.Options{
		Now:     time.Now(),
		Color:   os.Getenv("NO_COLOR") == "",
		Git:     gitinfo.Read,
		Agy:     snap,
		Columns: cols,
	}))
	return 0
}

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
	refresh := fs.Int("refresh", 1, "seconds between status line refreshes")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *refresh < 1 {
		fmt.Fprintln(errw, "claude-gauges: --refresh must be at least 1 second")
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
