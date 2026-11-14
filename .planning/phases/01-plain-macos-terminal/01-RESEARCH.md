# Phase 1: Plain macOS Terminal - Research

**Researched:** 2026-11-14
**Domain:** Strip-down refactor of a Go/Wails v2/React 19 codebase (forked AgentHub v4.2): delete agent-CLI, file-browser and Windows/Linux code; reshape the session model; replace awaiting-input notifications with BEL / OSC 9
**Confidence:** HIGH for the codebase inventory and coupling (read directly, baseline tests run). MEDIUM for the notification design (library behavior verified in source, end-to-end delivery needs a signed build). LOW only where flagged `[ASSUMED]`.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Session status & notifications
- Status model shrinks to running / exited / errored. Delete the agent output heuristics (`internal/status` pattern sets for claude/agy, the working/waiting/idle classification).
- Replace awaiting-input notifications with a generic trigger: when a session's PTY output contains a terminal bell (BEL `\x07` outside OSC strings) or an OSC 9 notification (`ESC ] 9 ; <text>`, excluding OSC 9;4 progress) AND the app window is hidden, fire one native macOS notification (de-duplicated or rate-limited per session). Keep the existing Settings → Behavior toggle (default OFF).
- Keep shell-exit behavior: clean exit gives a 5s countdown, toast, and Keep Open. Non-zero exit stays open with the error state.
- Keep OSC 9;4 progress (tab underline + tray quartile glyph) unchanged.

#### New session & CLI
- New sessions start `$SHELL` as a login shell. No shell picker and no arguments field.
- Starting directory defaults to the last-used directory, picked with the native folder picker (existing behavior).
- CLI `new [dir]` starts a shell. Drop the agent argument and the `--` arg passthrough. Keep the other CLI commands (list, attach, kill, daemon, etc.).
- Keep the "web-sharing a raw shell" warning as a one-time warning, re-enableable in Settings. It must not fire on every share.

#### Removal boundaries
- Remove the agent/CLI field from the session API, daemon state, and persisted session data. Old persisted sessions must still load, with the unknown field ignored (no crash).
- CI: delete the Windows and Linux build/release jobs and the winget distribute job. Keep the Playwright browser e2e on its ubuntu runner (it tests web pages, not a Linux app). macOS build/test jobs stay.
- Delete `packaging/winget`. Leave `packaging/homebrew` untouched (v2 distribution).
- Do NOT rename paths, socket, port, module path, or binary in this phase (Phase 3). While testing Portty builds in Phases 1-2, quit any installed AgentHub, because they share config paths, socket, and port 7443.
- Chat: remove the `@session` pinned target and the chat→PTY inject bridge, including its server-side endpoint. Keep `@alias` mentions, presence, typing, export, and unread badges.
- Remove: CLI detection/PATH discovery for agents (`internal/pty/detect.go` known CLIs), agent picker, agent badges/icons, per-agent args persistence, per-agent theme broadcast (OpenCode SIGUSR2), Antigravity, agent status UI (filter counts and dots that mean working/waiting).
- Remove the file browser/editor everywhere: `internal/files`, its daemon/webserver routes, the files capability, `FileBrowser*`/`Editor` components, remote-browse UI (`RemoteBrowseDNSWarning`), and web-share file panels.
- Remove Windows/Linux sources: `tray_{linux,windows}.go`, `notification_{linux,windows}.go`, `*_windows.go`/`*_linux.go` in internal packages (e.g. `internal/pty/win32input.go`, `notify_theme_windows.go`), and Windows/Linux-only dependencies (go-winio, godbus, etc.) if they become unused (`go mod tidy`).

### Claude's Discretion
- Order and granularity of removal plans, as long as every plan leaves the build and tests green.
- Whether the Hub keeps rendering in Phase 1 with agent bits stripped (expected: yes, minimal edits, since Phase 2 deletes it), or which Hub pieces are simply simplified.
- Exact BEL/OSC 9 rate-limit window.

### Deferred Ideas (OUT OF SCOPE)
- Path/socket/port/module/binary rename → Phase 3 (BRND-03).
- Hub removal and sidebar → Phase 2.
- Homebrew packaging → v2 (DIST).

### Approved UI contract (01-UI-SPEC.md, status: approved)
The UI-SPEC locks the visible copy and status visuals. Planners must follow it verbatim. The parts that drive code: New Session modal = Working Directory + read-only Shell line (helper text "Starts as a login shell. Change it in Settings → Paths."). Status = running (filled circle) / exited (hollow circle) / errored (rounded square), and unknown or legacy values (`idle`, `waiting`, empty) render as **running**. `isAttentionStatus` becomes errored-only. Hub status-pill group removed. Web-share warning fires on the first web-share of **any** session (drop `isShellCli`), and its "Cancel" button becomes **"Keep sharing off"**. Bell rate limit **resolved by UI-SPEC: at most one notification per session per 30 seconds**, stable per-session identifier. OSC 9 body `"{session name}: {OSC 9 text}"` (control chars stripped, truncated to 200 chars + "…"). BEL body `"{session name} needs your attention."`. Settings toggle label "Notify me when a session rings the bell". Welcome tab: drop Linux and Windows rows.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| STRP-01 | All agent-CLI code removed: CLI detection and PATH discovery, agent picker, agent badges, per-agent args and theme broadcast, Antigravity, agent working/waiting status detection | Removal Inventory §A, Session Data Model, Coupling Map rows C1–C6 |
| STRP-02 | File browser/editor (local and remote, GUI and web) removed, including its daemon API and capability | Removal Inventory §B, Coupling Map rows F1–F6 (remote-cap store and relay CORS helpers must be KEPT) |
| STRP-03 | Windows and Linux code paths, packaging, and CI jobs removed. Build, test, lint pass on macOS | Removal Inventory §C, CI section, Pitfalls 1–3 (Linux-compilable internals for e2e, BSD-grep traceability script) |
| STRP-04 | Tests for removed features deleted, kept-feature tests pass, security fuzz harness still covers relay and webserver | Test inventory per area, TESTING.md convention, Pitfall 9 (no relay/webserver Fuzz targets exist today) |
| TERM-01 | New local session starts the login shell in a chosen (or last-used) directory | Login shell section (`-l`, not argv0 `-`), single shell resolver |
| TERM-02 | Sessions keep running in the daemon when the GUI closes or restarts; reopen with scrollback | Already true (daemon holds PTYs + scrollback ring); keep `App.tsx` re-attach path; Pitfall 6 (stale daemon reuse in dev builds) |
| TERM-04 | Themes and xterm.js plugins keep working, controllable from Settings | No code in scope touches `TerminalPanel` plugin wiring; remove only `NotifyThemeChange` (OpenCode SIGUSR2). Regression = existing TerminalPanel/PluginsSection tests + manual check |
| CHAT-01 | Per-session chat works on desktop and web, `@session` inject bridge removed | Chat section; keep `SetChatAppendFn` (chat-send depends on it); retarget inject security tests |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

From `/Users/ken/dev/CLAUDE.md` (global), `./CLAUDE.md` and `./.claude/CLAUDE.md` (project):

- **Chesterton's Fence:** before removing anything, articulate why it exists. "Can't explain it? Don't touch it." The project CLAUDE.md repeats this for daemon, relay and capability code shared between kept and removed features. The Coupling Map below is written for this rule.
- **Silent fallbacks forbidden:** `or {}` style fallbacks that hide failures are not allowed. Let it crash. (Relevant: the webserver 404 sentinel, the shell resolver, and the bell-text decode.)
- **Premature abstraction:** need 3 real examples before abstracting. Do not build a generic "terminal event bus" for one bell hook.
- **RULE 0:** on unknown or catastrophic failure, stop and report. Do not keep retrying.
- **Never `kill node.exe`** / never kill node processes (Claude Code runs on Node).
- **Python:** n/a this phase. **Go:** `go fmt`, `golangci-lint`, context-aware functions. **JS/TS:** camelCase, PascalCase components, TypeScript types. **Node package manager:** pnpm.
- **Testing:** 80%+ coverage in critical components; integration tests for routes.
- **LSP preferred over grep** for navigation (gopls/pyright/TS installed).
- **UAT:** `/gsd-verify-work` browser UAT must use the dev-browser skill, not manual user checks.
- **Ponytail** (over-engineering guardrail) runs on executor/fixer/debugger agents. Run `/ponytail-review` before `/gsd-ship`, with the Chesterton gate on every suggested deletion.
- **GSD workflow enforcement:** edits only through GSD commands.
- **TESTING.md standing rule (`./CLAUDE.md`):** every phase that adds, renames or removes tests must update `TESTING.md`: Suite Manifest counts (Section 2), Manual checklist (Section 5), Traceability map (Section 4, `.go/.ts/.tsx/.sh` paths only), and run `bash tests/check-traceability-paths.sh` before committing. If `build.yml` matrix entries change, re-apply branch protection with the new check names (Section 3).

## Summary

This phase is codebase surgery, not ecosystem research. No new third-party package is needed. Everything the phase builds (the bell detector) can reuse a parser the hub already runs on every PTY byte. The baseline on this Mac is green: `go build ./...`, `go vet ./...`, `go test -race -short ./...` (13 packages), Vitest (148 files, 2412 tests), `tsc --noEmit`, `bash tests/build-script.test.sh` (38/38) and `wails build -platform darwin/arm64 -tags wailsassets` all pass. Playwright fails only because the pinned Playwright wants a browser build (`chromium_headless_shell-1217`) that is not installed. `golangci-lint` (default config, no repo config) reports 72 issues that already exist. It is not a CI gate.

Five surprises shape the plan. (1) Several "file browser" and "inject" names hide kept infrastructure. `RemoteCapStore`, `POST /api/remote-files/caps`, `GET /api/remote-files/caps/{id}/open-url`, `client_remote_files.go` and `wrapRelayWithRemoteFiles` carry **GUI remote session attach**. `relay.FilesCORS`/`FilesPreflight` carry the **chat history/export routes**. `hub.SetChatAppendFn` (commented as "inject machinery") carries **chat send**. (2) The Linux internals must stay **compilable on Linux**, because the kept Playwright e2e job builds `cmd/playwright-fixture` (which imports `internal/daemon`) on ubuntu. Deleting `exit_linux.go`/`reap_linux.go` without relaxing the `!linux && !windows` tags on the `_other.go` files would break e2e. (3) `tests/check-traceability-paths.sh` uses `grep -oP`, which BSD grep rejects, so on macOS the check silently passes with zero paths checked. Moving it to the macOS CI job requires a portable rewrite. (4) Shells today launch with `-i` (interactive, **not** login), and go-pty v0.2.3 discards `Args[0]`, so the classic `-zsh` argv0 trick is impossible. Use `-l`. (5) The hub already feeds every PTY byte to a `charmbracelet/x/vt` emulator whose parser fires a `Bell` callback only for a real C0 BEL (a BEL that terminates an OSC string dispatches the OSC instead) and accepts `RegisterOscHandler(9, …)`. That is the clean, already-correct hook for BEL / OSC 9.

