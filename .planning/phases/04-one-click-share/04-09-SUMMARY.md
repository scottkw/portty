---
phase: 04-one-click-share
plan: 09
subsystem: share
tags: [go, react, tailscale, magicdns, share-popover, uat-gap]

requires:
  - phase: 04-one-click-share
    provides: 04-08 TESTING.md counts (Go 112, vitest 82, total 202) and the share server, popover and browser stop banner from 04-02..04-06
provides:
  - magicDNSLabel in internal/daemon/peer_connect.go, one MagicDNS short-name rule for peers and the share server
  - Share server session info names the host by its tailnet short name, so the browser stop banner matches the GUI and CLI (UAT gap 2)
  - With Tailscale off, a shared session's toolbar Shared button stays enabled and opens the popover, whose switch turns Share off (UAT gap 3, user decision 2026-11-17)
  - 04-UI-SPEC.md reconciled (Share button rows, Interaction Contract, UI Considerations, Checker Notes decision)
affects: [04 UAT re-run (M-79, M-80, M-82), phase 04 verification]

actuals:
  tokens: 8557
  tasks: 2
  commits: 4
plan_head_before: d83b0740a88a70f16dac61f8e67dc263bc456fe7
plan_head_after: b3add8a0d6250c04489e5c9b70699328d3ec75b4

tech-stack:
  added: []
  patterns:
    - "Host labels come from the MagicDNS name's first label (magicDNSLabel); an IP literal stays whole"
    - "Toolbar Share is blocked only for turning Share on: shareDisabled = tailscaleOff && !activeShared"

key-files:
  created: []
  modified:
    - internal/daemon/peer_connect.go
    - internal/daemon/share.go
    - internal/daemon/share_test.go
    - frontend/src/App.tsx
    - frontend/src/components/TabBar.tsx
    - frontend/src/components/__tests__/App.tailscale.test.tsx
    - frontend/src/components/__tests__/App.test.tsx
    - .planning/phases/04-one-click-share/04-UI-SPEC.md
    - TESTING.md

key-decisions:
  - "The share server's session resolver returns magicDNSLabel(h.Domain) (the tailnet certificate domain's first label) instead of the engine's OS hostname; the OS hostname is no longer exposed to link holders"
  - "User decision 2026-11-17 (UAT gap 3): Button opens when shared. With Tailscale off, App passes shareDisabled={tailscaleOff && !activeShared}; unshared sessions stay aria-disabled and the daemon still refuses share-on with 503 tailscale_off"
  - "matchPeer keeps its own inline label match; only peerShortName moved to magicDNSLabel, so peer matching is byte-for-byte unchanged"

patterns-established:
  - "Mounted App tests can stub TerminalPanel with vi.mock('../TerminalPanel') to exercise toolbar and popover flows in jsdom"

requirements-completed: [SHAR-04, SHAR-03, SHAR-01]

coverage:
  - id: D1
    description: "The share server's /api/sessions/{id}/info returns the host's tailnet short name (first label of the certificate domain), not the OS hostname, so the browser's stopped-sharing banner names the host like the GUI and CLI"
    requirement: SHAR-04
    verification:
      - kind: integration
        ref: "internal/daemon/share_test.go#TestShare_WebInfoHostIsTailnetName"
        status: pass
      - kind: unit
        ref: "internal/daemon/share_test.go#TestMagicDNSLabel"
        status: pass
      - kind: unit
        ref: "internal/daemon/peer_connect_test.go#TestPeerConnect_HostMatching"
        status: pass
      - kind: unit
        ref: "internal/daemon/peer_connect_test.go#TestPeerConnect_FQDNPicksTheRightMac"
        status: pass
    human_judgment: false
  - id: D2
    description: "With Tailscale off, a shared session's Shared button opens the popover, which shows the NET-01 reason with the switch on and enabled, and the switch calls SetSessionShared(id, false); an unshared session's Share button stays aria-disabled and opens nothing"
    requirement: SHAR-01
    verification:
      - kind: automated_ui
        ref: "frontend/src/components/__tests__/App.tailscale.test.tsx#App — Tailscale off: the toolbar Share button (mounted)"
        status: pass
      - kind: unit
        ref: "frontend/src/components/__tests__/App.test.tsx#TabBar toggles the Share popover and gets the share and NET-01 state"
        status: pass
      - kind: integration
        ref: "internal/daemon/share_test.go#TestShare_TailscaleOff"
        status: pass
    human_judgment: false
  - id: D3
    description: "Live behavior on real Macs: the browser banner on a real share link names the tailnet host (M-79/M-80), and disconnecting Tailscale with a shared session lets the popover switch stop sharing (M-82)"
    requirement: SHAR-03
    verification:
      - kind: manual_procedural
        ref: "TESTING.md#M-79, M-80, M-82"
        status: unknown
    human_judgment: true
    rationale: "Needs the real Tailscale client toggled on a Mac, a real browser on the built share server and two Macs on a tailnet; jsdom and loopback tests cannot cover them"
  - id: D4
    description: "04-UI-SPEC.md gives one answer for the Tailscale-off Share button (table rows, Interaction Contract, UI Considerations) and records the user decision; TESTING.md rows and M-items match"
    verification:
      - kind: other
        ref: "grep acceptance criteria in 04-09-PLAN.md Task 2; bash tests/check-traceability-paths.sh"
        status: pass
    human_judgment: false

