# Phase 8: Debt Cleanup — Context

**Gathered:** 2026-11-19 (autonomous; the user chose "Fix debt first" at v1.1 close; all decisions technical and reversible)
**Status:** Ready for planning

<domain>
Close the tech debt in `.planning/v1.1-MILESTONE-AUDIT.md` (DEBT-01..07). There are no new user-facing features, apart from the partial-paste notice that DEBT-05 requires. User priority: rock solid; nobody can get in to fix the remote Mac. Source material:
- 07-REVIEW.md (iteration 3) and 07-REVIEW-FIX.md (iterations 1-3): the deferred items IN-02/03/05/06/08 and their reasons.
- 07-VM-CHECKS.md: H-05 and R-02 (the soft-wrap observation).
- The integration checker's W1/W2, quoted in the audit.
</domain>

<decisions>
| Req | Decision | Why |
|---|---|---|
| DEBT-01 alt-screen heap | Bound the alt scrollback while still in the alt screen. Viewers get the rendered alt screen, never the alt scrollback, so the hub can rebuild the live emulator in the alt screen once its alt scrollback passes a small threshold (e.g. a few hundred lines). Reuse the rebuild machinery from WR-02/03/06 and carry the main screen, history and alt screen over. The other option is to set vt's scrollback size to 0 while in the alt screen and restore it on exit, if `SetScrollbackSize` affects only new pushes and keeps the main history; check this against the vt source first and prefer it if it's correct, since it's far smaller. No vt fork. Soak test at 80 and 220 columns with a fixed bound (e.g. < 32 MiB growth), plus a VM check like H-05. | The residual is 128-270 MiB per session; that's real on an 8 GB Mac |
| DEBT-02 soft-wrap | Find where the wrap flag is lost: history push, `renderScreenLocked`, or the replay of history lines. Emit wrapped continuations without a CRLF so xterm.js and vt re-wrap them. Add a test with a 100-column line in an 80-column grid, through history and through the rebuild. | Seen in R-02; cosmetic but visible every time |
| DEBT-03 replay fidelity | IN-06: emit the mode loop before the screen text in `renderScreenLocked` as well as before the CUP; WR-08 already moved it before the CUP. IN-08: probe the G1 designation and the SO state, and re-emit them. Track an open OSC 8 link via a vt callback or a probe, and re-open it around the seed. Extend the fuzz generator with SO/SI, G1 designation and OSC 8, and keep it at 0 mismatches. | Deferred from review iterations |
| DEBT-04 snapshot store | Bound it by bytes (e.g. 8 MiB total) while keeping the entry cap. Key by host plus session. Delete on tab close (find the tab-close path in App.tsx or the tab store; a small hook or event is fine). W2: dispose the xterm even if the flush callback never fires (timeout fallback). IN-03: a pending-save promise that the next mount awaits, briefly (≤ 250 ms), before dialling. | Memory and correctness in the GUI |
| DEBT-05 partial paste | When sendInput splits a paste and the socket closes before every frame is sent, show a non-modal notice in the tab ("Paste interrupted: N of M KB sent"). Don't queue and resend: resending after a reconnect risks a double paste into a shell, which is worse. Make sure the frames already sent count as sent. Small UI: reuse an existing banner or toast component (house component over a new one). | Honest; no double input |
| DEBT-06 launch blip | Cold-launch the GUI in a VM after a reboot and capture the health error the 07-03 log line now prints. If it reproduces, fix the root cause (likely the App's long-lived client created before the socket exists, or a keep-alive or connection-pool issue) and keep the grace as a safety net. If it doesn't reproduce in 5 cold launches, record that in 08-VM-CHECKS.md and leave the diagnostics in place. | The root cause was never seen |
| DEBT-07 bookkeeping | Add `requirements_completed` to the frontmatter of 07-01..05 SUMMARY. Run the security audit for Phase 7 (gsd-security-auditor against the Phase 7 threat models) and write 07-SECURITY.md. | Audit hygiene |

### Claude's Discretion
How to split the plans (few plans, parallel where files don't overlap), test names, and thresholds within the bounds above.
</decisions>

<code_context>
- internal/relay/hub.go: newLiveEmulator, the rebuild paths (resize, alt exit), renderScreenLocked, stripRingQueries, history push.
- vt module source under `go env GOMODCACHE` (charmbracelet/x/vt): SetScrollbackSize, Scrollback, charset state.
- frontend/src/lib/peerSnapshots.ts, relayClient.ts (sendInput, encodeInputFrames); frontend/src/components/TerminalPanel.tsx; App.tsx (tab close).
- app.go (startDaemon, respawnDaemonIfDown, the launch log line), internal/daemon/client.go.
- TESTING.md standing convention.
</code_context>

<specifics>
- Toolchain: `export GOTOOLCHAIN=go1.26.11`. Gates: `go build ./... && go vet ./...`, `go test -race -short ./...` (no -skip), `cd frontend && pnpm vitest run`, `bash tests/check-brand.sh`, `bash tests/check-traceability-paths.sh`. Restore `frontend/src/wailsjs` after a Wails build.
- Run the full suites once per plan at most. Each new test binary raises a LuLu firewall alert on the user's Mac. The WR-07 tailnet guard keeps tests off the tailnet; keep it.
- VM checks: Tart VMs portty-test 192.168.64.108 and portty-test-2 192.168.64.109; method as in 07-VM-CHECKS.md. Never sign with the host Developer ID. Fault injection only inside the VMs.
- Leave token-gated share links, the CSP and the 32 KiB input limits untouched.
- Worktrees may start at an old commit; fast-forward to gsd/phase-07-hardening first.
</specifics>

<deferred>
- Update-notification click handler (needs a notarized build).
- Two-Mac M-84/M-85 and first-release M-93.
- IN-01 (removing the launch grace): kept as a safety net by decision.
</deferred>