**Primary recommendation:** Five sequential plans, each green on macOS: (1) platform strip + CI move + portable traceability script, (2) file browser/editor + files capability, (3) agent-CLI + session model + status model + login shell + CLI `new [dir]`, (4) chat `@session` inject bridge, (5) BEL/OSC 9 notifications + final TESTING.md reconciliation. Run the plans sequentially, not in parallel waves: plans 2–5 edit the same hot files (`app.go`, `App.tsx`, `internal/daemon/{api,engine,types}.go`, `internal/relay/{server,hub}.go`, `internal/webserver/server.go`).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Spawn login shell, resolve shell path | Daemon (`internal/daemon/engine.go`, `internal/pty`) | GUI shows the resolved path only (`GetShellPath`) | The daemon owns PTYs. The modal must show the **same** resolver output the daemon spawns |
| Session status (running/exited/errored) | Daemon (`ListSessions` derives it from `State` + `ExitCode`) | Frontend maps legacy/unknown → running | One source of truth. Peers and web clients also read `status` |
| BEL / OSC 9 detection | Relay hub (`internal/relay/hub.go` live emulator) | Daemon exposes counters via `SessionInfo` | Every PTY byte already passes through `recordFrame` → emulator. No extra subscriber, no replay |
| Fire native notification, window-hidden gate, rate limit | GUI process (`app.go` tray poller + `notification_darwin.go`) | — | Only the GUI knows window visibility and owns UNUserNotificationCenter |
| Files capability removal | Webserver + daemon capability issuance | Relay loopback mux | Routes and perms live server-side. The frontend just stops calling |
| Chat (keep), inject (remove) | Relay hub + both WS read pumps (relay, webserver) | Frontend ChatPanel | The inject gate was server-side. Removing it means the read pumps drop 0x35 frames |
| Remote session attach (keep) | Daemon (`RemoteCapStore`, WS reverse proxy on relay mux) | Frontend join-code modal | Lives in "remote_files"-named files. Keep it |
| CI / packaging | GitHub Actions | `build.sh`, `tests/*.sh` | — |

## Standard Stack

No libraries are added. The phase reuses what is already in `go.mod` / `package.json`.

### Core (reused, already pinned)
| Library | Version (go.mod / package.json) | Purpose in this phase | Why |
|---------|---------|---------|--------------|
| `github.com/charmbracelet/x/vt` | `v0.0.0-20260615092313-b57e5e6d29bb` | `Callbacks.Bell` + `RegisterOscHandler(9, …)` on the hub's existing live emulator | A correct VT parser already runs on every byte. A BEL inside an OSC string is consumed as the OSC terminator `[VERIFIED: ansi@v0.11.7/parser/transition_table.go:266-268]` |
| `github.com/aymanbagabas/go-pty` | `v0.2.3` | PTY spawn | Discards `Args[0]` on unix. Login shell must use the `-l` flag `[VERIFIED: go-pty@v0.2.3/cmd_unix.go:23-25]` |
| `github.com/wailsapp/wails/v2` | `v2.10.2` | GUI runtime, `WindowIsMinimised` | No `WindowIsVisible` in v2 runtime. Track hidden state yourself `[VERIFIED: wails pkg/runtime/window.go func list]` |
| UNUserNotificationCenter via `notification_darwin.go` + `tray_objc_darwin.m` | in-repo | Native notification with caller-supplied identifier | Reuse `sendNotificationFunc(identifier, title, body)`. A stable identifier replaces the prior notification |

### Removed dependencies (expected after `go mod tidy` / pnpm)
| Package | Ecosystem | Why it goes |
|---------|-----------|-------------|
| `github.com/gen2brain/beeep` | Go | Only imported by `notification_linux.go`, `notification_windows.go` `[VERIFIED: grep]` |
| `github.com/godbus/dbus/v5` | Go | Only `tray_linux.go` `[VERIFIED: grep]` |
| `github.com/tailscale/go-winio` | Go | Only `ipc_windows.go`, `socket_windows.go`, `socket_windows_test.go` `[VERIFIED: grep]` |
| `github.com/goreleaser/nfpm/v2` | Go | Only `tools.go` (Linux `.deb` in release.yml) `[VERIFIED: tools.go]` |
| `github.com/wailsapp/mimetype` | Go | Only `internal/files/mime.go` `[VERIFIED: grep]` |
| `@codemirror/*` (13 packages) + `codemirror` | npm | Only `components/Editor.tsx`, `lib/languageFor.ts` `[VERIFIED: grep]` |

**Keep** (look removable but are not): `github.com/kardianos/service` (launchd install for `agenthub daemon install`), `github.com/chromedp/*` (webserver browser e2e tests), `react-markdown`/`remark-gfm`/`rehype-sanitize` (HelpContent + ChatMessage), `@tanstack/react-virtual` + `react-textarea-autosize` (ChatPanel), `@xterm/addon-serialize` (HubBriefingModal).

**Installation:** none. After removals: `go mod tidy` and `cd frontend && pnpm remove codemirror @codemirror/commands @codemirror/lang-cpp @codemirror/lang-css @codemirror/lang-go @codemirror/lang-html @codemirror/lang-javascript @codemirror/lang-json @codemirror/lang-markdown @codemirror/lang-python @codemirror/lang-rust @codemirror/lang-yaml @codemirror/language @codemirror/language-data @codemirror/legacy-modes @codemirror/state @codemirror/view`. Then commit `go.mod`, `go.sum`, `frontend/package.json`, `frontend/pnpm-lock.yaml`. CI uses pnpm 9 with `--frozen-lockfile`. Local pnpm is 12.6.0. Confirm the regenerated lockfile still installs with `pnpm install --frozen-lockfile` (Pitfall 11).

## Package Legitimacy Audit

This phase installs **no** external packages. It only removes them, so the legitimacy gate does not apply.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| (none installed) | — | — | — | — | — | n/a |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram (BEL / OSC 9 notification, the only new behavior)

```
 PTY child (zsh / Claude Code)
        │ bytes
        ▼
 relay.Hub.Run() drain loop ──► recordFrame(frame, raw) [holds emuMu]
        │                          │
        │                          ├─► scrollback.Append(frame)   (replay for reconnects)
        │                          └─► liveEmu.Write(clean)       (charmbracelet/x/vt parser)
        │                                   │
        │                     ┌─────────────┴──────────────┐
        │              C0 BEL in ground state      OSC 9 dispatched (BEL- or ST-terminated)
        │              → Callbacks.Bell()           → OscHandler(data="9;<payload>")
        │                     │                           │  payload starts "4;" or == "4" → ignore (progress)
        │                     ▼                           ▼
        │               hub.bellSeq++  (atomic)    hub.bellSeq++, hub.bellText = payload
        ▼
 broadcast(frame) → WS subscribers (GUI tab, web guests)   [unchanged]

 daemon.ListSessions() ── reads hub.BellState() ──► SessionInfo{bellSeq, bellText}
        │  (unix socket, every 5 s)
        ▼
 GUI app.refreshTrayState() → maybeNotifyBell(sessions)
        │  toggle ON? ── no → just record seq
        │  first sight of session? → baseline only
        │  seq increased?
        │  window hidden? (setDockVisible(false) / minimised) ── no → record seq, no notify
        │  now − lastNotified[id] ≥ 30 s?
        ▼
 sendNotificationFunc("agenthub.session-bell."+id, "AgentHub", body)
        → notification_darwin.go → UNUserNotificationCenter (same identifier replaces)
```

### Recommended edit structure (no new directories)
```
internal/relay/hub.go          # + newLiveEmulator(), bell counters, BellState()
internal/daemon/types.go       # SessionInfo: −cli −homeDir −browseEnabled, +bellSeq +bellText; CreateRequest: −cli −args
internal/daemon/engine.go      # −status pkg, −sessionCLIs, −opencode, single shell resolver (login -l), derived status
app.go                         # −DetectCLIs/ListShells/NotifyThemeChange/DownloadFile/SetSessionBrowse; maybeNotifyWaiting → maybeNotifyBell
tray.go                        # setDockVisible also records windowVisible
frontend/src/wailsjs/go/main/App.{d.ts,js}, frontend/src/wailsjs/go/models.ts   # HAND-maintained bindings the app imports
```

### Pattern 1: Hook the hub's existing emulator (do not add a subscriber)
**What:** Build every live emulator through one helper that wires the bell hooks. Three construction sites exist in `hub.go`: eager build in `recordFrame` (line 488), rebuild in `resizeLiveEmulator` (line 529), empty build in `EnsureLiveEmulator` (line 945) `[VERIFIED: grep xvt.NewEmulator]`. Do **not** wire hooks on the fourth emulator, `internal/daemon/engine.go:793` (styled-tail preview). It replays scrollback and would re-count old bells.
**Why not `status.Watch`-style subscriber:** A subscriber replays `ScrollbackSnapshot()` on subscribe (old bells would fire), has a 256-slot channel that drops frames under load, and appears as an internal hub subscriber. `recordFrame` sees each byte exactly once.
**Example:**
```go
// Source: in-repo pattern + charmbracelet/x/vt callbacks.go:11-14, handlers.go:17,62-65, emulator.go:121-125
func (h *Hub) newLiveEmulator(cols, rows int) *xvt.Emulator {
	emu := xvt.NewEmulator(cols, rows)
	emu.SetScrollbackSize(liveEmulatorScrollbackLines)
	emu.SetCallbacks(xvt.Callbacks{Bell: func() { h.bellSeq.Add(1) }}) // C0 BEL only; OSC-terminating BEL never reaches here
	emu.RegisterOscHandler(9, func(data []byte) bool {               // data is "9;<payload>"
		p := bytes.TrimPrefix(data, []byte("9;"))
		if bytes.Equal(p, []byte("4")) || bytes.HasPrefix(p, []byte("4;")) {
			return false // OSC 9;4 progress: owned by xterm addon-progress in the frontend
		}
		h.setBellText(string(p)) // store raw; sanitize + truncate at display time in app.go
		h.bellSeq.Add(1)
		return true
	})
	return emu
}
```
Callbacks run inside `recordFrame` while `emuMu` is held, on the PTY drain goroutine. They must be non-blocking: atomics or a tiny dedicated mutex only, never `hub.mu` and never engine locks.

### Pattern 2: Daemon derives status, frontend tolerates legacy
`ListSessions`: `status = "running"` while `State=="running"`. When stopped: `"exited"` if the exit code is 0 (after the existing −1→0 normalization), otherwise `"errored"`. `GetSessionStatus(id)` uses the same function (it feeds the webserver session resolver and `GET /sessions/{id}/status`). Remove `statusMu`/`sessionStatuses`. `pollSessionStatus` then emits `session:status` = exited/errored automatically before `session:exit`, so tab dots update with no new event.

### Pattern 3: One shell resolver used by both spawn and display
`resolveShell()` = Settings `shellPath` (non-empty) → `$SHELL` (absolute, exists, executable) → `/bin/zsh`. `GetShellPath()` and `CreateSession` must call the **same** function, so the modal's read-only Shell line equals what spawns. Argv = `[]string{"-l"}`.

