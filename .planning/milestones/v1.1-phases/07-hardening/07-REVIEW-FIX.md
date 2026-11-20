---
phase: 07-hardening
fixed_at: 2026-11-19T04:29:56Z
review_path: .planning/phases/07-hardening/07-REVIEW.md
iteration: 3
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 7: Code Review Fix Report

**Fixed at:** 2026-11-19
**Source review:** .planning/phases/07-hardening/07-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 12 (CR-01, WR-01..WR-04, IN-01..IN-07)
- Fixed: 7 (CR-01, WR-01, WR-02, WR-03, WR-04 (comment only), IN-04, IN-07)
- Deferred: 5 (IN-01, IN-02, IN-03, IN-05, IN-06)

Verification ran in the agent's isolated worktree (`.claude/worktrees/agent-a23168798cf6c72b1`, branch `worktree-agent-a23168798cf6c72b1`, fast-forwarded to `gsd/phase-07-hardening` at b3c869a9 first), not in the main checkout.

## Fixed Issues

### CR-01: a query the regex misses wedges the session's hub and the styled tail

**Files modified:** `internal/relay/hub.go`, `internal/daemon/engine.go`, `internal/relay/emu_nohang_test.go` (new), `internal/daemon/engine_test.go`, `TESTING.md`
**Commit:** 322ac362
**Applied fix:** I added `relay.NewQuietEmulator`, which closes vt's reply pipe writer when it builds the emulator. Every reply then returns `ErrClosedPipe` at once. vt ignores that error at every one of its pipe writes (checked in the source), so `emu.Write` can no longer block, whatever bytes it gets. All three production emulators come from it: the hub's live emulator (`newLiveEmulator`, which is also used by the resize and alt-exit rebuilds) and the engine's styled-tail emulator. A grep found no other production `xvt.NewEmulator`; the remaining calls are in tests that feed known content.

This differs from the suggested drain goroutine on purpose. It closes the pipe writer, as asked, but does it once at construction instead of at teardown. So there is no reader goroutine to leak when an emulator is replaced, and nothing ever calls `emu.Read` or `emu.Close`. The #100 race (a concurrent `Read`/`Close` on `e.closed`, removed in 1376705e) cannot come back. The doc comments on `liveEmuQueryStripPattern`, `StripTerminalQueries`, the `emuMu` field and `recordFrameLocked` now say that the strip keeps replies out of the shared shell during a replay, and the closed pipe prevents hangs. The live feed still runs the strip, which keeps `?2048` out of `h.modes`.

**Tests:**
- `TestHub_UnstrippedQueryNeverWedges` feeds `\x9bc`, a DA1 split across two PTY reads, `ESC[?2004;2048h` and a seeded 2 MiB random blob through a running hub. It then checks that the next output is broadcast and that Join and a host resize return within 5 s. All four subtests failed (wedged) before the fix.
- `TestGetSessionStyledTailLines_UnstrippedQueryNeverHangs` covers the styled-tail path. With only the engine change reverted, all three subtests fail.

### WR-03: a resize in the alt screen loses the main screen and bypasses HARD-01

**Files modified:** `internal/relay/hub.go`, `internal/relay/altscreen_heap_test.go`, `TESTING.md`
**Commit:** 741317b7
**Applied fix:** When the screen is in the alt buffer, `resizeLiveEmulator` now does four things:
- writes `ESC[?1049l` into the old emulator and clears the `altExited` flag that write raises;
- pushes the main rows to history, the same way a main-screen resize does;
- builds the new emulator;
- writes `altScreenEnterSeq` into it.

The TUI's redraw therefore stays out of history, and the app's own exit still fires the HARD-01 rebuild. `TestAltScreenResize_KeepsMainScreenAndExitRebuild` failed before the fix.

### WR-01: peer-tab snapshot restore drops hidden modes and can restore a torn state

**Files modified:** `frontend/src/lib/peerSnapshots.ts`, `frontend/src/components/TerminalPanel.tsx`, `frontend/src/lib/__tests__/peerSnapshots.test.ts`, the `TerminalPanel.tabSwitch`, `.scale`, `.reconnect` and `.guestClose` tests (mock `parser` stubs only), `TESTING.md`
**Commit:** 6c37c737
**Applied fix:** `watchPeerTerm(term)` uses xterm's public parser hooks to track the state that `SerializeAddon` 0.14 leaves out and xterm's public API does not expose: cursor visibility `?25`, the SGR mouse encodings `?1006`/`?1016`, the alt buffer, and DECSTBM per buffer. It is cleared on RIS and DECSTR. The snapshot is `serialize()` plus the re-emitted modes. `serialize()` already covers mouse tracking and bracketed paste, and the tests confirm both survive.

