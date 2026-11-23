---
phase: 12-file-path-click
plan: 09
subsystem: testing
tags: [testing, vm, tart, checkpoint, path-click, security-review, gap-closure]
requires:
  - phase: 12-file-path-click
    provides: "12-07 first VM run (12-VM-CHECKS.md) and 12-08 viewer gap fixes"
provides:
  - "12-VM-CHECKS.md re-run section: gate, builds C, C2, HEAD, rows C-00R..C-07R, C-09R, U-01R, cleanup, Task 3 decisions"
  - "TESTING.md Section 5 Category BB, M-128..M-135"
  - "12-SECURITY-REVIEW.md Gap closure (12-08), VM evidence, residual risks and verdict (APPROVED)"
  - "Two todos for the findings F-1 and F-2"
affects: [phase-12-verification, phase-12-ship]
requirements-completed: [PATH-01, PATH-02]
key-decisions:
  - "Phase 12 approved on the re-run: all 10 re-run rows PASS; C-02, C-04, C-09 and U-01 superseded by their R rows (user, 2026-11-22: approve)"
  - "UI-1 (C-08 ssh localhost) and UI-2 (C-06 desktop underline and dark repaint) are manual checks for the user's own Macs (M-133, M-131); the Cmd-held underline stays, no hover fallback"
  - "F-1 (row 0 start directory) and F-2 (first Cmd-click after returning to a changed same cell) are residual risks and todos, not blockers"
  - "G-1 (internal/pty TestKillClean_NoOrphans, once, under load) accepted as an unrelated timing flake"
status: complete
completed: 2026-11-22
commits: 10
plan_head_before: 3a8972a6419e59597d9599a8cfa4f5c77dbded88
plan_head_after: 264a983c7cc37880a278d32e7a58ddd6924df90a
actuals:
  tokens: 22000
  tasks: 4
  commits: 10
---

# Phase 12 Plan 09: Re-run, user decisions and verdict Summary

The merged phase (12-01..12-08) passed a real-machine re-run on two Tart VMs over the tailnet, the user approved it and accepted three named limits as checks for their own Macs, and the security review now carries the 12-08 gap-closure section, the residual risks and an APPROVED verdict.

## Commits

