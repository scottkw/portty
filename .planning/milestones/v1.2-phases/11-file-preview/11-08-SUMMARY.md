---
phase: 11-file-preview
plan: 08
subsystem: cli
tags: [go, cli, preview, menu, osascript, install]
status: complete

requires:
  - "11-03: PORTTY_SESSION_ID in every new session"
  - "11-04: POST /sessions/{id}/preview, PreviewAPI 3, DaemonClient.SendPreview typed errors"
provides:
  - "`portty open <file>`: reads, checks and sends a file to its session's daemon, one UI-SPEC line per failure"
  - "Portty ▸ Install Command-Line Tool… (links /usr/local/bin/portty through the admin prompt)"
  - "README install step 2 points at the menu item"
affects: [11-10 (VM checks and TESTING.md rows), website repo (step 2 copy)]

actuals:
  tokens: 10300    # chars/4 over the realized diff (41265 chars, 11 files)
  tasks: 3
  commits: 3       # measured: git rev-list --count d4e8ac00..HEAD before the SUMMARY commit
plan_head_before: d4e8ac0074e6ded125a1e77ed36fe1f56541ee83
plan_head_after: 69118864c4a52a3eb138322de12ba16a7c98b3c1

tech-stack:
  added: []
  patterns:
    - "Package-level seams for the install (link path, executable resolver, privileged runner), like menuEmit"
    - "osascript argv items + AppleScript `quoted form of`: no Go string spliced into shell text"

key-files:
  created:
    - cmd_open.go
    - cmd_open_test.go
    - app_cli_install.go
    - app_cli_install_test.go
  modified:
    - main.go
    - cmd_cli.go
    - cmd_cli_test.go
    - dispatch_test.go
    - testmain_test.go
    - menu_test.go
    - README.md

key-decisions:
  - "`open` skips runCLI's daemon-flag parsing and ensureCLIDaemon entirely; cmdOpen drops --restart-daemon/--keep-daemon wherever they appear"
  - "Non-regular files (FIFO, device) are refused as an unsupported type, and the read is capped at 20,000,001 bytes, so /dev/zero or a pipe can't block or grow memory"
  - "Install seams are package vars, not App fields, so app.go stays untouched"

requirements-completed: [PREV-01, PREV-03, PREV-05]

duration: ~50min
completed: 2026-11-21
---

# Phase 11 Plan 08: `portty open` and Install Command-Line Tool… Summary

**`portty open <file>` reads the file in the shell, checks the type by content (or by extension plus content for SVG and Markdown), and posts the bytes, base name, MIME type and a path-derived key to the daemon hosting the session. It never starts or restarts that daemon. Portty ▸ Install Command-Line Tool… links `/usr/local/bin/portty` to this copy through the macOS admin prompt, and never replaces anything already at that path.**

This plan touched 11 files (accepted at plan check, W2: the CLI case and the menu item both live in main.go).

## CLI lines as built (cmd_open.go)

Errors go to stderr through runCLI (`%v\n`, exit 1). `<arg>` is the path as typed, in straight double quotes (`%q`). Success prints nothing and exits 0.

| Case | Line |
|------|------|
| no argument, or more than one | `usage: portty open <file>` |
| `-h` / `--help` alone (stdout, exit 0) | `usage: portty open <file>` then `Show an image, PDF or Markdown file in a preview tab.` |
| no `PORTTY_SESSION_ID`, not 32 lowercase hex, or the daemon answers 404/409 | `portty: this shell isn't a Portty session. Run portty open in a Portty terminal tab.` |
| missing | `portty: "<arg>" doesn't exist.` |
| folder | `portty: "<arg>" is a folder, not a file.` |
| stat/open/read error | `portty: couldn't read "<arg>": <OS reason>.` (e.g. `permission denied`) |
| unsupported (also non-regular files, and a daemon 415) | `portty: can't preview this file type ("<arg>"). Portty previews PNG, JPEG, GIF, WebP and SVG images, PDFs and Markdown files.` |
| over 20,000,000 bytes (from stat, rechecked on the bytes read, and on a daemon 413) | `portty: "<arg>" is too large to preview (20.0 MB; the limit is 20 MB).` (decimal MB, one decimal) |
| daemon API below PreviewAPI (3) | `portty: update needed. Portty's background service (version <v>) can't show previews; relaunch Portty to update it.` (`unknown` if the daemon reports no version) |
| dial/health failure or an unexpected status | `portty: couldn't reach Portty's background service: <detail>.` (a dial error shows `dial unix <socket>: connect: <reason>`) |
| 202 with 0 viewers | `portty: nothing is showing this session, so no preview opened.` |
| 429 after SendPreview's retry budget (the busy line, accepted in CONTEXT "Decided at planning") | `portty: another preview is still being sent to this session. Try again in a moment.` |

