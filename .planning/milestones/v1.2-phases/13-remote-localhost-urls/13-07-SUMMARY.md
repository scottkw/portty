---
phase: 13-remote-localhost-urls
plan: 07
subsystem: testing
tags: [testing, vm, tart, tailnet, checkpoint, security-review, localhost-proxy]
requires:
  - phase: 13-remote-localhost-urls
    provides: "13-01..13-06: printed check and port token, viewing forward, host grant and proxy routes, API 5 binding, click branch and consent sheet, revoke proofs and the security review parts 1"
provides:
  - "TESTING.md Sections 2 and 4 (Go 168, vitest 110, total 287; 18 URL-01..URL-03 rows) and Section 5 Category BD, M-137..M-144"
  - "13-VM-CHECKS.md: gate, builds A13/B13/HEAD and p13dev, rows L-00..L-08 (all PASS), Cleanup, User decisions"
  - "13-SECURITY-REVIEW.md: VM evidence, residual risks, T-13-36..T-13-41 and the approved verdict"
affects: [phase-13-verification, phase-13-ship]
requirements-completed: [URL-01, URL-02, URL-03]
key-decisions:
  - "Phase 13 approved on the real-machine run: all nine rows L-00..L-08 PASS, none through acceptance (user, 2026-11-22: \"approve (Recommended)\")"
  - "UI-1 (a different tailnet user): \"Accept automated-only\"; covered only by TestPortGrant_Refusals and TestPortProxy_Refusals (other-user rows); TESTING.md M-139"
  - "UI-2 (a real Vite or webpack HMR project): \"Check on my Macs\"; manual check M-140; no node exists in either VM"
  - "G-1 (internal/pty TestKillClean_NoOrphans, one FAIL in the single gate run): recorded as the known G-1 flake; the orchestrator's -count=5 re-run alone passed; internal/pty has no diff since 9574fd49"
status: complete
completed: 2026-11-22
commits: 6
plan_head_before: 2845d22f86fba715926328bc1cfdf22a05104928
plan_head_after: e0efb81bfb71a337abae000829c84d55686c71ee
actuals:
  tokens: 20000
  tasks: 4
  commits: 6
---

# Phase 13 Plan 07: Gate, VM checks, regression home and security verdict Summary

The merged phase passed a real-machine run on two Tart VMs over the real tailnet (rows L-00..L-08 all PASS), TESTING.md lists every Phase 13 test and manual check, and the security review is approved with the VM evidence and residual risks recorded.

## Commits

`commits: 6` is measured (`git rev-list --count 2845d22f..HEAD`); one of the six is an unrelated `docs(12)` commit (f85a804b) made on the branch meanwhile. This plan's five commits:

| Task | Commit | Message |
|------|--------|---------|
| 1 | ef44797d | docs(13-07): TESTING.md Sections 2 and 4 for Phase 13 |
| 1 | 2fc93d2b | docs(13-07): 13-VM-CHECKS.md header, gate result and builds |
| 2 | c288c693 | docs(13-07): VM rows L-00..L-06 |
| 2 | a1f66bee | docs(13-07): VM rows L-07 and L-08 and the Cleanup |
| 4 | e0efb81b | docs(13-07): Category BD, user decisions and the security review verdict |

## Gate (host, once, 17:44:55 to 17:49:13 UTC)

Log directory (private `mktemp -d`): `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.K2lRZqyKkX`. `GOTOOLCHAIN=go1.26.11`, no `-skip` flag.

