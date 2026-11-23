---
phase: 15-v1-2-hardening
review: 15-REVIEW.md
fix_report: 15-REVIEW-FIX.md
updated: 2026-11-23
---

# Phase 15 — Review Disposition

One row per finding from the final review (iteration 3), plus the warnings fixed in earlier iterations.

| Finding | Severity | Disposition | Evidence / Reason |
|---------|----------|-------------|-------------------|
| WR-01 (iter 1): socket connectable between bind and chmod; 0700 dir not enforced | warning | fixed | f6925dbb (DefaultSocketPath tightens config dir to 0700) |
| WR-01 (iter 2): process-wide umask leaks into forked shells / concurrent mkdir | warning | fixed | cc7e67fb (umask removed; Listen → Chmod 0600, fail closed) |
| WR-01 (iter 3): comment cited a peer-UID check that the control socket lacks; 0700 parent assumed not enforced | warning | fixed (comment) + accepted (no fail-closed parent check) | 9a277a74 corrects the comment. A hard fail on a non-0700 parent was not added: the production path creates/tightens the dir to 0700, and failing starts on custom socket paths or test dirs costs reliability on a remote Mac nobody can reach |
| IN-01: reaped guard no longer signals leftover background jobs of an exited shell | info | accepted | Intended by 15-02 (never signal a possibly reused pgid); leftover jobs were orphaned to launchd already |
| IN-02: duplicate `killSession` branches (`reaped` vs `cmd == nil`) | info | fixed | 97611082 (16-02): one merged early return, IN-01 sentence added to the doc comment |
| IN-03: ED 3 `History.Skip` counts main-screen scrollback while alt screen is active | info | fixed | 0af0cd46 (16-02): IsAltScreen guard on History.Skip, `TestCwd_AltScreenScrollbackEraseKeepsRows` |
| IN-04: non-Portty tailnet peers refusing the share port read as "Nothing shared" | info | accepted | Known limit of 15-07 decision; sidebar only lists probe-marked Portty peers |
| IN-05: notification delegate registered after launch; cold-launch click unproven | info | open | Covered by TESTING.md M-152 (must include the relaunch step) at /gsd-verify-work |
| IN-06: `joinPasteViewer` copies `dialResume` | info | fixed | 97611082 (16-02): `dialResumePing` shared by `joinPasteViewer` |
| IN-07: TestHandoffIT compares B's /health API with the test binary's APIVersion (same-commit assumption) | info | accepted | M-114/M-155 build A, B and the test binary from one commit by procedure |