Order: arguments → session id → stat/cap → read → type → key → `/health` and API gate → SendPreview → viewers. File errors never touch the socket (the test counts POSTs and asserts 0).

## Detection and key rules

- `http.DetectContentType` on the bytes: `image/png`, `image/jpeg`, `image/gif`, `image/webp` and `application/pdf` are accepted by content, whatever the extension.
- SVG: the `.svg` extension (case-insensitive) plus an `<svg` root (followed by whitespace, `>` or `/`) within the first 1 KiB, after an optional UTF-8 BOM, `<?…?>`, `<!--…-->` and `<!DOCTYPE…>`. `shortcut:` a DOCTYPE internal subset that contains `>` reads as unsupported (the safe direction).
- Markdown: `.md` or `.markdown`, valid UTF-8 and no NUL byte. A zero-byte `.md` is Markdown. A zero-byte image or PDF is unsupported.
- Key: lowercase hex of the first 16 bytes of SHA-256 over `filepath.EvalSymlinks(filepath.Abs(arg))`. The name sent is `filepath.Base(arg)`. The path never leaves the CLI.

## Install dialogs (app_cli_install.go)

All are sent through `a.dialog` with one **OK** button, from a goroutine started by the menu callback.

| Case | Type | Title / Message |
|------|------|-----------------|
| exe under `/Volumes/` or containing `/AppTranslocation/` | Warning | **Move Portty to the Applications folder first.** / **Portty is running from <folder containing Portty.app>, so a command installed now would stop working. Move Portty to Applications, open it from there, then try again.** |
| link resolves to this exe | Info | **The portty command is already installed.** / **/usr/local/bin/portty already points to this copy of Portty.** |
| any other symlink, dangling included | Warning | **Another portty command is already installed.** / **/usr/local/bin/portty points to <raw readlink>. Portty didn't change it. To use this copy instead, remove that link, then choose Install Command-Line Tool… again.** |
| anything else that exists | Warning | same title / **/usr/local/bin/portty is a file Portty didn't install. Portty didn't change it. …remove that file…** |
| cancel (exit 1 and `-128` in stderr) | none | nothing changed |
| other failure (runner error, or an exe or lstat error) | Warning | **Couldn't install the portty command.** / **<detail>. Try again, or create the link yourself in Terminal: sudo ln -s "<binary path>" /usr/local/bin/portty** (the raw osascript stderr is logged as `cli install: …`) |
| success | Info | **The portty command is installed.** / **/usr/local/bin/portty now points to Portty. Try “portty list” in Terminal.** |

osascript argv: `-e "on run argv"`, `-e 'do shell script "/bin/mkdir -p /usr/local/bin && /bin/ln -s " & quoted form of item 1 of argv & " " & quoted form of item 2 of argv with prompt "Portty wants to install the portty command in /usr/local/bin." with administrator privileges'`, `-e "end run"`, `<exe>`, `<link>`.

Menu: Portty ▸ About Portty, ─, Settings… ⌘,, **Install Command-Line Tool…** (no shortcut), ─, Hide Portty, ─, Quit Portty.

## Website step 2

For the website session (its own repo), the UI-SPEC copy:

> In Portty, choose Portty ▸ Install Command-Line Tool… to use portty in Terminal and other apps. Inside Portty it already works.

## Tests (for 11-10's TESTING.md rows)

| File | Tests |
|------|-------|
| `cmd_open_test.go` | `TestCmdOpen_EndToEnd` (real daemon API on a temp unix socket, hub with an io.Pipe, plain relay.Subscriber), `TestCmdOpen_Lines` (18 cases, exact lines, zero POSTs on file errors), `TestCmdOpen_TypeDetection`, `TestCmdOpen_Help` (also checks that the daemon flags are dropped), `TestCmdOpen_SameFileSameKey` |
| `app_cli_install_test.go` | `TestInstallCLI_Absent`, `_AlreadyOurs`, `_ForeignLink`, `_DanglingLink`, `_ForeignFile`, `_DiskImageAndTranslocated`, `_Cancel`, `_Failure`, `_OsascriptArgv` |
| `menu_test.go` | `TestAppMenu_Structure` (item at index 3, no accelerator, callback set) |
| `cmd_cli_test.go` / `dispatch_test.go` | `TestCLINeedsDaemon` (`open` → false), `TestDispatchHelp` (`open` in usage) |