duration: 14min
completed: 2026-11-17
status: complete
---

# Phase 4 Plan 09: Tailnet host name in the browser banner and Tailscale-off Share-off Summary

**The share server now names the host by its MagicDNS short name (one `magicDNSLabel` rule shared with `peerShortName`), and with Tailscale off a shared session's Shared button opens the popover so its switch can stop sharing.**

## Performance

- **Duration:** about 14 min
- **Started:** 2026-11-17T19:33:00Z (approximate)
- **Completed:** 2026-11-17T19:47:01Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments
- UAT gap 2: `/api/sessions/{id}/info` returns `office-air`-style tailnet short names instead of `Kens-MacBook-Air.local`, so the browser's `<host> stopped sharing this session.` matches the GUI and CLI. In the VM build the share domain is `127.0.0.1` and stays whole.
- UAT gap 3: with Tailscale off, the toolbar Shared button stays enabled for a shared session and opens the popover (reason shown, switch on and enabled); turning it off calls `SetSessionShared(id, false)`. Unshared sessions are unchanged: aria-disabled, `aria-describedby="ts-off-reason"`, no popover. Turning Share on is still blocked in the popover and refused by the daemon.
- 04-UI-SPEC.md and TESTING.md give one answer and record the decision.

## Task Commits

1. **Task 1 (tracer): share info host is the tailnet short name**
   - RED `46ddc6ac` (test): TestMagicDNSLabel, TestShare_WebInfoHostIsTailnetName, plus a compile stub
   - GREEN `95e4fdd5` (feat): magicDNSLabel, peerShortName uses it, resolver returns magicDNSLabel(h.Domain)
2. **Task 2: Tailscale-off Shared button opens the popover**
   - RED `9eaed899` (test): mounted App cases and the new shareDisabled source assertion
   - GREEN `b3add8a0` (feat): App activeShared and shareDisabled rule, TabBar docs, App.test.tsx pin, 04-UI-SPEC.md, TESTING.md

No REFACTOR commits; nothing needed cleanup.

## TDD Gate Compliance

- **Task 1 RED:** `go test -count=1 ./internal/daemon -run '^(TestMagicDNSLabel|TestShare_WebInfoHostIsTailnetName)$' -v` exit 1. The first run did not compile (`undefined: magicDNSLabel`, INVALID_RED build_failure), so a `return name` stub was added in the RED commit. Rerun: TestMagicDNSLabel failed on its FQDN cases and TestShare_WebInfoHostIsTailnetName failed with `info hostname = "Kens-Personal-MacBook-Air.local"; want ... "office-air"`. Classifier: `RED_EVIDENCE_OK` (target_test_failed, go-test format). Semantic assessment: the target ran and failed on the planned hostname assertion for the intended reason (the resolver returned the OS hostname).
- **Task 2 RED:** `pnpm --silent exec vitest run src/components/__tests__/App.tailscale.test.tsx --reporter=tap-flat` against the original App.tsx, exit 1. The shared-session mounted case failed at line 167 (`aria-disabled` present: expected true to be false) and the source assertion failed on the new expression. The unshared case passed, as expected for unchanged behavior. Classifier: `RED_EVIDENCE_OK`. Semantic assessment: intentional RED on the planned assertion. Note: the App.tsx edit was drafted before the test; it was reverted for the RED run with `git checkout -- frontend/src/App.tsx` and reapplied from a saved patch after the RED commit.
- **GREEN:** both tasks' tests pass (see Verification).

## Files Created/Modified
- `internal/daemon/peer_connect.go` - `magicDNSLabel` (trim trailing dot, IP literal stays whole, else first label); `peerShortName` uses it with its HostName fallback
- `internal/daemon/share.go` - session resolver returns the tailnet short name computed once from `h.Domain`
- `internal/daemon/share_test.go` - `TestMagicDNSLabel`, `TestShare_WebInfoHostIsTailnetName`
- `frontend/src/App.tsx` - `activeShared`; `shared={activeShared}`, `shareDisabled={tailscaleOff && !activeShared}`
- `frontend/src/components/TabBar.tsx` - shareDisabled prop doc and Share button comment reworded (no logic change)
- `frontend/src/components/__tests__/App.tailscale.test.tsx` - header, updated source assertion, mounted Tailscale-off cases with a TerminalPanel stub
- `frontend/src/components/__tests__/App.test.tsx` - TabBar props pin follows the new expression
- `.planning/phases/04-one-click-share/04-UI-SPEC.md` - two Tailscale-off button rows, Interaction Contract, UI Considerations, Checker Notes decision line
- `TESTING.md` - Section 2 note (counts unchanged at 202), two SHAR-04 rows, NET-01 row reworded, M-82/M-79/M-80 updated

