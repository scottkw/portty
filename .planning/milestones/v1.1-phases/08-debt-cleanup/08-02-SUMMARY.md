---
phase: 08-debt-cleanup
plan: 02
subsystem: relay
status: complete
tags: [DEBT-03, IN-06, IN-08, replay-fidelity, alt-screen, osc8, charset]
requires: [08-01]
provides:
  - "Rendered replays restore DEC modes before any text (before the history in a Join), with ?7 after the text, before the CUP"
  - "Hub-side charset/link tracker (charstate.go): G0/G1 designations and the OSC 8 link through registered handlers, GL through a byte scan persisted across reads; every replay ends with its suffix"
  - "fixRenderedLinks: OSC 8 in rendered rows reaches viewers params then URI (vt stores the two swapped), and a no-params link vt leaves open at the row end is closed"
  - "Alt-exit rebuild re-opens the current and saved OSC 8 links and resyncs the tracker from its probes; the rebuild fuzz covers SO/SI, G0/G1 designations and OSC 8 at 0 mismatches"
  - "The CR-01 wedge test measures progress, not wall time"
affects: [internal/relay/hub.go, frontend/src/lib/__tests__/fixtures/ris-replay.json, frontend/src/lib/__tests__/fixtures/wrapped-replay.json]
requirements-completed: [DEBT-03]
tech-stack:
  added: []
  patterns:
    - "Track terminal state the emulator hides by registering handlers that return false (vt still runs its own), plus a byte scan for C0 controls vt does not let you hook"
    - "Negative controls: each new xterm.js check was run once against the hazard it guards (restores between history and screen; the suffix stripped)"
key-files:
  created:
    - internal/relay/charstate.go
    - frontend/src/lib/__tests__/fixtures/charset-link-replay.json
  modified:
    - internal/relay/hub.go
    - internal/relay/history_test.go
    - internal/relay/replay_fidelity_test.go
    - internal/relay/altscreen_heap_test.go
    - internal/relay/emu_nohang_test.go
    - frontend/src/lib/__tests__/replayFidelity.test.ts
    - frontend/src/lib/__tests__/fixtures/ris-replay.json
    - frontend/src/lib/__tests__/fixtures/wrapped-replay.json
decisions:
  - "IN-06: restores go before the history in a Join (xterm.js homes on ?6, which would overwrite history rows still in the viewport); ?7 is the one restore after the text"
  - "vt ignores SO/SI (its handlers are registered in the C1 loop), so G1/GL are tracked hub-side only, the rebuild probes G0 and the links but not G1, and the xterm.js fixture compares a viewer against an xterm.js host fed every byte"
  - "The charset/link suffix is appended by RenderSnapshot and Join, not the screen body, so the alt-exit rebuild can emit the probe-resynced state once, last"
  - "A genuine resize writes the tracker's suffix into the new emulator, so the hub keeps rendering line drawing after a resize"
metrics:
  duration: "~25 min (10:48 to 11:13 wall clock)"
  completed: 2026-11-19
estimate:
  tokens: 125000
  tasks: 3
actuals:
  tokens: 68600 # chars/4 over the realized diff incl. fixtures; code and tests alone about 14,300
  tasks: 3
  commits: 7 # git rev-list --count plan_head_before..plan_head_after; the SUMMARY commit is the 8th
plan_head_before: 8a7acc2567dfb7e25959e47aab0f3fe8997c15d9
plan_head_after: 76d0f2dc2b4fafa98abb9fd088a1395bf4ba0d33
---

# Phase 8 Plan 02: Replay fidelity (modes, charsets, OSC 8) Summary

Rendered replays now restore DEC modes before any text: before the history in a Join, with autowrap restored after the text. A viewer joining through a rendered replay gets the host's G0/G1 designations, SO and open OSC 8 link at the end of the replay. Replayed links now arrive params then URI, so their URI survives. The alt-exit rebuild re-opens the current and saved links and resyncs the tracker. Real xterm.js and a vt guest both confirm it, and the extended rebuild fuzz stays at 0 mismatches.

## What changed

- **hub.go, IN-06.** `renderScreenLocked` was split into `modeRestoresLocked` (every mode except 7 and the excluded 47/1047/1048/1049/2048/2026) and `screenBodyLocked` (alt prefix, rows, `?7`, CUP, pending-wrap re-print).
  - RenderSnapshot sends restores, body, suffix.
  - Join sends restores, history, the 08-01 boundary separator, body, suffix.
  - The alt-exit seed is restores plus body. The rebuild emits the charset and link state itself, last, from its probes.
  - The WR-08 "before the CUP" comment is replaced by the modeRestoresLocked comment, which says why the restores go before the history and why ?7 goes after the text.
