---
created: 2026-11-23
area: frontend tests
source: Phase 15 re-verification regression run
---
Two DoS-bound vitest tests hit the default 5 s timeout in a full `pnpm --dir frontend test` run while another heavy process ran: `localhostUrl.test.ts > classifyLoopbackUrl hostile input > returns null for empty, scheme only and huge input without throwing` and `markdown.test.ts > renderMarkdown DoS bounds (PREV-02, T-11-25) > caps an indentation-nested list too`. Both pass alone (3/3). Make them load-robust without loosening the bound they prove (e.g. an explicit per-test timeout sized to the input, or measure the bound by work done rather than wall time).

## Resolution

Resolved in Phase 16 plan 16-02 (commit 7aa3fbb3). Both cases carry an explicit 30 s per-test timeout; inputs and assertions are unchanged. The slow input is `trimLinkTail`'s quadratic trailing-paren scan (recorded in 16-02-SUMMARY); no product change.