### Anti-Patterns to Avoid
- **Grep-and-delete by filename.** `remote_files.go`, `relay_remote_files.go`, `client_remote_files.go`, `FilesCORS` and `SetChatAppendFn` contain kept code (see Coupling Map).
- **Stripping browser-side OS checks.** `TerminalPanel.tsx:844` (Cmd-F vs Ctrl-F) and `lib/openLink.ts:32` (Cmd/Ctrl-click) serve **web-share guests on any OS**. Keep them. Only host-side platform branches (daemon `GOOS`, `platformHint`) are dead.
- **Renaming wire/config names in Phase 1.** Keep the `notifyOnWaiting` settings key, `/settings/notify-on-waiting` route, `/api/remote-files/...` paths, socket and port. Phase 3 does the rename. Changing a JSON key silently resets the user's saved toggle.
- **Editing only the generated Wails tree.** The app imports the hand-maintained `frontend/src/wailsjs/go/main/App.{d.ts,js}` and `frontend/src/wailsjs/go/models.ts`, not `frontend/src/wailsjs/wailsjs/go/*` `[VERIFIED: funnelBinding.contract.test.tsx:6 comment + App.tsx imports]`.

## Removal Inventory (deliverable 1)

Paths come from `git ls-files` this session. "Delete" means `git rm`. "Edit" means partial change.

### A. Agent-CLI code (STRP-01)

**Go: delete**
- `internal/status/detector.go`, `internal/status/detector_test.go` (whole package; importers: `app.go`, `internal/daemon/engine.go`)
- `internal/pty/detect.go`, `internal/pty/detect_test.go` (knownCLIs claude/codex/gemini/opencode/agy)
- `internal/daemon/notify_theme_unix.go`, `internal/daemon/notify_theme_windows.go` (OpenCode SIGUSR2)
- `internal/daemon/opencode_ansi_test.go`
- `internal/pty/shells.go` + `internal/pty/shells_test.go`: delete `DiscoverShells`/`DetectShell`/`knownShellSpecs`/`readEtcShells`. No shell picker remains. Keep or inline only what the single resolver needs.

**Go: edit**
- `internal/daemon/engine.go`: remove `opencodeTUIConfig` + `ensureOpenCodeTUIConfig` (writes `opencode-tui.json` on every start), `sessionCLIs`, `statusMu`/`sessionStatuses`, `isShellSession`, `knownShells` + the stale-CLI-path filter in `loadSettingsFromDisk`, `resolveShellSpawn` (pwsh/legacy-Windows branches), `DiscoverShells`, `NotifyThemeChange`. Shrink `resolveDefaultShellPath` (drop `windows`/`linux` cases). Change the `CreateSession` signature to drop `cli`, `args`, `onStatus`. Remove the `status.Watch` wiring at lines 472–485. **Keep** `cliPaths` + `UpdateCLIPath`/`GetCLIPaths`: `app.go:1415` and `app.go:1724` read `paths["tailscale"]` for the Tailscale binary override.
- `internal/daemon/types.go`: remove `CLI` from `SessionInfo` and `CreateRequest`, `Args`, `DetectedShell`, `ShellsResponse`. Update the `Status` comment.
- `internal/daemon/api.go`: remove `GET /shells`, `POST /theme/notify`. Update the `handleCreateSession` call. The `SetSessionResolver` closures (lines ~543 and ~1259) drop `s.CLI`.
- `internal/daemon/client.go`: `CreateSession(name, workDir, cols, rows)`, remove `ListShells`, `NotifyThemeChange`.
- `internal/daemon/path.go`: `AugmentServicePath`, drop agent dirs (`~/.local/bin` "Anthropic native installer", volta, nvm, cargo) and Linux dirs. Keep `/opt/homebrew/bin`, `/usr/local/bin` (Homebrew/Tailscale). See A3.
- `app.go`: remove `displayNameForCLI`, `maybeNotifyWaiting` + `lastWaitingStatus` (replaced in plan 5), `DetectCLIs`, `ListShells`, `NotifyThemeChange`, the `cli` key in the `session:exit` payload, `SessionInfo.CLI`, `RemoteSession.CLIType`, the `status` package import, and the comment "extended to 5min for long-running agents".
- `main.go`: `runCLI` drops `splitDashDash`/`extraArgs`. `new` → `cmdNew(client, args)` creates a shell in `[dir]`. Usage text updated.
- `cmd_cli.go`: delete `cmdNewShell`, rewrite `cmdNew`. `list` drops the AGENT column and `CLIType`. `cmdSettings` keeps printing cli-paths (tailscale only).
- `cmd_attach.go`: drop `AgentType` and the `cli` banner segment. `internal/statusbar/bar.go`: drop `Options.AgentType` (line 38, 195).
- `cmd_remote.go`, `internal/tailnet/sessions.go`, `internal/webserver/server.go`: drop `CLIType` / `cli_type` from `PeerSession`, `ShareableSessionMeta`, the anonymous decode struct, `sessionMetaItem`, `sessionListItem`, the resolver signature (see Coupling C4).
- `internal/pty/session.go`, `internal/pty/backend.go`: `Session.CLI` / `CreateRequest.CLI` hold the **binary path**. Keep them (internal). Renaming to `Path` is optional and not required.

**Frontend: delete**
- `src/lib/agentBadge.ts`, `src/lib/agentBadge.test.ts`, `src/lib/shellCli.ts`, `src/lib/shellCli.test.ts`

**Frontend: edit** (UI-SPEC governs copy and visuals)
- `src/App.tsx`: `DetectCLIs`, `ListShells`, `NotifyThemeChange` calls (lines 16–17, 27, 309, 499–520, 1459–1475). `detectedCLIs`/`detectedShells` state. `cli` on every `Tab` literal (98–103, 1172, 1198, 1223). `s.name || s.cli` → `s.name || 'shell'` (585, 759, 1488). `createTab(cli, workDir, args)` → `createTab(workDir)`. ExitState `cli`/`finalStatus`. `isShellCli` web-share gate. `tailscaleInstalled` uses `detectedCLIs.some(c => c.Name === 'tailscale')`, which is dead because knownCLIs never contained tailscale. Drop it.
- `src/components/NewSessionModal.tsx`: per UI-SPEC §1. Removes `ARGS_KEY`/`SHELL_ARGS_KEY` localStorage, agent list, Extra Arguments. `onConfirm(workDir)`.
- `src/components/TabBar.tsx`: `Tab.cli`, `agentDisplayName`, `tab__agent-badge`. Status dot classes per UI-SPEC. `type` union drops `'file-browser'` (see B).
- `src/components/ExitToast.tsx`: drop `exit.cli` / `finalStatus` segments.
- `src/components/SettingsTab.tsx`: `clis` prop, the agent rows of the Paths table, the "No CLIs detected…" paragraph. Notify toggle copy. Auto-close copy. Optional: drop `platformHint === 'linux'|'windows'` hint branches (757–759).
- `src/components/Hub/*`: `SessionCard.tsx` (agent chip, `data-agent`, waiting/idle icons, "Browse files" item), `HubFilterBar.tsx` (remove pill group), `HubPanel.tsx` (search on `s.cli`, waiting counts), `HubModal.tsx` (status map, `session.cli` badge), `SessionCardGrid.tsx` (props). `src/lib/hubStatus.ts` (`HubStatus` union, `isAttentionStatus` errored-only), `src/lib/hubGroupCounts.ts` (waiting counts).
- `src/lib/remoteAdapter.ts`, `src/lib/remoteSession.ts`: `cliType`.
- `src/wailsjs/go/main/App.d.ts`, `App.js`, `src/wailsjs/go/models.ts`: drop `cli`, `DetectedCLI`, `DetectCLIs`, `ListShells`, `DetectedShell`, `NotifyThemeChange`. New `CreateSession(name, workDir, cols, rows)`.
- `src/style.css`: `.tab__status--idle/--waiting`, `.tab__agent-badge*`, `.new-session-modal__agent-btn*`, `.hub-card[data-agent]` palettes, needs-input/attn-waiting usages. Add `.tab__status--exited` (UI-SPEC).
- `src/content/help/getting-started.md`: agent-picker sentences (minimal edit. Phase 3 rewrites Help).

### B. File browser/editor + files capability (STRP-02)

**Go: delete**
- `internal/files/` (all 11 files incl. `FuzzSandboxPath`, `FuzzSandboxWrite`)
- `internal/daemon/remote_files.go`, **partial**: delete `handleRemoteFiles{List,Stat,Read,Write,Upload,Delete,Rename,Mkdir}`, `proxyRemoteFiles`, `isUnresolvableMagicDNS`. **Keep** `handleRemoteSessionOpenURL`, `handleRegisterRemoteCap`, `newRemoteFilesHTTPClient`, `remoteFilesClient`, `redactCapTokenFromError` if still referenced. Optionally move the survivors into `remote_caps.go`.
- `internal/daemon/client.go`: `filesURL`, `ListFiles`, `StatFile`, `ReadFile`, `HeadFile`, `WriteFile`, `UploadFile`, `DeleteFile`, `RenameFile`, `MkdirFile`, `SetSessionBrowse`.
- Tests: `internal/webserver/files_routes_test.go`, `internal/relay/server_files_test.go`, `internal/daemon/remote_files_test.go`, `internal/daemon/relay_remote_files_test.go`, `internal/daemon/remote_files_test_helpers_test.go`. **First move** `newDaemonAPIWithUpstreamCert` and `depositCapOnSocket` into `remote_ws_proxy_test.go` (or a renamed helper file), because `remote_ws_proxy_test.go` uses them `[VERIFIED: grep]`.