## Decisions Made
- Followed the locked user decision "Button opens when shared" exactly; no special case for an open popover (after turning Share off with Tailscale off, the button becomes aria-disabled, as for any unshared session).
- `matchPeer` was left alone (it has its own inline label match). Only `peerShortName` moved to the helper, as the plan specified, so peer matching is unchanged.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] App.test.tsx pinned the old TabBar props**
- **Found during:** Task 2 (full vitest run)
- **Issue:** `App.test.tsx` "TabBar toggles the Share popover and gets the share and NET-01 state" asserted `shareDisabled={tailscaleOff}` and the inline `shared=` expression; the file was not in the plan's file list.
- **Fix:** the pin now checks `const activeShared = ...`, `shared={activeShared}` and `shareDisabled={tailscaleOff && !activeShared}`; the TESTING.md 04-09 note lists the file.
- **Files modified:** frontend/src/components/__tests__/App.test.tsx, TESTING.md
- **Verification:** full vitest 82 files / 1047 tests pass
- **Committed in:** b3add8a0

**2. [Rule 1 - Bug in the plan's verify command] Task 1 verify counts subtests**
- **Found during:** Task 1
- **Issue:** `grep -c -- '--- PASS: Test' | grep -qx 4` also counts the six `    --- PASS: TestPeerConnect_HostMatching/...` subtest lines, giving 10, so the command fails even when all four tests pass.
- **Fix:** none to code; checked with the anchored `grep -c -- '^--- PASS: Test'`, which printed 4. No test failed.
- **Committed in:** n/a

**3. [Rule 1 - Bug in the plan's verify command] Task 2 vitest grep is color-sensitive**
- **Found during:** Task 2
- **Issue:** `grep -E 'Test Files +3 passed'` does not match vitest's colored output (ANSI codes between the words).
- **Fix:** none to code; reran with `NO_COLOR=1`, which printed `Test Files  3 passed (3)`.
- **Committed in:** n/a

---

**Total deviations:** 3 (1 blocking test pin fixed, 2 verify-command defects worked around without code changes)
**Impact on plan:** No scope creep. The production changes are the ones the plan specified.

## Issues Encountered
- Pre-existing, not caused by this plan (deferred-items.md, 04-08): `TestAPIAppearance_GetPatch` and `TestAPITerminalFont_GetPatch` fail in `go test -race -short ./...` because they read the real settings.json. They are the only failures in the Go suite; every other package and test passes.

## Verification
- `go build ./... && go vet ./... && go vet -tags playwrightfixture ./cmd/playwright-fixture/ && go vet -tags vmpreview ./...`: pass
- `go test -race -short ./...`: pass except the two known settings tests above
- Task 1 tests (TestMagicDNSLabel, TestShare_WebInfoHostIsTailnetName, TestPeerConnect_HostMatching, TestPeerConnect_FQDNPicksTheRightMac): 4/4 pass under -race
- `pnpm -C frontend exec tsc --noEmit`: pass
- `pnpm -C frontend exec vitest run`: 82 files, 1047 tests pass
- `bash tests/check-traceability-paths.sh` (111 paths), `tests/check-share-removal.sh`, `tests/check-brand.sh`: clean
- All acceptance-criteria greps for both tasks pass
- Manual M-82 / M-79 / M-80 left for the end-of-phase UAT

## Threat Flags

None. The hostname field now carries the tailnet short name, which the link holder already sees in the link URL (T-04-36, accepted). T-04-37 is mitigated as planned: only shared sessions get an enabled button, the popover blocks turning Share on, the daemon still answers 503 tailscale_off (TestShare_TailscaleOff passes), and the mounted test checks the switch only calls `SetSessionShared(id, false)`. No package installs; go.mod, go.sum and package.json unchanged. CSP, capability gate and Origin allowlist untouched.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- UAT gaps 2 and 3 are closed in code; re-run M-79, M-80 and M-82 on real Macs at the phase-end UAT.
- Gap 1 (M-83) still depends on the 04-08 daemon.log evidence from a live repro.

## Self-Check: PASSED
- All 9 modified files present; commits 46ddc6ac, 95e4fdd5, 9eaed899, b3add8a0 are ancestors of HEAD; evaluation-scope resolves the plan's commits on this branch.

---
*Phase: 04-one-click-share*
*Completed: 2026-11-17*
