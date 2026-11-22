# Phase 12 Security Review: File Path Click

## Scope and parts

Scope: what Phase 12 (PATH-02) changes at the security boundary. A viewer that can write may click a path the session printed, and the host then previews that file. That amends the Phase 11 rule "the daemon never opens a path given to it" (PREV-03). This file is written in two parts:

- **12-05 (this part):** the amendment, the argument, the honest scope, accepted risks, shell-integration rules, what stays unchanged, the threat register and the tests that pin each point.
- **12-09 (appended, after 12-07 ended at its checkpoint and 12-08 closed the viewer gaps):** "Gap closure (12-08)", then "VM evidence and verdict" with the residual risks.

No share link, capability token, tailnet host or path from a real Mac appears in this file. Hosts in policies are written as `<host>`.

## PREV-03 amended

**Phase 11:** the daemon never opens a path given to it.

**Phase 12:** it opens a path only when the text is a token in that session's own rendered output, the requesting subscriber can write, and the file passes the `portty open` checks; still no HTTP route or wire message reads an arbitrary path; the click rides the session's authenticated WebSocket.

The three conditions and where each is enforced:

| Condition | Enforced in | Pinned by |
|-----------|-------------|-----------|
| (a) the text is a token in that session's own rendered output | `internal/relay/pathclick.go`: whole-token search over the ANSI-stripped history rows plus the live screen (soft-wrap joins by the hub's wrap marks, then the full-row fallback). The raw byte ring is never searched, so window titles and escape strings do not count | `TestPathClick_TokenTable`, `TestPathClick_NotPrintedAbsolute`, `TestPathClick_NotPrintedEdges`, `TestPathClick_WrapFallback` (`internal/relay/pathclick_test.go`) |
| (b) the requesting subscriber can write | The share server's read pump (`internal/webserver/server.go`, `case relay.MsgPathClick`, skipped for read-only subscribers), the loopback relay pump (`internal/relay/server.go`) and the top of `Hub.PathClick`. Read-only subscribers also get no capability bit on the replay meta | `TestWebPathClick_ReadCapRefused`, `TestWebPathClick_ReadCapNoCapabilityBit`, `TestPathClick_RelayPumpReadOnly`, `TestPathClick_Refusals`, `TestPathClick_CapabilityBit` |
| (c) the file passes the `portty open` checks | `internal/previewfile` `Open`, moved unchanged from `portty open` in 12-01: stat, non-blocking open, descriptor re-check that it is a regular file, bounded read, 20,000,000-byte cap (`relay.PreviewMaxBytes`), closed type allowlist, name cleaning | `TestOpen_Order` (`internal/previewfile/previewfile_test.go`), `TestCmdOpen_FIFORefusedPromptly` (`cmd_open_test.go`), `TestPathClick_Outcomes` |

## Why the contract holds

This is the PREV-03 argument, point by point.

1. **No HTTP route and no new socket takes a path.** The preview route stays on the owner-only unix socket and is absent from the loopback relay handler and from the share server. The click is a frame (`0x13` plus the clicked text) on the WebSocket the viewer already holds. Pinned by `TestPreview_NotOnRelayHandler` (`internal/daemon/preview_test.go`) and `TestPreviewRoute_NotOnShareServer` (`internal/webserver/files_removed_test.go`), both unchanged.
2. **The authentication in front of the stream is unchanged.** A share viewer still passes the token gate, the WhoIs gate and the tailnet-only listener before its read pump runs; a peer daemon's tab proxies the same stream. The read pump arm is reached only after those. Pinned by the existing capability and origin tests (`internal/webserver/capability_test.go`, `origin_integration_test.go`).
3. **The host resolves against that session's own state.** The hub searches its own history and screen and resolves a relative text against its own directory log (OSC 7 reports from that session). A click cannot name another session's output or directory.
4. **Only text the session printed is accepted, as a whole token, from rendered output.** `/x/.ssh/id_rsa` is not a token of `/x/.ssh/id_rsa.pub`; `a/b.png:12:3` resolves to `a/b.png`. The rendered rows are searched, not the raw ring. Pinned by `TestPathClick_TokenTable`, `TestPathClick_NotPrintedAbsolute`, `TestPathClick_NotPrintedEdges`.
5. **The file must exist, be a regular file and be an allowlisted type under 20,000,000 bytes.** All through `previewfile.Open` (condition (c)). Folders, FIFOs and devices are refused; a missing or too-large file produces a fixed notice for the clicker only.
6. **The click path cannot touch the file system by itself.** `TestPathClick_SourceGate` (`internal/daemon/preview_test.go`) pins that `internal/relay/pathclick.go` imports none of `os`, `os/exec`, `io/fs`, `path/filepath`, `syscall` or `io/ioutil`, that `previewfile.go` has no `os/exec`, and that `pathclick.go` is the only non-test relay file importing `previewfile`. `TestPreview_SourceGate` (unchanged) still pins the Phase 11 preview code.
7. **The payload is bounded and validated before any work.** At most 1024 bytes, valid UTF-8, no control character; one resolve per hub at a time (the resolve runs on its own goroutine, off the pump); clicks during a resolve, a preview, a handoff freeze or after the session ended are dropped. Pinned by `TestPathClick_Refusals` and `TestPathClick_ReturnsAtOnce`.
8. **Nothing about the click is logged or broadcast.** One outcome-code line per resolve with the byte count of the file (never the text, path or name); notices go to the clicker only. Pinned by `TestPathClick_NeverLogsPath` and `TestPathClick_Outcomes`. The share pump logs nothing per click.
9. **A click cannot move the terminal grid.** Focus goes through `Preview.FocusSub`; the typing claim is never called. `TestPathClick_LocalViewerAbsolute` checks the previous typist stays unfocused.