The watch also follows the output bytes with a code-point state machine (ESC, CSI, OSC, DCS/SOS/PM/APC, C1, UTF-8). If the last read ended mid-sequence, or a non-full scroll region is live, no snapshot is stored, and the returning tab dials without a resume and gets the full replay.

**Tests:** in real xterm.js, a restored tab has SGR encoding, a hidden cursor, mouse tracking and bracketed paste back. Seven torn tails each veto the snapshot, as does a scroll region. TerminalPanel stores nothing after a `$ \x1b[3` frame.

### WR-02: the alt-exit rebuild loses emulator state

**Files modified:** `internal/relay/hub.go`, `internal/relay/altscreen_heap_test.go`, `TESTING.md`
**Commit:** 57ffaae0
**Applied fix:**
- **Region and saved cursor:** new handlers track the main screen's DECSTBM region (CSI r) and DECSC saved cursor (ESC 7, CSI s without DECLRMM, `?1048h`, and the `?1049` save). They are cleared on RIS and on a resize rebuild.
- **Pen and charset:** the rebuild prints one probe cell into the old emulator, which is about to be discarded, and reads the current pen and DEC line-drawing G0 from that cell.
- **Re-seeding:** it re-applies all four after the rendered seed.
- **Cut-off reads:** it carries over the bytes the read cut off (`incompleteTail`: an unfinished escape sequence or UTF-8 character after the last ground-state point), so the next read completes them.

`TestAltScreenExit_RebuildMatchesReference` reproduces the reviewer's probe cases: mid-CSI, mid-UTF-8, inside an OSC, red pen, shell region, DECSC, and DEC line drawing. It checks that the rebuilt emulator matches one fed every byte. `TestIncompleteTail` covers the scanner.

A `shortcut:` comment records what is still lost: tab stops, G1-G3/SO, DECSC's saved pen, and a pending wrap after a region or saved-cursor restore.
**Status:** fixed: requires human verification (logic change in the state-rebuild path)

### WR-04: the HARD-01 bound is per session and scales with width

**Files modified:** `internal/relay/hub.go`
**Commit:** 9eeb38bb
**Applied fix:** comment only, as instructed. The `shortcut:` comment in `newLiveEmulator` now gives the cost per session and by width: 10,000 × cols × about 112-120 B. That is about 128 MiB at 100 columns and about 270 MiB at the 220-column fallback, or 0.4-0.8 GiB for three overnight `less +F` sessions. It also says when and how to bound it.

### IN-04: the replay strip lets some answers through

**Files modified:** `internal/relay/hub.go`, `internal/relay/querystrip_test.go`, `TESTING.md`
**Commit:** 028adbe6
**Applied fix:** the OSC 10/11/12 alternative now takes `(?:;\?)+`, so it matches `OSC 10;?;?`. XTWINOPS now covers `13/14/16/18/19/21 t`, with 21 t being the title report. I added 6 strip subtests and 2 keep subtests (title pop `23;0 t` and an OSC 10 colour set).

Not done: dropping the 0x01 bytes from the raw replay. That changes replay byte offsets and was not in the requested scope.

### IN-07: testmain ignores errors and skips cleanup

**Files modified:** `internal/daemon/testmain_test.go`
**Commit:** b32e735d
**Applied fix:** the Setenv errors now panic. The temp HOME is removed by a defer in `runInTempHome`, which returns the exit code to `os.Exit`. A comment notes that `XDG_CONFIG_HOME` only matters on Linux.

The fix is partial by nature: a panic inside a test's own goroutine still kills the process before any defer runs, so the comment says so.

## Skipped Issues

### IN-01: launch respawn grace guards an unreproduced blip

**File:** `app.go:310-349`
**Reason:** deferred. This is a necessity finding, and whether to keep or remove the grace is the user's scope decision. The `now`-before-`Health()` skew is harmless, as the review itself notes.
**Original issue:** the grace has never fired outside unit tests, and its time is judged from before `Health()`.

