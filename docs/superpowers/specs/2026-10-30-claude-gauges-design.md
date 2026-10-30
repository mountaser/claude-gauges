# claude-gauges Design

## Purpose

`claude-gauges` is a status line for the Claude Code CLI. It shows, under the prompt, the project and branch, the model and its effort level, how full the context window is, how much of the 5-hour and 7-day usage limits remain with a countdown to each reset, and how long the prompt cache stays warm.

It is a single compiled Go program with no runtime dependencies. Claude Code runs it for every status line update.

## Output

Two lines, with `│` between segments:

```
screentether │ master │ Opus 5.5 · high │ ctx ◑ 420k/1M
5h ◑ 42% ↺ 2h14m │ 7d ◕ 69% ↺ 3d 6h │ cache ● 3m12s
```

### Line 1

| Segment | Content | Source |
|---|---|---|
| Project | Repository folder name. In a linked git worktree, the main repository's folder name. Outside git, the base name of the current directory. | `.git` files, `workspace.current_dir`, `cwd` |
| Branch | Current branch, or the first 7 characters of the commit when HEAD is detached. In a worktree, the worktree folder name follows in parentheses unless the branch name and the folder name contain one another. Omitted outside git. | `.git/HEAD` |
| Model | Family and version taken from `model.id`, for example `claude-opus-5-5` gives `Opus 5.5`. The id may carry a provider prefix (`us.anthropic.claude-sonnet-4-5-...`); suffixes such as `[1m]`, `@20250805`, `-v1:0` and date segments are ignored. Falls back to `model.display_name` when the id has no `claude-` part. | `model.id`, `model.display_name` |
| Effort | `· <level>` after the model name. Omitted when `effort` is absent. | `effort.level` |
| Context | `ctx <pie> <used>/<size>`, for example `ctx ◑ 420k/1M`. Counts up from 0. | `context_window.total_input_tokens`, `context_window.context_window_size` |

### Line 2

| Segment | Content | Source |
|---|---|---|
| 5-hour limit | `5h <pie> <remaining>% ↺ <time to reset>` | `rate_limits.five_hour` |
| 7-day limit | `7d <pie> <remaining>% ↺ <time to reset>` | `rate_limits.seven_day` |
| Cache | `cache ● <time left>` while warm, `cache ◐ <time left>` in the last minute, `▲ cold · +<n> tokens to reload` once expired | `prompt_cache` |

## Rules

### Pie glyph

One character chosen from five by fraction filled `f`:

| Range | Glyph |
|---|---|
| f < 0.125 | `○` |
| 0.125 ≤ f < 0.375 | `◔` |
| 0.375 ≤ f < 0.625 | `◑` |
| 0.625 ≤ f < 0.875 | `◕` |
| f ≥ 0.875 | `●` |

- Limits show what is left: `f = remaining / 100`, so an untouched limit is `●` and an exhausted one is `○`.
- Context shows what is used: `f = used / size`, so an empty window is `○` and a full one is `●`.

### Colors

- Limits, by remaining percentage `r`: green when `r > 50`, yellow when `20 ≤ r ≤ 50`, red when `r < 20`.
- Context, by used fraction `f`: green when `f < 0.5`, yellow when `0.5 ≤ f < 0.8`, red when `f ≥ 0.8`.
- Cache: green while more than a minute remains, yellow in the last minute, orange when cold.
- Effort: `low` dim, `medium` cyan, `high` magenta, `xhigh` orange, `max` red.
- Labels and separators are dim, project is magenta, branch is green, model is blue.
- No escape sequences are written when the `NO_COLOR` environment variable is set to a non-empty value.

### Number formats

- Tokens: below 1,000 as is; from 1,000 to the million mark as thousands rounded to the nearest integer plus `k` (`420k`); a whole number of millions as `1M`; otherwise one decimal plus `M` (`1.5M`).
- Reset countdown: `<h>h<mm>m` below one day (`2h14m`, `0h26m`), `<d>d <h>h` from one day up (`3d 6h`). Past times show `0h00m`.
- Cache countdown: `<m>m<ss>s` below one hour (`3m12s`), `<h>h<mm>m` from one hour up.
- Remaining percentage is clamped to 0 through 100 and rounded to a whole number first; the pie, the color and the number shown all use the rounded value.
- Numeric input fields may arrive as numbers or numeric strings. A value that cannot be read counts as absent and never fails the whole parse.

