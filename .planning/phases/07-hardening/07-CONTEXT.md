# Phase 7: Hardening — Context

**Gathered:** 2026-11-18 (smart discuss, autonomous; all decisions technical and reversible)
**Status:** Ready for planning

<domain>
Close the v1.0 reliability and test-hygiene debt (HARD-01..HARD-08). Each requirement maps to a pending todo in `.planning/todos/pending/` tagged `resolves_phase: 7` — read the todo for problem, evidence and solution sketch. No new user-facing features. User priority (memory): rock solid; people rely on the remote Mac with no other way in.
</domain>

<decisions>

### Decided autonomously (reversible technical)
| Req | Decision | Why |
|---|---|---|
| HARD-01 alt-screen heap | When the app leaves the alternate screen, drain main-screen history into `Hub.history` and rebuild the live emulator (same path a resize already uses, hub.go ~599). No upstream fork of charmbracelet/x/vt. Add a heap soak test modelled on `TestHistory_HeapBoundAfterBurst` (20,000+ alt-screen lines, fixed bound). Also bound growth while still *in* alt screen if the vt API allows (check `SetScrollbackSize` reach); if not, document the limit in a `shortcut:` comment. | Reuses existing rebuild path; local change |
| HARD-02 large pastes | Split input in the frontend `relayClient` into frames of ≤ 32 KiB minus header (one place, covers local/peer/browser since all use RelayClient — verify browser path), keep order. CLI caps each stdin read so header+payload ≤ 32 KiB (32767 payload). Keep every server-side 32 KiB input limit unchanged (DoS guard). Fix the misleading comment at `internal/daemon/remote_ws_proxy.go` ~66. Split on UTF-8 boundaries (encode first, slice bytes; a split multibyte char is fine at byte level because the PTY receives the byte stream in order — confirm, else split on code points). | Keeps the guard; smallest fix |
| HARD-03 query strip | Extend the query-strip patterns identically in `internal/relay/hub.go` ReconnectPreamble and the copy in `internal/daemon/engine.go`; one test per added sequence (CSI 14/16/18 t, XTGETTCAP `DCS + q`, DECRQSS `DCS $ q`, DA3 `CSI = c`, OSC 4 `;?`, OSC 52 `;?`); fix doc comment ("mode sets stay" is wrong for CSI ? 2048 h). If the two copies can share one function without an import cycle, share it; otherwise keep identical with a test asserting parity. Known limit stays: query split across two PTY reads. | Local |
| HARD-04 spurious respawn | Find why the first poller tick sees the daemon unreachable with a healthy daemon (likely the tick fires before startup's EnsureDaemon/client is ready). Fix: the respawn path does nothing until the first healthy observation after startup (or startup completion). Seam test in `app_respawn_test.go`. Keep the respawn behaviour itself unchanged (Phase 5 R6). | Local |
| HARD-05 tab switch | Keep each peer tab's last relay offset across panel unmount (e.g. a per-tab map in App or the RelayClient cache keyed by tab id) and rejoin with `resume=true` + offset. Do NOT keep N sockets open for hidden tabs. Fallback to full replay if the host says the offset is gone (existing delta/full meta). Vitest coverage + VM spot check. | Lower cost than hidden mounted panels |
| HARD-06 settings tests | Point `testDaemon` (internal/daemon/api_test.go) at `t.TempDir()` for settings and socket (use `ConfigDirForTest` / HOME isolation); ensure no daemon test resolves the real socket ("daemon already running at …/portty/daemon.sock" must disappear). Remove the `-skip` wherever it remains (TESTING.md, scripts, CI; roadmapper found none outside .planning — confirm). Closes both settings todos. | Test-only |
| HARD-07 fan-out flake | Reproduce with `go test -race -count=50 -run TestHub_TwoClientsFanOut ./internal/relay/` under load; fix the race in code if real, else make the test deterministic (wait for both subscribers registered before writing). | Test or local fix |
| HARD-08 docs | Add a "Known issues" entry to user docs (README or docs/ help) for stray multi-line prompt lines after narrowing: cause (zsh redraws a wrapped multi-line prompt per resize) and remedies (`clear`/Ctrl-L, single-line prompt via `p10k configure`). Keep brand gate clean. | Docs |

### Claude's Discretion
Plan split (coarse granularity: few plans, parallel where files don't overlap), exact test names, file placement.
</decisions>

<code_context>
- `internal/relay/hub.go`: emulator setup ~504 (`SetScrollbackSize(HistoryMaxLines)`), rebuild-on-resize ~599 (`if old := h.liveEmu; !old.IsAltScreen()`), alt-screen replay ~688-717, `Join` ~835, ReconnectPreamble query strip.
- `internal/daemon/engine.go`: byte-identical query-strip copy.
- `frontend/src/lib/relayClient.ts:27` `encodeInputFrame`, send ~498; `cmd_attach.go` / `internal/attach` stdin read buffer (32 KiB).
- `internal/daemon/remote_ws_proxy.go` ~66 comment.
- `app.go` `respawnDaemonIfDown` (5 s poller, daemonMu, backoff, streak), `app_respawn_test.go`.
- `frontend/src/App.tsx`, `TerminalPanel.tsx`, `WebShareSessionView.tsx` (peer tab lifecycle, resume offset kept until new replay meta).
- `internal/daemon/api_test.go` `testDaemon`; `ConfigDirForTest`.
- `internal/relay` `TestHub_TwoClientsFanOut`.
- TESTING.md standing convention (Section 2 manifest, Section 4 traceability, `bash tests/check-traceability-paths.sh`).
</code_context>

<specifics>
- Toolchain: `export GOTOOLCHAIN=go1.26.11`. Gates: `go build ./... && go vet ./...`, `go test -race -short ./...` (after HARD-06, with NO -skip), `cd frontend && pnpm vitest run`, `bash tests/check-brand.sh`, `bash tests/check-traceability-paths.sh`. Restore `frontend/src/wailsjs` after any Wails build.
- VM spot check (Tart VMs portty-test 192.168.64.108 / portty-test-2 192.168.64.109, see v1.0 05-VM-MATRIX.md for how): large paste into a peer tab and `portty attach`; tab switch rejoin is delta (daemon.log viewer-diag / meta); launch logs no respawn attempt; alt-screen scroll soak memory stays bounded. Fault injection only inside VMs; never on the host.
- Keep token-gated share links and CSP untouched.
</specifics>

<deferred>
- Update-notification click handler (needs notarized build).
- Two-Mac M-84/M-85 and first-release M-93.
</deferred>