### IN-02: peerSnapshots caps entries, not bytes

**File:** `frontend/src/lib/peerSnapshots.ts:18-36`
**Reason:** deferred. Bounding by bytes and evicting on tab close needs a byte budget and a close hook from App.tsx's tab model, which is more than a trivial change. WR-01 already stores fewer snapshots, because torn or region-holding states store none.
**Original issue:** eight full serializations can hold hundreds of MB, and a closed tab's content stays.

### IN-03: deferred putPeerSnapshot races a quick remount

**File:** `frontend/src/components/TerminalPanel.tsx:559-565`
**Reason:** deferred. A pending-promise handshake is a design change. The race only costs a full replay plus a stale but consistent entry, never wrong content.
**Original issue:** a remount before the write callback finds no snapshot.

### IN-05: splitting a paste removes atomicity on a socket drop

**File:** `frontend/src/lib/relayClient.ts:528-533`
**Reason:** deferred. The fix is a UX notice in the reconnect banner, not a trivial change, and the 32 KiB input limit must stay.
**Original issue:** a drop mid-paste delivers a prefix with no indication.

### IN-06: the alt-exit seed writes width modes after the text

**File:** `internal/relay/hub.go` (renderScreenLocked)
**Reason:** deferred. Reordering `?2027` changes the shared rendered replay that Join sends to every viewer, which needs its own fidelity test. WR-02's reference test passes for the cases covered. Only emoji/ZWJ rows under `?2027` are affected.
**Original issue:** a wide-grapheme row can wrap and scroll the seed by one line.

## Gate

The gate ran in the isolated worktree:
- `go build ./... && go vet ./...`: clean
- `go test -race -short ./...`: all 12 packages ok on the first run, no failures and no reruns (relay 85.8 s, daemon 27.1 s, root 31.6 s)
- `pnpm vitest run`: 90 files, 1232 tests passed
- `pnpm exec tsc --noEmit`: clean
- `bash tests/check-traceability-paths.sh`: OK, 147 paths
- `bash tests/check-brand.sh`: clean

---

_Fixed: 2026-11-19_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_

# Iteration 2

**Source review:** .planning/phases/07-hardening/07-REVIEW.md (iteration 2, f6b68df4)

**Summary:**
- Findings in scope: 6 (WR-05, WR-06, WR-07, the IN-04 remainder, IN-08, IN-09)
- Fixed: 5 (WR-07, WR-06, WR-05, IN-04, IN-09)
- Deferred: 1 (IN-08)

Verification ran in the agent's isolated worktree (`.claude/worktrees/agent-aa44fd1bee676bdd9`, branch `worktree-agent-aa44fd1bee676bdd9`, fast-forwarded to `gsd/phase-07-hardening` at f6b68df4 first), not in the main checkout.

## Fixed Issues

### WR-07: the Go test suites reach the user's real tailscaled and probe every peer

**Files modified:** `internal/tailnet/live.go` (new, not a test), `internal/tailnet/tailnet.go`, `internal/tailnet/sessions.go`, `internal/webserver/tailscale.go`, `internal/webserver/whois.go`, `internal/webserver/server.go`, `internal/daemon/share.go`, `internal/daemon/peer_client.go`, `internal/tailnet/tailnet_test.go`, `internal/webserver/tailscale_test.go`
**Commit:** d663e738
**Applied fix:** `tailnet.LiveAllowed()` is `!testing.Testing() || PORTTY_LIVE_TAILNET_TESTS=1`, and `tailnet.ErrLiveTailnetInTest` is the sentinel. Guarded choke points, each checked against every caller:
- `DiscoverPeers` returns `nil, ErrLiveTailnetInTest`, which also covers `DiscoverAndProbe` and the daemon's default `discoverPeers`/`probePeers`.
- `ProbePeer` returns false.
- `FetchPeerSessionsMeta` returns `nil, MetaUnreachable`, which covers `FetchAllPeerSessionsMeta` (one unreachable group per peer).
- `webserver.CheckHealth` returns the not-connected zero value, which covers `defaultShareHealth`, `cmdHealth` and `App.tailscaleCheck`.
- `NewWebServer`'s default WhoIs client becomes `noLiveWhoIs` (every lookup fails, so `requireSameUser` refuses with 503).
- `Start`'s `GetCertificate` hook fails when no `TLSConfig` is set.
- `defaultWarmCert` returns the sentinel.

