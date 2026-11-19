---
phase: 07-hardening
reviewed: 2026-11-19T03:12:32Z
depth: deep
iteration: 2
files_reviewed: 15
files_reviewed_list:
  - internal/relay/hub.go
  - internal/relay/altscreen_heap_test.go
  - internal/relay/emu_nohang_test.go
  - internal/relay/querystrip_test.go
  - internal/daemon/engine.go
  - internal/daemon/engine_test.go
  - internal/daemon/testmain_test.go
  - frontend/src/lib/peerSnapshots.ts
  - frontend/src/lib/__tests__/peerSnapshots.test.ts
  - frontend/src/components/TerminalPanel.tsx
  - frontend/src/components/__tests__/TerminalPanel.tabSwitch.test.tsx
  - frontend/src/components/__tests__/TerminalPanel.scale.test.tsx
  - frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx
  - frontend/src/components/__tests__/TerminalPanel.guestClose.test.tsx
  - TESTING.md
findings:
  critical: 0
  warning: 3
  info: 2
  total: 5
  necessity: 0
status: issues_found
---

# Phase 7: Code Review Report (iteration 2)

**Reviewed:** 2026-11-19T03:12:32Z
**Depth:** deep
**Files Reviewed:** 15
**Status:** issues_found

## Summary

Scope: `git diff b3c869a9..HEAD -- . ':!.planning'` (fix commits 322ac362..028adbe6), checked against the iteration-1 findings and 07-REVIEW-FIX.md. I also read the connected code: the pinned `charmbracelet/x/vt@v0.0.0-20260615092313-b57e5e6d29bb` (every `e.pw` write, `setMode`/`setAltScreenMode`, the DECSTBM/DECSC handlers, `Screen.SaveCursor`, `fullReset`), Go's `io.Pipe` close semantics, `@xterm/xterm` 6.0.0's DECSET/DECRST/DECSTBM/DECSTR handlers, and `@xterm/addon-serialize` 0.14's `_serializeModes` and `_serializeString`. I probed the rebuild with a throwaway test in `go test -overlay`, so no source file changed. The probe fed 3,000 random shell-output streams with random read cuts into the hub and into a reference emulator.

Verified:
- **CR-01 is fixed.** All 14 `e.pw` writes in vt discard the error (`_, _ =` or `//nolint:errcheck`). `PipeWriter.Close` closes `p.done`, so every later write returns `ErrClosedPipe` at once: no block, retry, log loop or panic. `Resize`'s `?2048` write is never reached, because the hub rebuilds instead of resizing. The only production `xvt.NewEmulator` call is inside `NewQuietEmulator`. Its callers are `newLiveEmulator` (eager build, `ensureLiveEmulatorLocked`, the resize rebuild, the alt-exit rebuild) and `engine.go:513`. The webserver and preview paths build no emulator. The #100 race (drain goroutine `Read` vs `Close` on `e.closed`, commit 1376705e) can't come back. Nothing calls `emu.Read` or `emu.Close` any more, and the pipe is closed once, synchronously, before the emulator is published.
- **Lock order holds.** The rebuild and the WR-03 resize path take only `emuMu`, and nothing new calls `Cols()`/`Rows()` under it. `mainRegion`/`savedCursor` are touched only by handlers that run inside `Write` under `emuMu`, by the RIS handler, and by the resize path.
- **The WR-01 hooks are correct.** Every handler returns `false`, so xterm still applies the sequence. Disposal happens in both cleanup branches. The DECSTBM, DECSTR and RIS semantics match xterm 6.0.0's `setScrollRegion`, `softReset` and `reset`. The `?1006`/`?1016` reset-to-DEFAULT matches `resetModePrivate`. `serialize()` already restores the pen (`_curAttrData` diff at the end), so the appended `?25l`/`?1006h` are the only missing modes and can't contradict anything serialize emits. `groundTracker` errs conservative: where it disagrees with xterm's parser it reports "not ground" and vetoes the snapshot. It never reports ground mid-sequence.
- **Security is unchanged.** The diff doesn't touch `relay/server.go`, `webserver/`, `capability_mw.go`, `csp_mw.go` or `relayClient.ts`. The 32 KiB input limit, share tokens and CSP are as before.
- **Gate:** `go build ./...` and `go vet` (relay, daemon) are clean. `go test -race` passes the CR-01/WR-02/WR-03/IN-04 tests and the styled-tail test. vitest passes `peerSnapshots` and `TerminalPanel.tabSwitch` (24 tests). `check-traceability-paths.sh` passes (147 paths).

