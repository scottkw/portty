---
created: 2026-11-23
area: frontend tests
source: Phase 15 re-verification regression run
---
Two DoS-bound vitest tests hit the default 5 s timeout in a full `pnpm --dir frontend test` run while another heavy process ran: `localhostUrl.test.ts > classifyLoopbackUrl hostile input > returns null for empty, scheme only and huge input without throwing` and `markdown.test.ts > renderMarkdown DoS bounds (PREV-02, T-11-25) > caps an indentation-nested list too`. Both pass alone (3/3). Make them load-robust without loosening the bound they prove (e.g. an explicit per-test timeout sized to the input, or measure the bound by work done rather than wall time).
