---
phase: 14-mobile-web-app
plan: 14
subsystem: verification
tags: [security-review, csp-signoff, testing-md, real-iphone, checkpoint, cleanup]
requires:
  - phase: 14
    provides: plans 14-01..14-13 (gate, pages, key bar, pinch, manifest, fakes, VM rows P-00..P-07, simulator rows S-00..S-08)
provides:
  - 14-SECURITY-REVIEW.md (gate contract, CSP Amendment 4 sign-off, threat register T-14-01..T-14-43, fidelity caveat, evidence and an approved verdict)
  - TESTING.md Section 5 Category BE, M-145..M-151, with last-run lines
  - the user's decisions in their own words, the S-02 accepted limit, and the 14-14 cleanup record
affects: [phase 14 verification, ship]
status: complete
actuals:
  tokens: 19749
  tasks: 3
  commits: 2
plan_head_before: ffa0e7faf60ace2dcdfa49724c8a86b55c9b2d56
plan_head_after: 2d06535bd85f48b8f4d26d6f033e7c30bd10b6ad
tech-stack:
  added: []
  patterns:
    - "evidence plans keep links and tokens out of every written file and delete rig leftovers in every outcome"
key-files:
  created:
    - .planning/phases/14-mobile-web-app/14-SECURITY-REVIEW.md
    - .planning/todos/pending/2026-11-22-linkifier-reconnect-flake.md
  modified:
    - TESTING.md
    - .planning/phases/14-mobile-web-app/14-VM-CHECKS.md
    - .planning/phases/14-mobile-web-app/14-SIM-CHECKS.md
    - .planning/phases/14-mobile-web-app/deferred-items.md
key-decisions:
  - "Decision: accept-limit (the user's 'Approve (Recommended)' answer, which carried one accepted limit, S-02); verdict approved"
  - "CSP Amendment 4 (manifest-src 'self' only) approved; UI-A accepted; UI-B keep uncached; UI-C accepted"
requirements-completed: []  # plan rule: requirements.mark-complete is not run here
---

# Phase 14 Plan 14: Security review, the checkpoint and the phase verdict Summary

The security contract was written and committed before the pause, the user approved the phase (one accepted limit, S-02) with passing real-iPhone results for the Home Screen launch and the sleep and network reconnect, and the review now carries its evidence and an approved verdict.

## Commits

| Commit | What |
|--------|------|
| 9f3935d2 | Task 1: 14-SECURITY-REVIEW.md part 1 (gate contract, check order, CSP delta for sign-off, token storage, lone Esc, fakes, launcher scope, cache basis, fidelity caveat, threat register T-14-01..T-14-43) |
| 2d06535b | Task 3: user decisions, Cleanup (14-14), S-02 rewritten as PASS with the accepted-limit note, TESTING.md Category BE (M-145..M-151), the review's evidence and verdict, the linkifier todo, the deferred-items updates |

## The checkpoint answer (Task 2), in the user's words

**Decision:** accept-limit. The user answered "Approve (Recommended)"; as the question offered it, that is an approval of everything with one accepted limit (S-02), so the recorded word is `accept-limit`.

- **S-02, accepted limit, user decision 2026-11-22:** "the gap is iOS's own form-assist bar (⌃ ⌄ ✓), which iOS shows above the keyboard for any web text field even in the standalone app; Portty's key bar sits directly on top of it (confirmed in the user's real-iPhone screenshot, standalone)".
- **PHONE-1 (M-145), real iPhone, PASS:** the page loaded in "less then 5 sec"; the icon landed "on the home page"; the standalone launch kept the query (full screen, ticks counting, key bar shown). iOS version and pinch not reported.
- **PHONE-2 (MOB-03, M-148), real iPhone, PASS:** "All three worked, no gaps" (10-minute lock, Wi-Fi off and on, 2-minute app switch). Seconds to resume not reported.
- **UI-A:** accepted (the other account is shown with `vmwhois`; tests pin the rest). **UI-B:** keep uncached (82.8 ms estimate). **UI-C:** accept the simulator rows as evidence; the real-phone checks back them up. **CSP Amendment 4:** approved.
- **64 s Safari load (P-03):** a VM Safari effect; the real phone loaded in under 5 s. Kept as a residual note.
- **Linkifier flake:** "Todo to fix later" (filed). **G-1 pty flake:** already accepted. **Firefox 14-01 deferral:** fixed at `9b3e6b2a`, marked resolved.

