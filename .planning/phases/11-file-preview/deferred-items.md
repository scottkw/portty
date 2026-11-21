# Phase 11 deferred items

## 11-07: marked's emphasis parsing time grows with the square of the input (open, needs a decision)

marked 18.1.3 takes time that grows with the square of the number of unclosed emphasis delimiters in one paragraph. Measured in jsdom (`*a **b _c` repeated), with and without the 11-07 configuration:

| Size | Time |
|------|------|
| 10 KB | 0.3 s |
| 20 KB | 1.3 s |
| 40 KB | 5 s |
| 80 KB | 23 s |

A 2 MB paragraph did not finish in minutes. There is no upstream advisory (checked 2026-11-21). The parse runs on the main thread, so a viewer's window freezes until it ends. This is availability only: no script runs and nothing is fetched.

Options for the planner and the user:

- 11-09 renders only the visible preview tab, never a background one;
- parse in a Web Worker terminated after a time limit (the share CSP would need `worker-src 'self'`);
- a much lower size cap for Markdown than the 20 MB transfer cap;
- markdown-it (research's alternative), if it proves linear on this input.

Details are in 11-SECURITY-REVIEW.md, "Residual risk: marked's emphasis parsing time".