Mutation checks (each reverted): contacting the daemon before the file checks failed `TestCmdOpen_Lines` (6 POSTs). Dropping the 0-viewers check failed `zero_viewers`. Removing the cancel `return` failed `TestInstallCLI_Cancel`. Disabling the symlink branch failed AlreadyOurs, ForeignLink and DanglingLink.

All install tests use temp dirs and fake runners. Nothing ran osascript or touched `/usr/local/bin`, the user's config dir or the user's daemon.

## Gate (Task 3)

- README: `sudo ln -sf` gone; the menu item is named.
- `go build ./...` and `go vet ./...`: pass.
- Real-launch check: `go build -o /tmp/p11-08-portty .`, run with `env -u PORTTY_SESSION_ID HOME=/tmp/p11-08-home.WTAKfY XDG_CONFIG_HOME=…`:
  - `open --help` exited 0 and printed the usage and description (`/tmp/p11-08-help.out`).
  - `open x.png` exited 1 with the not-a-session line (`/tmp/p11-08-nosession.out`).
  - No `daemon.sock` exists under the temp HOME (only an empty `Library/Application Support/portty` dir, created by `DefaultSocketPath`). No GUI started.
- One `go test -race -short -count=1 ./...` (`/tmp/p11-08-gate-go.log`, exit 0): all 13 packages `ok`, including the root package (13.9 s), `internal/daemon` (82.8 s) and `internal/relay` (137.1 s). No FAIL, DATA RACE or panic.
- `--- SKIP` lines: none.
- `tests/check-brand.sh` and `tests/check-traceability-paths.sh`: pass (`/tmp/p11-08-brand.log`, `/tmp/p11-08-trace.log`).
- `internal/` and `frontend/` are unchanged since d4e8ac00; `frontend/src/wailsjs` is untouched.

## Commits

- `3b6a5811` feat(11-08): portty open
- `1febd8b6` feat(11-08): Install Command-Line Tool… menu item
- `69118864` docs(11-08): README install step 2 points at the menu item

## Deviations from Plan

1. **[Test seam] The busy case uses a 429 without Retry-After.** The plan says to shorten the client's retry budget "through its test seam", but `frozenRetryBudget` is unexported in `internal/daemon`, and this plan must not change `internal/`. A 429 without Retry-After makes `SendPreview` return `ErrPreviewBusy` immediately, the same result as after the budget runs out. That tests cmdOpen's mapping; the budget itself is pinned by 11-04's `TestSendPreview_WaitsOutBusy`.
2. **[Structure] Install seams are package vars** (`cliInstallLink`, `cliInstallExecutable`, `cliInstallRun`), not App fields, so `app.go` (not in files_modified) is untouched. This is the existing `menuEmit` pattern.
3. **[Rule 2] Non-regular files are refused** as an unsupported type, and the read goes through `io.LimitReader(cap+1)`. Without this, `portty open /dev/zero` or a FIFO would hang or read forever (the stat size is 0).
4. **[Robustness] `open` bypasses runCLI's leading-flag parsing**, so `portty open x.png --keep-daemon` doesn't exit 2 with the placement error. cmdOpen drops both flags wherever they appear ("ignored").
5. **[TDD note]** Tests and implementation were written together; the mutation checks above show the tests catch real faults.
6. **Plan-head ledger:** not written inside `.git` (the worktree sandbox refuses it, as in 11-04). The base, d4e8ac00, is recorded in `/tmp/p11-08-base`, and `commits:` is measured from it.

## Known Stubs

None.

## Threat model

- T-11-27 (argv-only paths, `quoted form of`, absolute `/bin/mkdir` and `/bin/ln`, no `-f`): covered by `TestInstallCLI_OsascriptArgv` with a hostile path.
- T-11-28 (lstat first, never overwrite, DMG/translocation refused): covered by `TestInstallCLI_*`.
- T-11-29 (`open` excluded from ensure; flags ignored; no socket in the real-launch check): covered.
- T-11-30 (only the base name and a SHA-256 key are sent): covered.
- T-11-31 (content sniffing for raster images and PDF; extension plus content for SVG and Markdown): covered.
- T-11-32 (stat cap plus a capped read): covered.

No new surface beyond the threat model.

## Self-Check: PASSED

- FOUND: cmd_open.go, cmd_open_test.go, app_cli_install.go, app_cli_install_test.go
- FOUND: 3b6a5811, 1febd8b6, 69118864 (ancestors of HEAD)