## Verdict

**Verdict:** approved. P-02 (another account refused on every gated route on real machines), S-04 (the Home Screen launch keeps the query on the finished UI, with PHONE-1 on a real iPhone) and the CSP delta (`manifest-src 'self'` only, approved by the user) are each PASS or approved; S-02 is an accepted limit and UI-A, UI-B and UI-C are accepted in the user's words above (2026-11-22).

## Cleanup (14-14)

- The `phone` share was ended inside the office VM (`portty unshare phone` printed "Stopped sharing"; the first `portty kill` by name failed with "session not found", the kill by id succeeded; `list --local` empty on both VMs).
- Office `~/.zsh_history` and `~/.zshrc` hashes equal P-00's, so no restore was needed. `/health` is `v1.2.0-vm14` on both VMs. The VMs were never stopped; no `tailscale down` or `logout`.
- Deleted from the host: the phone-check directory (with the 0600 link file) and both screenshot directories, after 30 of 30 and 10 of 10 files matched the sha256 lists in 14-SIM-CHECKS.md. The host clipboard is empty.
- Not cleaned (the user's iPhone): the Home Screen icon and Safari's history for the link; the user was asked to remove them.

## Deviations from Plan

**1. [Process] Task 1 and Task 3 ran in separate invocations.** The first run stopped at the blocking-human checkpoint as instructed and handed back; Task 3 ran after the user's answer.

**2. [Rule 1 - Bug in my own first draft] Review wording corrected before commit.** The first draft said the share warning was pinned in `SharePopover.test.tsx` through `SHARE_WARNING` and listed `KeyBar.test.tsx` as pinning T-14-10; neither was accurate (the sentence is pinned in `cmd_share_test.go`, `SharePopover.test.tsx` and `launcher_copy_test.go`; the no-bar-without-write test is `WebShareSessionView.keybar.test.tsx`). Fixed in the file before Task 1's commit. The icon-dies-on-key-regeneration and daemon-restart claims are recorded as the launcher's wording, not exercised.

**3. [Command slip] `portty kill phone` by name failed.** The CLI needs the session id; re-run by id. No effect.

**4. [Decision word] `accept-limit` was chosen over `approve`.** The user's answer carried a named accepted limit (S-02), and Task 3's verify needs the S-02 FAIL rewritten as PASS with the user's words, which is the accept-limit path.

Otherwise the plan ran as written.

## Known Stubs

None. No product code changed.

## Threat Flags

None. No product code was touched. T-14-37 (no link, token, tailnet URL or address in the review, TESTING.md or the decision record: 0 matches in each) and T-14-38 (the phone-check share, its link file and both screenshot directories gone) were mitigated as planned; T-14-SC: no package installed.

## Not verified

- The real iPhone's iOS version, the pinch, Open PDF in the Home Screen app, re-share then add the icon again, and the resume timings were not reported or run.
- Another account on real machines beyond the `vmwhois` rewrite (UI-A, accepted).
- Whether the `pathLinks.linkifier` flake is only a timing flake (it passes alone; the fix is the filed todo).
- The user's real-iPhone screenshot is not in the repository; its content is recorded from the user's report.

## Self-Check: PASSED

- Files present: 14-SECURITY-REVIEW.md, TESTING.md (Category BE, M-145..M-151), 14-VM-CHECKS.md (`## User decisions`, `## Cleanup (14-14)`), 14-SIM-CHECKS.md, deferred-items.md, the linkifier todo.
- Commits 9f3935d2 and 2d06535b are ancestors of HEAD; `git rev-list --count ffa0e7fa..HEAD` was 2 before this SUMMARY's own commit.
- Gates: `tests/check-traceability-paths.sh` OK (228 paths), `tests/check-share-removal.sh` clean, no `| FAIL |` row left, the secret-pattern grep is 0 over the review, 14-VM-CHECKS.md and the Category BE region, the Verdict line names P-02, S-04 and the CSP delta.