The grep found one path the review did not list: the daemon's default per-peer client (`API.peerClient`, used by `/peers/connect` and the remote WS proxy). It now refuses to dial unless `SetPeerClientForTest` set a client. There are no other `local.Client` or `tsnet` uses outside tests, and the remaining dialers (`cmd_attach.go`, `attach/loop.go`, `daemon/socket.go`, `ipc.go`) reach the local daemon only. Seams still bypass the guard: `discoverPeers(ctx, fakeStatus)`, `probePeer(ctx, peer, client)`, `checkHealth(...)`, `SetWhoIsClientForTest`, `SetShareSeamsForTest`, `SetPeerSeamsForTest` and `SetPeerClientForTest`. In the real binary `testing.Testing()` is false, so production behaviour does not change.

Tests:
- `TestLiveTailnetGuard` replaces `TestDiscoverPeers_Public`, which accepted either outcome. It asserts the sentinel from `DiscoverPeers` and `DiscoverAndProbe`, false from `ProbePeer`, and unreachable from both meta fetches, with a counting listener that sees 0 connections.
- `TestLiveTailnetGuard_Webserver` covers `CheckHealth` and the default WhoIs client.
- `TestProbePeer_NoIPFallbackWithoutIPs` now dials through a failing transport, because it made a real DNS lookup of `unresolvable.example.invalid` that MagicDNS hands to tailscaled. It still tests the same thing (no IP fallback without IPs).
- The listed tests (`cmd_cli_test.go` `TestCmdList_*` lines 48/73/89/110/269/310, `app_test.go` `TestGetRemoteSessionsWithMeta_ReachableField`, `TestProbePeer_Public`, `TestFetchAllPeerSessionsMeta_IncludesUnreachablePeers`, `TestFetchPeerSessionsMeta_IPFallback`) pass unchanged. They already accepted "error or empty".

**Proof (no tailscaled or peer access):**
- `GODEBUG=netdns=go+2` (the pure-Go resolver with per-lookup tracing) on `./internal/tailnet` plus the root `TestCmdList|TestGetRemoteSessionsWithMeta|TestCmdHealth` tests: all pass with 0 `hostLookupOrder` lines. Before the transport change, the same run printed one lookup (`unresolvable.example.invalid`).
- `./internal/webserver ./internal/daemon` under the same tracing: both ok, 0 lookups.
- The full gate `go test -race -short ./...` under the same tracing: 12/12 packages ok, 0 lookups in the whole log.
- tailscaled's LocalAPI goes over a socket, not DNS. `TestLiveTailnetGuard` proves every LocalAPI entry point returns the sentinel before it builds a `local.Client`.
- The root package now takes 8.9 s under `-race`, down from 31.6 s in iteration 1, because it no longer waits on live peer probes.

### WR-06: the re-seed's saved-cursor block drops the pending wrap, saves the wrong pen and ignores origin mode

**Files modified:** `internal/relay/hub.go`, `internal/relay/altscreen_heap_test.go`
**Commit:** 915eb31a
**Applied fix:** the old emulator is discarded, so the rebuild probes it for the state vt does not expose:
- **Pending wrap:** after CAN, one probe print. Only a pending wrap wraps it, which puts it on column 1 (column 2 when the CAN flushed a cut-off UTF-8 character as a replacement cell). *Corrected in iteration 3 (WR-09): one CAN completes the character only when it is one byte short. vt counts CAN as a byte of an open UTF-8 character, so the probe now sends one CAN per missing byte.*
- **Current pen and G0:** read from `q` printed at the top left.
- **Saved cursor and its pen:** `ESC 8`, then `q`. This replaces the hub-side `savedCursor` tracking: the field, the ESC 7 / CSI s / ?1048 / AltScreen hooks and their resets are deleted.

The new emulator then gets, in order:
1. The seed.
2. The saved pen.
3. `CUP saved` + `ESC 7`.
4. `CUP pos`.

All of these sit inside `?6l` ... `?6h` when DECOM is set, so the absolute CUPs land where they should.