**Go: edit**
- `internal/daemon/api.go`: `filesHandler` field + construction (lines 50–56, 159–169), the 9 `/api/files/*` routes, the 9 `/api/files/remote/{sessionID}/*` routes, `POST /sessions/{id}/browse` + `handleSetSessionBrowse`, `ws.SetFilesHandler(...)` (lines 581, 1290), the browse branch in `issueCapabilitiesForSession` (lines 1474–1479 → always `rPerms := "read"`, `wPerms := "read,write"`), `relay.NewServer(..., a.filesHandler)` (line 366).
- `internal/daemon/relay_remote_files.go`: remove the 9 proxy routes + 8 OPTIONS preflights. **Keep** `GET /api/relay/remote/{sessionID}/ws` and the `"/"` fallthrough. Optional rename to `relay_mux.go`.
- `internal/daemon/engine.go`: `sessionWorkDirs` (keep: Hub groups cards by `WorkDir`), `sessionBrowse`, `browseEnabledFor*`, `SetSessionBrowse`, `sessionCwdIsHome*`, `cwdEqualsHome`, `filesWriteDefault`, `FilesWrite` settings field.
- `internal/daemon/types.go`: `FileEntry`/`FileListResponse` aliases + the `files` import, `HomeDir`, `BrowseEnabled`, `SessionBrowseRequest`, `IssueCapabilitiesResponse.HomeDir`.
- `internal/relay/server.go`: `NewServer` drops the `filesHandler` param and the files routes. **Keep** `withCORS`, `isAllowedFilesOrigin`, `handleFilesPreflight`, `FilesCORS`, `FilesPreflight` (used by `internal/daemon/chat_routes.go:94-103`). Optionally rename to `LoopbackCORS`/`LoopbackPreflight` and narrow the advertised methods to GET/OPTIONS.
- `internal/webserver/server.go`: `filesHandler` field, `SetFilesHandler`, `filesDispatch` + 10 routes (lines 919–963). **Keep** the RO derivation `readonly := !capability.HasPerm(claims.Perms, "write")` (line ~1369). Tokens minted before this phase may carry `files.read`, and whole-token matching still makes them read-only.
- `internal/webserver/capability_mw.go`: delete `requireFilesRead`, `requireFilesWrite`, `originAllowedForWrite` (only reachable via `requireFilesWrite` `[VERIFIED: grep]`). **Keep** `requireCapability` and `isRWGated`/`SetRWGate` (WS write gate).
- `internal/capability/capability.go`: delete `PermFilesRead = "files.read"`, `PermFilesWrite = "files.write"` `[VERIFIED: capability.go:30,37]`. Keep `HasPerm`. Its tests use literal strings and can stay generic.
- `app.go`: `SessionInfo.HomeDir`/`BrowseEnabled`, `SetSessionBrowse`, `DownloadFile` (only `FileBrowser/DownloadButton.tsx` calls it). Keep `OpenFileDialog` (SettingsTab path picker) and `SaveTerminalSession`.
- `cmd/playwright-fixture/main.go`: remove `files` import, sandbox seeding, `SetFilesHandler`, the `files.read`/`files.write` caps, `WRITE_CAP`/`SESSION_CWD` output, remote-peer `/api/files/*` canned routes.

**Frontend: delete**
- `src/components/FileBrowserTab.tsx`, `src/components/FileBrowser/**` (incl. `modals/`, `__tests__/` = 11 test files), `src/components/Editor.tsx`, `src/components/RemoteBrowseDNSWarning.tsx`, `src/components/HomeDirWriteWarning.tsx`
- `src/lib/filesApi.ts`, `filesTypes.ts`, `useFilesCapability.ts`, `useFilesWrite.ts`, `humanSize.ts`, `languageFor.ts`
- Tests: `components/__tests__/{Editor.save,Editor,EditorComponents,HomeDirWriteWarning,App.fileBrowserMode,App.remoteFileBrowser}.test.tsx`, `lib/__tests__/{filesApi,humanSize}.test.ts`, `lib/__tests__/{useFilesCapability,useFilesWrite}.test.tsx`
- e2e: `frontend/e2e/files-browser.spec.ts`, `frontend/e2e/files-write.spec.ts`

**Frontend: edit**
- `src/App.tsx`: `FileBrowserTab`/`fileBrowserTabId`/`EnableWebSharingTakeover`/`RemoteBrowseDNSWarning` imports. `handleOpenFileBrowser`, `handleBrowseFilesRemote`, the default (file-browse) branch of `handleModalExchange` (keep the `'open-session'` and `'hub-modal'` intents), the web-share bootstrap that auto-opens a file-browser tab (~1230–1266), file-browser tab rendering (~1744–1812), `onBrowseFiles` props.
- `src/components/TabBar.tsx` ("Browse files" item, 399–416), `Hub/SessionCard.tsx` (456–458), `Hub/HubPanel.tsx`, `Hub/SessionCardGrid.tsx`, `Hub/SessionShareModal.tsx` (browse toggle, `SetSessionBrowse`, `HomeDirWriteWarning`), `SessionShare/TailnetTab.tsx` (link descriptions per UI-SPEC).
- `src/style.css`: `.file-browser*` (139 rule heads), `--home-write-warning` modifier.
- `src/content/help/faq.md` ("How do I enable remote file browsing?"), `getting-started.md` ("Using the File Browser").
- e2e: `web-share-scope.spec.ts` (drop the 4 files tests), `web-csp.spec.ts` (drop the "editor + write-cap" test), `global-setup.ts`/`fixture-env.ts` (`writeCap`, `sessionCwd`).

### C. Windows / Linux (STRP-03)

**Go: delete**
- Root: `tray_linux.go`, `tray_linux_test.go`, `tray_windows.go`, `tray_windows_test.go`, `notification_linux.go`, `notification_windows.go`
- `internal/attach/attach_windows.go`
- `internal/daemon/ipc_windows.go`, `socket_windows.go`, `socket_windows_test.go`, `path_windows.go`, `path_windows_test.go`, `process_windows.go`, `notify_theme_windows.go` (also in A)
- `internal/pty/cleanup_windows.go`, `exit_linux.go`, `exit_linux_test.go`, `exit_windows.go`, `job_windows.go`, `reap_linux.go`, `reap_windows.go`, `win32input.go`, `win32input_parse.go`, `win32input_test.go`, `win32input_other.go`, `kill_windows_test.go`
- `internal/files/concurrent_read_{unix,windows}_test.go` (go with B)

**Go: edit**
- `internal/pty/reap_other.go`, `exit_other.go`: change `//go:build !linux && !windows` → no tag (or `!windows`), so Linux still compiles for the e2e fixture (Pitfall 1). `job_other.go` (no-op `assignJobObject`) and `Session.job` field: delete both with the Windows job object (golangci already flags `job` unused). `native.go`: remove the `runtime.GOOS == "windows"` resize branch, the job-object and Linux exit-detector comments, and the `startExitDetector` call if `exit_other.go` becomes a pure no-op (then delete that too).
- `internal/daemon/socket.go`: drop Windows branches in `DefaultSocketPath`, `ValidateSocketPath`, `isWindowsNamedPipe`/`cleanupStaleWindowsPipe`. Delete `socket_nonwindows.go` (panic stub). `ipc_nonwindows.go`, `process_unix.go`, `attach_unix.go`: the `!windows` tag can stay or go.
- `internal/daemon/path_other.go` + `platformExtraBins`: inline/delete.
- `internal/daemon/engine.go:1220-1223` (GOOS switch), `internal/webserver/tailscale_paths.go` (`linux`/`windows` cases).
- `main.go`: `WEBKIT_DISABLE_DMABUF_RENDERER` block (67–74), `GOOS != "linux"` menu guards (115, 125, 130) → unconditional.
- `tools.go`: drop the `nfpm` import (keep `wails`).
- Test skip guards `if runtime.GOOS == "windows" { t.Skip(...) }` (~100 sites in tests): harmless dead branches. Optional mechanical sweep (see Open Question 4).

**Packaging / scripts / docs / artifacts: delete**
- `packaging/winget/` (7 files). **Keep** `packaging/homebrew/`.
- `scripts/install.sh` (Linux installer) + `tests/install-sh.test.sh`
- `build/linux/` (6 PNGs), `build/windows/` (`icon.ico`, `info.json`, `wails.exe.manifest`)
- `bin/linux-amd64/agenthub` (a committed ELF binary). The other `bin/*` Mach-O binaries and the root `playwright-fixture` binary are also committed build artifacts (hygiene, optional, not platform)
- `docs/superpowers/specs/2026-05-17-windows-daemon-named-pipe-ipc-design.md`, `docs/install-links-fix.md`

**Edit**
- `build.sh`: remove `build_windows`, `build_linux`, `--all`, the linux/windows platform values. `tests/build-script.test.sh`: remove the assertions for `build_windows`, `build_linux`, `docker run`, `docker info` (lines 219–223, 245–246, 276–277). Keep invalid-platform tests.
- `README.md`: Linux/Windows install sections and cross-platform claims (minimal. Phase 3 rewrites the README).
- `frontend/src/components/WelcomeTab.tsx` (Linux/Windows rows) + `WelcomeTab.install.test.tsx`
- `frontend/src/components/LocalNetworkBanner.tsx:88-90` (host-side `platformHint` linux/windows hints, optional)

**CI**
- `.github/workflows/build.yml`: delete the 3 non-mac matrix rows, the Linux deps step, the Windows branches, the Linux/Windows upload steps. **Move** `bash tests/build-script.test.sh`, `pnpm install && pnpm test`, and the traceability check to the macOS job (they are gated `runner.os == 'Linux'` today). Delete the `install-sh` step. Rewrite `tests/check-traceability-paths.sh` portably first (Pitfall 2). Branch-protection check names change. Follow TESTING.md §3.
- `.github/workflows/release.yml`: `validate` runs on `ubuntu-24.04` with apt GTK/WebKit deps. Move it to `macos-latest` (root package no longer compiles on Linux once `tray_linux.go`/`notification_linux.go` are gone). Delete `build-windows`, `build-linux` (incl. nfpm). `publish.needs` → `[build-macos, sign-macos]`. Update checksums/attestation globs.
- `.github/workflows/distribute.yml`: delete `submit-winget`. Keep `update-homebrew-tap`.
- `.github/workflows/e2e.yml`: keep it on `ubuntu-24.04`. It only builds the fixture (Pitfall 1).
- `.github/dependabot.yml`: the `go-webview2`/wails ignore entries guarded the Windows build. Optional cleanup. Leaving them is harmless.

## Coupling Map: where removed and kept features touch (deliverable 2)

Each row: the entangled piece, why it exists (Chesterton), and the safe cut line.

