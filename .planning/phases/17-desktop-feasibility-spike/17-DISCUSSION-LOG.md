# Phase 17: Desktop Feasibility Spike - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-11-24
**Phase:** 17-desktop-feasibility-spike
**Areas discussed:** Test rig, Capture-host decision method, Timebox

---

## Test rig

| Option | Description | Selected |
|--------|-------------|----------|
| Home, then office | Build/test on this Mac, then office Mac rows | |
| Home Mac only | Everything on this Mac | |
| Office Mac only | Whole spike on the office Mac | |

**User's choice:** "Can we test with the VMs you have used in the past?" → VMs carry all rows except hardware encoder and display sleep.

| Option | Description | Selected |
|--------|-------------|----------|
| This Mac, short run | ~10 min user-started run on the personal Air | ✓ |
| Office Mac later | User runs the rows at the office | |
| Defer to Phase 19 | Mark unknown | |

---

## Capture-host decision method

| Option | Description | Selected |
|--------|-------------|----------|
| Two tiny probes | Permission probe in both shapes, full pipeline in one | ✓ |
| Full path in both | Whole pipeline twice | |
| Helper only | Assume helper | |

---

## Timebox

| Option | Description | Selected |
|--------|-------------|----------|
| ~1 day | Unanswered rows marked unknown | |
| ~2-3 days | Room for surprises | |
| Until all answered | No timebox | ✓ |

---

## Decided autonomously

[auto] [Spike code] — Q: "Where does spike code live?" → Selected: "Separate Go module under spike/desktop/, never imported by the app" (reversible technical — decided autonomously: throwaway by design)
[auto] [Signing] — Q: "How are spike binaries signed?" → Selected: "Developer ID Application (S2K7P43927), never ad-hoc" (reversible technical — decided autonomously: a build flag)
[auto] [Monthly re-approval] — Q: "How is the monthly re-approval measured?" → Selected: "Read stored grant timestamps; real 30-day prompt is an M-NN item" (reversible technical — decided autonomously: observation only)
[auto] [Viewer] — Q: "What does the test viewer look like?" → Selected: "One plain HTML page, vanilla RTCPeerConnection" (reversible technical — decided autonomously: thrown away)

## Claude's Discretion

Probe implementation language, matrix row order after the VM-capture check, matrix format.

## Deferred Ideas

None.
