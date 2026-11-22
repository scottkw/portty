# Phase 12 Security Review: File Path Click

## Scope and parts

Scope: what Phase 12 (PATH-02) changes at the security boundary. A viewer that can write may click a path the session printed, and the host then previews that file. That amends the Phase 11 rule "the daemon never opens a path given to it" (PREV-03). This file is written in two parts:

- **12-05 (this part):** the amendment, the argument, the honest scope, accepted risks, shell-integration rules, what stays unchanged, the threat register and the tests that pin each point.
- **12-07:** the "VM evidence and verdict" section, appended after this one.

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
| T-12-27 | Repudiation: the PREV-03 amendment | medium | mitigate | this file, with the verdict added by 12-07 |
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
