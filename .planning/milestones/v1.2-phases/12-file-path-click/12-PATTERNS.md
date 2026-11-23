# Phase 12: File Path Click - Pattern Map

**Mapped:** 2026-11-21
**Files analyzed:** 22 new/modified
**Analogs found:** 22 / 22 (two are role-match only: zsh shim script, pathLinks.ts provider)

All analog paths were checked with `git ls-files` and are tracked source. Line numbers are from the working tree as of this mapping.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match |
|---|---|---|---|---|
| `internal/previewfile/previewfile.go` (new) | utility | file-I/O | `cmd_open.go` lines 60-93, 159-241 (moved, not reinvented) | exact (move) |
| `cmd_open.go` (modify) | CLI command | request-response | itself; keeps `readErr/unsupportedErr/tooLargeErr` | exact |
| `internal/daemon/zshshim.go` (new) | service | file-I/O | `internal/daemon/clilink.go` | exact |
| `internal/daemon/zsh_zshenv` (new, go:embed) | config/script | event-driven | RESEARCH Pattern 1 script (verified on zsh 5.9) | no repo analog |
| `internal/daemon/engine.go` (modify, ~L234) | service | request-response | `Env: e.cliPathEnv()` at engine.go:234 | exact |
| `internal/daemon/types.go` (modify) | config | n/a | `PreviewAPI` constant and `APIVersion` | exact |
| `internal/relay/cwd.go` (new) | utility | transform | `internal/relay/hub.go` OSC 8 handler 1354-1357 plus RESEARCH `parseOSC7` | role-match |
| `internal/relay/pathclick.go` (new) | service | request-response | `internal/relay/preview.go` (`SendPreview`, `queueIfRoom`) | role-match |
| `internal/relay/hub.go` (modify, `newLiveEmulator`) | service | streaming | OSC 8 handler registration, hub.go:1354 | exact |
| `internal/relay/history.go` (modify) | model | CRUD | itself (`Push`, ring fields) | exact |
| `internal/relay/protocol.go` (modify) | model | request-response | `MsgInput/MsgPing` consts and `MetaPayload` | exact |
| `internal/relay/preview.go` (modify) | service | pub-sub | `Focus: sub == owner` at preview.go:209 | exact |
| `internal/relay/handoff_state.go` (modify) | model | batch | `HistoryState` / `RingState` structs | exact |
| `internal/relay/server.go` (modify, ~L178) | controller | request-response | `case MsgInput` / `MsgResize2` arms | exact |
| `internal/webserver/server.go` (modify, ~L959) | controller | request-response | same arms, `relay.`-prefixed | exact |
| `frontend/src/lib/pathLinks.ts` (new) | utility | event-driven | `TerminalPanel.tsx` WebLinksAddon handler 825-870 + `openLink.ts isModifierPressed` | role-match |
| `frontend/src/lib/relayClient.ts` (modify) | service | request-response | `sendInput`/`sendClaim` (L619-655), `handleMeta` (L525-550) | exact |
| `frontend/src/components/TerminalPanel.tsx` (modify) | component | event-driven | webLinks arm 825-870; banner render 1215-1236 | exact |
| `frontend/src/components/SessionEndedBanner.tsx` (modify) | component | request-response | `paste-interrupted` kind | exact |
| `frontend/src/components/__tests__/TerminalPanel.web-links.test.tsx` (modify) | test | n/a | lines 126-133 (edit in place) | exact |
| Go tests: `cwd_test.go`, `cwd_hub_test.go`, `pathclick_test.go`, `zshshim_test.go`, `previewfile_test.go` (new) | test | n/a | `internal/daemon/clilink_test.go`, `cmd_open_test.go` | role-match |
| vitest: `pathLinks.test.ts`, `TerminalPanel.pathClick.test.tsx`; `TESTING.md` rows | test/doc | n/a | existing `relayClient.test.ts`, TESTING.md Sections 2/4/5 | role-match |

## Pattern Assignments

### `internal/previewfile/previewfile.go` (utility, file-I/O) and `cmd_open.go`

**Analog:** `cmd_open.go` (move the file section; behaviour and CLI strings unchanged).