New problems: the WR-02 re-seed is incomplete in reachable cases (WR-05, WR-06), and, raised by the coordinator and confirmed here, the Go test suites talk to the user's real tailnet (WR-07).

## Original findings: status

| ID | Title | Status | Notes |
|---|---|---|---|
| CR-01 | Unstripped query wedges the hub and styled tail | **Fixed** | Verified against vt source and by the 4 + 3 regression subtests. |
| WR-01 | Peer-tab restore drops modes and restores torn state | **Fixed** | Hooks verified against xterm 6.0.0. IN-09 is a minor conservative gap. |
| WR-02 | Alt-exit rebuild loses emulator state | **Partially fixed** | Correct for the probe cases. WR-05 and WR-06 are reachable gaps the fix introduced or left. |
| WR-03 | Resize in alt screen bypasses HARD-01 | **Fixed** | Main rows reach history and the app's later exit rebuilds. The new emulator's main screen is blank until the shell redraws, consistent with the main-screen resize design. |
| WR-04 | HARD-01 bound is per session and scales with width | **Documented, risk accepted** | Comment only, as instructed. The memory cost itself stands. |
| IN-01 | Respawn grace guards an unreproduced blip | Deferred | User's scope decision. |
| IN-02 | `peerSnapshots` caps entries, not bytes | Deferred | WR-01's veto stores fewer snapshots. The byte cap is still open. |
| IN-03 | Deferred put races a quick remount | Deferred | Costs a full replay, never wrong content. |
| IN-04 | Replay strip lets some answers through | **Partially fixed** | `OSC 1x;?;?` and `CSI 13/14/16/18/19/21 t` are now stripped. Still open: (a) a query split across two ring frames (`ESC [ SOH 6 n`); (b) `CSI 11 t`, `15 t` and `20 t` (20 t is the icon-label echo-back twin of 21 t). The deferral reason for (a), "changes replay byte offsets", contradicts `hub.go:1087-1088`: the client takes its position from the offset meta and never counts bytes. |
| IN-05 | Split paste loses atomicity on drop | Deferred | |
| IN-06 | Seed writes `?2027` after the text | Deferred | |
| IN-07 | testmain ignores errors, skips cleanup | **Fixed** | The defer is registered before the Setenv panics, so it runs on them too. |

## Narrative Findings (AI reviewer)

## Warnings

### WR-05: WARNING: the rebuild's pen/charset probe is swallowed when the read ended inside a DCS introducer or mid-UTF-8, so the rebuilt emulator gets the wrong pen or G0

**File:** `internal/relay/hub.go:651-654`
**Issue:** `rebuildAfterAltExitLocked` writes `ESC[1;1H` and `q` into the old emulator. It does this while the old parser is still in whatever state the read's `incompleteTail` left it, which is the very state the fix replays into the new emulator. In several of those states the probe never lands. I fed `ab ESC[1;31m cd` plus the tail, then the probe, into the pinned vt:

| Tail left in the old parser | Probe position | Cell read | Pen read |
|---|---|---|---|
| none, `ESC`, `ESC[3`, OSC, `ESC P +`, APC/PM/SOS, `9b 31` | (0,0) | `q` | bold red (correct) |
| `ESC P` or `0x90` (bare DCS introducer) | (4,0), the CUP was swallowed | blank | **default** |
| `e2 94` (mid-UTF-8) | `[1;1H` printed as text | `H` (or the next row if pending wrap) | correct only by luck; the G0 check sees `H`, not `─` |

