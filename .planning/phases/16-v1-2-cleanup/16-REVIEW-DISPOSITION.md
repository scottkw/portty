---
phase: 16-v1-2-cleanup
review: 16-REVIEW.md
fix_report: 16-REVIEW-FIX.md
updated: 2026-11-23
---

# Phase 16 — Review Disposition

One row per finding from the final review (iteration 2), plus the warnings fixed in iteration 1.

| Finding | Severity | Disposition | Evidence / Reason |
|---------|----------|-------------|-------------------|
| WR-01 (iter 1): kill/rename accept an ID prefix of any length, so `portty kill b` can kill an unrelated session | warning | fixed | 0e08afca (lookupSessionStrict, minDestructivePrefix = 8, kill/rename only; share/unshare/attach unchanged); TestCmdKillRename_ShortPrefixRefused; confirmed by iteration-2 re-review |
| WR-02 (iter 1): empty argument matches a session with an empty name | warning | fixed | c8158c94 (resolveSession rejects ""); TestResolveSession empty-arg case; confirmed by iteration-2 re-review |
| IN-01: exact name shadows an exact full ID | info | open | Contrived (a session renamed to another session's full ID); not in fix scope |
| IN-02: ED 3 in the alternate screen still clears hub history and wraps | info | open | Predates Phase 16; candidate todo for v1.3 |
| IN-03: `arg != ""` check in the prefix pass is now dead code | info | open | Harmless redundancy after c8158c94; not in fix scope |
