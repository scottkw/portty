---
phase: 10-input-path-reliability
plan: 02
subsystem: frontend
status: complete
tags: [peer-tabs, snapshots, retry, sidebar, INPT-04]
requires: []
provides:
  - "webSessionTabId(sessionId, host?) in frontend/src/lib/tabs.ts"
  - "peerTabsRemoved(prev, next) in frontend/src/lib/tabs.ts"
  - "host-qualified peer tab ids in every App lookup; retryInit drops replaced peer tabs' snapshots"
affects: [frontend/src/App.tsx, frontend/src/components/Sidebar.tsx]
tech-stack:
  added: []
  patterns: ["peer tab id host = peerSnapshotHost(tab) (connectHost ?? peerHost), the same value as the snapshot key"]
key-files:
  created: []
  modified:
    - frontend/src/lib/tabs.ts
    - frontend/src/lib/__tests__/tabs.test.ts
    - frontend/src/App.tsx
    - frontend/src/components/Sidebar.tsx
    - frontend/src/components/__tests__/App.open-remote.test.tsx
    - frontend/src/components/__tests__/TerminalPanel.tabSwitch.test.tsx
    - frontend/src/components/__tests__/App.daemon-recovered.test.tsx
    - frontend/src/components/__tests__/Sidebar.test.tsx
decisions:
  - "Peer tab id is __websession__<host>/<sessionId>, host = peerSnapshotHost(tab) (MagicDNS name, else hostname); the hostless web bootstrap tab keeps __websession__<sessionId>"
  - "The connect-failed peer tab now carries connectHost, so the re-click's retry finds the same tab id"
  - "Sidebar highlight compares the tab's peerHost with the row's peer hostname (as planned), plus the session id"
requirements-completed: [INPT-04]  # key renamed by Phase 16 (16-03) so summary-extract reads it; 10-VERIFICATION.md passed
metrics:
  duration: ~45m
  completed: 2026-11-20
gate: blocked-go-test-pty-exhaustion
actuals:
  tokens: 7450
  tasks: 2
  commits: 4
plan_head_before: d2998ad2
plan_head_after: f5e10b7a
---

# Phase 10 Plan 02: Peer tab identity by host plus session Summary