The pending wrap is re-created by setting it at the alt screen's edge and switching back (`?1047h CUP x ?1047l`). vt keeps one wrap flag for both screens, and an app's exit or a DECRC can leave it set on any column, so this covers both. The current pen and charset come last. The block runs only when the region is set, the saved cursor differs from a fresh one (home, default pen), or the seed's pending-wrap guess was wrong.

The fuzz test also found three bugs in the rebuild:
- `CellAt` returns a pointer into the buffer, and the saved-pen probe overwrote the cell the current-pen probe had read. Cells are now copied.
- `renderScreenLocked` replayed `?1048` as a mode. `?1048l` restores the cursor over the replay's CUP. This affected the viewer's rendered replays too, not only the rebuild. 1048 is now excluded like 1047/1049.
- The cell re-print lost its pen. This was fixed by the alt-screen wrap approach.

**Fuzz test:** `TestAltScreenExit_RebuildFuzz` runs 600 fixed-seed (PCG 7, 2026) random streams at 20x6. Each stream is shell output with SGR, ESC 7/8, CSI s/u, DECSTBM, DECOM, CUP, line drawing, UTF-8, ?25/?7, OSC, DCS, EL and ED, around a ?1049, ?1047 or ?1048+?1047 visit, cut into random 1-24-byte reads. It compares `Render()` and the cursor read for read against an emulator fed every byte.
- Mismatches: 229 of 600 before the fix, 0 of 600 after.
- A wider scratch run (18,000 streams over 6 seeds at 20x6, 40x10, 12x4 and 80x24, not committed) left 2.
- Both of those are an explained residual, now a `shortcut:` comment: a pending wrap on column 0 whose wrap scrolls the region looks the same to the probe as no wrap. It only arises from vt's DECRC/alt-exit quirk (xterm.js keeps the flag per buffer, so the viewer never sees it).
- 7 new `TestAltScreenExit_RebuildMatchesReference` subtests (saved pen, origin mode, three pending-wrap cases, the last column without a wrap before a split UTF-8 character, `?1048`). 5 of them fail on the pre-fix code.

The remaining documented losses are tab stops, G1-G3/SO, an OSC 8 link (IN-08), and the saved cursor's link and visibility.

### WR-05: the probe is swallowed after a DCS introducer or mid-UTF-8