| # | Entangled piece | Why it exists | Safe cut line |
|---|-----------------|---------------|---------------|
| F1 | `RemoteCapStore` (`internal/daemon/remote_caps.go`) + `POST /api/remote-files/caps` + `GET /api/remote-files/caps/{sessionID}/open-url` | Built for remote file browse (Phase 122). Since Phase 146/168 it is also how the GUI **opens a remote session**: `handleModalExchange` `'open-session'` → `ExchangeJoinCodeAtURL` → `RegisterRemoteCap` → `OpenRemoteSessionURL` → in-app web-session tab. The held-cap reuse path in `handleOpenRemoteSession` also depends on it | **Keep all three** and `App.{ExchangeJoinCodeAtURL,RegisterRemoteCap,OpenRemoteSessionURL}`, `client_remote_files.go`. Delete only the 9 `/api/files/remote/...` proxy handlers |
| F2 | `wrapRelayWithRemoteFiles` (`relay_remote_files.go`) | Parent mux on the relay loopback port. It exists because `*API` methods can't live in `relay` (import cycle). It also mounts `GET /api/relay/remote/{sessionID}/ws` (`relay_remote_files.go:74`), the cap-gated WS reverse proxy that `HubInteractiveModal`/`HubBriefingModal` use for **remote** sessions | Keep the wrapper, the WS route and the `"/"` fallthrough. Remove the file routes + OPTIONS |
| F3 | `relay.FilesCORS` / `FilesPreflight` / `withCORS` / `isAllowedFilesOrigin` | Relay CORS for the cross-origin Wails webview → `127.0.0.1:<relayPort>`. Used by `internal/daemon/chat_routes.go:94-103` for **chat history/export** | Keep (rename optional). Remove only the files route registrations in `relay.NewServer` |
| F4 | `issueCapabilitiesForSession` perm matrix | Browse ON added `files.read`/`files.write` to minted tokens | Always mint `"read"` / `"read,write"`. Keep the RO derivation by whole-token `HasPerm(perms,"write")` in both read pumps. Old `read,files.read` tokens remain read-only, and their bits are ignored |
| F5 | `sessionWorkDirs` + `GetSessionWorkDir` | Added for the file sandbox (FS-02). The Hub also groups cards by `SessionInfo.WorkDir` (GRID-02) | Keep `sessionWorkDirs`/`WorkDir`. Remove the files resolver closure only |
| F6 | `HomeDir` field + `sessionCwdIsHome` | Only drives `HomeDirWriteWarning` (file-write warning in the Share modal) | Delete end-to-end (`types.go`, `app.go`, `IssueCapabilitiesResponse.HomeDir`, frontend). No terminal feature reads it `[VERIFIED: grep]` |
| F7 | `RemoteBrowseDNSWarning` / `TailscaleHealth.acceptDns` | Warns that accept-dns=false breaks MagicDNS for remote **browse**. The same condition also breaks remote **attach** to MagicDNS URLs | Delete the banner (locked decision). Keep the backend `acceptDns` probe in `TailscaleHealth` so Phase 2/4 can re-surface it. Record as risk A6 |
| C1 | `hub.SetChatAppendFn` (engine.go:463–470, comment says "inject persist callback") | `HandleChatSend` reads `h.chatAppendFn` and errors with "chatAppendFn not wired" when nil `[VERIFIED: hub.go:861-890]`. **Chat send needs it** | Keep the wiring. Fix the comment. Delete only `HandleInject`, `ErrReadOnly`/`ErrInjectNotRecorded`/`MaxInjectTextBytes`/`ErrInjectTooLarge`, `InjectErrorReason`, `InjectPayload`, `MakeInjectErrorFrame`, `SanitizePTYText` (only `HandleInject` uses it) |
| C2 | `MsgSessionInject = 0x35`, `MsgInjectError = 0x36` (`protocol.go:85-86`) | The @session bridge wire frames | Delete the `case MsgSessionInject` branches in `internal/relay/server.go` (~407–425) and `internal/webserver/server.go` (~1590–1612). With no case, unknown frames are silently ignored (there is no `default:` arm). Keep the byte values documented as **retired/reserved** in a comment so they are never reused. `ChatMessage.SessionInject` can go: old JSONL with `"sessionInject":true` still loads, because `encoding/json` ignores unknown fields and `chat.go` uses plain `json.Unmarshal` |
| C3 | `isShellCli(session.cli)` gate for the web-share warning | The warning targeted raw shells, not agents | Every session is a shell. The gate becomes "warning enabled && not yet warned" (UI-SPEC §3). The daemon-persisted `shellWebShareWarned` flag + Settings re-arm stay |
| C4 | Webserver session resolver `func(id) (name, cliType, status, hostname)` | Feeds `/api/sessions`, `/api/sessions/meta`, `/api/sessions/{id}/info` | `handleSessionInfo` uses `name == id && cliType == ""` as its **404 sentinel** `[VERIFIED: webserver/server.go:1284-1285]`. Change the signature to return `ok bool` (or keep a non-empty field). Do not just delete `cliType`, or every unknown session 200s. Add a regression test |
| C5 | `cliPaths` map + `/settings/cli-paths` | Agent binary overrides **and** the Tailscale binary override (`app.go:1415, 1724` read `paths["tailscale"]`) | Keep the map and routes. Drop agent rows in the UI. Old `"claude": …` keys load harmlessly |
| C6 | `AugmentServicePath` | Launchd/Finder launches don't source shell init. Agent CLIs had to be on the daemon PATH. Also used for `brew`/`tailscale` lookups. Spawned shells inherit the daemon env | Trim agent and Linux entries. Keep Homebrew dirs. Login shells rebuild PATH from `/etc/zprofile` + user dotfiles (A3) |
| N1 | Notification plumbing (`sendNotificationFunc`, `requestNotificationAuthFunc`, `notifyOnWaiting` atomic, permission-denied/granted events) | Built for awaiting-input (Phase 167). Bundle-id guard prevents crashes in unbundled/dev builds | Keep everything except `maybeNotifyWaiting`/`displayNameForCLI`/`lastWaitingStatus`. Re-point to bell events |
| N2 | `QuitGUIOnly` notification ("still running in the background") | Kept feature | Unchanged. It counts active sessions with `s.Status != "stopped"`, which still works with running/exited/errored |
| H1 | Hub (kept until Phase 2) | — | Minimal edits per UI-SPEC: remove agent chip/badge, pill group, waiting/idle states, "Browse files". `HubBriefingModal` stays, reachable for errored sessions only once `isAttentionStatus` is errored-only |

## Session Data Model: removing `cli` (deliverable 3)

**Where the field lives (all verified this session):**

| Layer | Location | Current shape |
|-------|----------|---------------|
| Daemon wire | `internal/daemon/types.go:20-41` | `ID string \`json:"id"\``, `CLI string \`json:"cli"\``, `Status string \`json:"status"\` // heuristic status: running/idle/waiting/errored`, `HomeDir bool \`json:"homeDir"\``, `BrowseEnabled bool \`json:"browseEnabled"\``, … |
| Create request | `internal/daemon/types.go:44-51` | `CLI string \`json:"cli"\``, `Name`, `WorkDir`, `Args []string \`json:"args,omitempty"\``, `Cols`, `Rows` |
| Engine state | `internal/daemon/engine.go:40` | `sessionCLIs map[string]string // sessionID -> raw CLI name (e.g. "opencode")` |
| PTY layer | `internal/pty/session.go:31`, `backend.go:13` | `CLI string` = binary path. Internal, keep |
| Wails binding (Go) | `app.go:31-64`, `68-74` | `CLI string \`json:"cli"\``, `RemoteSession.CLIType string \`json:"cliType"\`` |
| Wails binding (TS, hand-maintained) | `frontend/src/wailsjs/go/main/App.d.ts:6-27` | `cli: string`, `homeDir`, `browseEnabled` |
| Frontend tab | `frontend/src/components/TabBar.tsx:12-13` | `cli: string`, `type?: 'terminal' \| 'welcome' \| 'settings' \| 'file-browser' \| 'hub' \| 'help' \| 'web-session'` |
| Hub status type | `frontend/src/lib/hubStatus.ts:11` | `'running' \| 'idle' \| 'waiting' \| 'errored' \| 'stopped-ok' \| 'stopped-err'` |
| Peer/web wire | `internal/tailnet/sessions.go:22,122,292`, `internal/webserver/server.go:58,72` | `CLIType string \`json:"cli_type"\`` |
| CLI output | `cmd_cli.go:193-197` | AGENT column |
| Persisted settings | `internal/daemon/engine.go:113-125` (`settings.json`) | `cliPaths`, `notifyOnWaiting`, `filesWrite`, … |

**There is no on-disk session store.** Sessions live only in daemon memory (`pty.SessionRegistry` + engine maps). The daemon writes exactly: `settings.json` (`engine.go:267`), `aliases.json` (`alias_store.go:123`), `chats/<id>.jsonl` (`chat.go:289`), the capability signing key (`keystore.go:61`), `opencode-tui.json` (`engine.go:89`), and the GUI's CT-disclosure marker (`app.go:1177`) `[VERIFIED: grep os.WriteFile/OpenFile]`. So "old persisted sessions still load" means these cases:

1. **settings.json from AgentHub** with `"cliPaths":{"claude":"…","tailscale":"…"}`, `"filesWrite":true`, `"notifyOnWaiting":true`. Go `json.Unmarshal` ignores keys that leave the struct. `cliPaths` stays a `map[string]string`. Keep `notifyOnWaiting` as the key so the user's toggle survives. The next save drops `filesWrite`. No `DisallowUnknownFields` is used on settings load `[VERIFIED: grep]`. Add a test: load a legacy fixture (extend `tests/fixtures/settings_v3.2.json` or a new `settings_agenthub_v4.2.json`) → no error, tailscale override + shellPath + notify toggle preserved.
2. **Chat JSONL lines with `"sessionInject":true`** keep loading after the field is removed. Add a test.
3. **A still-running older daemon** (dev builds skip the version check, see Pitfall 6) returns `cli`, `homeDir`, `browseEnabled` and statuses `idle`/`waiting`. New Go structs ignore the extra fields. The frontend maps unknown statuses to running (UI-SPEC).
4. **Tailnet peers still on AgentHub** send `cli_type` and `waiting`/`idle` statuses in `/api/sessions/meta`. Decoding ignores the field. Frontend legacy-status mapping covers the status.
5. **Old clients sending `{"cli":"claude","args":[…]}` to `POST /sessions`**: `handleCreateSession` uses a plain `json.NewDecoder(...).Decode` (no `DisallowUnknownFields`), so the extra fields are ignored and a shell spawns. Acceptable.
6. **localStorage**: `agenthub:args:<cli>` and `agenthub:args:shell:<name>` become orphaned. Harmless. `agenthub:lastWorkDir` stays in use.

**Removal steps (one plan, compile-driven):** delete the field from `daemon.SessionInfo` and `CreateRequest` first. `go build ./...` + `go vet ./...` then list every consumer (engine, api resolver closures, client, app, cmd_cli, cmd_attach, playwright fixture, tests). Then do the TS side: `App.d.ts` → `tsc --noEmit` lists every consumer.

## Notification Rework (deliverable 4)

**Current path `[VERIFIED: read]`:** `status.Watch` (daemon goroutine, agent sessions only. Shells skip it at `engine.go:477`) → `sessionStatuses` → `ListSessions.Status` → GUI tray poller every 5 s (`app.go:1556-1590`) → `maybeNotifyWaiting` edge-detects non-waiting→waiting (cold-start baseline, prune) → `sendNotificationFunc("agenthub.session-waiting."+s.ID, "AgentHub", body)` (`app.go:198`). It never checked window visibility.

**Cleanest hook:** the hub's live emulator, through `newLiveEmulator` (Pattern 1). Reasons:
- `recordFrame` feeds the emulator every PTY byte exactly once, from the first frame (eager build, M-51 fix). There is no replay of old bells. `EnsureLiveEmulator` builds empty and no longer bootstraps from scrollback (`hub.go:925-950`).
- The `charmbracelet/x/ansi` parser treats `0x07` in `OscStringState` as `DispatchAction` → `GroundState` `[VERIFIED: transition_table.go:266-268]`. The `Bell` callback is wired only to the C0 `BEL` control handler (`vt/handlers.go:230-235`). A BEL-terminated OSC therefore never fires `Bell`. This meets the "BEL outside OSC strings" requirement with no hand-written state machine.
- OSC handlers receive `data` including the numeric prefix (the title handler splits `"0;title"`, `vt/osc.go:21-30`). For OSC 9: strip the `"9;"` prefix and ignore `"4"` / `"4;…"` (progress). Use `TrimPrefix`, not `Split`, because notification text can contain `;`.
- `liveEmuQueryStripPattern` strips only DA/DECRQM/OSC 10-12 queries before the emulator write, never BEL or OSC 9 (`hub.go:150-160`).

