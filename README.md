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

Options: `--settings <path>`, `--dest <dir>`, `--refresh <seconds>` (default 1).

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