Failure scenario: a script exits `less` and prints, the 32 KiB read ends exactly on `ESC P` (any DCS: XTGETTCAP, DECRQSS, a sixel image), and the shell pen is bold or coloured. The rebuilt emulator prints everything after it in the default pen. Every later rendered replay and `h.history` carries that, until the shell sets SGR again. The UTF-8 case loses DEC line drawing in the same way. In the 3,000-stream fuzz run with DECRC excluded, 4 of the 6 mismatches I inspected were this case. The other 2 were the pending wrap in WR-06.
**Fix:** Abort any in-progress sequence before probing. CAN returns vt's parser to ground from every state, which I verified for all 16 tails above. The old emulator is discarded, so the abort costs nothing:
```go
old.Write([]byte("\x18\x1b[1;1H")) //nolint:errcheck // CAN: probe from ground whatever the tail left
```
Add `ESC P` and mid-UTF-8-with-pen cases to `TestAltScreenExit_RebuildMatchesReference`.

### WR-06: WARNING: the re-seed runs its saved-cursor block on every `?1049` exit, and that block drops the pending wrap, saves the wrong pen and uses absolute CUPs under origin mode

**File:** `internal/relay/hub.go:659-672` (with `:730-734`)
**Issue:** The `AltScreen(false)` callback sets `h.savedCursor` whenever `?1049` was in use, which covers vim, less, htop and Claude Code's TUI pickers. So the `if r != [2]int{} || s != nil` branch runs after almost every app exit, not only for "shells that set a region" as the `shortcut:` comment implies. That branch has three defects:
1. **Pending wrap is cleared.** `renderScreenLocked` re-prints the last-column cell to recreate the deferred wrap. The trailing `CUP pos` then clears it. Fuzz cases: the read after the exit ends with the cursor in pending wrap, so the next character overwrites the last column instead of wrapping to the next line.
2. **DECSC saves the default pen.** `ESC 7` runs before `cell.Style.String()` is written, so vt's saved cursor holds the seed's pen. vt's `?1049h` saved the main cursor *with its pen*, so after a later `ESC 8` the reference keeps (say) bold green and the rebuilt emulator drops to default. This caused most of the fuzz mismatches: taking the `ESC 8`/`CSI u` pieces out dropped them from 160 to 11. The comment documents "DECSC's saved pen" as lost, but here the loss comes from the order of the writes, not from a vt limit.
3. **Origin mode.** The seed restores `?6h` from `h.modes` earlier in the same write. `CUP s` and `CUP pos` are absolute positions but run region-relative (vt `setCursor(..., margins=true)`). Probe: with `ESC[3;5r ESC[?6h`, the rebuilt cursor is (3,4) where it should be (3,3), and `ESC 8` lands one row low.

Failure scenario (2, the common one): the shell saves the cursor with `ESC 7` (or a prompt framework uses `tput sc`/`rc`) under a coloured pen. The user runs `less` and quits. The next `ESC 8` drops the colour in the emulator, and every later rendered replay shows the wrong colours.
**Fix:** Reorder, and neutralise DECOM around the absolute CUPs:
```go
if cell != nil { // pen and G0 first, so ESC 7 saves them as vt's ?1049h did
	if cell.Content == "─" { b.WriteString("\x1b(0") }
	b.WriteString(cell.Style.String())
}
if r, s := h.mainRegion, h.savedCursor; r != [2]int{} || s != nil {
	om := h.modes[6]
	if om { b.WriteString("\x1b[?6l") }
	if r != [2]int{} { fmt.Fprintf(&b, "\x1b[%d;%dr", r[0], r[1]) }
	if s != nil { fmt.Fprintf(&b, "\x1b[%d;%dH\x1b7", s.Y+1, s.X+1) }
	fmt.Fprintf(&b, "\x1b[%d;%dH", pos.Y+1, pos.X+1)
	if om { b.WriteString("\x1b[?6h") } // vt's DECOM set does not move the cursor
	// re-print the pending-wrap cell here (factor renderScreenLocked's block into a helper),
	// then re-emit cell.Style.String() so the pen stays the probe's
}
```
Also narrow the `shortcut:` comment to what is still lost: tab stops, G1-G3/SO, a hyperlink (IN-08), and a DECSC pen that differs from the current one.