**Data path:** `Hub.BellState() (seq uint64, text string)` → `engine.ListSessions` (already calls `e.manager.Get(id)` for `RemoteViewerCount`) → `SessionInfo{BellSeq uint64 \`json:"bellSeq"\`; BellText string \`json:"bellText,omitempty"\`}` → mirror in `app.go` `SessionInfo`. The web/peer structs (`sessionListItem`, `sessionMetaItem`) do **not** need the fields.

**GUI notifier (`maybeNotifyBell`, replaces `maybeNotifyWaiting` in `refreshTrayState`):**
- State, single goroutine: `lastBellSeq map[string]uint64`, `lastBellNotified map[string]time.Time`, injectable `nowFunc` for tests (mirror the `sendNotificationFunc` injection pattern).
- Toggle off → update `lastBellSeq` only (so enabling later does not flush history).
- Unknown session → baseline (record seq, no notify). Same cold-start rule as today.
- `seq > last` and window hidden and `now - lastNotified ≥ 30s` → notify. Record seq in every branch, so bells that rang while the window was visible never fire later.
- Window hidden = `!a.windowVisible.Load()` (set inside `setDockVisible` in `tray.go:92`, which every show/hide path calls: `app.go:219-220, 319-320, 333-334`, `tray.go:54-84`. Initial value false matches `StartHidden: true`). OR `runtime.WindowIsMinimised(a.ctx)` (A7).
- Identifier `"agenthub.session-bell."+id` (stable per session, so Notification Center replaces). Title `"AgentHub"`. Body per UI-SPEC. Sanitize OSC text: strip C0/C1 and NUL (C strings truncate at NUL), truncate to 200 runes + "…". `relay.SanitizeChatContent` already strips bidi/C0/C1. Reuse it rather than writing a new stripper.
- Latency ≤ 5 s (tray poll interval). Acceptable for "Claude needs input". Do not add a new event channel.
- Settings toggle: keep `GetNotifyOnWaiting`/`SetNotifyOnWaiting` + key `notifyOnWaiting` (Go identifiers may be renamed. The JSON tag and route stay). Relabel the copy per UI-SPEC. Keep `requestNotificationAuthFunc` on enable.

**Claude Code specifics `[ASSUMED]`:** Claude Code's notification channel setting can emit a terminal bell or an iTerm2-style OSC 9 message. Both paths are covered. ConEmu also defines other numeric OSC 9 subcommands (9;1 … 9;12). The locked decision excludes only 9;4, so other numeric payloads would notify with their raw text. See Open Question 2.

## Build / Test / Lint Commands (deliverable 5)

All run on this Mac (darwin/arm64, Go 1.27.4, Node 24.22.1, pnpm 12.6.0, Wails CLI from `~/go/bin`) on 2026-11-14:

| Purpose | Command (from repo root unless noted) | Baseline result |
|---------|---------------------------------------|-----------------|
| Go build | `go build ./...` | PASS |
| Go vet (Go lint gate) | `go vet ./...` | PASS |
| Go tests (CI form) | `go test -race -short ./...` | PASS, 13 pkgs (~1–2 min) |
| Go tests (quick) | `go test -short ./internal/<pkg>/...` | PASS |
| Linux compile guard for e2e (new, recommended) | `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags playwrightfixture -o /dev/null ./cmd/playwright-fixture && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./internal/...` | PASS |
| Vitest | `cd frontend && pnpm test` | PASS, 148 files / 2412 tests (~55 s) |
| TS typecheck (frontend lint gate) | `cd frontend && pnpm exec tsc --noEmit` | PASS |
| Frontend build | `cd frontend && pnpm build` (`tsc && vite build`) | (part of wails build) PASS |
| App build | `wails build -platform darwin/arm64 -tags wailsassets` (CI uses `darwin/universal`) | PASS (~36 s) |
| build.sh tests | `bash tests/build-script.test.sh` | PASS 38/38 |
| install.sh gate | `bash tests/install-sh.test.sh` | (deleted this phase) |
| Traceability | `bash tests/check-traceability-paths.sh` | Prints "OK" but **checks nothing on macOS** (BSD grep has no `-P`, Pitfall 2) |
| Action SHA-pin gate | `bash scripts/grep-gate.sh` | PASS (not wired to CI. Use after workflow edits) |
| Playwright | `cd frontend && pnpm exec playwright install chromium && pnpm exec playwright test --project=chromium` | 19 fail / 11 pass. **All failures = missing `chromium_headless_shell-1217`** (installed: 1208/1234/1255). Run `playwright install` once |
| Fuzz | `go test -run='^$' -fuzz=FuzzVerify -fuzztime=10s ./internal/capability` | Target exists. `internal/files` fuzz targets go away (Pitfall 9) |
| golangci-lint | `golangci-lint run ./...` | 72 existing issues (50 errcheck, 20 staticcheck, 2 unused). No repo config, not in CI. Use only as a "no new issues" delta check if at all |

Suggested per-plan verify block: `go build ./... && go vet ./... && go test -race -short ./... && (cd frontend && pnpm exec tsc --noEmit && pnpm test) && bash tests/build-script.test.sh && bash tests/check-traceability-paths.sh && <linux compile guard>`. Add `wails build -platform darwin/arm64 -tags wailsassets` at phase end, then **revert regenerated runtime stubs** (Pitfall 5).

## Suggested Removal Order (deliverable 6)

Sequential. Each plan ends with the full verify block and a TESTING.md update.

1. **Plan 01: Platform strip + CI on macOS.** Rewrite `tests/check-traceability-paths.sh` portably **first**. Then delete Windows/Linux sources/tests, relax `_other.go` tags, clean socket/path/main/tailscale_paths/native. Delete packaging/winget, install.sh (+test), build/linux, build/windows, bin/linux-amd64, Windows/Linux docs. Trim build.sh (+ its test). Rewrite build.yml/release.yml/distribute.yml, `tools.go`, `go mod tidy`, WelcomeTab rows, README install rows. TESTING.md: remove Categories F (Windows CI), N (Linux install), O (WinGet), the `install-sh.test.sh`/`path_windows_test.go` rows and counts. *Why first:* least coupled to features, shrinks the tree, and makes CI verify on macOS for every later plan.
2. **Plan 02: File browser/editor + files capability.** Go (files pkg, routes, perms, browse toggle, HomeDir, remote-files proxy, client file methods, fixture), frontend (components, libs, App.tsx branches, Share modal toggle, tab/card menu items, CSS, help), e2e specs, deps (`pnpm remove @codemirror…`, `go mod tidy` drops mimetype). Keep F1–F5. TESTING.md: Category E, traceability rows for `internal/files/*`, browse-matrix (SHARE-03 row in `api_test.go` notes). *Why second:* the largest deletion. It removes the `files` import from relay/webserver/daemon before the session-model edits touch the same files.
3. **Plan 03: Agent-CLI removal + session/status model + login shell + CLI `new [dir]`.** `internal/status`, `detect.go`, `shells.go` discovery, OpenCode theme, `cli` field everywhere (compile-driven), derived status, `resolveShell` + `-l`, NewSessionModal/TabBar/ExitToast/SettingsTab/Hub per UI-SPEC, web-share warning gate + "Keep sharing off", bindings, CSS, CLI. Remove `maybeNotifyWaiting` here (the feature is briefly absent between plans 03 and 05, but the build stays green). Alternatively keep it compiling against the new status until plan 05. TESTING.md: Category I (Live Agent Launch), detector/detect rows, `agentBadge`/`shellCli` rows.
4. **Plan 04: Chat `@session` inject bridge.** relay/webserver read-pump cases, hub inject machinery, `SanitizePTYText`, protocol constants (keep a "retired" comment), `ChatMessage.SessionInject`, chat export marker, frontend ChatPanel/MentionPopover/ChatMessage/relayClient, help `chat.md`, e2e `chat-parity.spec.ts` (drop ROCHAT-02 inject + SC-4 inject-indicator tests). **Retarget** `internal/relay/server_inject_test.go` and `internal/webserver/inject_test.go` into "0x35 frame from an RO **and** RW client → zero PTY writes, connection stays open, chat unaffected". Do not just delete them. Could merge into plan 03 if the planner wants four plans. Kept separate because it touches the security-sensitive read pumps.
5. **Plan 05: BEL / OSC 9 notifications + phase reconciliation.** Hub emulator hooks + `BellState`, `SessionInfo` fields, `maybeNotifyBell` + `windowVisible`, Settings copy, unit tests (hub: BEL, BEL-terminated OSC 0 = no bell, OSC 9 text, OSC 9;4 ignored, split-across-frames, resize rebuild keeps hooks. app: baseline, hidden gate, 30 s limit, identifier, sanitize/truncate). Optional relay/webserver fuzz targets (Pitfall 9). TESTING.md: Category U rewrite, new M-NN manual items (signed-build notification delivery, login-shell check), final Suite Manifest counts. Full verify + `wails build` + dev-browser UAT.

## Runtime State Inventory

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `~/Library/Application Support/agenthub/settings.json` (macOS `os.UserConfigDir()`): `cliPaths` agent keys, `filesWrite`, `notifyOnWaiting`. `chats/*.jsonl` lines with `sessionInject`. `aliases.json`, signing key unaffected | **Code edit only.** Loader ignores unknown keys. Keep `cliPaths` + `notifyOnWaiting` keys. No data migration. Add legacy-load tests |
| Live service config | Daemon process (in-memory sessions, `RemoteCapStore`, grants). Tailnet peers that run AgentHub and advertise `cli_type`/`waiting` | None in config. **Operational:** stop the old daemon before testing new builds (Pitfall 6). Peers are decode-tolerant |
| OS-registered state | `agenthub daemon install` launchd plist (kardianos/service) may exist on the dev Mac. Notification Center may hold old `agenthub.session-waiting.<id>` notifications. macOS notification permission is keyed to bundle ID `AgentHub` (unchanged this phase) | None (names unchanged until Phase 3). Old notifications age out. Note for UAT: a launchd-managed daemon restarts itself, so use `agenthub daemon stop` |
| Secrets/env vars | `$SHELL` read by the daemon (may be unset under launchd → `/bin/zsh` fallback). No secret names change | None. Optional: resolve the user shell via directory services if `$SHELL` is unset (A2) |
| Build artifacts | `~/Library/Application Support/agenthub/opencode-tui.json` (rewritten every start, stops being written). Committed binaries `bin/linux-amd64/agenthub` (delete) and `bin/*`, root `playwright-fixture` (hygiene). `frontend/src/wailsjs/wailsjs/{runtime,go}` tracked-but-ignored files rewritten by `wails build`. `frontend/dist`, `.playwright/`, `cmd/playwright-fixture/dist` (ignored) | Leave the orphan `opencode-tui.json` (harmless) or delete it once by hand. Revert or regenerate-and-commit the Wails tree deliberately (Pitfall 5) |

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Detect BEL outside OSC, parse OSC 9 | A byte state machine like `oscabsorb.go`, or regexes like `engine.go`'s ANSI strip | The hub's existing `charmbracelet/x/vt` emulator (`Callbacks.Bell`, `RegisterOscHandler(9, …)`) | Handles split frames, ST vs BEL terminators, CAN/SUB aborts, DCS/APC strings. Already fed every byte |
| Strip control/bidi chars from notification text | New sanitizer | `relay.SanitizeChatContent` | Existing, tested display-safety transform |
| Notification delivery | New cgo or osascript path | `sendNotificationFunc` → `notification_darwin.go` | Has the bundle-id guard (no crash in `wails dev`), auth flow, permission events |
| Window visibility | Polling Wails internals | Record in `setDockVisible` (single chokepoint) | Wails v2 has no `WindowIsVisible` |
| Login shell | argv0 `-zsh` via patched go-pty | `$SHELL -l` | go-pty drops `Args[0]` (`cmd_unix.go:23-25`) |
| Rate limiting | Token bucket lib | `map[id]time.Time` + 30 s compare in the single tray goroutine | One goroutine, one rule |

