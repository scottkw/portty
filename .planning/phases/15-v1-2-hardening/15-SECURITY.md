---
phase: "15"
slug: "v1-2-hardening"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-11-23"
---

# Phase 15 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| CLI attach ↔ relay hub | `portty attach` resize/claim frames set the shared PTY grid | grid size (cols/rows) |
| Local user ↔ daemon control socket | Unix socket (0600) under the 0700 config dir; no peer-UID check on the control socket (only the handoff socket has one) | full daemon control (session create/kill/handoff) |
| Old daemon ↔ new daemon (takeover) | HELLO/HELLO-OK handshake and fd passing during upgrade | live PTY fds, session state, old daemon pid |
| macOS notification center ↔ app | Notification click enters Go via the `onNotificationClick` cgo export | notification identifier string |
| Viewer ↔ peer share server (tailnet) | Sessions-meta fetch over the tailnet; refused vs timeout classification | session list, reachability |
| Terminal output ↔ path click | Printed relative paths resolved against the cwd log | file paths (preview only, printed-token checked) |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-15-01 | Tampering | `attach.BarRows` / frame build | low | mitigate | Subtract only when `reserve > 0 && rows > reserve`; `TestBarRows` (internal/attach/attach_test.go), `TestInitialResizeFrame` (cmd_attach_test.go) | closed |
| T-15-02 | Tampering | `--status-top` bar row | low | accept | Display-only limit (no origin mode); user-accepted, recorded in cmd_attach.go and TESTING.md M-153 | closed |
| T-15-03 | Tampering | `pty.killSession` dismiss of an exited session | medium | mitigate | `reaped` flag set after every `cmd.Wait`; early return before `signalGroup`; `TestKillSession_ReapedSkipsGroupSignal` (internal/pty/adopt_test.go) | closed |
| T-15-04 | Tampering | dismiss between child exit and reap | low | accept | Zombie leader keeps pid/pgid unreusable until reaped; documented in `killSession` comment | closed |
| T-15-05 | Tampering | flake fixes loosening product behaviour | medium | mitigate | 15-02 Task 2 touched only `_test.go` files; dead-viewer test keeps 100 ms; product heartbeat unchanged | closed |
| T-15-06 | Elevation of Privilege | `listenDaemonSocket` socket mode | medium | mitigate | `daemonSocketMode = 0o600` chmod right after listen, fail closed (internal/daemon/ipc.go); `TestListenDaemonSocket_Mode0600` | closed |
| T-15-07 | Denial of Service | `fdpass.Recv` | low | mitigate | `rightsFromOOB` returns fds collected before a parse error; Recv closes each once; `TestRightsFromOOB`, `TestRecv_TruncatedClosesFds` | closed |
| T-15-08 | Spoofing | takeover HELLO-OK pid | medium | mitigate | `takeoverPeerPID` (LOCAL_PEERPID) compared with HELLO-OK; mismatch/error → `takeoverRefused`, old daemon serves; `TestHandoff_RefusesPIDMismatch`; real-process check M-155 pending at verify-work | closed |
| T-15-09 | Elevation of Privilege | socket inherited from a pre-fix daemon | low | mitigate | `tightenSocketMode` after COMMIT, logged on failure; `TestHandoff_TightensSocketMode`; M-155 pending | closed |
| T-15-10 | Information Disclosure | cwd row numbering (Hub, History) | low | mitigate | `History.Skip` keeps rows monotonic across ED 3; printed-token and previewfile checks untouched; `TestPathClick_RelativeAfterClear` | closed |
| T-15-11 | Tampering | pathLinks mouseleave invalidate | low | accept | Client-side hover memory only; clicks still pass host printed-token/previewfile checks; read-only viewers refused | closed |
| T-15-12 | Spoofing | peer-tab ended verdict | low | accept | Flag only selects between two fixed banners after the server's own session-ended close; no access decision depends on it | closed |
| T-15-13 | Information Disclosure | `MetaRefused` classification | low | mitigate | Only `errors.Is(err, syscall.ECONNREFUSED)` maps to reachable (internal/tailnet/sessions.go); refused peer gets no sessions or capability; `TestMetaStatus_TriState/timeout` | closed |
| T-15-14 | Tampering | `handleNotificationClick` identifier | low | mitigate | Acts only on exact `updateNotificationID`; never passed to shell/URL/path; `TestNotificationClick_IgnoresOtherIDs` | closed |
| T-15-15 | Elevation of Privilege | click starting an update without consent | medium | mitigate | Handler only shows window + emits `update:show`; handler body has 0 mentions of download; Go + vitest assert zero downloads | closed |
| T-15-16 | Denial of Service | delegate completion handler | low | mitigate | `completionHandler()` called on every path (tray_objc_darwin.m:106); Go export returns at once | closed |
| T-15-17 | Repudiation | todo moves | low | mitigate | Todos moved only after 15-09's single clean run; each Resolution names plan and commits | closed |
| T-15-18 | Tampering | TESTING.md traceability | low | mitigate | `bash tests/check-traceability-paths.sh` → "OK: 228 traceability paths exist" | closed |
| T-15-SC | Tampering | npm/pip/cargo installs | high | accept | No package installed or added in any Phase 15 plan | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-15-01 | T-15-02 | `--status-top` overdraw by absolute-addressing apps; display-only, bottom bar (default) fully fixed | user (Phase 15 discuss, "You decide" → recorded limit) | 2026-11-22 |
| AR-15-02 | T-15-04 | Residual window between child exit and reap is safe: pgid cannot be reused while the leader is a zombie | plan 15-02 | 2026-11-23 |
| AR-15-03 | T-15-11 | Hover-memory reset is client-only; host-side checks unchanged | plan 15-04 | 2026-11-23 |
| AR-15-04 | T-15-12 | Banner choice only; no authorization depends on the ended flag | plan 15-05 | 2026-11-23 |
| AR-15-05 | T-15-SC | No dependencies added | all plans | 2026-11-23 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-11-23 | 19 | 19 | 0 | /gsd-secure-phase (orchestrator, ASVS L1 grep-depth; auditor skipped per short-circuit rule) |

Note: T-15-08 and T-15-09 are mitigated in code and unit-tested in-process; the real two-build upgrade chain (TESTING.md M-155, ship blocker) runs in the `portty-test` VM at `/gsd-verify-work`.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-11-23
