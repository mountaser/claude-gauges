# Delegation lines design

## Purpose

While agy-foreman is delegating work to Gemini, claude-gauges adds two lines under its own two, in the same visual style. They say what is happening, how far the plan is and what Gemini has produced. When nothing is delegated the lines are not shown.

The data comes from the `status.json` file agy-foreman writes (format in agy-foreman's `docs/superpowers/specs/2026-10-30-status-snapshot-design.md`). claude-gauges only reads that file. It never starts agy-foreman and never writes to it.

This extends `2026-10-30-claude-gauges-design.md`.

## Output

```
screentether │ main │ Opus 5.5 · high │ ctx ◑ 420k/1M
5h ◑ 42% ↺ 2h14m │ 7d ◕ 69% ↺ 3d 6h │ cache ● 3m12s
agy-foreman ●●◐○○ 2/5 │ #4 Render the two lines › editing render.go
5h ● 94% ↺ 3h12m │ 7d ◕ 70% ↺ 4d 0h │ plan 10m51s │ tok 424k │ diff 2 files +217
```

### Line 3

`agy-foreman <strip> <done>/<total>[ · N running][ · N failed] │ <jobs>`

- **Strip:** one glyph per entry of `plan.steps`, in order: `passed` green `●`, `running` cyan spinner, `failed` red `●`, `queued` and `pending` dim `○`. The spinner is one of `◐ ◓ ◑ ◒`, chosen by the current second (`unix time mod 4`), so it moves with each refresh.
- **Count:** `plan.done` over the number of steps, in bold. `· N running` appears when more than one step is running, `· N failed` (red) when `plan.failed` is above 0.
- **One job:** `#<step> <title> › <activity>`. The `#<step> ` part is left out when `step` is null.
- **Several jobs:** each job as `#<step> <activity>`, joined by ` · `. Titles are left out.
- **No jobs listed:** the part after `│` is left out.

### Activity wording

| Phase | Kind | Shown |
|---|---|---|
| `queued` | | `queued` (dim) |
| `awaiting_approval` | | `waiting for approval` (yellow) |
| `prepare` | | `preparing worktree` |
| `contract` | | `checking contract tests` |
| `brief` | | `writing brief` |
| `verify` | | `verifying` |
| `report` | | `committing` |
| `run` | none yet | `working` |
| `run` | `reading` | `reading <target>` |
| `run` | `editing` | `editing <target>` |
| `run` | `testing` | `running tests` |
| `run` | `git` | `checking changes` |
| `run` | `command` | `running a command` |
| `run` | `thinking` | `thinking <n>s` when the activity is 5 seconds old or more, else `thinking` |

The verb is cyan and the target is bold. `<target>` is left out when the activity has none. Any other phase is shown as its own name in cyan.

### Line 4

`5h … │ 7d … │ plan <time> │ tok <n> │ diff …`, left to right:

- **Limits:** `5h` and `7d` from `quota.fiveHour` and `quota.weekly`, each drawn exactly like the Claude limits on line 2: a pie for the remaining percentage, the rounded percentage in bold, and `↺ <time to reset>` computed from `resetAt`. A limit that is missing is left out, and so are both when `quota` is missing.
- **plan:** `plan <time>` with `plan.elapsedSec` formatted like the cache countdown (`10m51s`, `1h05m`).
- **tok:** `tok <n>` with `plan.tokens` formatted like context tokens (`424k`, `1.2M`, `0`).
- **diff:** `diff <n> file(s) +<added>` in bold, with green added lines and, when above 0, a red `−<deleted>`. Without a diff, or with 0 files, `diff –`.

The pie and its colors follow the same thresholds as the Claude limits.

### Fitting a narrow terminal

The width is the `COLUMNS` environment variable when it is a positive number. Without it nothing is shortened.

- **Line 3** with exactly one job: when it is wider than the terminal, the title is shortened with `…` by exactly the excess, as long as the title keeps at least 8 characters.
- **Line 4**: when too wide, drop in this order and stop as soon as it fits: the reset countdowns of both limits, then `tok`. Nothing else is dropped.

Widths are counted in characters.

## When the lines show

The two lines are shown only when the snapshot file exists, can be read, has `version` 1, has `active: true`, has an `updatedAt` no more than 15 seconds before now and has a plan with at least one step. In every other case claude-gauges prints its own two lines only.

The file is read from agy-foreman's state folder: `FOREMAN_STATE_DIR` if set, else `%LOCALAPPDATA%\agy-foreman` on Windows, `~/Library/Application Support/agy-foreman` on macOS, and `$XDG_STATE_HOME/agy-foreman` or `~/.local/state/agy-foreman` on Linux, plus `status.json`.

## Refresh

`claude-gauges install` registers the status line with `refreshInterval: 1`, so the countdowns, the spinner and the job lines move every second.

## Out of scope

- Failed or finished jobs once nothing is active. The lines disappear.
- Reading any other agy-foreman file than `status.json`.
- Colors other than the existing palette.
