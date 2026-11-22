---
phase: 12-file-path-click
plan: 07
subsystem: testing
tags: [testing, vm, tart, checkpoint, path-click]
status: ended-at-checkpoint-fix
requires:
  - phase: 12-file-path-click
    provides: "12-01..12-06 path click end to end"
provides:
  - "TESTING.md Sections 2 and 4 for Phase 12 (bd7fcf9c)"
  - "12-VM-CHECKS.md with every row run on the two Tart VMs"
affects: [12 gap plan, 12-07 re-run]
actuals:
  tasks: 3
  commits: 7
plan_head_before: bcabec11
plan_head_after: 7fe40d50
---

# 12-07 Summary: ended at Task 3 with "fix"

Tasks 1 and 2 ran. At Task 3 the user chose **fix** (2026-11-21, chosen from the
checkpoint options: "fix (Recommended)"). Per the plan, Task 4 did not run:
TESTING.md Section 5 (M-128..M-135) and the security review's verdict wait for
the re-run of 12-07 after the gap plan.

**Route:** `/gsd-plan-phase 12 --gaps`, then execute the gap plans, then re-run 12-07.

## Gate (Task 1, once, 01:13 to 01:17 UTC)

- Go build and vet clean; `go test -race -short -count=1 ./...` 17 ok, 0 FAIL, DATA RACE or panic.
- vitest 106 files / 1576 tests; tsc clean; check-brand, check-share-removal, check-traceability clean; build-script test 40/0.
- Skip list: the run is not `-v`, so the log has no `--- SKIP` lines; 12-02 recorded no TestZshShim skip on this Mac.
- TESTING.md counts: Go 162, vitest 106, Playwright 7, build-script 1, total 276. Section 4: 4 PATH-01 and 13 PATH-02 rows.

## Builds (all adhoc-signed; VM sha256 equals host)

| App | Version | sha256 |
|---|---|---|
| A (pre-phase dfff923d) | v1.2.0-vmA | 0341d0f803bbb353fe610db1bf05c75efa2f34b92fd4485209b9c6c5f2f04a1f |
| B | v1.2.0-vmB | 1dedd51f190b0c7d2a840d4e7f35edfab1a8373efd98c2055e613d18a4298d1f |
| B2 | v1.2.0-vmB2 | 84df0188281d361db9c78da3100aac014c214c98f4fb69365084e41ea820b0cb |
| HEAD | dev | bfddb9cfedf1be1697464e93c148dadb138c85b827bde3c94f968dcb980feceb |

## Rows (evidence in 12-VM-CHECKS.md)

| Row | Result | Note |
|---|---|---|
| C-00 | PASS | setup |
| C-01 | PASS | local Cmd-click previews; plain click inert; URL confirm popover |
| C-02 | FAIL | **open, defect:** desktop peer tab never sends a path click (viewer side). Likely: peer tab gets `pluginConfig` undefined, `/api/plugin-config` cross-origin fetch has no CORS header, `webLinks` stays falsy, `registerPathLinks` never runs. Predates Phase 12 (build A same) |
| C-03 | PASS | Safari previews and switches; others get background tabs |
| C-04 | FAIL | **open:** only the peer-tab notice clause, blocked by C-02 |
| C-05 | PASS | read-only viewer refused |
| C-06 | FAIL | **open:** app underline unobservable in the VM (blank web-view screenshots). Mechanism works in Safari; needs one look on a real Mac. Fallback remedy in UI-SPEC |
| C-07 | PASS | vim `mouse=a` Cmd-click previews (TUI outcome: preview). Claude Code not installed in VMs |
| C-08 | FAIL | **open, rig limit:** `ssh localhost` cannot authenticate (no self key; copying the rig key refused). Every other clause passed for both dotfile setups |
| C-09 | FAIL | **open:** after widening the window, Cmd-clicks sent nothing for a while, then recovered. Wrap results: soft wrap both rows PASS, history PASS, identical hard row above: rows 1-2 preview, hard row `not-printed`; real newline split: neither sends (accepted limit) |
| U-01 | FAIL | **open, defect:** host side right (A→B handoff, 3/3 `mode=delta` in 1.0-1.9 s; B→B2 1.6 s, all delta; cwd log moved with the session; u1 relative click `no-cwd` as designed). Surviving viewer lost path links after B→B2; fresh page works. Same symptom once in C-03 and C-09 |

## Gaps for the gap plan

1. C-02/C-04: path links in desktop peer tabs (remote web tab plugin config).
2. U-01 (and C-03/C-09 instances): viewer loses path links after reconnect, upgrade or grid change.
3. C-08: decide an `ssh localhost` method for the rig, or record the clause as rig-limited.
4. C-06: real-Mac check of the Cmd-held underline.

Security review verdict: not written (Task 4 did not run).