### WR-07: WARNING: the Go test suites reach the user's real tailscaled and probe every peer on the tailnet (raised by the coordinator; confirmed with one correction and two more paths)

**File:** `internal/daemon/api.go:84-90`; `cmd_cli.go:164-168`; `cmd_cli_test.go:48,73,89,110,269,310`; `app_test.go:608-609`; `internal/tailnet/tailnet_test.go:440,466`
**Issue:** `NewAPI` wires production discovery as the default for every API a test builds. `discoverPeers = tailnet.DiscoverPeers`, a LocalAPI status call to tailscaled. `probePeers` goes through `tailnetCache.getOrRefresh(ctx, tailnet.DiscoverAndProbe)`, which sends an HTTPS GET to every online peer on `DefaultProbePort`, five at a time. Tests that don't call `SetPeerSeamsForTest` therefore run real discovery. Point by point:
1. **`api.go` default: confirmed.** The defaults are at `:84` and `:87-89`. Each `NewAPI` gets a fresh `tailnetCache`, so every test re-runs discovery. The 30 s TTL doesn't help.
2. **`cmd_cli_test.go`: confirmed, plus one more line.** `testSetup` (`:20-43`) builds `daemon.NewAPI` with no peer seam. `cmdList` without `--local` calls `client.ListTailnetPeers()` → `GET /tailnet/peers` → `a.probePeers`. That covers lines 48, 73, 89, 110 and 269, and also **310** (`TestCmdList_JSON_WithHostField`, `--json`). When a Portty peer answers, `cmdList` also calls `tailnet.FetchAllPeerSessionsMeta` (`cmd_cli.go:168`). That is a session-list request to the user's other Mac.
3. **`peer_connect_test.go:130`: refuted.** `newConnectEnv` (`:72-78`) already stubs `probe`, and `SetPeerSeamsForTest`'s nil arguments keep the current seam (`peer_connect.go:33-43`). The FQDN also matches the stubbed discovery, so the unknown-host branch at `peer_connect.go:151` never runs there. `TestPeerConnect_UnknownHostListsProbed` gets the stubbed list. `cmd_attach_test.go:607` stubs discovery and fails before `probePeers` runs.
4. **`tailnet_test.go:440` `TestDiscoverPeers_Public`: confirmed.** It makes a real LocalAPI status call to the local tailscaled and reads the user's peer list. Nothing leaves the host, but the test isn't hermetic.
5. **Additional: `app_test.go:608-609`.** Case 3 of `TestGetRemoteSessionsWithMeta_ReachableField` uses `testApp(t)` (no peer seam). `GetRemoteSessionsWithMeta` runs `ListTailnetPeers` (full probe) and then `FetchAllPeerSessionsMeta` against every Portty peer found (`app.go:1108-1115`).
6. **Additional, minor: `tailnet_test.go:466`.** `TestProbePeer_Public` resolves `unreachable.example.invalid`. With MagicDNS this DNS query goes to tailscaled's resolver. It doesn't reach a peer.

Other tailscaled paths I checked are already seamed. The share flow (`webserver.CheckHealth`, `local.Client.CertPair`) is replaced by `SetShareSeamsForTest` in every share test (`testShareSetup`, `share_test.go`, `app_test.go:275`). `api_test.go:372-419` pre-fills `tailnetCache`.

