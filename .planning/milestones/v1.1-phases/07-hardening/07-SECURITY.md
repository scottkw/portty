---
phase: "07"
slug: "hardening"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-11-19"
---

# Phase 07 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

The register comes from the `<threat_model>` blocks of 07-01..07-05-PLAN.md (authored at plan time). Each mitigation was checked against Phase 7 as shipped, pinned to commit `2e27e73c`: every implementation file was read with `git show 2e27e73c:<path>` (a `git archive 2e27e73c` extract), never from a working tree that later Phase 8 merges may have changed. Config: security enforcement on, ASVS level 1, block on `high`. The audit verified that mitigations exist; it did not scan for new threats. No SUMMARY of 07-01..07-05 raised a threat flag.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| PTY output → joining terminal | bytes any program in the shared shell printed, possibly hours ago, are replayed into a viewer's terminal, which may answer queries | terminal output (may hold secrets the user printed) |
| joining terminal → shared shell | a viewer terminal's answers travel back as input and are typed into the host shell | keystrokes into a live shell (high impact) |
| PTY output → daemon heap | output volume from any program decides how much memory the live emulator holds | memory / availability |
| peer host stream → viewer memory | remote terminal content is kept in the viewing Mac's renderer memory while its tab is hidden | terminal content |
| viewer → peer host (`?resume=`) | the client tells the host which stream offset it already has | an integer offset |
| GUI poller → daemon spawn | the app starts a detached daemon when it judges the local one dead | process lifecycle |
| test binary → host config dir | tests run on the developer's Mac next to the real Portty daemon and settings | settings, socket, lock, capability key |
| app stderr → logs | respawn diagnostics are written where support captures may read them | socket path, dial errors |
| viewer (GUI, browser, CLI) → shared shell | every input frame becomes keystrokes in the host's shell | keystrokes |
| network → input readers | any client on the tailnet with a link can send frames to the share server | WebSocket frames |
| executor → VMs / host | kills, restarts and daemon swaps run in disposable VMs; builds and tests run on the user's Mac | process control |
| evidence file → repository | log excerpts from live sessions are committed | logs (must hold no tokens) |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-07-01 | Tampering | internal/relay/hub.go Join replay (historic CSI t, XTGETTCAP, DECRQSS, DA3, OSC 4 queries) | medium | mitigate | `liveEmuQueryStripPattern` (hub.go:163) covers CSI 11/13/14/15/16/18/19/20/21 t, DA3 `CSI = c`, XTGETTCAP `DCS + q`, DECRQSS `DCS $ q`, OSC 4 `index;?` queries; `StripTerminalQueries` (hub.go:188) runs on the live feed (hub.go:562) and `stripRingQueries` (hub.go:198) on raw full and delta replays in `Join` (hub.go:1192). One subtest per sequence in querystrip_test.go (`addedQueries` plus `regression`, run as `strip/<name>`), plus `TestJoin_StripsQuerySplitAcrossReads`, `TestJoin_StripsAddedQueries`, join_test.go `TestJoin_DeltaStripsTerminalQueries` | closed |
| T-07-02 | Information disclosure | OSC 52 clipboard read replayed to a viewer | medium | mitigate | Pattern alternative `\x1b\]52;[a-z0-9]*;\?(?:\x07\|\x1b\\)` strips only the read form; subtests `OSC 52 read BEL` and `OSC 52 read ST empty selection` in querystrip_test.go; writes are not matched | closed |
| T-07-03 | Tampering | over-broad strip patterns | low | mitigate | Anchored alternatives (XTWINOPS list excludes the 8/22/23 sets; OSC 4 only `index;?` pairs; DECRQSS requires `$` so sixel DCS passes). `keep/<name>` subtests: window resize set, title stack push/pop, fullscreen undo, de-iconify, OSC 10 colour set, OSC 4 palette set, sixel DCS, SGR, bracketed paste, alt screen, plain text | closed |
| T-07-04 | Denial of service | live emulator alt-screen scrollback (hub.go) | medium | mitigate | Rebuild on alt-screen exit (`altExited`, hub.go:565, set by the vt AltScreen callback hub.go:820); `shortcut:` comment documents the in-alt cap (hub.go:795). altscreen_heap_test.go `TestHistory_HeapBoundAfterAltScreenScroll` asserts the vt-derived ceiling (line 75), a plateau across two bursts and under 32 MiB after exit (line 96); `TestAltScreenExit_*` keep main screen and history. The 07-VM-CHECKS H-05 FAIL (0.19 MiB over a plan-time ceiling) was dispositioned by the orchestrator as a ceiling calibration miss: growth plateaus at vt's cap and is freed on exit. The residual heap is Phase 8 DEBT-01 | closed |
| T-07-05 | Denial of service | regexp cost on replay bytes | low | accept | Go regexp is RE2 (linear time); `Join` strips outside emuMu (hub.go:1190 comment) | closed (accepted at plan time) |
| T-07-06 | Information disclosure | frontend/src/lib/peerSnapshots.ts (hidden tab content) | low | accept | Module-level `Map` in renderer memory; no localStorage/sessionStorage or logging in peerSnapshots.ts; `PEER_SNAPSHOT_MAX = 8` | closed (accepted at plan time) |
| T-07-07 | Tampering | snapshot/offset mismatch (TerminalPanel cleanup) | medium | mitigate | TerminalPanel.tsx:494 `client.resumePoint()` read before close; serialize inside the `term.write('', …)` flush callback (lines 565-569), skipped when the watcher reports a torn state (WR-01); host falls back to a full replay when the resume is outside the ring or not past `geomEnd` (hub.go:1232). Tests: TerminalPanel.tabSwitch.test.tsx, peerSnapshots.test.ts, join_test.go `TestJoin_DeltaWhenResumeInRing` / `TestJoin_FullWhenResumeOutOfRange` | closed |
| T-07-08 | Denial of service | snapshot memory growth | low | mitigate | `putPeerSnapshot` evicts the oldest past `PEER_SNAPSHOT_MAX = 8` (peerSnapshots.ts:32-37); `takePeerSnapshot` deletes the entry it returns (line 27). Byte bound and tab-close deletion are Phase 8 DEBT-04 | closed |
| T-07-09 | Spoofing | `?resume=` value sent to the host | low | accept | `ParseResume` (protocol.go:106) accepts only `strconv.ParseUint(s, 10, 64)`; out-of-ring values get a full replay; `TestParseResume` | closed (accepted at plan time) |
| T-07-10 | Denial of service | app.go launch gate suppressing a needed respawn | medium | mitigate | Gate at app.go:343 requires `!seenHealthy`, a non-zero `launchedAt` (set only on a successful startup, app.go:281) and less than `respawnLaunchGrace = 10 s` (app.go:302). Seam tests in app_respawn_test.go: `TestRespawnDaemon_LaunchGraceSkips`, `_LaunchGraceEnds`, `_HealthySeenEndsGrace`, `_FailedStartupNoGrace`, `TestStartDaemon_SetsLaunchedAtOnSuccess` | closed |
| T-07-11 | Tampering | daemon tests writing the real settings.json, socket, lock or capability key | high | mitigate | internal/daemon/testmain_test.go `TestMain` points HOME and XDG_CONFIG_HOME at `os.MkdirTemp("/tmp", "pdh-")` before any test (lines 17-34); api_test.go `testDaemon` adds a per-test `t.Setenv("HOME", t.TempDir())` (line 29); guard `TestTestMain_IsolatesConfigDir` asserts the config dir and socket lie under the temp HOME; `TestTestDaemon_IgnoresSettingsFile` | closed |
| T-07-12 | Information disclosure | health error text in the GUI's stderr | low | accept | Format is `portty: daemon unreachable (%v), respawn attempt %d: %s` / `daemon not answering right after launch (%v)` (app.go). A real capture (08-VM-CHECKS B-08) holds only the socket path and `connect: connection refused` | closed (accepted at plan time) |
| T-07-13 | Denial of service | server input read limits (relay server, proxy client side, share server) | medium | mitigate | No `SetReadLimit` on any input reader: the only call is the proxy's upstream output direction (remote_ws_proxy.go:131, `relay.ViewerReadLimit`); input readers keep coder/websocket's 32768-byte default. `MaxInputPayload = 32767` (protocol.go:94). Guard tests `TestServer_InputFrameOverLimitDropped` (relay/input_limit_test.go:60), `TestRemoteSessionWS_InputFrameOverLimitClosed` (remote_ws_proxy_test.go:316), `TestShare_InputFrameOverLimitDropped` (webserver/input_limit_test.go:39) | closed |
| T-07-14 | Tampering | ordering of split input frames | medium | mitigate | `encodeInputFrames` (relayClient.ts:33) slices at `MAX_INPUT_PAYLOAD = 32767` in order over one WebSocket; relayClient.test.ts "100,000 ASCII characters split into 4 frames … in order" and "a multibyte character straddling a frame boundary still concatenates byte-for-byte"; relay input_limit_test.go `TestServer_InputFrameAtLimitReachesPTY` (a full 32767-byte frame reaches the PTY) | closed |
| T-07-15 | Tampering | input sent while reconnecting | low | mitigate | `sendInput` returns early unless state is `open` (relayClient.ts:529, T-05-14/WR-03); test "a large paste is sent as ordered frames once open, and nothing while reconnecting (HARD-02)" | closed |
| T-07-16 | Tampering | host machine network, power and GUI | high | mitigate | 07-VM-CHECKS.md (at 2e27e73c): every kill, daemon swap and click ran in the VMs over SSH; "Nothing ran on the host besides the build and the SSH commands"; daemon tests run in a temp HOME (T-07-11) | closed |
| T-07-17 | Information disclosure | 07-VM-CHECKS.md | medium | mitigate | `git show 2e27e73c:.planning/phases/07-hardening/07-VM-CHECKS.md` has 0 matches for `cap=[A-Za-z0-9]`, `tskey-` or a URL with a query string | closed |
| T-07-18 | Repudiation | TESTING.md accuracy | low | mitigate | tests/check-traceability-paths.sh exists at 2e27e73c; 07-05-SUMMARY records counts from `git ls-files` (Go 134, vitest 90) and `OK: 146 traceability paths exist` | closed |
| T-07-19 | Tampering | regenerated frontend/src/wailsjs after Wails builds | low | mitigate | `git diff --stat f54a223a 2e27e73c -- frontend/src/wailsjs` is empty: no regenerated bindings were committed during Phase 7 | closed |
| T-07-SC | Tampering | npm/pip/cargo installs (all five plans) | high | mitigate | `git diff --stat f54a223a 2e27e73c -- frontend/package.json frontend/pnpm-lock.yaml go.mod go.sum` is empty: no dependency was added in Phase 7 | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

