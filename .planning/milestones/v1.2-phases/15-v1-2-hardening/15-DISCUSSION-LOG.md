# Phase 15: v1.2 Hardening - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-11-22
**Phase:** 15-v1-2-hardening
**Areas discussed:** Update notification click

---

## Update notification click

| Option | Description | Selected |
|--------|-------------|----------|
| Start the download | One click gets the update going; brings Portty forward showing the banner at 'Downloading…'. Skips the chance to say no from the banner. | |
| Show window + banner | Brings Portty forward with the 'Portty X is available' banner and its Download button. One extra click, but nothing starts without consent. | ✓ (Claude) |
| You decide | Claude picks. | ✓ |

**User's choice:** You decide
**Notes:** Claude chose show window + banner: consent before download, smallest change.

---

## Decided autonomously

[auto] [Attach bar] — Q: "How to keep full-screen apps off the status bar row?" → Selected: "rows-1 in initial frame and every SIGWINCH claim; existing grid-owner model makes it win; test pins it" (reversible technical — decided autonomously: two call sites, local revert)
[auto] [Ended peer tab] — Q: "What does clicking an already-ended peer tab show?" → Selected: "keep ended state; no_such_session maps to ended once seen" (reversible technical — decided autonomously: frontend verdict mapping only)
[auto] [Ended peer tab] — Q: "Fix the brief 'Can't reach this Mac' line after a kill?" → Selected: "yes, short grace keeping last good state; split to a todo if cause not found" (reversible technical — decided autonomously: local UI state)
[auto] [Stale pgid] — Q: "How to stop signalling a stale process group?" → Selected: "mirror killAdopted's exited guard; items 2/3 guard or doc only" (reversible technical — decided autonomously: small internal guard)
[auto] [Socket hardening] — Q: "Which audit items?" → Selected: "O-1, O-2, O-6 all done" (reversible technical — decided autonomously: internal; refused takeover falls back to old daemon)
[auto] [Path click] — Q: "F-1 row-0 directory" → Selected: "fix relay cwd log lookup, pathclick_test.go case" (reversible technical — decided autonomously: internal lookup)
[auto] [Path click] — Q: "F-2 same-cell click after leave" → Selected: "invalidate() on mouseleave, real Linkifier test" (reversible technical — decided autonomously: one event hook)
[auto] [Flakes] — Q: "How to deflake?" → Selected: "event waits instead of wall-clock; no product change" (reversible technical — decided autonomously: test-only)

## Claude's Discretion

- Update notification click behaviour.

## Deferred Ideas

- A flag to turn the attach status bar off.