Peer tabs are now keyed `__websession__<host>/<sessionId>` (host = the tab's snapshot host), so two Macs that hand out the same session id open two tabs with their own snapshot keys and resume offsets, and Retry drops the snapshot of any peer tab it replaces.

## What changed

- `frontend/src/lib/tabs.ts`: `webSessionTabId(sessionId, host?)` and `peerTabsRemoved(prev, next)`, both pure and unit-tested.
- `frontend/src/App.tsx`:
  - The local session-only `webSessionTabId` is gone.
  - `openWebSessionTab` builds its id with `webSessionTabId(sessionId, peer ? peerSnapshotHost(peer) : undefined)`.
  - `handleOpenRemoteSession` computes `connectHost` first and looks the tab up by `webSessionTabId(session.id, connectHost)`. `connectingRef` is keyed by that tab id. The connect-failed call now passes `connectHost` as well.
  - `dropPeerSession` matches `type === 'web-session' && peerHost && sessionId`.
  - `<WebShareSessionView key>` is now `${activeId}:${wsCapToken}`.
  - `retryInit` drops the snapshot of every tab `peerTabsRemoved(tabsRef.current, restoredTabs)` returns, before `setTabs`.
- `frontend/src/components/Sidebar.tsx`: a peer row is `aria-current` only when both the session id and the host (`g.peer.hostname === activeTab.peerHost`) match.

## Final tab id format

- Peer tab: `__websession__<peerSnapshotHost(tab)>/<sessionId>`. Example: `__websession__office-air.tail1234.ts.net/sess-remote-1`.
- Web-mode bootstrap tab (no host): `__websession__<sessionId>`, unchanged. The render branch's `activeId.slice('__websession__'.length)` fallback only runs for this tab.

## Tests (for TESTING.md Section 4, copied in by 10-07)

| File | New case names |
|------|----------------|
| frontend/src/lib/__tests__/tabs.test.ts | `webSessionTabId (INPT-04 peer tab identity)`: "a peer tab id carries the host and the session id", "the hostless web bootstrap tab keeps the session-only id", "two hosts with one session id give two ids"; `peerTabsRemoved (INPT-04 Retry cleanup)`: "returns the peer tabs the next list lacks", "returns nothing for local tabs or the hostless web bootstrap tab", "returns nothing when every peer tab stays" |
| frontend/src/components/__tests__/App.open-remote.test.tsx | `App — two peers with the same session id (INPT-04)`: "same session id on two hosts opens two tabs with two snapshot hosts; re-clicking A only activates A", "share stopped on B marks only B's tab: re-clicking A still only activates A", "a MagicDNS peer whose connect fails, then succeeds on a re-click, has exactly one tab" |
| frontend/src/components/__tests__/TerminalPanel.tabSwitch.test.tsx | "two hosts with one session id each rejoin with their own delta (resume=104 for host-a, resume=204 for host-b)" |
| frontend/src/components/__tests__/App.daemon-recovered.test.tsx | "Retry that replaces a peer tab opened while it awaited drops that tab's snapshot (takePeerSnapshot is undefined)" |
| frontend/src/components/__tests__/Sidebar.test.tsx | "with two hosts listing the same session id, aria-current marks only the active host's row" |

## Source pins that changed (App.open-remote.test.tsx)

- The FIX-03 tab-id pin was `webSessionTabId(sessionId)` and is now `webSessionTabId(sessionId, peer ? peerSnapshotHost(peer) : undefined)`. It still checks that the id is keyed by this call's own arguments.
- The CR-03 key pin was ``key={`${wsSessionId}:${wsCapToken}`}`` and is now ``key={`${activeId}:${wsCapToken}`}``. It still checks that the view is keyed per tab with the cap included.
- The double-click case's selector was `tabpanel-__websession__sess-remote-1` and is now `tabpanel-__websession__office-air.tail1234.ts.net/sess-remote-1`.
- In Sidebar.test.tsx, the existing case "aria-current marks the peer row of an active web-session tab, and only that row" now gives its active tab a `peerHost`. The highlight requires one now.

## TDD record

- Task 1 RED (`9a45019a`): 8 new or updated cases failed as expected. The per-host rejoin case in TerminalPanel.tabSwitch already passed at HEAD, as the plan predicted, because TerminalPanel already keys snapshots by `snapshotHost`. The failed-then-ok case also passed at HEAD, because the old id was session-only. Under the new ids it guards the `connectHost` passing.
- Task 1 GREEN (`d9007e74`): all 64 cases in the three files pass. tsc is clean.
- Task 2 RED (`1d622876`): the peerTabsRemoved cases, the Sidebar two-hosts case and the Retry case failed. The Retry case failed on `takePeerSnapshot` still returning the snapshot.
- Task 2 GREEN (`f5e10b7a`): the four touched files pass (105 cases). tsc is clean.

## Gate (run once)

**Result: BLOCKED. The Go suite failed for an environmental reason (PTY pool exhausted on the host), not because of this plan's diff. Per RULE 0 it was stopped and reported, and not re-run.**

| Step | Result | Output file |
|------|--------|-------------|
| `pnpm vitest run` (full) | PASS: 95 files, 1316 tests | /tmp/p10-02-vitest.log |
| `pnpm exec tsc --noEmit` | PASS | /tmp/p10-02-tsc.log |
| `go build ./...` / `go vet ./...` | PASS / PASS | /tmp/p10-02-build.log, /tmp/p10-02-vet.log |
| `go test -race -short -count=1 ./...` (once) | **FAIL** in `portty` (root), `internal/daemon` and `internal/pty`. 10 packages ok, including relay, webserver and attach | /tmp/p10-02-gate-go.log |
| check-brand / check-traceability | PASS / PASS (163 paths) | /tmp/p10-02-brand.log, /tmp/p10-02-trace.log |
| greps: peerTabsRemoved=1, dropPeerSnapshot( calls=2, takePeerSnapshot in Retry test=3 | PASS | |
| `git diff --quiet d027e86d -- frontend/src/wailsjs internal/webserver/csp_mw.go` | PASS (unchanged) | |

**Go failure analysis (read-only, nothing re-run):**
- 139 lines in the log say `create session: open pty: device not configured` (ENXIO from openpty).
- The other failures trace to the same cause. The `api_test.go` "decode create response: invalid character 'c'" errors come from decoding that same error text. TestAPIListSessionsHostname fails with "expected at least 1 session" because its create failed.
- The host's limit is `kern.tty.ptmx_max` 511, and 527 `/dev/ttys*` nodes existed after the run. The PTY pool was exhausted during the run.
- Likely causes: the parallel Phase 10 executors' Go runs (10-01 loops the daemon package with `-count=20`) and other PTY holders on the host. This plan changes no Go files.
- Not fixed or retried: freeing PTYs would mean touching processes outside this worktree.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Stubbed WebShareSessionView in App.daemon-recovered.test.tsx**
- **Found during:** Task 2 RED
- **Issue:** The real view opened by the peer row hit `ReferenceError: EventSource is not defined` in jsdom. The ErrorBoundary contained it, but it put noise into the run.
- **Fix:** Added the same kind of `vi.mock('../WebShareSessionView')` stub that App.open-remote.test.tsx uses.
- **Files modified:** frontend/src/components/__tests__/App.daemon-recovered.test.tsx
- **Commit:** 1d622876

**2. [Rule 2 - Coverage] Extra INPT-04 assertions beyond the plan's behavior list**
- A "share stopped on B marks only B's tab" App case. This covers the plan's must-have truth that share-stopped marking matches host plus session.
- The per-host rejoin case also checks that host-a's own snapshot survives host-b's mount (offset 160). For that, it sends an offset meta after the reconnect delta, because TerminalPanel stores nothing without a known resume point.

**3. [Rule 3 - Blocking] Installed locked frontend deps in the worktree**
- The worktree had no `frontend/node_modules`. Ran `pnpm install --frozen-lockfile --offline`: 129 packages from the lockfile and the local store, nothing downloaded and no new packages.

## Threat surface

- T-10-02 (mitigated): tab ids and snapshot keys both carry the host. Host A's content never restores into host B's tab (two-peers App test and per-host rejoin test).
- T-10-04 (mitigated): Retry drops the snapshots of the peer tabs it replaces (Retry test).
- T-10-03 (accepted, unchanged): the daemon's `RemoteCapStore` and proxy route are still keyed by session id alone. No Go change.
- No new network endpoints, auth paths or CSP changes. The wailsjs stubs are untouched.

## Known Stubs

None.

## Unresolved

- The `go test -race -short ./...` gate must be re-run once the host has free PTYs. That needs a quiet host: no parallel executors' Go suites running.

## Self-Check: PASSED (files and commits)

- FOUND: frontend/src/lib/tabs.ts (webSessionTabId, peerTabsRemoved)
- FOUND commits: 9a45019a, d9007e74, 1d622876, f5e10b7a (d2998ad2..HEAD = 4)
- The gate's Go step is BLOCKED (see Gate). That is an environment block, not a missing artifact.

## Gate re-run (orchestrator, after rebase onto 10-03 at 5d363351)

The first Go gate failed only on host PTY exhaustion (`open pty: device not configured`) caused by a concurrent `-count=20` daemon run. After the rebase, one re-run of each gate: `go build/vet` ok; `go test -race -short -count=1 ./...` rc 0 (scratchpad p10-02-regate-go.log); vitest 95 files / 1319 tests passed; tsc rc 0.