- **charstate.go (new), IN-08 for viewers.**
  - `charState`: G0/G1 final bytes, GL, open link (params, URI), and the parser scan state.
  - `designate` and `osc8` are fed by handlers that newLiveEmulator registers. There are six SCS ESC handlers and one OSC 8 handler, and each returns false so vt's own handler still runs.
  - `reset` comes from the RIS handler. `scan` handles GL: SO/SI in ground, ESC or CSI state, never inside a string, and `ESC c`, in byte order, persisted across reads. `suffix` emits the state.
  - `fixRenderedLinks` swaps the OSC 8 fields back. It also closes a link that has no params when vt leaves it open at the row end: uv's renderLine closes at the row end only when `Link.URL`, which holds the params after vt's swap, is set. The test "link without params at the row end stays in its row" fails without that close.
  - The file comment has the residuals as a `shortcut:` line.
- **hub.go, the rest of IN-08.**
  - incompleteTail's state machine is now `vtScan.step`, which incompleteTail and the GL scan share. TestIncompleteTail is unchanged and green.
  - Every rendered row goes through fixRenderedLinks: screen rows, drained history lines, the resize push and the pending-wrap re-print. The 08-01 wrap keys stay on the raw render.
  - recordFrameLocked scans each read before writing it.
  - A genuine resize writes the tracker's suffix into the new emulator.
- **hub.go, rebuild.**
  - The probe cells give the current and saved links. A saved link now counts toward `keepSaved`. It is re-opened (fields un-swapped) before `ESC 7` and reset after it.
  - The probed G0 and current link resync the tracker. The tracker's suffix then ends the rebuild, after the pen. This replaces the single `ESC ( 0` test.
  - The `shortcut:` list is now: tab stops, G2/G3 and single shifts, the UK national set, the saved cursor's visibility, zero-width cells.

## vt ignores SO and SI (found during Task 2)

vt registers its SO and SI handlers inside the loop over C1 controls (`for i := ansi.PAD; i <= ansi.APC`, handlers.go:260-285). `ansi.SO` (0x0E) and `ansi.SI` (0x0F) are never in that range, so the cases never match. vt does nothing on SO or SI and never prints through G1 (utf8.go:63-72 uses `charsets[gl]`, and gl stays 0). TestVT_IgnoresShiftOut pins this.

The plan assumed vt handled SO. This is what follows from it not doing so:

- **For viewers (xterm.js, which honours SO):** the tracker and suffix work as planned. The charset-link-replay fixture now holds the host bytes. The vitest feeds them to a second xterm.js as the host reference, then requires the joined viewer's screen and cursor to match it, along with a plain-text expectation (e.g. `┌──┐` for G1 via SO). Plain vt cannot be the reference here.
- **vt guest tests:** the vt guest and the vt reference both ignore SO, so those subtests prove the bytes do no harm in vt. They also assert the exact suffix each replay ends with (`ESC ) 0 SO`, `ESC ) 0`, `ESC ( 0`, the link open, none after a RIS or an SO inside an OSC).
- **Rebuild:** G1 never reaches the screen in vt, so it cannot be probed. The plan's SI/SO probe cells would always read the same as G0. The rebuild probes G0 (through the existing current-pen probe) and the links. G1 and GL stay as the tracker has them, and the suffix carries them into the new emulator, where vt ignores them as it ignored the originals. A vt reference and the rebuilt emulator therefore agree on every SO/G1 case.
- **Residual, not fixed here:** rows a program drew through SO/G1 before a viewer joined are rendered by vt as ASCII (`lqqk`, not `┌──┐`), in replays and in the hub's own screen. The host's live xterm.js is unaffected. Fixing it needs vt to honour SO (an upstream fix or a version bump; TestVT_IgnoresShiftOut fails when that happens) or a feed-side workaround, which I judged out of scope.

## Tests (for 08-05 / TESTING.md)

- **internal/relay/replay_fidelity_test.go**:
  - TestRenderScreen_ModesBeforeText (new).
  - TestRenderScreen_RISKeepsCursor gains the case "RIS, then 60 lines, rendered Join replay with history" (vt guest: screen, cursor, last 20 history rows). The fixture schema gains `screen` and `historyTail`.
  - TestFixRenderedLinks, TestCharState_Scan, TestVT_IgnoresShiftOut (new).
  - TestRenderedReplay_KeepsCharsetAndLink (new), 10 subtests:
    - the 7 charset/link cases;
    - "linked words in history and on screen";
    - "link without params at the row end stays in its row";
    - "hub emulator keeps the charset across a genuine resize".
  - TestRenderedReplay_CharsetLinkFixture (new; writes or checks charset-link-replay.json).
