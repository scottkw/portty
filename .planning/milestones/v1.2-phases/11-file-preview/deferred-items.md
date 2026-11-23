# Phase 11 deferred items

## 11-07: marked's emphasis parsing time grows with the square of the input (closed: mitigated by the 11-07 fix)

- **Status:** resolved

**Resolution (2026-11-21, 11-07-FIX.md).** Decided in 11-CONTEXT.md "Decided during execution": the parse runs in a fresh Web Worker per render, terminated after 3 s (`MARKDOWN_PARSE_TIMEOUT_MS`), when the parse settles, or on unmount; on timeout the pane shows "Couldn't Show File". DOMPurify stays on the main thread (it has no DOM in a worker and would return its input unchanged). The share CSP gained `worker-src 'self'`. marked stays at 18.1.3. Per the same decision, 11-09 renders only the visible preview tab. The input that reproduces it is `'*a **b _c'` repeated with no separator (with a trailing space marked is linear). Still to check in 11-10: the worker loads on the desktop page (`wails://`) and on the share page in Safari.

The original entry:

marked 18.1.3 takes time that grows with the square of the number of unclosed emphasis delimiters in one paragraph. Measured in jsdom (`*a **b _c` repeated), with and without the 11-07 configuration:

10 KB: 0.3 s; 20 KB: 1.3 s; 40 KB: 5 s; 80 KB: 23 s.

A 2 MB paragraph did not finish in minutes. There is no upstream advisory (checked 2026-11-21). The parse runs on the main thread, so a viewer's window freezes until it ends. This is availability only: no script runs and nothing is fetched.

Options for the planner and the user:

- 11-09 renders only the visible preview tab, never a background one;
- parse in a Web Worker terminated after a time limit (the share CSP would need `worker-src 'self'`);
- a much lower size cap for Markdown than the 20 MB transfer cap;
- markdown-it (research's alternative), if it proves linear on this input.

Details are in 11-SECURITY-REVIEW.md, "Residual risk: marked's emphasis parsing time".