**Check order to preserve** (cmd_open.go:60-94): stat errors, IsDir, non-regular, size over cap, `O_NONBLOCK` open, descriptor re-check, `LimitReader(cap+1)`, grew-after-stat, `previewMIME`, `previewKey`.
```go
case fi.IsDir():
	return fmt.Errorf("portty: %q is a folder, not a file.", arg)
case !fi.Mode().IsRegular(): // a FIFO or device would block or never end
	return unsupportedErr(arg)
case fi.Size() > relay.PreviewMaxBytes:
	return tooLargeErr(arg, fi.Size())
...
f, err := os.OpenFile(arg, os.O_RDONLY|syscall.O_NONBLOCK, 0)
...
if fi, err := f.Stat(); err != nil { return readErr(arg, err) } else if !fi.Mode().IsRegular() { return unsupportedErr(arg) }
data, err := io.ReadAll(io.LimitReader(f, relay.PreviewMaxBytes+1))
if len(data) > relay.PreviewMaxBytes { return tooLargeErr(arg, int64(len(data))) }
mime := previewMIME(arg, data)
key, err := previewKey(arg)
```
Move also: `previewMIME` (159-179), `hasSVGRoot` (187-226), `previewKey` (231-241). `cleanPreviewName` lives in `internal/daemon/preview.go:87` (research). Return typed sentinels (not-exist, dir, not-regular, unsupported, too-large) and keep `readErr`/`unsupportedErr`/`tooLargeErr` in `cmd_open.go`, mapping sentinels to them. Leaf package: must not import `relay` or `daemon`, because `relay` imports it. `relay.PreviewMaxBytes` is therefore passed as the `maxBytes` argument (`Open(path, maxBytes)`); do not import `relay` from `previewfile`. `cmd_open_test.go` calls `previewMIME` (:261) and `previewKey` (:359) directly: move those tests or retarget the call sites; `TestCmdOpen_*` is the regression.

---

### `internal/daemon/zshshim.go` (service, file-I/O)

**Analog:** `internal/daemon/clilink.go` (whole file, 85 lines).

**Imports and executable seam** (clilink.go:3-21): `crypto/rand`, `encoding/hex`, `fmt`, `log`, `os`, `path/filepath`, `strings`.

**Env method returning nil on failure** (clilink.go:32-46):
```go
func (e *SessionEngine) cliPathEnv() []string {
	binDir := filepath.Join(e.configDir, "bin")
	if err := refreshCLILink(binDir); err != nil {
		log.Printf("cli link: %v", err)
		return nil
	}
	...
	return []string{"PATH=" + strings.Join(path, string(os.PathListSeparator))}
}
```
Copy as `zshEnv()`: `refreshZshShim()` then `log.Printf("zsh shim: %v", err); return nil`; on success return `[]string{"ZDOTDIR=" + dir, "PORTTY_ZSH_ZDOTDIR=" + os.Getenv("ZDOTDIR")}` (always pass the second entry, even empty).