**Key insight:** the bell feature is about 40 lines because the hard parts (VT parsing, native delivery, single-goroutine polling) already exist. Most of this phase's risk is in deleting the right things, not in building.

## Common Pitfalls

### Pitfall 1: Breaking the ubuntu e2e job by deleting Linux files
**What goes wrong:** `e2e.yml` (kept on `ubuntu-24.04`) runs `global-setup.ts`, which does `go build -tags=playwrightfixture,wailsassets ./cmd/playwright-fixture`. The fixture imports `internal/daemon` → `internal/pty`. `ReapNaturalExit`/`startExitDetector` exist only in `reap_linux.go`/`exit_linux.go` (tag `linux`) and `reap_other.go`/`exit_other.go` (tag `!linux && !windows`). Delete the Linux files and Linux has no definition.
**How to avoid:** make the `_other.go` variants untagged (or `!windows`) in the same commit. Add the Linux compile guard command to every plan's verify.
**Warning signs:** `undefined: (*Session).ReapNaturalExit` only in CI e2e.

### Pitfall 2: Traceability check silently passes on macOS
**What goes wrong:** `tests/check-traceability-paths.sh:29` uses `grep -oP`. `/usr/bin/grep` on macOS prints `grep: invalid option -- P`, the process substitution yields no lines, and the script prints `OK` (verified this session). After moving the step to the macOS CI job, deleted test files would leave stale TESTING.md rows that nothing catches.
**How to avoid:** rewrite with POSIX tools, e.g. `grep -oE '\| [^|]+\.(go|ts|tsx|sh) \|' TESTING.md | sed -E 's/^\| //; s/ \|$//' | tr -d ' '` (yields 117 unique paths today). Add `set -o pipefail`, and fail if zero paths were extracted.

### Pitfall 3: The release `validate` job still on Linux
**What goes wrong:** `release.yml` `validate` installs GTK/WebKit on `ubuntu-24.04` and runs `go test ./...`. Once `tray_linux.go`/`notification_linux.go` are gone the root `main` package has no `initTray`/`sendNotification` on Linux, and the release fails at tag time (not on push).
**How to avoid:** move `validate` to `macos-latest` in plan 01. Consider a `workflow_dispatch` dry run.

### Pitfall 4: Deleting kept code that has "files"/"inject" in its name
See Coupling F1–F3, C1. **Warning signs:** "Open in tab" for remote sessions stops working, `HubInteractiveModal` remote terminal blank, chat history 404/CORS error in the Wails webview, chat send logs `chatAppendFn not wired`.

### Pitfall 5: `wails build` rewrites tracked files
**What goes wrong:** `frontend/src/wailsjs/wailsjs/runtime/runtime.{d.ts,js}` (tracked, though their directory is gitignored) gained +142 lines during this session's build. `frontend/src/wailsjs/wailsjs/go/models.ts` (tracked) will be rewritten once Go binding types change. The app does not import that tree.
**How to avoid:** after `wails build`, either `git checkout -- frontend/src/wailsjs/wailsjs/` or decide once to `git rm --cached` the tree (it is already in `.gitignore`). Do not let executors commit build noise. Edit the **hand-maintained** `frontend/src/wailsjs/go/main/App.{d.ts,js}` + `frontend/src/wailsjs/go/models.ts` for binding changes.

### Pitfall 6: Dev builds reuse a stale daemon
**What goes wrong:** `EnsureDaemon` compares versions only when `BuildVersion != "dev"` (`process.go:175`). A dev build connects to whatever daemon owns `daemon.sock`: an installed AgentHub daemon, or a previous-plan build. The new GUI then sends `CreateRequest` without `cli` to an old daemon, which runs `ResolveCLI("")` and the spawn fails. Or old daemon behavior masks new code.
**How to avoid:** before UAT, `agenthub daemon stop` (if launchd-installed) or quit via the tray "Quit" (QuitAll → `POST /shutdown`). Closing to tray (`QuitGUIOnly`) leaves the daemon running. Or build with `-ldflags "-X main.Version=v0.0.0-phase1"`.

### Pitfall 7: Webserver 404 sentinel depends on `cliType`
`handleSessionInfo` treats `name == id && cliType == ""` as not-found (`server.go:1284-1285`). Removing `cliType` without changing the resolver contract makes every unknown session return 200 with empty fields. Return an explicit `ok bool`. Keep or extend the existing not-found test.

### Pitfall 8: CSS source-gate tests
`style.hub.test.ts`, `style.redesign.test.ts`, `themeTokens.test.ts`, `style.contrast.test.ts` assert the presence/values of CSS rules (e.g. agent-badge palettes, status dots). Deleting CSS without updating these fails Vitest. Treat them as part of each UI edit.

### Pitfall 9: "Fuzz harness covers relay and webserver" is not true today
The only Go fuzz targets are `internal/capability/capability_fuzz_test.go:FuzzVerify` and `internal/files/sandbox_test.go:FuzzSandboxPath/FuzzSandboxWrite` `[VERIFIED: grep func Fuzz]`. The second file is deleted with `internal/files`. `.security-review/` is gitignored and absent from this checkout. The "harness" that remains in relay/webserver is the security-review-derived test suite (`internal/webserver/capability_test.go` + `capability_test_helpers.go`, origin tests, `security_regression_test.go` in both packages). To meet STRP-04 literally, add small `Fuzz*` targets: `relay.ParseFrame`, `(*relay.InputAbsorber).Filter`, `relay.SanitizeChatContent`, and a webserver `requireCapability` fuzz over the `cap` query via `httptest`. Optionally a fuzz over the new bell hook (feed random bytes, assert no panic and that OSC-terminating BELs never count). See Open Question 1.

### Pitfall 10: Hub test files assert removed UI
`HubPanel.test.tsx` (1127 lines), `SessionCard.test.tsx`, `SessionCardGrid.test.tsx`, `ChatPanel.test.tsx` (1253 lines), `SessionShareModal.test.tsx` (1074 lines) each contain many assertions on agent chips, waiting filters, browse toggles, @session. Budget for editing them, and do not delete whole files that also cover kept behavior (chat, share, funnel).

### Pitfall 11: pnpm major mismatch
CI pins pnpm 9 (`pnpm/action-setup … version: 9`) with `--frozen-lockfile` in e2e. Local is pnpm 12.6.0. A lockfile rewritten by `pnpm remove` on pnpm 12 may use a format pnpm 9 rejects `[ASSUMED]`. Either run `pnpm remove` via `npx pnpm@9` or bump CI's pnpm version in the same plan. Verify with `npx pnpm@9 install --frozen-lockfile`.

### Pitfall 12: Session exit after 5 minutes is not reported (existing behavior)
`pollSessionStatus` stops after `maxPollWindow = 300 * time.Second` (`app.go:406`, comment "extended to 5min for long-running agents"). A shell that exits after 5 minutes emits no `session:exit` to the GUI, so there is no countdown, toast, or exited dot from this path. This is not caused by this phase, but "keep shell-exit behavior" UAT may hit it with long-lived shells. Do not silently expand scope. Record it, or move exit detection into the 5 s tray poller if the planner chooses (Open Question 3).

### Pitfall 13: Auto-close semantics differ from CONTEXT wording
CONTEXT says "clean exit gives a 5s countdown, toast, and Keep Open". The current `session:exit` handler closes the tab **immediately** when auto-close is on (default) and shows the toast when it is off (`App.tsx:713-748`). The UI-SPEC copy ("Tab closes in {n}s") also exists in `ExitCountdownBanner`. Preserve current behavior exactly. Do not "fix" it toward the CONTEXT paraphrase without a decision.

## Code Examples

### Login-shell spawn in `CreateSession`
```go
// Source: in-repo engine.go CreateSession + go-pty cmd_unix.go:23-25 (Args[0] is discarded)
shell := e.resolveShell()               // settings shellPath → $SHELL → /bin/zsh (same func backs GetShellPath)
sess, err := e.backend.Create(ctx, pty.CreateRequest{
	CLI:     shell,                       // field name kept; holds the binary path
	Args:    []string{"-l"},              // login shell; interactive because stdin is a tty
	Cols:    cols, Rows: rows,
	WorkDir: workDir,                     // "" → $HOME guard kept (WR-01)
})
```

### Derived status
```go
// Source: in-repo ListSessions exit-code handling (-1 → 0 normalization)
func sessionStatus(stopped bool, exitCode int) string {
	switch {
	case !stopped:
		return "running"
	case exitCode == 0:
		return "exited"
	default:
		return "errored"
	}
}
```

### Retired protocol bytes (keep the numbers from being reused)
```go
// Source: internal/relay/protocol.go:85-86 (values 0x35, 0x36)
// 0x35 (MsgSessionInject) and 0x36 (MsgInjectError) are RETIRED (Portty Phase 1,
// CHAT-01). Do not reuse. Read pumps ignore unknown frame types, so an old
// client's 0x35 frame is dropped and never reaches the PTY.
```

