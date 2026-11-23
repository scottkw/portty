# Phase 9: Upgrades Keep Sessions — Context

**Gathered:** 2026-11-19 (autonomous; open questions from 09-RESEARCH.md decided as reversible technical choices; the user approved the roadmap and the phase goal)
**Status:** Ready for planning

<domain>
Upgrading Portty never costs a session: the old daemon hands its sessions to the new daemon and viewers carry on by themselves; a failed takeover leaves the old daemon serving (UPGR-01..05). The user's top priority is that the unattended office Mac stays reachable and its sessions survive. Read 09-RESEARCH.md in full: it holds the probes, the protocol and 13 pitfalls.
</domain>

<decisions>

### Locked by research and roadmap
- **Mechanism:** a two-phase handoff over the existing unix socket with SCM_RIGHTS, as recommended in 09-RESEARCH.md. No self-exec.
  - EnsureDaemon in the new build starts `portty daemon --takeover`.
  - The old daemon freezes, then sends state and fds (PTY master and slave, API socket listener, relay listener, share listener, daemon.lock).
  - The new daemon loads without serving and answers READY. The old daemon sends COMMIT, the new one answers COMMITTED, and the old one exits WITHOUT unlinking the socket and WITHOUT sending close 1000.
  - Until COMMIT the old daemon keeps every fd and resumes on any failure (UPGR-03).
- **State that moves byte for byte:** ring with its end offset, History, modes, wraps, charstate, geometry and grid, share records and grants, session names and cwd.
- **Rebuilt in the new daemon:** the vt emulator, from a rendered seed (refactor `rebuildAfterAltExitLocked` and carry the main screen behind an alt-screen app). Covered by a round-trip test against a reference hub.
- **Reloaded from disk:** the capability key.
- **Child processes:** reaping uses kqueue NOTE_EXIT|NOTE_EXITSTATUS on adopted pids. Where macOS won't report the status for a non-child, the exit code is "unknown", shown as today's default.
- **PTY reads:** a non-blocking master duplicate carries ALL reads and writes, so the freeze can stop reads without losing a byte (probe P3). The "writes must use the same fd" pitfall must be pinned by a large-paste test.
- **Viewers at commit:** close with 1012 (service restart), never 1000. Local GUI tabs and local `portty attach` gain reconnect. Peer tabs, browser viewers and remote attach already reconnect, with delta resume (offsets stay valid because the ring end offset moves with the session).
- **First upgrade from v1.1:** an old daemon without /handoff falls back to the DEBT-08 consent guard (criterion 9.3).

### Decided autonomously (reversible)
| Question | Decision | Why |
|---|---|---|
| A crash between COMMIT and COMMITTED could lose a few bytes | Accept, and keep the window as small as possible: the old daemon sends COMMIT only after READY, and any PTY output buffered in the old daemon is flushed into the transferred ring before the fds are sent. Record it as a `shortcut:` comment and in the threat model. | Inherent to any two-process handoff. The window is milliseconds, and the shells survive either way. |
| A LaunchAgent-managed daemon | Portty doesn't install one (confirm). Hand off the same way whoever started the daemon. | No special case needed. |
| The share server reports an API version | Yes: the same `api` field on the share server's health/info route, so Phases 11 and 13 can gate on it across Macs. | Cheap now, and later phases need it. |
| "Could not be moved" wording | GUI: "Portty couldn't move your sessions to the new version. They're still running on the old version." The DEBT-08 choice follows. CLI: `portty: couldn't move the running sessions to version Y (reason); they are still running on version X.` Then the DEBT-08 refusal applies. | Plain, says nothing was lost. |
| Wails cancels a system logout or restart and shows the quit dialog | Fix in this phase (UPGR-05 neighbour): when macOS asks the app to terminate for a logout, restart or shutdown, quit the GUI without the dialog. Logout ends the daemon anyway, so there's nothing to protect. | An unattended Mac must be able to restart for macOS updates. |
| UPGR-05 SIGTERM | `signal.Reset` of Wails' SIGTERM handler. Portty's handler quits the GUI only; the daemon and sessions keep running. | Matches criterion 9.5. |
| Quit All vs an in-flight restart (IN-07) | EnsureDaemon takes a ctx that Quit All cancels. It never kills a takeover daemon that might already own the sessions: after READY or COMMIT, Quit All waits for the handoff to finish, then stops the daemon normally. | Never risk the sessions. |
| Cmd-Q under the restart dialog | One modal at a time. Cmd-Q while the restart dialog is open answers "Not Now", then shows Quit. | Simple and predictable. |
| Stopped sessions in a handoff (09-REVIEW IN-04) | Accepted loss: only live sessions move. A stopped session's card (exit code, duration, final output) is gone after an upgrade. A `shortcut:` comment on `handoffEntries` says what moving it would take. | Moving it needs an fd-less SESSION frame, a stopped session type and a hub without a reader, for a card the user can recreate by rerunning. No shell is lost. |
| API version (UPGR-04) | An integer `api` in /health, starting at 2 (v1.1 is implicitly 1). A GUI or CLI that needs a higher version shows "update needed" (app banner; CLI one-line message, exit 1). A compatible older daemon kept with Not Now keeps working. | Satisfies the todo from 08-06 IN-03. |

### Claude's Discretion
- How to split the plans (the research suggests 5; 09-01 to 09-03 in parallel), file names, test names, timeouts.
</decisions>

<code_context>
All file:line references are in 09-RESEARCH.md. Hotspots:
- internal/pty (creack/pty pty_darwin.go:14 opens the master blocking)
- internal/relay/hub.go (freeze/thaw, export/import, seed)
- internal/daemon (api.go handleShutdown, process.go EnsureDaemon/staleVersion/RestartPolicy, the lock)
- internal/webserver (share listener on port 7444)
- app.go (beforeClose, the poller, Quit All)
- cmd_attach.go:364-366; frontend/src/components/TerminalPanel.tsx:498; frontend/src/App.tsx:1297-1328
</code_context>

<specifics>
- Toolchain: `export GOTOOLCHAIN=go1.26.11`.
- Gates: `go build ./... && go vet ./...`, `go test -race -short ./...` (nothing skipped), `cd frontend && pnpm vitest run` and tsc, check-brand, check-traceability. Restore frontend/src/wailsjs after Wails builds; the stubs in frontend/src/wailsjs are hand-maintained.
- Run full suites at most once per plan: the user gets a LuLu alert per test binary. Keep the WR-07 tailnet guard.
- A real launch must be checked for anything that touches main() routing (lesson dcbe999d). The two-binary VM rig is in v1.1 08-VM-CHECKS.md (U-01, G-00..G-06). Ad-hoc signing only; fault injection only inside the VMs.
- Keep the share tokens, CSP and 32 KiB limits unchanged.
- Worktrees start at an old commit: fast-forward to gsd/v1.2 first. Don't commit planning files on gsd/v1.2 while executors run (it forces cherry-picks).
</specifics>

<deferred>
- Interrupting a running takeover from the GUI beyond the Quit All ctx.
- Handing off to an OLDER version (a downgrade): not supported. Older-version daemons are refused, or go through DEBT-08.
</deferred>