### Missing data

Claude Code sends some fields only after the first reply, only to subscribers, or only for some models. A missing piece is shown as a dash and never causes a failure:

- No `rate_limits.five_hour` gives `5h –`; no `rate_limits.seven_day` gives `7d –`.
- No `prompt_cache`, or `caching_observed` false, gives `cache –`.
- A context window size of 0 or absent gives `ctx –`.
- A limit whose `resets_at` is 0 or absent shows no reset countdown.
- No `effort` leaves the model name alone.

The cache is cold when `warm` is false or `expires_at` is not in the future. The text `+<n> tokens to reload` appears only when `recache_tokens_if_cold` is above 0.

### Failure behavior

Rendering always exits with status 0. Input that is not valid JSON prints `claude-gauges: could not read session data` on one line.

## Commands

| Command | Behavior |
|---|---|
| (no arguments) | Reads the session JSON on stdin and prints the two lines. |
| `install` | Installs the program and registers it as the status line. |
| `uninstall` | Restores the previous status line. |
| `version` | Prints the version. |
| `help` | Prints usage. |

### install

Flags: `--force`, `--settings <path>` (default `~/.claude/settings.json`), `--dest <dir>` (default `~/.claude`), `--refresh <seconds>` (default 1).

1. Read `settings.json`. A missing or empty file counts as `{}`. Invalid JSON stops the install and leaves the file untouched.
2. If a `statusLine` entry exists whose command is not this program's command, stop with an error unless `--force` was given. Nothing has been copied or changed at this point. With `--force`, save the old entry to `claude-gauges.previous.json` in the destination folder.
3. Make `--settings` and `--dest` absolute, create the destination folder, and copy the running executable into it as `claude-gauges` (`claude-gauges.exe` on Windows). Copying is skipped when the executable already is the destination file. On Windows a running copy is renamed to `.old` so it can be replaced.
4. Copy the existing `settings.json` to `settings.json.bak`.
5. Set `statusLine` to `{"type": "command", "command": "<path>", "refreshInterval": <seconds>}` and keep every other key. A symlinked `settings.json` is written through to its target and file permissions are kept. Text such as `&&` in other entries is not escaped. The file is written to a temporary file and renamed into place. Key order in the file may change.

`--refresh` must be at least 1.

The path in `command` uses forward slashes on every platform and is wrapped in double quotes when it contains a space.

### uninstall

- If `statusLine` is not this program's entry, stop with a message and change nothing.
- If `claude-gauges.previous.json` exists, restore that entry and delete the file. Otherwise remove the `statusLine` key.
- Back up `settings.json` to `settings.json.bak` first.
- Delete the installed executable. If it cannot be deleted because it is running, say so and succeed anyway.

## Technical choices

- Language: Go, standard library only. Module path `github.com/mountaser/claude-gauges`.
- Branch detection reads `.git/HEAD` directly and never starts `git`.
- Layout: `internal/session` (input), `internal/gauge` (pie and number formatting), `internal/gitinfo` (branch and worktree), `internal/render` (the two lines), `internal/install` (settings editing), `main.go` (command dispatch).
- Time is passed into rendering as a value so output is deterministic in tests.
- Output is UTF-8 on every platform. The pie glyphs need a terminal font that contains the geometric shapes block.

## Out of scope

- Images or graphics in the status line.
- Reading usage from any network service.
- Configuration files or per-segment toggles.
- Automated release builds. Builds are local for now; release automation can be added later.
- Verifying Linux and macOS behavior at runtime. They are cross-compiled to confirm the code builds.

## Licensing

MIT. The worktree handling follows the approach of the MIT-licensed moo-statusline project, so a `NOTICE.md` credits it and reproduces its copyright notice.