## Honest scope

A write-capable viewer already controls the shell (SHAR-02: a share always grants full control) and could `cat` any file the user can read. The printed-token check and the type allowlist are defence in depth, not the boundary.

The boundary this phase adds is narrower and real: **a read-only viewer cannot trigger a path click.** It is refused in the share pump, in the loopback relay pump and again at the top of `Hub.PathClick`, and it never gets the capability bit, so a correct viewer never offers a link to it. The two pump checks and the hub check are redundant on purpose: every minted share is read,write today, so a read-only subscriber exists only with a legacy or hand-signed capability, and a single check could rot unnoticed. `TestWebPathClick_ReadCapRefused` proves the negative against a positive control on the same hub: the read-only click gets no preview and no notice within 500 ms, then a write capability's identical click on the same hub gets a preview header, so the hub was able and only the capability differed.

A read-only viewer **does receive the preview** that a writable viewer's click triggers, exactly as with Phase 11's `portty open` (`SendPreview` has no read-only filter); the writer could equally `cat` the file onto the screen. User decision 2026-11-21: keep this, recorded as an accepted risk below.

## Accepted risks

- **A program in the session can print any path or forge OSC 7.** It already runs as the user and can run `portty open`; the printed check bounds viewer-originated requests only (T-12-10, T-12-23).
- **OSC 7 from ssh peers is ignored by the host check.** That check is not a security boundary.
- **Nested shells, ssh, and bash or fish without their own OSC 7 leave the outer directory.** A relative click can then name a same-named file in the outer directory. With no directory record at all, a relative click is refused, never seeded from the spawn directory.
- **Hard-wrapped TUI paths cannot be clicked.** Rows that are not full are never joined.
- **A path split by a real newline in the output (for example `docs/dia` then `gram.png`) is never clickable.** User decision 2026-11-21: accepted as a known limit, because joining across hard breaks would loosen the printed check.
- **Clicks arriving while a resolve or preview is in flight, during a handoff freeze, or after the session ended are dropped, not queued.** The user clicks again. A resolve already running when the hub starts freezing still finishes (`SendPreview` does not check the freeze, as in Phase 11).
- **Read-only viewers receive previews triggered by writable viewers.** User decision 2026-11-21, as Phase 11.
- **Where the hub has no wrap mark for a full-width row (`wraps.go`'s heuristic), 12-04's full-row fallback counts both readings as printed:** the row joined to the next, and the next row on its own. Still whole tokens, and never across a row that is not full (`TestPathClick_WrapFallback`).
- **A write-capable viewer can read any file the user can.** Same as typing `cat` (T-12-24).

## Shell integration

The zsh OSC 7 shim (12-02) is what gives relative clicks a directory. Its safety rules:

- It lives only in Portty's 0700 config directory, as a 0600 `.zshenv`, rewritten atomically from the embedded script whenever it differs, at every session creation (`TestZshShim_WriterAtomic`).
- It is read back and verified equal to the embedded script before `ZDOTDIR` is set. On any failure, or an empty or relative config directory, nothing is set, the session still spawns and a `zsh shim:` line is logged (`TestZshShim_FailureStillSpawns`, `TestZshShim_RefusesRelativeConfigDir`).
- The user's own startup files are never touched; the daemon writes only under its config directory (`TestZshShim_UserFilesUntouched`).
- The user's original `ZDOTDIR` is passed explicitly on every spawn (empty when there was none), overriding any stale inherited value (`TestZshShim_StaleInheritedVar`).
- Reports go to `/dev/tty`, never stdout, and subshells emit nothing, so scripts that capture `$(cd ...)` output are not affected (`TestZshShim_SubshellAndRedirect`).
- It uses quoted builtins and `emulate -L zsh` inside an `always` block so a future zsh cannot break startup (`TestZshShim_CleanStartupNoErrors` runs the real `/bin/zsh`).
- No viewer input reaches it: the script is embedded and fixed, and the host parses OSC 7 payloads defensively (scheme, host, absolute path, control characters, 1024-byte path and 2048-byte payload caps; `TestParseOSC7`). The directory log is bounded and lives in memory only (`TestCwdLog_Rules`); a handoff import validates it (`TestHandoffState_CwdValidates`).

## What stays unchanged

- **The share page CSP** is Phase 11's Amendment 3 exactly, with `frame-ancestors 'none'` last. No new resource types are needed: the preview renderers already exist. Pinned by `TestCSPHeaders_PreviewDelta` (`internal/webserver/csp_mw_test.go`), unchanged. 12-05's gate also checks that `internal/webserver/csp_mw.go` still contains `frame-ancestors 'none'`.
- **The token gate, the WhoIs gate and the tailnet-only listener** are not touched. The only change to `internal/webserver/server.go` is one `case` in the read pump.
- **`web/assets/terminal.js`** is out of scope: it has no preview support and gets no path clicks.
- **Version compatibility.** `APIVersion` is 4 and `PathClickAPI` is 4 (beside `PreviewAPI` 3); `MinDaemonAPI` stays 1, so no "Update needed" banner appears for a daemon on API 3, which simply has no path clicks. Viewers do not read the version: they follow the host's per-connection capability bit on the replay meta, which also covers peer and browser viewers that never see `/health`. An older host drops the unknown `0x13` frame (no case and no default arm in either pump). Pinned by `TestHealthInfo` (`internal/daemon/client_test.go`) and `TestAPIHealth` (`internal/daemon/api_test.go`).

## Threat register

Rows from the threat models of 12-01 to 12-05. The T-12-SC row (package installs) is in each plan: no package was installed in any of them, so the package-legitimacy gate never triggered.

| ID | Threat | Severity | Disposition | Pinned by |
|----|--------|----------|-------------|-----------|
| T-12-01 | Tampering: `previewfile.Open` check order | medium | mitigate | `TestOpen_Order`, `TestCmdOpen_FIFORefusedPromptly` |
| T-12-02 | Elevation: `previewfile` reachable from relay | high | mitigate | `TestPathClick_SourceGate` |
| T-12-03 | Tampering: preview names | low | mitigate | `TestPreview_NameIsPercentDecodedAndCleaned` |
| T-12-04 | Denial of service: user's shell startup | high | mitigate | `TestZshShim_FailureStillSpawns`, `TestZshShim_RefusesRelativeConfigDir` |
| T-12-05 | Tampering: `<configDir>/zsh/.zshenv` | medium | mitigate | `TestZshShim_WriterAtomic` |
| T-12-06 | Spoofing: inherited `PORTTY_ZSH_ZDOTDIR` | medium | mitigate | `TestZshShim_StaleInheritedVar` |
| T-12-07 | Tampering: scripts capturing `$(cd ...)` output | medium | mitigate | `TestZshShim_SubshellAndRedirect` |
| T-12-08 | Denial of service: shim errors on a future zsh | medium | mitigate | `TestZshShim_CleanStartupNoErrors` |
| T-12-09 | Tampering: user's own startup files | high | mitigate | `TestZshShim_UserFilesUntouched` |
| T-12-10 | Spoofing: OSC 7 from a program or remote host | low | accept | none (accepted; see Accepted risks) |
| T-12-11 | Tampering: OSC 7 payload content | medium | mitigate | `TestParseOSC7` |
| T-12-12 | Denial of service: cwd log growth | medium | mitigate | `TestCwdLog_Rules` |
| T-12-13 | Tampering: handoff import of cwd state | medium | mitigate | `TestHandoffState_CwdValidates` |
| T-12-14 | Information disclosure: directories in memory and logs | low | mitigate | `TestPathClick_NeverLogsPath` (no directory or path in any line); the log is memory-only |
| T-12-15 | Information disclosure: read-only viewer clicks | high | mitigate | `TestPathClick_RelayPumpReadOnly`, `TestPathClick_Refusals`, `TestPathClick_CapabilityBit` |
| T-12-16 | Information disclosure: a path the session never printed | high | mitigate | `TestPathClick_TokenTable`, `TestPathClick_NotPrintedAbsolute`, `TestPathClick_NotPrintedEdges`, `TestPathClick_WrapFallback` |
| T-12-17 | Information disclosure: the notice | medium | mitigate | `TestPathClick_Outcomes` |
| T-12-18 | Information disclosure: logs | medium | mitigate | `TestPathClick_NeverLogsPath` |
| T-12-19 | Denial of service: click spam, slow disk | medium | mitigate | `TestPathClick_Refusals`, `TestPathClick_ReturnsAtOnce` |
| T-12-20 | Tampering: FIFO or device swapped in after the check | medium | mitigate | `TestOpen_Order`, `TestCmdOpen_FIFORefusedPromptly` |
| T-12-21 | Tampering: the PTY grid | medium | mitigate | `TestPathClick_LocalViewerAbsolute` |
| T-12-22 | Elevation: file reads outside the contract | high | mitigate | `TestPathClick_SourceGate`, `TestPreview_SourceGate` |
| T-12-23 | Spoofing: a program prints a path or forges OSC 7 | low | accept | none (accepted; see Accepted risks) |
| T-12-24 | Information disclosure: a write-capable viewer reads any file the user can | low | accept | none (accepted; see Honest scope) |
| T-12-25 | Elevation: share pump with a read-only capability | high | mitigate | `TestWebPathClick_ReadCapRefused` (with a write-capability positive control), `TestWebPathClick_ReadCapNoCapabilityBit` |
| T-12-26 | Spoofing: version confusion between hosts and viewers | low | mitigate | `TestWebPathClick_WriteCapPreviews` (the bit, not the version, enables it); `TestHealthInfo`, `TestAPIHealth` |
| T-12-27 | Repudiation: the PREV-03 amendment | medium | mitigate | this file, with the verdict added by 12-09 |
| T-12-28 | Information disclosure: this file | medium | mitigate | the 12-05 gate greps this file for capability and tailnet link patterns |

## Tests that pin it

- Source gates: `TestPathClick_SourceGate`, `TestPreview_SourceGate` (`internal/daemon/preview_test.go`)
- Printed-token check: `TestPathClick_TokenTable`, `TestPathClick_NotPrintedAbsolute`, `TestPathClick_NotPrintedEdges`, `TestPathClick_WrapFallback` (`internal/relay/pathclick_test.go`)
- Refusals, outcomes, logs, capability bit: `TestPathClick_Refusals`, `TestPathClick_RelayPumpReadOnly`, `TestPathClick_Outcomes`, `TestPathClick_NeverLogsPath`, `TestPathClick_CapabilityBit` (`internal/relay/pathclick_test.go`)
- Share server: `TestWebPathClick_ReadCapRefused`, `TestWebPathClick_ReadCapNoCapabilityBit`, `TestWebPathClick_WriteCapPreviews` (`internal/webserver/pathclick_test.go`)
- Shell integration: `TestZshShim_FailureStillSpawns`, `TestZshShim_UserFilesUntouched` (`internal/daemon/zshshim_test.go`), and the other `TestZshShim_*` tests cited above
- Directory reports: `TestParseOSC7` (`internal/relay/cwd_test.go`), `TestHandoffState_CwdValidates` (`internal/relay/handoff_state_test.go`)
- CSP, unchanged: `TestCSPHeaders_PreviewDelta` (`internal/webserver/csp_mw_test.go`)
- API values: `TestHealthInfo` (`internal/daemon/client_test.go`), `TestAPIHealth` (`internal/daemon/api_test.go`)

## Gap closure (12-08)

12-08 changed the viewer only (no Go file, no CORS header, no route, no listener, no CSP). What it changed at the security boundary:

- **Desktop peer tabs take this Mac's own `webLinks` and `webLinksConfig` and no other setting** (user-ratified 2026-11-21, "This Mac's setting"). `App.tsx` builds the peer config from this Mac's plugin settings; `WebShareSessionView` no longer self-fetches the host's plugin config for a remote tab. WebGL, clipboard, search, serialize and progress stay off in a peer tab (pinned by `a peer tab follows this Mac's web-links toggle and modifier and turns on no other addon`).
- **No CORS header, route, listener or CSP change.** The peer tab's cross-origin request had no `Access-Control-*` answer and was refused; the fix removes the request instead of allowing it. No `Access-Control-Allow-Origin` exists in non-test `internal/webserver` code, and `frontend/index.html` and `internal/webserver/csp_mw.go` are identical to `81ada526` (checked by the 12-08 and 12-09 gates).
- **Peer tabs no longer send cap-bearing cross-origin requests** for the plugin config or its event stream, and the retry loop that went with them is gone (pinned by `a peer tab never requests /api/plugin-config or its stream, even with no config`).
- **URL links in peer tabs go through the existing handler**: the allowed-scheme check, the modifier gate and the IDN, typosquat and OSC 8 confirm switches, all from this Mac's own settings, exactly as in a local tab. The VM check opened the confirm popover for a punycode URL in a peer tab and Cancel closed it (C-02R).
- **The link refresh** (a surviving viewer keeps its links after a reconnect or resize) resets two private xterm Linkifier fields (`_lastBufferCell`, `_activeLine`) behind a presence guard, then re-hovers with a **non-bubbling** synthetic `mousemove` on `.xterm-screen`. xterm's mouse-report listener sits on `term.element` in the bubble phase and never sees it, so a mouse-tracking program gets no input (pinned by the zero-`onData` test with mouse modes 1003 and 1006 on; the vim `mouse=a` row C-07R found 0 SGR reports). An xterm upgrade that renames the fields fails `xterm 6 still has the Linkifier fields the refresh resets`.
- **Read-only viewers still get no link.** `provideLinks` is unchanged and offers nothing while the capability bit is false; the host refusal is unchanged (C-05R).

Threat register additions (rows from the 12-08 threat model):

| ID | Threat | Severity | Disposition | Pinned by |
|----|--------|----------|-------------|-----------|
| T-12-40 | Tampering: `pathLinks.ts` link refresh (synthetic `mousemove`) | high | mitigate | dispatched with `bubbles: false` on `.xterm-screen`; `the link refresh sends no input to a program with mouse tracking on` (zero `onData` with 1003 and 1006 on) |
| T-12-41 | Information disclosure: `WebShareSessionView` peer-tab config fetch and stream | medium | mitigate | `a peer tab never requests /api/plugin-config or its stream, even with no config` |
| T-12-42 | Elevation of privilege: share server CORS | high | mitigate | no CORS header added; the 12-08 gate greps non-test `internal/webserver` code for it and fails on any Go change |
| T-12-43 | Spoofing: URL links now present in peer tabs | medium | mitigate | the existing WebLinksAddon handler and its confirm switches, from this Mac's own settings; C-02R opened the popover |
| T-12-44 | Tampering: private xterm fields `_lastBufferCell`, `_activeLine` | low | mitigate | guarded access; `xterm 6 still has the Linkifier fields the refresh resets` |
| T-12-45 | Information disclosure: read-only viewers and the link refresh | high | mitigate | `provideLinks` unchanged; `while the capability is false, no link is offered and nothing is sent` and the 12-06 gating tests; C-05R |

(12-09's own register, T-12-46..T-12-50 and T-12-SC, covers the VM run: host untouched, no secrets in the evidence, the throwaway signer deleted, no network drop, VM settings and dotfiles restored.)

## VM evidence and verdict

Evidence is in `12-VM-CHECKS.md`: the first run (12-07, builds A, B, B2) and the re-run after the gap plans (12-09, builds C, C2, HEAD), both on two Tart VMs over a real tailnet. Decisions by the user on 2026-11-22 are quoted as the user's words.

- **Read-only refusal (C-05R, first run C-05): PASS.** A read-only Safari viewer's typing was refused (`passed=0`, tail unchanged); a Meta-hover over 12 positions on the printed path never turned the cursor to a pointer and drew no underline; a hand-sent `0x13` plus the printed path left no `path click` line for its conn, no preview and no notice anywhere. A writable viewer's click then reached the read-only viewer as a background tab: the accepted risk, as `portty open`.
- **Notices only to the clicker, including a peer tab (C-04R): PASS.** From the office tab, home Safari and the home peer tab (the clause the first run could not make), the `.txt` and the 20,000,001-byte file showed their exact fixed notices only in the clicking window, gone after about 5 s; a missing path and a folder were silent everywhere with the session tail unchanged.
- **The TUI click (C-07R): PASS.** In vim with `mouse=a` the Cmd-click previewed, 0 SGR mouse reports reached the session, vim quit cleanly and the shell click previewed. Claude Code inside a session is a check for the user's own Macs (M-132).
- **Shell integrity and the ssh decision (C-08, UI-1): PASS by an accepted limit.** With and without a `~/.zshenv` that exports `ZDOTDIR` and a heavy oh-my-zsh rc, the prompt, alias, `$ZDOTDIR`, `$HISTFILE`, PATH and `portty open` matched a plain login zsh, `$(cd …)` captured no escape byte, nested zsh worked, the cwd was reported after `cd`, and the log had no `zsh shim:` line. The one clause the rig could not reach, `ssh localhost`, is accepted as a check for the user's own Macs (M-133): "Check on my Macs" (user decision 2026-11-22).
- **Upgrades with surviving viewers (U-01, U-01R): PASS.** A to B in the first run (3 of 3 `mode=delta` within 1.9 s, 0 grid lines, old shell `no-cwd`, new session resolves) and C to C2 in the re-run (3 of 3 `mode=delta` within 1.62 s, 0 grid lines; the office tab, the home peer tab and the same Safari page each previewed the pre-upgrade relative path on the first click with nothing typed and no pointer move).
- **Resize and wrap on a real Mac (C-09R, first run C-09): PASS.** After narrowing, widening, a grid claim by the home tab and its restore, the first Cmd-click at the same point previewed each time; wrapped rows, history, the identical hard row and the hard-break limit behaved as specified.
- **Other re-run rows PASS:** C-00R, C-01R, C-02R (the peer tab previews and its URL click opens the confirm popover), C-03R, C-06R (Safari underline follows Meta; the notice repaints dark on the same conn). The desktop app's Cmd-held underline and live dark repaint cannot be observed in the VMs; the user kept the Cmd-held underline with no fallback and left the check to their own Macs (M-131): "Check on my Macs" (user decision 2026-11-22).
- **Gate:** one `internal/pty` `TestKillClean_NoOrphans` failure under load in the re-run's full gate, accepted by the user (2026-11-22) as an unrelated timing flake: alone with `-count=5` and the full package once it passed, and `internal/pty` has no diff since `81ada526`.

### Residual risks

1. A program in the session can print any path or forge OSC 7 (T-12-10, T-12-23); the printed check bounds viewer-originated requests only.
2. Nested shells, `ssh` and bash or fish without their own OSC 7 leave the outer directory; a relative click after a remote `cd` can name a same-named file in the outer directory. The `ssh localhost` run itself is left to the user's own Macs (UI-1).
3. Hard-wrapped TUI paths and paths split by a real newline are never clickable (accepted 2026-11-21).
4. **F-1 (todo, not a blocker; user: "Todos, not blockers"):** a relative path printed on screen row 0 of a never-scrolled session resolves against the shell's start directory (seen as `outcome=missing`; a same-named file in the start directory would be previewed instead, the same class as the nested-shell limit above, and the text is still a printed token). Rows 1 and below and absolute paths are unaffected. Todo `.planning/todos/pending/2026-11-22-row0-relative-path-start-dir.md`.
5. **F-2 (todo, not a blocker; user: "Todos, not blockers"):** the first Cmd-click fails after the pointer returns to the same cell whose text changed; the viewer sends nothing (no host involvement). Todo `.planning/todos/pending/2026-11-22-first-cmd-click-same-cell-after-leave.md`.
6. The desktop app's Cmd-held underline and dark-mode notice repaint, `ssh localhost` and Claude Code inside a session were not observed in the VMs; the user checks them on their own Macs (M-131, M-133, M-132).
7. Read-only viewers receive previews triggered by writable viewers (user decision 2026-11-21, as Phase 11).

**Verdict:** APPROVED. C-04R PASS (notices only to the clicker, peer tab included), C-05R PASS (a read-only viewer gets no link and its hand-sent click is refused), C-07R PASS (a TUI click previews and types nothing) and C-08 PASS (shell startup unchanged; the `ssh localhost` clause accepted as a check for the user's own Macs, user decision 2026-11-22); the PREV-03 amendment holds with the residual risks above.