### Notifier gate (shape only)
```go
// Source: mirrors app.go maybeNotifyWaiting (baseline/prune/injected sender)
if seq > prev && a.windowHidden() && now.Sub(a.lastBellNotified[s.ID]) >= 30*time.Second {
	a.lastBellNotified[s.ID] = now
	a.sendNotificationFunc("agenthub.session-bell."+s.ID, "AgentHub", bellBody(s.Name, s.BellText))
}
a.lastBellSeq[s.ID] = seq
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Per-agent regex status (claude/agy pattern sets) | Program-agnostic BEL / OSC 9 signals | This phase | Works for any CLI that rings the bell or sends OSC 9, including Claude Code |
| `$SHELL -i` (interactive, non-login) | `$SHELL -l` (login) | This phase | Sources `/etc/zprofile` (path_helper) + `~/.zprofile`, matching Terminal.app behavior `[ASSUMED]` |

**Deprecated/outdated in this repo:** the `.security-review/` references in comments and docs point to a gitignored snapshot that predates remediation (`docs/plugin-architecture-assessment.md:102`).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `zsh -l`, `bash -l`, `fish -l` start an interactive login shell when stdin is a tty. `tcsh` accepts `-l` only as the sole flag (so pass only `-l`, not `-il`) | Login shell, Code Examples | Shell starts non-interactive or rejects the flag. Mitigate with an integration test that spawns via the engine and checks `$0`/`[[ -o login ]]` for zsh |
| A2 | When the GUI is launched from Finder, the launchd user session provides `SHELL`. Under a launchd-installed daemon it may be absent → `/bin/zsh` fallback | Runtime State | A user whose login shell isn't zsh gets zsh until they set Settings → Paths. Optional `dscl . -read /Users/$USER UserShell` lookup |
| A3 | Removing agent dirs (`~/.local/bin`, volta, nvm, cargo) from `AugmentServicePath` doesn't regress PATH inside login shells, because the user's dotfiles rebuild PATH | Coupling C6 | A tool vanishes from PATH inside Portty shells. Low. Easy to restore entries |
| A4 | Claude Code emits BEL and/or iTerm2-style `OSC 9;<text>` for "needs input" | Notification | Notifications don't fire for Claude. The user can enable the terminal-bell channel in Claude Code settings |
| A5 | A pnpm 12 rewritten `pnpm-lock.yaml` may not be accepted by CI's pnpm 9 `--frozen-lockfile` | Pitfall 11 | CI install fails. Pin via `npx pnpm@9` or bump CI |
| A6 | Deleting `RemoteBrowseDNSWarning` loses the only accept-dns=false hint, which also affects remote attach | Coupling F7 | Silent MagicDNS failures on attach in Phase 2/4. Keep `acceptDns` in `TailscaleHealth` |
| A7 | Calling `runtime.WindowIsMinimised(ctx)` from the tray-poller goroutine is safe in Wails v2.10 | Notification | Possible main-thread assertion. Fall back to `windowVisible` only (hidden-to-tray), which matches the locked decision's "hidden" |
| A8 | "Window hidden" = hidden to tray (and optionally minimised), not "app not frontmost" or Cmd-H | Notification | The user expects notifications while Portty is merely in the background. Confirm in discuss/UAT |

## Open Questions (RESOLVED)

Resolved by the orchestrator on 2026-11-14 and given to the planner as locked decisions:
- **Q1:** Keep `capability.FuzzVerify` and add small relay and webserver fuzz targets (plan 08).
- **Q2:** Ignore OSC 9 payloads matching `^\d+(;|$)`, which covers 9;4 and the other ConEmu numeric subcommands.
- **Q3:** Fix it in this phase. The 5 s tray poller emits exit events for long-lived sessions and for sessions reopened after a relaunch (plan 09).
- **Q4:** Remove the trivial `GOOS == "windows"` test guards in plan 01.
- **Q5:** Drop `new shell [dir]` and keep only `new [dir]`.
- **Q6:** Untrack the committed binaries and gitignore them (plan 01).

1. **What does "security fuzz harness" mean in STRP-04?**
   - What we know: no `.security-review/` in the checkout. No `Fuzz*` targets in relay/webserver. The file-sandbox fuzzers are deleted with the feature.
   - Recommendation: add 3–4 small relay/webserver fuzz targets in plan 04 or 05 (Pitfall 9), and record them in TESTING.md. If the user meant a local out-of-repo harness, they should say where it lives.
2. **OSC 9 numeric subcommands other than 4** (ConEmu 9;1–9;12)?
   - Recommendation: follow the locked decision (exclude only 9;4). Optionally also ignore payloads matching `^\d+(;|$)` to avoid notifying on ConEmu control codes. This is a one-line discretion call.
3. **Long-lived shell exit (>5 min) not reported** (Pitfall 12).
   - Recommendation: out of scope unless UAT of "keep shell-exit behavior" requires it. Cheapest fix: emit `session:exit` from the existing 5 s tray poller when a session flips to stopped.
4. **Sweep the ~100 `runtime.GOOS == "windows"` test skip guards?**
   - Recommendation: optional mechanical cleanup in plan 01. They are dead code, but not "Windows sources". Ponytail would leave them unless they block lint.
5. **`agenthub new shell [dir]` legacy form.** With `new [dir]`, `new shell` would treat `shell` as a directory name.
   - Recommendation: no alias. The user is the only consumer and the CLI changes again in Phase 3 (`portty`). Update `cmd_cli_test.go`.
6. **Untrack committed build artifacts** (`bin/*`, root `playwright-fixture`, `node_modules/.vite/...results.json`, `.claire/worktrees/...`)?
   - Recommendation: delete `bin/linux-amd64/agenthub` (platform). The rest is hygiene the planner may fold into plan 01 or leave.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | all Go work | ✓ | go1.27.4 darwin/arm64 (go.mod `go 1.26.4`) | — |
| Wails CLI | app build | ✓ | `~/go/bin/wails` (go.mod pins wails v2.10.2) | — |
| Xcode/clang (cgo, UserNotifications) | darwin build | ✓ | (wails build succeeded) | — |
| Node | frontend | ✓ | v24.22.1 (CI: 22) | — |
| pnpm | frontend | ✓ | 12.6.0 (CI: 9) | `npx pnpm@9` for lockfile edits |
| Playwright browsers | e2e | partial | installed chromium 1208/1234/1255. Needs `chromium_headless_shell-1217` | `pnpm exec playwright install chromium` (network) |
| shellcheck | `install-sh.test.sh` | ✓ | /opt/homebrew/bin | script is deleted anyway |
| golangci-lint | optional lint | ✓ | 2.14.0 | `go vet` is the real gate |
| Go module cache / network | first build | ✓ | modules were downloaded this session | — |
| Tailscale (two Macs) | remote-attach UAT | not probed | — | Manual checklist item. Not needed for unit/integration verify |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** Playwright headless shell 1217 (install once).

## Security Domain

Config: `security_enforcement: true`, `security_asvs_level: 1`, `security_block_on: high`.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no change | Join codes / HMAC caps untouched |
| V3 Session Management | no change | — |
| V4 Access Control | **yes** | Capability model kept. Mint only `read` / `read,write`. Keep whole-token `HasPerm(perms,"write")` RO derivation in both read pumps. Keep `requireCapability`, `isGrantActive`, `isRWGated`. Removed routes must return 404/405 (add negative tests for `/api/files/list`, `POST /sessions/{id}/browse` on daemon, relay and webserver muxes) |
| V5 Input Validation | **yes** | OSC 9 text is attacker-influenced terminal output → `SanitizeChatContent`, strip NUL, truncate 200 runes before `C.CString`. 0x35 frames dropped server-side |
| V6 Cryptography | no change | `capability.Sign/Verify` (HMAC-SHA256) untouched |
| V12 Files and Resources | **yes (removal)** | Deleting `internal/files` removes the sandbox/path-traversal surface entirely. Verify no route still reaches `os.Open` on user paths |
| V13 API | **yes** | Daemon socket + relay loopback + webserver route tables shrink. Funnel and CSP middleware unchanged |
| V14 Configuration | yes | SHA-pinned actions (`scripts/grep-gate.sh`), no new secrets. Release signing jobs untouched |

### Known Threat Patterns for this change

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Notification spoofing: any program (or a RW web guest typing `printf '\e]9;…'`) puts phishing text in a native notification | Spoofing | Body always prefixed with session name. Toggle default OFF. Fires only while the window is hidden. 30 s per-session rate limit. Text sanitized and truncated. RO guests cannot write to the PTY |
| Notification flooding (`yes $'\a'`) | DoS | 30 s per-session limit + stable identifier (replace, not stack). Hub callback is O(1) atomic, so the drain loop is not slowed |
| Residual inject path after removal | Elevation/Tampering | Delete both `case MsgSessionInject` arms. Retargeted tests assert 0 PTY writes for RO and RW 0x35 frames on both relay and webserver paths |
| Stale `files.read` caps still in circulation | Elevation | Routes removed (404). Whole-token RO check unchanged |
| Silently vacuous CI gate (traceability) | Repudiation | Portable rewrite + "zero paths extracted = fail" |
| Over-deletion of origin/CORS guards | Tampering | Keep `relay/security_regression_test.go` and `webserver/security_regression_test.go` (source-grep guards on `server.go`) green. Keep `isAllowedFilesOrigin` allowlist for chat routes |

## Sources

### Primary (HIGH confidence, read this session)
- Repo source: `internal/daemon/{types,engine,api,client,remote_caps,remote_files,relay_remote_files,client_remote_files,process,socket,path}.go`, `internal/relay/{hub,server,protocol,sanitize,oscabsorb}.go`, `internal/webserver/{server,capability_mw,tailscale_paths}.go`, `internal/capability/capability.go`, `internal/status/detector.go`, `internal/pty/{detect,shells,native,session,*_other,*_linux}.go`, `app.go`, `main.go`, `cmd_cli.go`, `cmd_attach.go`, `notification_darwin.go`, `tray.go`, `cmd/playwright-fixture/main.go`
- Frontend: `App.tsx`, `NewSessionModal.tsx`, `TabBar.tsx`, `SettingsTab.tsx`, `Hub/*`, `lib/{hubStatus,shellCli,agentBadge,hubGroupCounts}.ts`, `wailsjs/go/main/App.{d.ts,js}`, `wailsjs/go/models.ts`
- CI/scripts: `.github/workflows/{build,release,distribute,e2e}.yml`, `build.sh`, `tests/*.sh`, `scripts/*.sh`, `TESTING.md`
- Module source: `charmbracelet/x/vt@v0.0.0-20260615092313-b57e5e6d29bb` (`callbacks.go`, `handlers.go`, `emulator.go`, `osc.go`), `charmbracelet/x/ansi@v0.11.7/parser/transition_table.go`, `aymanbagabas/go-pty@v0.2.3/{cmd_unix.go,pty_unix.go}`, `wailsapp/wails/v2@v2.10.2/pkg/runtime/window.go`
- Executed baselines: `go build/vet`, `go test -short` and `-race -short`, Linux cross-compile of fixture + internals, Vitest, `tsc --noEmit`, `wails build`, `build-script.test.sh`, `grep-gate.sh`, traceability script (+ `/usr/bin/grep -oP` failure), Playwright (chromium), golangci-lint

### Secondary (MEDIUM)
- none

### Tertiary (LOW, flagged `[ASSUMED]`)
- Shell `-l` semantics across zsh/bash/fish/tcsh, Claude Code's BEL/OSC 9 behavior, pnpm 9 vs 12 lockfile compatibility, launchd `SHELL` propagation

## Metadata

**Confidence breakdown:**
- Removal inventory / coupling: HIGH. Every path came from `git ls-files`/grep, and kept-vs-removed calls came from reading call sites
- Session data model / compat: HIGH. No on-disk session store, decoders verified non-strict
- Notification design: MEDIUM-HIGH. Parser behavior verified in module source. Native delivery needs a signed build (existing M-41 history)
- Build/test commands: HIGH. All executed this session

**Research date:** 2026-11-14
**Valid until:** 2026-12-14 (codebase-internal. Re-verify only if plans land out of order)
