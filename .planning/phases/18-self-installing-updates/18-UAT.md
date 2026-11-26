---
phase: 18-self-installing-updates
status: pending
source: 18-VERIFICATION.md (human_verification)
created: 2026-11-26
---

# Phase 18 UAT: tracked human verification items

Phase 18 was closed on 2026-11-26 by user decision with these items tracked, not passed. Each one needs a real Mac. Run them with `/gsd-verify-work 18`. TESTING.md Category BI holds the full steps.

| # | Item | Status | Where |
|---|------|--------|-------|
| 1 | M-169 parts 1 and 2 on the office Mac. Part 1 is the Replace from a release DMG. Part 2 is clicking Update in the banner at the next release, with sessions and a share link over the real tailnet. ROADMAP SC1's office-Mac half stays unmet until part 2 runs. | pending | TESTING.md M-169 |
| 2 | Real Gatekeeper enforcement on a quarantined, notarized release DMG, plus the relaunch after an update. VM assessments are disabled, and row R4-f-gatekeeper is BLOCKED. | pending | covered by M-169 part 1 |
| 3 | M-170: the one-time Downloads-folder prompt with a Developer ID release (Allow, Don't Allow, no second ask). | pending | TESTING.md M-170 |
| 4 | M-171: Finder Put Back of the old Portty.app in the Trash. | pending | TESTING.md M-171 |

Resolved by decision: the observation 2 / RC-WR02 residual (a late first-install failure gives no message). On 2026-11-26 the user chose a small fix, done as a GSD quick task: the installer posts a native notification, and mover mode shows an earlier failed result.

Tracked separately: row RC-WR04-stall, where both sessions' output stalled for about 17 minutes during a keep-daemon rollback run, cause unknown. An orchestrator debug soak investigates it in parallel. See the STATE.md concerns.