- **internal/relay/history_test.go**:
  - TestFullReplay_ModesBeforeHistory (new).
  - TestFullReplay_ModeRestores and TestFullReplay_AltScreenOrder are re-pinned to the new order.
- **internal/relay/altscreen_heap_test.go**:
  - TestAltScreenExit_RebuildMatchesReference gains 9 subtests, and the tracker state after the rebuild is checked where a case defines one:
    - G1 via SO;
    - SO then SI;
    - G0 and G1 with SI;
    - an open link with params;
    - an open link without params;
    - a link closed before the exit;
    - the saved cursor carries a link;
    - a link restored by DECRC (tracker drift, resynced);
    - a ?2027 grapheme row to the last column.
  - rebuildFuzzStream gains SO, SI, `ESC ( 0/B`, `ESC ) 0/B`, an OSC 8 open over printed text, and the close.
- **internal/relay/emu_nohang_test.go**: TestHub_UnstrippedQueryNeverWedges, where the PTY-write phase now uses `progressing` (see below).
- **frontend/src/lib/__tests__/replayFidelity.test.ts**:
  - The ris-replay describe checks the history case (rows above the viewport, screen, cursor).
  - New describe "charset and link survive a rendered replay (charset-link-replay fixture)": a viewer compared with an xterm.js host, plus the OSC 8 payloads recorded by `parser.registerOscHandler(8)`, params then URI.

## Fuzz result

TestAltScreenExit_RebuildFuzz keeps its fixed seed (PCG 7, 2026) and 600 streams. The generator's switch went from `IntN(20)` to `IntN(24)`, with the new pieces as cases 15-17 and `$ ` as the default. Before the rebuild change, 277 of 600 streams diverged. Most were links the tracker thought open after an alt-screen switch, which the probe resync now corrects. After it, 0 diverge. No residual needed minimising.

## Pre-change failure of the xterm.js history case

The plan expected the new ris-replay history case to fail on the pre-change bytes. **It did not.** The old order put the restores after the screen text and just before the CUP, so the ?6 home was undone by the CUP and never overwrote history. I regenerated the fixture on the pre-change code and the case passed (10/10).

The hazard that case guards is a naive IN-06 fix that puts the restores between the history and the screen. I checked that once: I temporarily emitted the restores between the two, regenerated, and the case failed. xterm.js showed `hist 30..34` where `hist 50..54` belonged, because the screen rows overwrote the history rows still in the viewport. The Go order tests (TestRenderScreen_ModesBeforeText, TestFullReplay_ModesBeforeHistory, ModeRestores, AltScreenOrder) are what fail on the pre-change order.

The same kind of negative control for the charset fixture: stripping the suffix (`ESC ) 0 SO`, or `ESC ( 0`) from the replay makes the viewer's screen differ from the xterm.js host. This was checked once with a throwaway vitest file, which was then deleted.

## Tracker residuals (as written in charstate.go)

