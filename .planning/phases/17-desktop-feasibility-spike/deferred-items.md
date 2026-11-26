# Phase 17 deferred items

Out-of-scope discoveries logged by executors. Not fixed by the plan that found them.

## 17-09 found: 17-06's R15, R16 and R17 results sit in the Routing table, not the Rows table

- **Where:** `17-SPIKE-MATRIX.md`. Lines 56 to 64 (inside `## Routing`) hold full result rows for R15, R15-A, R15-B, R16, R16-A, R16-B, R17, R17-A and R17-B. The real `## Rows` entries for R15, R16 and R17 (lines 156 to 158) still read `PENDING`, and the Routing rows that belong there (`| R15 | VM | ... |` and the R16 and R17 equivalents) are gone.
- **Cause (likely):** the 17-06 edit replaced the first `| R15 |` line it found, which is the Routing one.
- **Why it was not fixed here:** it was made by plan 17-06, not by the 17-09 changes (scope boundary). 17-09's own R19 and R21 rows were edited by an exact line match on the `## Rows` table text and are in the right place.
- **Impact:** every 17-06 verify grep passes (it matches anywhere in the file), but 17-12's consolidation and the FEAS-02 decision read the Rows table, where R15 to R17 look unanswered. The evidence files and the 17-06 SUMMARY are correct.
- **Suggested fix (for 17-12 or a quick task):** move lines 56 to 64 into `## Rows` (replace the three PENDING lines, keep the sub-rows after their base row), and restore the three Routing rows as `| R15 | VM | R00 GO: lock-screen frames measured in the guest |` style lines.