**Files modified:** `internal/relay/hub.go`, `internal/relay/altscreen_heap_test.go`
**Commit:** c089d80b
**Applied fix:** the probe starts with CAN (`"\x18\x1b[1;1H"`, kept in WR-06's restructure as `"\x18x"` followed by the CUP).

Seven subtests cover the tail states: ESC, ESC P, ESC P +, C1 DCS (0x90), APC, C1 CSI (0x9b) and line drawing with a mid-UTF-8 tail, all under a bold red pen. ESC P and 0x90 failed before the fix.

One finding along the way: for a cut-off UTF-8 character, vt's CAN does not abort silently. It prints the partial bytes as one replacement cell. The pen probe is unaffected, and WR-06's wrap probe accounts for the extra cell.

*Corrected in iteration 3 (WR-09):* the paragraph above holds only for a character cut one byte short. CAN returns vt's parser to ground from every escape-sequence state, but not from an open UTF-8 character: vt counts the CAN, and any byte after it, as one of the character's bytes until the length is reached. A tail 2 or 3 bytes short swallowed the probe `x` (and, for `f0`, the CUP's ESC). The pen probe still landed, but the wrap probe misread. The probe now completes the character with one CAN per missing byte.

### IN-04 (remainder): CSI 11/15/20 t, and queries split across ring frames

**Files modified:** `internal/relay/hub.go`, `internal/relay/querystrip_test.go`
**Commit:** 0e6eb34d
**Applied fix, part (b):** the XTWINOPS pattern is now `(?:1[134-689]|2[01])`, which adds 11 t (window state), 15 t (screen pixels) and 20 t (icon label echo-back). New subtests: 3 strip cases, plus keep cases for `CSI 10;0 t` and `CSI 1 t`.

**Applied fix, part (a):** first I checked the claim at `hub.go:1087-1088`, and it holds.
- `relayClient.ts` `handleFrame` sets `offsetValid = false` at the replay meta, counts output frames only while `offsetValid`, and sets the offset from the offset meta.
- `internal/attach/loop.go` does the same (lines 314-339).
- So no client counts replay bytes.

Join's raw paths now use `stripRingQueries`:
- It matches the strip pattern with the ring's 0x01 frame headers skipped.
- A match also drops the headers inside it.
- Every other byte stays, headers included.

A query-free replay is therefore still exactly the raw ring, and the 11 tests that pin raw replay bytes (the #109 byte-fidelity tests among them) pass unchanged.

I tried and dropped a simpler version that removed every 0x01 from raw replays: it broke those 11 pinned expectations. `TestJoin_StripsQuerySplitAcrossReads` covers DSR, OSC 11 and CSI 21 t, each split by a read boundary, in full and delta replays.

### IN-09: the ground tracker is not reset on the in-band reset

**Files modified:** `frontend/src/components/TerminalPanel.tsx`, `frontend/src/components/__tests__/TerminalPanel.tabSwitch.test.tsx`, `frontend/src/components/__tests__/TerminalPanel.hot-swap.test.tsx`
**Commits:** 74183d9e, 23bbb9b4
**Applied fix:** `onReset: () => { peerWatch?.feed(new Uint8Array([0x18])); resetForFullReplay(term) }`. CAN puts the tracker back to ground, in order with the bytes fed after it.

I did not use the reviewer's other suggestion, a reset inside the RIS hook, because it would race. xterm parses the queued RIS after the replay bytes have already been fed to the tracker, so the hook could report ground mid-sequence.

New tabSwitch case: a socket drops inside an OSC, a plain-text full replay follows, and the tab still snapshots and redials with `resume=200`. It fails on the pre-fix code.

The hot-swap source gate pins the `onReset` line, so its regex was updated. Its invariant is unchanged: one `resetForFullReplay` and no `.reset()`.

## Skipped Issues

### IN-08: the charset probe misreads SO-shifted line drawing, and an open OSC 8 link is dropped

**File:** `internal/relay/hub.go` (rebuildAfterAltExitLocked)
**Reason:** deferred, because neither half is trivial.
- SO needs three more probes (current GL, G0 after SI, G1 after SO) to decide between `ESC(0`, `ESC)0` and SO, plus SO/SI in the fuzz generator.
- The link needs the saved and current `uv.Link` re-opened in the right order around `ESC 7`, with its own reference cases.

Both are rare under `TERM=xterm-256color`, and both are already listed in the rebuild's `shortcut:` comment.

**Original issue:** with GL = G1 the rebuild designates G0 instead, and an open hyperlink is not re-opened.

## Gate (iteration 2)

The gate ran in the isolated worktree:
- `go build ./... && go vet ./...`: clean
- `GODEBUG=netdns=go+2 go test -race -short ./...`: all 12 packages ok on the first run (root 8.9 s, daemon 37.7 s, relay 113.2 s, webserver 30.6 s, tailnet 6.6 s), 0 DNS lookups in the log
- `pnpm vitest run`: 90 files, 1233 tests passed. A first run, made while the Go race suite was running, failed `relayClient.test.ts` "a socket open 30 s or more restarts the schedule" (a timing flake; it passed on rerun) and the hot-swap source gate (fixed in 23bbb9b4).
- `pnpm exec tsc --noEmit`: clean
- `bash tests/check-traceability-paths.sh`: OK, 147 paths
- `bash tests/check-brand.sh`: clean
- TESTING.md: commit 7d4f6541 (Section 2 note, no new test files, so the counts are unchanged; Section 4 rows for HARD-01, HARD-03, HARD-05 and HARD-06)

---

_Fixed: 2026-11-19_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 2_

# Iteration 3

**Source review:** .planning/phases/07-hardening/07-REVIEW.md (iteration 3, 110e560e)
**Fixed at:** 2026-11-19T04:29:56Z

**Summary:**
- Findings in scope: 4 (WR-09, WR-08, IN-10, IN-11)
- Fixed: 4
- Skipped: 0

Verification ran in the agent's isolated worktree (`.claude/worktrees/agent-abdc7b21b0955cca2`, branch `worktree-agent-abdc7b21b0955cca2`, fast-forwarded to `gsd/phase-07-hardening` at 110e560e first), not in the main checkout. Each new test was run against the pre-fix code and failed there.

## Fixed Issues

### WR-09: the pending-wrap probe misreads a read that ends two or more bytes short of a UTF-8 character

**Files modified:** `internal/relay/hub.go`, `internal/relay/altscreen_heap_test.go`, `.planning/phases/07-hardening/07-REVIEW-FIX.md`
**Commit:** af591113
**Applied fix:** the probe now sends one CAN per missing byte (`size - len(tail)`, where a plain `switch` on the lead byte gives the size), then the `x`. The cut-off character prints as one U+FFFD, and the probe `x` and the CUP start from ground. Before, a single CAN completed the character only when it was one byte short.

vt's `ansi.Parser.advanceUtf8` collects any byte, CAN and ESC included, until the character's length is reached. That confirms the reviewer's reading. The WR-05 paragraph and the WR-06 pending-wrap bullet in the iteration-2 section above now carry an in-place correction. The `incompleteTail` comment "CAN, SUB abort any sequence" made the same over-broad claim. It now says they abort an escape sequence, plus a `shortcut:` line: the scan assumes well-formed UTF-8.

**Tests:**
- `TestAltScreenExit_RebuildMatchesReference` gains 14 subtests: every cut of `é` (1/2), `─` and `日` (1/3, 2/3) and `😀` (1/4, 2/4, 3/4), each on column 1 without a wrap and in pending wrap. 7 fail on the pre-fix code, matching the reviewer's table.
- `rebuildFuzzStream` now also emits `❯` (3-byte), `日` (3-byte, wide) and `😀` (4-byte, wide). The fuzz test diverged on 1 of 600 streams with the pre-fix probe and on 0 of 600 after the fix.

### WR-08: rendered replays restore DEC modes after the cursor CUP, so `?6l`/`?6h` home every xterm.js viewer's cursor

**Files modified:** `internal/relay/hub.go`, `internal/relay/replay_fidelity_test.go`, `internal/relay/history_test.go`, `frontend/src/lib/__tests__/replayFidelity.test.ts`, `frontend/src/lib/__tests__/fixtures/ris-replay.json` (new fixture), `frontend/src/lib/__tests__/fixtures/wrapped-replay.json` (regenerated)
**Commit:** a381513f
**Applied fix:** `renderScreenLocked` now writes the screen text, then the mode loop, then the CUP, then the pending-wrap re-print. The re-print comes after the `?7` restore because `pendingWrapLocked` requires autowrap.

**xterm.js check:** I read xterm.js 6.0.0 `InputHandler.setModePrivate` and `resetModePrivate`.
- Only mode 6 moves the cursor (`_setCursor(0, 0)` in both directions).
- Mode 3 resizes and resets only with `windowOptions.setWinLines`, which the frontend never sets.
- 47/1047/1048/1049 move the cursor too, but the loop already skips them.

The viewer's full-replay reset is a RIS (`resetForFullReplay`), and rendered replays send no DECSTBM. So the region is the full screen, and a CUP under DECOM stays absolute.

**vt check:** vt's `setMode` moves the cursor only for 1048/1049/47, all skipped. So the rebuild seed (which is `renderScreenLocked`) is unaffected, and the rebuild reference and fuzz tests still pass.

**Tests:**
- `TestRenderScreen_RISKeepsCursor` has three cases: a RIS then a prompt, a RIS then a full row (pending wrap), and an app's bare `?6h ?6l`. Each asserts:
  - no `ESC[?6` after the cursor CUP
  - a vt guest fed the replay ends at the host cursor, before and after the next live bytes
- The test also writes `ris-replay.json` (same `-update-replay-fixture` flag as the wrapped fixture), and `replayFidelity.test.ts` feeds it to real xterm.js after `resetForFullReplay`.
- On the pre-fix bytes, xterm.js put the cursor at (0,0) in all three cases. The Go test failed all three as well.
- One normalisation in the vitest: xterm.js reports a pending wrap as `cursorX == cols`, and vt as `cols - 1`. The next live byte (`Y` lands at (1,1)) proves the wrap really is pending.

**Existing tests updated:** `TestFullReplay_ModeRestores` and `TestFullReplay_AltScreenOrder` pinned the old CUP-then-modes order. They now check the same restores (ascending, no 47/1047/1048/1049/2048/2026) just before the last CUP. `ModeRestores` also checks that nothing follows the CUP. `wrapped-replay.json` was regenerated (its mode bytes moved before the CUP), and its vitest passes unchanged.

### IN-10: the tail replay re-executes C0 controls the old emulator already ran inside the cut-off escape sequence

**Files modified:** `internal/relay/hub.go`, `internal/relay/altscreen_heap_test.go`
**Commit:** c67fdcef
**Applied fix:** before the tail replay, a tail that starts with ESC or 0x9b loses its C0 bytes other than ESC (CAN and SUB never reach a tail, because `incompleteTail` treats them as ground).

**Tests:** 4 subtests: ESC LF, CSI with BS, CSI with CR, and C1 CSI with BS. 3 fail on the pre-fix code; CR is idempotent and stays as a guard.

**One residual found along the way, recorded as a `shortcut:` comment:**
- vt's `ExecuteAction` sets `p.cmd = int(b)`, so a C0 inside a CSI overwrites the command bits collected so far, and the final byte is ORed in later. An LF turns a later `m` into `o`; BS and CR leave `m` intact.
- So vt fed `ESC[3 LF 1m` applies no SGR, while the rebuilt emulator (C0 dropped) applies SGR 31. xterm.js executes the C0 and keeps the sequence, so the rebuild follows xterm.js there.
- The rebuild cannot copy vt's corruption without re-running the C0. Shells rarely put a C0 inside a CSI.

### IN-11: the seed cannot re-create vt's zero-width cells

**Files modified:** `internal/relay/hub.go`
**Commit:** b022dbf2
**Applied fix:** comment only. The rebuild's `shortcut:` list now names vt's zero-width cells (an orphan combining mark, which the re-ingested seed merges into the previous cell or drops).

## Final status of every Phase 7 review finding

| ID | Status | Reason |
|---|---|---|
| CR-01 | Fixed (iteration 1) | |
| WR-01, WR-02, WR-03, WR-04 | Fixed (iteration 1) | |
| WR-05 | Fixed (iteration 2) | Claim corrected in iteration 3 |
| WR-06 | Fixed (iteration 2) | Its UTF-8 regression fixed as WR-09 |
| WR-07 | Fixed (iteration 2) | |
| WR-08 | Fixed (iteration 3) | a381513f |
| WR-09 | Fixed (iteration 3) | af591113 |
| IN-04 | Fixed (iterations 1 and 2) | |
| IN-07 | Fixed (iteration 1) | |
| IN-09 | Fixed (iteration 2) | |
| IN-10 | Fixed (iteration 3) | c67fdcef; vt's C0 command-bit quirk left as a `shortcut:` |
| IN-11 | Fixed (iteration 3, comment only) | b022dbf2 |
| IN-01 | Deferred | Necessity finding: keeping or removing the launch grace is the user's scope decision |
| IN-02 | Deferred | A byte budget needs a close hook from App.tsx's tab model; WR-01 already stores fewer snapshots |
| IN-03 | Deferred | A pending-promise handshake is a design change; the race costs only a full replay |
| IN-05 | Deferred | Needs a UX notice in the reconnect banner; the 32 KiB input limit stays |
| IN-06 | Deferred | WR-08 moved the modes before the CUP but still after the text, so `?2027` still follows the seed text; only emoji/ZWJ rows under `?2027` are affected |
| IN-08 | Deferred | SO-shifted line drawing and the OSC 8 link need extra probes; listed in the `shortcut:` comment |

## Gate (iteration 3)

The gate ran once, in the isolated worktree, with `GOTOOLCHAIN=go1.26.11`:
- `go build ./... && go vet ./...`: clean
- `go test -race -short ./...`: all 12 packages ok on the first run (root 8.5 s, attach 5.9 s, daemon 27.9 s, relay 66.9 s, webserver 20.8 s, tailnet 6.3 s)
- `pnpm vitest run`: 90 files, 1236 tests passed on the first run (run after the Go suite finished, not alongside it)
- `pnpm exec tsc --noEmit`: clean
- `bash tests/check-traceability-paths.sh`: OK, 147 paths
- `bash tests/check-brand.sh`: clean
- TESTING.md: commit 79d9bedc (Section 2 note; no new test files, so no Section 4 rows and the counts are unchanged)

The 32 KiB limits, share tokens, CSP and the WR-07 tailnet guard were not touched.

---

_Fixed: 2026-11-19T04:29:56Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 3_