> shortcut: a link that DECRC or an alt-screen switch restores in vt (which keeps the link in each screen's cursor) is not seen until the next alt-exit rebuild resyncs the tracker from its probes; G2/G3, LS2/LS3, single shifts and GR are not tracked. Track them if a program needs them.

## Extra item: wedge check measures progress (TestHub_UnstrippedQueryNeverWedges)

The PTY-write phase used to fail after 10 s of total time, which the 2 MiB random blob exceeds under `-race` on a loaded host. It now uses `progressing(t, 5s, ...)`, which counts completed pipe writes. Each write returns only after the hub has read it, and the hub reads again only after the emulator has taken the previous read. The test fails only if no write completes for 5 s. A real wedge still fails fast: with the reply pipe left open (NewQuietEmulator's Close removed for one run) it failed at 5.02 s, "PTY writes made no progress for 5s after 1 steps". The broadcast, Join and resize checks stay 5 s `within` calls: each is a single operation. Commit `76d0f2dc`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] vt ignores SO/SI, so the plan's SO probes and vt-guest SO expectations could not work as written**
- **Found during:** Task 2 (the vt reference printed `lqqkqqx` after `ESC ) 0 SO`)
- **Fix:**
  - The tracker carries G1/GL to viewers. The xterm.js fixture compares against an xterm.js host fed every byte.
  - The vt-guest cases assert the replay suffix.
  - The rebuild probes G0 and the links only, and keeps the tracked G1/GL.
  - TestVT_IgnoresShiftOut pins the quirk.
  - The plan's grep `x0e` in hub.go is met by the rebuild comment that names vt's ignored 0x0e/0x0f, not by an SO probe.
- **Files:** charstate.go, hub.go, replay_fidelity_test.go, altscreen_heap_test.go, replayFidelity.test.ts
- **Commits:** 21127a00, 206127f3

**2. [Rule 1 - Bug] a no-params OSC 8 link at the row end leaked into the next replayed row**
- uv closes a link at the row end only when the params field is set. fixRenderedLinks now appends a close when the row's last link is open.
- Tested by "link without params at the row end stays in its row", which fails without the close: the guest's next row came out linked.
- The TestFixRenderedLinks cases are real rendered rows (text and closes), not the plan's bare sequences, because of this rule.
- **Commit:** 21127a00

**3. [Rule 2 - Missing] a genuine resize lost the hub emulator's charset state**
- resizeLiveEmulator builds an empty emulator, so after a resize the hub rendered `qqq` while the host still had G0 line drawing.
- It now writes the tracker's suffix into the new emulator. Covered by the subtest "hub emulator keeps the charset across a genuine resize".
- **Commit:** 21127a00

**4. [Design] the suffix lives outside the screen body**
- The plan put the suffix in the body helper and had the rebuild's probe re-emit follow it. If the tracker had drifted (a link closed by an alt switch, say), the seed's suffix would leave a stale link or designation that the re-emit would have to undo.
- So RenderSnapshot and Join append the suffix, and the rebuild emits the resynced suffix once, last. The vt order is unchanged otherwise.
- **Commit:** 206127f3

**5. [Test design] the plan's "tracker equals the probed state" check is taken right after the rebuild read**
- After the "saved cursor carries a link" case's final `ESC 8`, vt has the link and the tracker does not: that is the documented DECRC residual.
- **Commit:** a30358aa

**6. pnpm install --frozen-lockfile --offline in the worktree's frontend/**
- The worktree had no node_modules. The install used the lockfile only and added no package.

## Known Stubs

None.

## Threat Flags

None. The new surface is the one in the threat model:
- T-08-07: links re-opened for viewers come from the session's own output.
- T-08-23: fixRenderedLinks rewrites only the exact BEL-terminated three-field form uv emits. It is unit-tested on params, no params, the close, plain text and two links.

Share tokens, the CSP, the 32 KiB input limits and the WR-07 guard were not touched.

## Verification

All commands ran with `GOTOOLCHAIN=go1.26.11` and `set -o pipefail`.

- Task 1 verify passed:
  - vet;
  - `go test -short -run 'TestRenderScreen_|TestFullReplay_|TestReplayFidelity_|TestSoftWrap_|TestHistory_|TestAltScreenExit_'` ok;
  - vitest replayFidelity 10 passed;
  - the three grep checks;
  - fixtures clean.
- Task 2 verify passed:
  - vet;
  - the `TestFixRenderedLinks|TestCharState_|TestRenderedReplay_|...` set ok;
  - vitest 17 passed;
  - `RegisterOscHandler(8` 1, `func fixRenderedLinks` 1, `charset-link-replay` 2;
  - the fixture exists and is clean.
- Task 3 verify passed:
  - `go build ./...` OK; vet OK;
  - `go test -short -run 'TestAltScreenExit_|TestIncompleteTail|TestAltScreenResize_'` ok;
  - the grep checks: old shortcut wording 0, `SetHyperlink` 1, `x0e` in hub.go 1 (a comment, see deviation 1), `x0e` in the test 3, `]8;` in the test 7.
- **The one full run, `go test -race -short -count=1 ./internal/relay/`, passed in 136.2 s: 157 PASS, 0 FAIL, 1 SKIP** (TestHub_SlowClientDisconnected, which skips under -short). No data race was reported.
  - TestAltScreenExit_RebuildFuzz passed in 11.3 s.
  - The 2 MiB-blob wedge subtest passed in 6.45 s.
- After the last code change:
  - `pnpm vitest run src/lib/__tests__/replayFidelity.test.ts` reported 17 passed;
  - `pnpm exec tsc --noEmit` OK;
  - regenerating all four fixtures left `git diff --quiet -- frontend/src/lib/__tests__/fixtures/` clean;
  - `frontend/src/wailsjs` was untouched (no Wails build).
- Not run: the other Go packages, the full vitest suite, check-brand and check-traceability (no TESTING.md edit in this plan; 08-05 owns it).

## Self-Check: PASSED

- FOUND: internal/relay/charstate.go, frontend/src/lib/__tests__/fixtures/charset-link-replay.json
- FOUND commits: 03b0ee83, 65037965, 92f9aea3, 21127a00, a30358aa, 206127f3, 76d0f2dc