- Build and vet clean; `pnpm install --frozen-lockfile` clean; vitest 110 files / 1674 tests passed; tsc clean; check-brand, check-share-removal and check-traceability (207 paths) clean; build-script test 40 passed, 0 failed; `frontend/src/wailsjs` clean; `frontend/index.html` and `csp_mw.go` identical to 9574fd49.
- `go test -race -short -count=1 ./...`: 15 package lines ok, **one FAIL: `internal/pty` `TestKillClean_NoOrphans`** (the same flake as Phase 12's G-1, under load beside the 233 s daemon and 123 s relay packages). By the plan's rule it was not patched or re-run on the host; the remaining gate steps and the VM rows ran anyway. User decision: recorded as the known G-1 flake; the orchestrator re-ran it alone with `-count=5` and it passed; `internal/pty` has no diff since 9574fd49.
- Final TESTING.md counts equal `git ls-files`: Go 168, vitest 110, Playwright 8, build-script 1, total 287.

## Builds and helper (ad-hoc, `Signature=adhoc`, `TeamIdentifier=not set`)

| App | Version | Binary sha256 |
|-----|---------|---------------|
| A13 (9574fd49, API 4) | v1.2.0-vm13a | `a78aed0269da297d5945a336d13682c17f95ec4f1f4023b6c5e38b49528b195a` |
| B13 (API 5) | v1.2.0-vm13b | `e1f41a8e41c4cd76fa20a5b10b2f06b40802824b09fb03f5748dd2f817ffff8d` |
| HEAD (left on both VMs) | dev | `a01ae0e147c03409590ef7b7fa8447e036ffba93b12372d0afff44f17d2e0c20` |
| p13dev helper | n/a | `5bf16608e481f748c02d35aa5d4638ef59c13abc1f3d5c02560ff8006061f6c4` |

## Row results (all PASS; evidence in 13-VM-CHECKS.md)

| Row | Result | Key evidence |
|-----|--------|--------------|
| L-00 | PASS | Both VMs on the tailnet; office A13 (api 4), home B13 (api 5); no python3 (no Command Line Tools) and no node |
| L-01 | PASS | Old host: `portty-vm-office needs a Portty update to open its local pages.`, `outcome=host-outdated`; after the handoff upgrade the pre-upgrade URL asked consent |
| L-02 | PASS | Banner `Serving HTTP on :: port 8000 (http://[::]:8000/) ...`; title `p13 site`, assets 200, then `p13 sub` with `?q=a%20b` byte-exact |
| L-03 | PASS | Exact sheet copy; Return and Esc open nothing; `lsof` exactly `127.0.0.1:8001` and `[::1]:8001` |
| L-04 | PASS | Six identical 403s with `X-Portty: 1`; tripwire 0; LAN curl exit 7; 7444 only on the tailnet address |
| L-05 | PASS | Counters rise; reload to `load:2` in about 1.2 s; `::1`-only server works |
| L-06 | PASS | `outcome=revoked` 50 ms after unshare, streams closed under 0.7 s; no-stream window `outcome=stopped` at about 4.1 s after the re-share, nothing proxied |
| L-07 | PASS | `Port 8500 is in use on this Mac.` and `Port 8501 ...`; home servers kept answering |
| L-08 | PASS | Local click direct, popover in both tabs, `https://localhost:8600/` unchanged (the recorded gap) |

Servers used: static `p13dev -static` (no python3 in the VMs), dev `p13dev -dev` (no node, so no Vite or webpack).

## L-06 end times (from the trigger)

| Event | Time |
|-------|------|
| Share-off to `outcome=revoked` (home log) | 50 ms |
| Share-off to WebSocket and SSE closed, listener gone | under 0.7 s (poll granularity) |
| No-stream window: re-share to the page's own retry (`outcome=stopped`) | about 4.1 s |
| That request to the forward closed | 0.6 s |

## Task 3 (checkpoint:decision, gate blocking-human): the user's answers, 2026-11-22

- Verdict: "approve (Recommended)". All nine L-rows PASS.
- G-1: recorded as the known G-1 flake (see Gate).
- UI-1 (a different tailnet user): "Accept automated-only". Covered only by `TestPortGrant_Refusals` and `TestPortProxy_Refusals` (other-user rows).
- UI-2 (a real Vite or webpack HMR project): "Check on my Macs". Manual check M-140.

## Security review verdict

`13-SECURITY-REVIEW.md` "VM evidence and verdict": **Verdict:** approved, citing L-03, L-04 and L-06 (each PASS). Residual risks recorded: the no-stream window, `https://localhost` and `*.localhost` keeping today's behaviour, no deny list for well-known ports, the slower `::1`-only page start seen once in L-05, and UI-1 and UI-2.

## Deviations from Plan

**1. [Process] G-1 gate failure did not stop the run.** The plan says any gate failure is a finding: stop and report. It was reported at Task 3 instead, and the rows ran, as in 12-07, because no row depends on `internal/pty`. Resolved by the user as above.

**2. [Rig incident, cleaned] Command Line Tools prompt.** Running `strings` in the home VM opened the system "Install Command Line Developer Tools" dialog. Nothing was installed or downloaded; its process was killed and no tool needing the CLT was run again.

**3. [Rig] Interactions.** `p12ui` has no key command, so Esc is a typed ESC character; the AX dump has no focused-element attribute, so focus on Don't Allow was shown by Return opening nothing; the first sidebar click with `click2` did not open a tab (a `p12ui` click did). The Phase 12 F-2 todo (a Cmd-click on the same cell twice does nothing) was seen again and avoided by moving the pointer between clicks.

No product code changed. `requirements.mark-complete` was not run (the plan forbids it; the frontmatter lists the ids as covered only).

## Known Stubs

None.

## Threat Flags

None. T-13-36..T-13-41 results are in the review; the host ran only the gate, builds, the helper build and SSH.

## Not verified

- Another tailnet user on real machines (UI-1, automated-only by the user's decision).
- A real Vite or webpack HMR project (UI-2, on the user's own Macs, M-140).
- The Wails web view's focused element for the sheet (shown by behaviour only).

## Self-Check: PASSED

Files present: TESTING.md (Category BD, 18 URL rows), 13-VM-CHECKS.md (rows L-00..L-08 PASS, Cleanup, User decisions), 13-SECURITY-REVIEW.md (VM evidence and verdict). Commits ef44797d, 2fc93d2b, c288c693, a1f66bee, e0efb81b are ancestors of HEAD. Both VMs on HEAD (api 5), settings and dotfiles restored, no worktree registered, `frontend/src/wailsjs` clean.