Failure scenario: `go test ./...` on the office or home Mac. The test binaries connect to every device on the tailnet, which is what the firewall shows. When the other Mac runs Portty, the tests also fetch its session list over the network. Results depend on which Macs are online, and the tests leak who the test host can see. HARD-06 intended that tests never touch host state, and this contradicts it.
**Fix (structural, not per call site):** Refuse live tailnet access by default in any test binary, at the lowest choke points, so no call site can forget a seam:
```go
// internal/tailnet/live.go
// ErrLiveTailnetInTest is returned instead of touching tailscaled or a peer
// from a test binary. Set PORTTY_LIVE_TAILNET_TESTS=1 for a deliberate integration run.
var ErrLiveTailnetInTest = errors.New("tailnet: live tailscaled/peer access disabled in tests")

func liveAllowed() bool {
	return !testing.Testing() || os.Getenv("PORTTY_LIVE_TAILNET_TESTS") == "1"
}
```
Guard every entry point that builds a `local.Client` or dials a peer with a default client: `DiscoverPeers` (return `nil, ErrLiveTailnetInTest`), `ProbePeer` (return false), the default-client `FetchPeerSessionsMeta`/`FetchAllPeerSessionsMeta` (an unreachable group), and in `webserver`, `CheckHealth`, the whois `local.Client` and `defaultWarmCert` in `internal/daemon/share.go`. `testing.Testing()` (Go 1.21+, module is on 1.26) is true in every test binary, including `package main`'s root tests and external `_test` packages, which a `TestMain` in one package can't reach. Importing `testing` only for `Testing()` registers no flags. The unexported seams the unit tests use (`discoverPeers(ctx, fakeStatus)`, `probePeer(ctx, peer, client)`, `httptest` clients via `SetPeerClientForTest`) bypass the guard, so seamed tests keep working. `cmdList` and `GetRemoteSessionsWithMeta` then see "Tailscale off" and fall back to local-only, which is what those tests assert. Add one test that sets nothing and asserts `DiscoverPeers` returns `ErrLiveTailnetInTest`, so a later refactor can't remove the guard. Optionally, still stub `testSetup`/`testApp` explicitly for readability.

## Info

### IN-08: The charset probe misreads SO-shifted line drawing as G0, and an open OSC 8 hyperlink is dropped

**File:** `internal/relay/hub.go:653`, `:668-671`
**Issue:** If GL is G1 (after SO) with G1 = DEC line drawing, the probe `q` renders `─` and the rebuild emits `ESC(0`, which designates **G0**. After the shell's SI, the reference emulator returns to ASCII, but the rebuilt one keeps drawing `qqq` as `───` until an `ESC(B`. Separately, an OSC 8 hyperlink still open at the exit is lost: the probe shows the pen, but `Style.String()` carries no link. The probe confirmed that the rebuilt emulator renders `link` with no hyperlink. Both are rare with `TERM=xterm-256color`.
**Fix:** Emit `ESC(0` only when GL is G0. vt doesn't expose GL, so also probe after `SI` (`\x0f`), or document SO alongside G1-G3. Re-open the link with `cell.Link` (`\x1b]8;%s;%s\x07`) when it is non-empty.

### IN-09: `groundTracker` isn't reset on the in-band reset, so a torn stream before a reconnect can veto the next snapshot needlessly

**File:** `frontend/src/components/TerminalPanel.tsx:456`, `frontend/src/lib/peerSnapshots.ts:111-114`
**Issue:** `onReset` writes `ESC c` through `term.write` and never through `peerWatch.feed`. The RIS hook resets the modes, but not the byte tracker. If the socket dropped mid-sequence, the tracker stays non-ground into the full replay. It re-syncs at the first ESC, so it never reports ground wrongly. A short plain-text replay can still leave it non-ground, which costs that tab its HARD-05 delta on the next switch.
**Fix:** `onReset: () => { peerWatch?.feed(new Uint8Array([0x18, 0x1b, 0x63])); resetForFullReplay(term) }`, or expose a `resetGround()` that the RIS hook calls.

---

_Reviewed: 2026-11-19T03:12:32Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
_Iteration: 2_