`commits: 10` is measured (`git rev-list --count 3a8972a6..HEAD` before this SUMMARY's own commit); `plan_head_after` is Task 4's commit.

| Task | Commit | Message |
|------|--------|---------|
| 1 | 816eeda5 | docs(12-09): re-run header, gate and builds |
| 2 | c8e26e76 | docs(12-09): re-run rows C-00R, C-01R, C-02R |
| 2 | 9e64a877 | docs(12-09): re-run rows C-03R, C-04R |
| 2 | c40b6129 | docs(12-09): re-run row C-05R |
| 2 | 9d9ec921 | docs(12-09): re-run row C-06R |
| 2 | 8dda8965 | docs(12-09): re-run row C-07R |
| 2 | b884a93d | docs(12-09): re-run row C-09R |
| 2 | 610e80ae | docs(12-09): re-run row U-01R |
| 2 | 480f54ef | docs(12-09): re-run cleanup, both VMs back on HEAD |
| 4 | 264a983c | docs(12-09): user decisions, TESTING.md Category BB and security review verdict |

Task 3 was the checkpoint (no commit).

## Gate (host, once, 03:53 to 04:03 UTC)

Host log directory (private, `mktemp -d`): `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.AAqPnuOrmD`; Task 4's traceability log is in `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.iV3hiV18zy`.

- `GOTOOLCHAIN=go1.26.11`: `go build` and `go vet` clean. `go test -race -short -count=1 ./...`: 13 package lines `ok`, **one FAIL: `internal/pty` `TestKillClean_NoOrphans`** (process group still existed 300 ms after Kill, while the 231 s daemon and 123 s relay packages ran in parallel).
- The Go failure ended the gate script, so the remainder ran once afterwards: `pnpm install --frozen-lockfile` clean; vitest 108 files / 1600 tests passed; `tsc --noEmit` clean; `check-brand`, `check-share-removal`, `check-traceability-paths` (199 paths) clean; `tests/build-script.test.sh` 40 passed, 0 failed. `frontend/src/wailsjs` clean afterwards; `frontend/index.html` and `internal/webserver/csp_mw.go` identical to `81ada526`.
- **G-1, user decision 2026-11-22:** accepted as an unrelated timing flake under load. The orchestrator re-ran the test alone with `-count=5` and the full `internal/pty` package once: all `ok`. `internal/pty` has no diff since `81ada526`. Recorded in the gate section of 12-VM-CHECKS.md. Nothing was patched.
- Task 4 re-ran its own checks: every automated check in the plan's verify block passed, and `bash tests/check-traceability-paths.sh` exits 0 (199 paths).

## Builds (ad-hoc signed; the sha256 in each VM equals the host's)

| App | Version | Binary sha256 |
|-----|---------|---------------|
| C | v1.2.0-vmC | `d779668ee89c8c9777dad86a8ab72c9a4d74d75a522a7937598c8016da3f7dbc` |
| C2 (C's tree, version only) | v1.2.0-vmC2 | `b1b1f39aded09c581a415937a42d68d71e73844104acbe466a5038a0199acb62` |
| HEAD (deployed on both at the end) | dev | `4e6452ecb8f06705d994ff6f8690753472b175377c4e88b425cc7133b4dcc09f` |

## Re-run rows (evidence in 12-VM-CHECKS.md)

| Row | Result | Note |
|-----|--------|------|
| C-00R | PASS | both VMs on the tailnet; C on both (`"api":4`); the office VM started on a leftover debug build with one test session, killed by id (recorded) |
| C-01R | PASS | local Cmd-click `outcome=preview`, plain click inert, absolute path, URL popover |
| C-02R | PASS | home peer tab conn=2 `path click … outcome=preview`; the home tab switched, the office got a background tab; the peer tab's URL Cmd-click opened the confirm popover (gap 1 closed) |
| C-03R | PASS | Safari previews and switches; the others get background tabs |
| C-04R | PASS | notices exact and only in the clicking window, now including the peer tab; missing and folder silent, tail sha256 unchanged |
| C-05R | PASS | read-only Safari: `passed=0`, no pointer or underline, hand-sent 0x13 refused; a writable click reached it as a background tab (accepted risk) |
| C-06R | PASS | Safari cursor follows Meta; notice repainted dark in 0.9 s on the same conn |
| C-07R | PASS | vim `mouse=a` click previews, 0 SGR mouse reports, clean quit |
| C-09R | PASS | see below (gap 2 closed) |
| U-01R | PASS | see below (gap 2 closed) |

**C-09R same-point clicks** (office tab, row 2 col 5, no typing and no other pointer move): narrow (85x29 to 57x28) `outcome=preview` 04:47:31; widen (to 83x29) the click that failed in the first run `outcome=preview` 04:47:45; home peer tab claimed the grid (83x29 to 60x29 by=web conn=2) `outcome=preview` 04:49:02; restore (60x29 to 83x29) `outcome=preview` 04:49:21. Wrap rows 0 and 1, history, identical-row and hard-break clauses as specified. Deviation: the plan's Cmd+= does not change the grid (the zoom key is Shift+=), so the home claim was made by resizing its window, which takes the same `why=claim` path.

**U-01R** (C to C2, session u2, office tab conn, home peer tab, a Safari page loaded before the upgrade): handoff committed in 87 ms; `mode=delta` for 3 of 3 viewers at +0.83 s, +1.43 s and +1.62 s after the commit; 0 `viewer-diag: grid` lines. Post-upgrade first clicks with nothing typed and no pointer move: office `outcome=preview` (conn=1), home peer tab `outcome=preview` (conn=2), the same Safari page `outcome=preview` (conn=3).

Cleanup: both VMs back on HEAD, 0 sessions, light appearance, settings.json and dotfile sha256 restored (cmp clean), Safari state scrubbed, no signer or link files left.

## Task 3 answer (user, 2026-11-22, through the checkpoint)

- Verdict: "approve". All 10 re-run rows are PASS. C-02, C-04, C-09 and U-01 are superseded by their R rows.
- G-1: accepted as an unrelated timing flake (see Gate).
- UI-1, C-08 `ssh localhost`: "Check on my Macs". Manual check M-133; not a blocker. First-run C-08 rewritten PASS with this decision.
- UI-2, C-06 desktop underline and dark repaint: "Check on my Macs". The Cmd-held underline stays (no fallback); manual check M-131. First-run C-06 rewritten PASS with this decision; the Safari half is C-06R.
- F-1 (relative path on row 0 of a never-scrolled session resolves against the start directory) and F-2 (first Cmd-click fails after the pointer returns to the same cell whose text changed): "Todos, not blockers". Listed as residual risks in the security review and in TESTING.md (M-128, M-134), with todos `.planning/todos/pending/2026-11-22-row0-relative-path-start-dir.md` (relay) and `.planning/todos/pending/2026-11-22-first-cmd-click-same-cell-after-leave.md` (frontend).
- C-07, Claude Code inside a session: a check for the user's own Macs (M-132).

## Verdict (12-SECURITY-REVIEW.md)

**Verdict:** APPROVED. C-04R PASS (notices only to the clicker, peer tab included), C-05R PASS (a read-only viewer gets no link and its hand-sent click is refused), C-07R PASS (a TUI click previews and types nothing) and C-08 PASS (shell startup unchanged; the `ssh localhost` clause accepted as a check for the user's own Macs, user decision 2026-11-22); the PREV-03 amendment holds with the residual risks above.

## Deviations from Plan

- **[Rule 3 - Blocking] Category BB position.** The plan places Category BB after M-127 and before the `---` above Section 6; Category BC (M-136, a separate debug session) already sits there, so BB was inserted between BA and BC to keep the numeric order. Commit 264a983c.
- **Rig notes** from Task 2 (Cmd+= versus Shift+=, the leftover office build, the Safari WebDriver restart) are in 12-VM-CHECKS.md; none changes a result.
- No product code changed; `requirements.mark-complete` and ROADMAP success criteria were not touched, per the plan. `state.record-session` skipped per the dispatch.

**Total deviations:** 1 (Rule 3). **Impact:** none on content.

## Known Stubs

None.

## Not done or left open

- The desktop app's Cmd-held underline and dark repaint (M-131), `ssh localhost` (M-133) and Claude Code inside a session (M-132) are unverified in the VMs and left to the user's own Macs, by the user's decision.
- F-1 and F-2 are open todos.
- `requirements.mark-complete` not run (plan rule); PATH-01 and PATH-02 stay for the phase verifier.

## Threat Flags

None. No product code, endpoint, header or auth path changed in this plan.

## Self-Check: PASSED

All ten commits above are ancestors of HEAD; 12-VM-CHECKS.md, TESTING.md, 12-SECURITY-REVIEW.md and both todo files exist and are committed; the plan's automated verify block and the traceability gate pass.