Unchanged controls (checked because Phase 7 touched neighbouring code): `frontend/index.html` (CSP) has no diff across Phase 7 (`f54a223a..2e27e73c`). The `internal/tailnet`, `internal/webserver` and `internal/daemon/share.go` diffs only add the WR-07 test-binary guard (`tailnet.LiveAllowed()`, which is true outside `go test`; `noLiveWhoIs` refuses lookups with 503 in tests). No token, capability or CSP logic changed.

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-07-01 | T-07-05 | RE2 regexp is linear time; strip runs outside emuMu on replays; bounded character classes | 07-01 plan-time threat model (low severity) | 2026-11-18 |
| AR-07-02 | T-07-06 | Same bytes were already in the tab's xterm buffer; renderer memory only, never persisted or logged, capped at 8, gone at quit | 07-02 plan-time threat model (low severity) | 2026-11-18 |
| AR-07-03 | T-07-09 | Unchanged contract: decimal uint64 only; any out-of-ring value gets a full replay; no new server input | 07-02 plan-time threat model (low severity) | 2026-11-18 |
| AR-07-04 | T-07-12 | Text is the socket path plus a dial/HTTP error; no session content, tokens or capability values (confirmed by the 08-VM-CHECKS B-08 capture) | 07-03 plan-time threat model (low severity) | 2026-11-18 |

All four were accepted in the plans' own threat models, are low severity (below the `high` block threshold), and were not newly accepted by this audit.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-11-19 | 20 | 20 | 0 | gsd-executor (08-04), self-audit per gsd-security-auditor.md, ASVS L1, code pinned at 2e27e73c |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-11-19