**Atomic write via random-suffix temp + rename** (clilink.go:70-83):
```go
suffix := make([]byte, 8)
if _, err := rand.Read(suffix); err != nil { return err }
tmp := filepath.Join(binDir, ".portty-"+hex.EncodeToString(suffix))
...
if err := os.Rename(tmp, link); err != nil { _ = os.Remove(tmp); return err }
```
Adapt: `os.MkdirAll(dir, 0700)`, write temp file 0600 in the same dir, `Rename` to `.zshenv`, then `os.Stat` to verify it exists and is readable BEFORE returning (a ZDOTDIR without a readable `.zshenv` silently skips every user rc file). Embed the script with `//go:embed zsh_zshenv`. Skip the write when content already matches (analog's idempotent early return at :67-69).

**Wiring at engine.go:234** (only for the real shell, per research):
```go
Env:     e.cliPathEnv(), // portty on the session PATH (PREV-05)
```
Becomes `append(e.cliPathEnv(), e.zshEnv()...)` guarded by `cliPath == sessionShell` (engine.go:179 const `sessionShell = "/bin/zsh"`; `cliPath, args := e.spawnCommand()` at ~L190) so tests that spawn `/bin/cat` are untouched.

**Test analog** (`internal/daemon/clilink_test.go`): `TestCLILink_FailureStillSpawns` (:240-262; unwritable dir, session still spawns, `logs.has("cli link: ")`), `TestCLILink_ConcurrentRefresh` (:162), `TestCLILink_SurvivesZshLoginPathHelper` (:266; real zsh, dir with a space). Helpers: `captureLog(t)`, `newEnvEngine(t)`, `envSession(t, e)`, `fakeExe(t)`, `setCLIExecutable`. Reuse these for `zshshim_test.go`.

---

### `internal/daemon/zsh_zshenv` (config script)

**No repo analog.** Use the script in `12-RESEARCH.md` Pattern 1 (lines 178-213) verbatim as the starting point; it was run against zsh 5.9 across the failure matrix. External references only (Ghostty `.zshenv`).

---

### `internal/relay/hub.go` OSC 7 handler + `internal/relay/cwd.go`

**Analog:** hub.go:1354-1357 (registered inside `newLiveEmulator`, the only emulator constructor).
```go
emu.RegisterOscHandler(8, func(data []byte) bool {
	h.cs.osc8(data)
	return false
})
return emu
```
Add `emu.RegisterOscHandler(7, ...)` beside it. Return `false` so vt's default also runs. Parse `data` yourself (it includes the leading `7;`). Hub-owned state (like `cs`, `modes`, `wraps`), touched only under `emuMu`. Row formula and pruning rules are in RESEARCH Pattern 2. `parseOSC7` code: RESEARCH "Code Examples". `cwd.go` itself is pure and unit-testable without an emulator.

**`history.go` additions:** `History` struct (history.go:19-26: `lines`, `wrapped`, `head`, `bytes`, caps; "touched only under Hub.emuMu"). Add `pushed uint64` incremented in `Push`; add snapshot accessors for lines/wrap flags. `Clear` must not reset `pushed`.

**`handoff_state.go` (additive gob fields):** follow the `HistoryState` struct style (handoff_state.go ~L80-86) and add `Cwds []CwdRecord` and `HistoryPushed uint64` to `HubState`; on import with no `HistoryPushed`, set `pushed = len(lines)`. Add a round-trip assertion in the existing handoff state test.

---

### `internal/relay/protocol.go` (model)

**Analog:** protocol.go:12-27 constants and 86-98 `MetaPayload`.
```go
MsgInput   byte = 0x10 // Client keyboard input → PTY stdin
MsgResize2 byte = 0x11
MsgPing    byte = 0x12 // Keep-alive ping
...
type MetaPayload struct {
	ViewerCount *int    `json:"viewerCount,omitempty"`
	Replay      *string `json:"replay,omitempty"`
	Offset      *uint64 `json:"offset,omitempty"`
}
```
Add `MsgPathClick byte = 0x13` next to `MsgPing`; add `PathClick *bool \`json:"pathClick,omitempty"\`` and `Notice *string \`json:"notice,omitempty"\`` (pointer fields, same comment style as `Replay`/`Offset`). Add a `MakePathClickFrame` modeled on `MakeInputFrame` (protocol.go:40-46).

---

### `internal/relay/preview.go` (focus override) and `internal/relay/pathclick.go`

**Analog:** preview.go:175-220 `SendPreview`.
```go
id, owner := h.previewID, h.owner   // preview.go:202
...
header := MakePreviewHeader(PreviewHeader{ID: id, Name: p.Name, MIME: p.MIME, Size: len(p.Data), Key: p.Key, Focus: sub == owner})
```
Add `FocusSub *Subscriber` to `Preview`; when non-nil use `Focus: sub == p.FocusSub`, else the existing owner rule. Do NOT call `NoteTyping` (it claims the grid).

**Unicast helper:** `queueIfRoom(sub, frame)` (preview.go:282-289, caller holds `h.mu`, never blocks, never kicks). Use it for the `Notice` meta to ONE subscriber; never `broadcast`/`BroadcastMeta`.

**Guards to copy:** busy handling mirrors `h.previewBusy` under `h.mu` (preview.go:184-187); add an atomic CAS `pathClickBusy`. Log style `log.Printf("viewer-diag: ... session=%s ...", DiagID(h.sessionID), ...)` (preview.go:205); log outcome codes and sizes only, never the path or text.

**Source gate:** `TestPreview_SourceGate` (`internal/daemon/preview_test.go:335`) forbids `os`, `io/fs`, `path/filepath` in `internal/daemon/preview.go` and `internal/relay/preview.go`. Leave it unchanged. Put the file-system-touching logic only in `previewfile` and `relay/pathclick.go`, and add a second gate assertion.

---

### `internal/relay/server.go` and `internal/webserver/server.go` (read pumps)

**Analog:** relay/server.go:174-205 and webserver/server.go:955-1010. Add the arm next to `MsgInput`; no `default` arm exists by design.
```go
// 0x30-0x37 are RETIRED (...): no case and no default arm, so old clients' frames are dropped.
switch msgType {
case MsgInput:
	if !sub.ReadOnly { // MC-03: discard input for read-only clients
		...
	}
case MsgResize2:
	if cols, rows, claim, ok := ParseResize(payload); ok && !sub.ReadOnly { ... }
case MsgPing:
	pongs.Owe()
}
```
New arm (both files; webserver uses `relay.` prefixes):
```go
case MsgPathClick:
	if !sub.ReadOnly { // refuse in the pump AND again at the top of Hub.PathClick
		hub.PathClick(sub, payload) // returns at once; resolver runs in its own goroutine
	}
```
`PathClick` must never do disk I/O on this goroutine (a slow disk stops pong handling).

**Capability bit:** set `PathClick: &true` in the `MakeMeta(MetaPayload{Replay: &mode})` built in `Replay.Frames()` (hub.go) when `!sub.ReadOnly`.

**Version:** bump `APIVersion` to 4 and add `PathClickAPI = 4` beside `PreviewAPI` in `internal/daemon/types.go`; `internal/daemon/client_test.go:47` pins `DaemonAPI(h) != 3 || APIVersion != 3 || PreviewAPI != 3` and must change.

---

### `frontend/src/lib/relayClient.ts` (service)

**Analog:** `sendClaim` (relayClient.ts:650-655) for a dropped-unless-open sender; `handleMeta` (:525-550) for meta handling.
```ts
sendClaim(cols: number, rows: number): void {
  if (this.reconnect && this.state !== 'open') return
  if (this.ws?.readyState !== WebSocket.OPEN) return
  this.send(encodeClaimFrame(cols, rows))
}
...
const { replay, offset } = m as { replay?: unknown; offset?: unknown }
```
Add `export const MSG_PATH_CLICK = 0x13` beside `MSG_INPUT` (:9), `sendPathClick(text)` with the same two guards (never queued), destructure `pathClick` and `notice` in `handleMeta` (accept only `pathClick === true`; map `notice` by exact string compare to a fixed kind), and clear the capability in `ws.onclose` next to `onPreviewReset`. Frame bytes for tests: share a JSON fixture with Go, as Phase 11 did (`frontend/src/lib/__tests__/fixtures/preview-frames.json`).

---

### `frontend/src/lib/pathLinks.ts` (utility, event-driven)

**Analog:** `TerminalPanel.tsx` 825-870 (modifier gate) plus `openLink.ts` `isModifierPressed` (:30-41).
```ts
const modifier = (cfg?.modifier || 'platform') as ModifierMode
if (!isModifierPressed(event, modifier)) return
```
Use that exact gate in `activate`. Use the provider skeleton in RESEARCH "Provider skeleton" and the detection contract in UI-SPEC "What counts as a path link". Pure `findPathLinks`/`stripLineCol` exported for table tests. No existing `registerLinkProvider` usage in the repo, so the provider shape comes from xterm 6 typings only.

---

### `frontend/src/components/TerminalPanel.tsx` (component)

**Analog:** webLinks arm at TerminalPanel.tsx:825-870 (`if (pluginConfig?.webLinks) { if (!webLinksAddonRef.current) {...} } else {...}`), with `webLinksAddonRef` as the lifecycle model. Register the path provider in that arm, hold an `IDisposable` ref, dispose in the else-branch and on unmount; make `provideLinks` return `undefined` unless a `pathClickEnabledRef` is true (set from the host's `pathClick` bit, cleared on close). Keep `registerLinkProvider` out of this file (it lives in `lib/pathLinks.ts`).

**Banner slot** (TerminalPanel.tsx:1215-1233): ended banner, then `pasteNotice`, then reconnecting. Add the path notice as its own state (`pathNotice`) rendered LAST in the slot, only when no other banner is showing; 5000 ms timer restarting on replacement:
```tsx
{pasteNotice && (
  <SessionEndedBanner kind="paste-interrupted" pasteSentKB={...} pasteTotalKB={...}
    onDismiss={() => setPasteNotice(null)} />
)}
```

### `frontend/src/components/SessionEndedBanner.tsx`

**Analog:** the `paste-interrupted` kind. Extend the `kind` union (line ~62) with `'path-unsupported' | 'path-too-large'`; add the two fixed strings to the nested ternary in the message span (:84-95); the dismiss condition (`kind !== 'reconnecting' && kind !== 'unreachable'`) stays. Copy is fixed (UI-SPEC): "Can't preview this file type. Portty previews images, PDFs and Markdown files." and "Too large to preview. The limit is 20 MB." Straight apostrophes. The wire value is never rendered.

### `frontend/src/components/__tests__/TerminalPanel.web-links.test.tsx`

**Edit in place** (lines 126-133). Current assertion: `expect(src).not.toContain('registerLinkProvider')` and `not.toContain('getHyperlinkId')`. Keep `getHyperlinkId` absent, rename the test (names Phase 12), assert `TerminalPanel.tsx` imports `pathLinks`, and assert `registerLinkProvider` appears only in `src/lib/pathLinks.ts`. Also add `registerLinkProvider = vi.fn(() => ({ dispose: () => {} }))` to the six `class MockTerminal` test mocks (RESEARCH Pitfall 10).

## Shared Patterns

### Read-only gate (apply at three layers)
**Source:** relay/server.go:178-179, webserver/server.go:959-961 (`if !sub.ReadOnly { // MC-03 ...`), plus `relay.Subscriber.ReadOnly` (hub.go:32). Check in both pumps AND at the top of `Hub.PathClick`; one test per layer with `Subscriber{ReadOnly: true}`.

### Failure never blocks a session spawn
**Source:** `clilink.go:32-37` (log, return nil) with `TestCLILink_FailureStillSpawns` (clilink_test.go:240). Apply to `zshEnv()`. Never set `ZDOTDIR` unless the shim was written and verified.

### Never log paths or typed text
**Source:** preview.go:205 `viewer-diag:` lines (counts only). Apply to all of `pathclick.go`.

### Non-kicking unicast
**Source:** `queueIfRoom` (preview.go:282). Apply to the notice; never `broadcast`.

### Fixed copy for wire-derived UI
**Source:** `SessionEndedBanner.tsx` header comment, "the wire reason is still never rendered". Apply to both notice kinds.

### Regression suite bookkeeping (project CLAUDE.md)
Update `TESTING.md`: Section 2 counts and a new Category BB; Section 4 traceability rows for PATH-01/PATH-02 (repo-relative `.go/.ts/.tsx/.sh` paths only); Section 5 manual items from M-128. Run `bash tests/check-traceability-paths.sh` before committing.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `internal/daemon/zsh_zshenv` | shell script | event-driven | No shell-integration script exists in the repo; use RESEARCH Pattern 1 (verified under real zsh 5.9) |
| xterm `registerLinkProvider` usage in `pathLinks.ts` | provider | event-driven | The repo uses only `WebLinksAddon`; provider API shape comes from `@xterm/xterm` 6.0.0 typings (`xterm.d.ts:1393-1453`) and RESEARCH Pattern 7 |
| Absolute-row cwd log and token-boundary printed-check | resolver | transform | New logic; RESEARCH Patterns 2 and 5 are the spec, hub `History` is the only data source |

## Metadata

**Analog search scope:** repo root `cmd_open.go`, `internal/daemon/`, `internal/relay/`, `internal/webserver/`, `frontend/src/{lib,components}`.
**Files read:** about 14 (targeted line ranges).
**Pattern extraction date:** 2026-11-21
**Caveat for the planner:** `hub.go` OSC handler site, `types.go`, `handoff_state.go` struct location and the TerminalPanel `class MockTerminal` test files were located from RESEARCH.md line references plus spot reads; confirm line numbers with `graft` or grep before editing.
