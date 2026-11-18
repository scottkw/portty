# Phase 5: Overnight Survival - Pattern Map

**Mapped:** 2026-11-17
**Files analyzed:** 22 (new + modified, incl. tests/docs)
**Analogs found:** 21 / 22

Most of this phase modifies existing files, so in most cases the "analog" is the file's own current code. The excerpts below show the exact lines to change and the conventions to keep.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/relay/history.go` (NEW) | model (bounded ring) | transform | `internal/relay/scrollback.go` | exact |
| `internal/relay/heartbeat.go` (NEW) | utility | event-driven (ticker) | `cmd_attach.go:291-321` ticker goroutine + RESEARCH Pattern 1 | role-match |
| `internal/relay/hub.go` | service | streaming | self (`recordFrame` :408, `resizeLiveEmulator` :459, `EnsureLiveEmulator` :536, `ReconnectPreamble` :626) | exact |
| `internal/relay/scrollback.go` | model | streaming | self (`Append`/`Truncated`/`Snapshot`) | exact |
| `internal/relay/protocol.go` | config (wire consts) | request-response | self (const block :11-22, `MetaPayload` :62) | exact |
| `internal/relay/close_reasons.go` | utility | — | self | exact |
| `internal/relay/server.go` | controller (WS) | streaming | self (:90-135 join, :169 MsgPing, write pump) | exact |
| `internal/webserver/server.go` | controller (WS) | streaming | `internal/relay/server.go` (mirror) + self :739-930 | exact |
| `internal/daemon/remote_ws_proxy.go` | middleware (proxy) | streaming | self (:86 RawQuery, :116-118, :126-146) | exact |
| `internal/attach/attach.go` (+ possible new `internal/attach/loop.go`) | service (client loop) | streaming | self (`AttachSession` :40, `StdinPump` :71, `WsOutputPump` :120) | exact |
| `cmd_attach.go` | controller (CLI) | request-response | self (`runAttach` :230-330) | exact |
| `frontend/src/lib/relayClient.ts` | service (client) | streaming | self | exact |
| `frontend/src/lib/closeReasons.ts` | utility | — | self + `internal/relay/close_reasons.go` | exact |
| `frontend/src/components/ReconnectingBanner.tsx` (NEW, or a `kind` on SessionEndedBanner) | component | event-driven | `frontend/src/components/SessionEndedBanner.tsx` | exact |
| `frontend/src/components/TerminalPanel.tsx` | component | event-driven | self (:227-235 state, :373-400 RelayClient wiring, :1002 banner render) | exact |
| `frontend/src/components/WebShareSessionView.tsx` | component | request-response | self (:80-90 wsURL) | exact |
| `frontend/src/App.tsx` | component (root) | request-response | self (:760-775 `/info` probe, :791 `ConnectPeerSession`, :327/:832 `GetRelayPort`) | exact |
| `internal/relay/history_test.go`, `heartbeat_test.go`, hub tests | test | — | `internal/relay/hub_test.go`, `scrollback_altscreen_test.go` | exact |
| `internal/daemon/attach_viewers_test.go` (extend) | test (integration) | streaming | self (`newAttachScene` :187, `viewerTap` :79, `dialCLI` :274) | exact |
| `frontend/src/lib/relayClient.test.ts`, `TerminalPanel.guestClose.test.tsx` (rewrite) | test | — | self (MockWebSocket :102-128) | exact |
| `TESTING.md` | docs | — | existing §2/§4/§5 rows (M-84..M-87 next) | exact |
| Blackhole TCP forwarder test helper | test utility | streaming | none | no analog |

## Pattern Assignments

### `internal/relay/history.go` (NEW, ring of rendered lines)

**Analog:** `internal/relay/scrollback.go` (whole file, 80 lines)

Copy the shape: package doc comment per const, mutex-protected struct, constructor with max, `Append`-style trim-from-front, defensive copy on snapshot.
```go
// scrollback.go:27-60
type Scrollback struct {
	mu        sync.Mutex
	buf       []byte
	max       int
	truncated bool // true once any oldest bytes have been dropped (ring wrapped)
}
func NewScrollback(maxBytes int) *Scrollback { ... }
func (s *Scrollback) Append(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf = append(s.buf, data...)
	if len(s.buf) > s.max {
		excess := len(s.buf) - s.max
		copy(s.buf, s.buf[excess:])
		s.buf = s.buf[:s.max]
		s.truncated = true
	}
}
```
New: `Push(line string)` trims by both `HistoryMaxLines` (10_000) and `HistoryMaxBytes` (4 MiB); `Clear()` (ED3/RIS); render joined with `"\r\n"` (RESEARCH Pattern 5). History is only touched under `hub.emuMu`, so its own mutex is optional; document whichever is chosen.

The scrollback.go `ViewerReadLimit` doc comment (:8-25) says "the host sends the catch-up preamble as one message". Update it when replays are chunked (keep the 1 MiB limit as a backstop).

---

### `internal/relay/scrollback.go` (offset counter)

Add an `end uint64` field incremented in `Append` by `len(data)`, plus `SnapshotFrom(off) (bytes, end, ok)` using `start = end - len(buf)`. Keep framed bytes, because `engine.go:472,532` read `ScrollbackSnapshot()` as framed. Keep `Truncated()` (the raw-intact path is the #109 fix).

---

### `internal/relay/hub.go` (service, streaming)

**Three emulator construction sites, all must switch to one `newLiveEmulator(cols, rows)` helper** (Pitfall 12):
```go
// hub.go:424-426 (recordFrame eager build)
emu := xvt.NewEmulator(h.Cols(), h.Rows())
emu.SetScrollbackSize(liveEmulatorScrollbackLines)
h.liveEmu = emu
// hub.go:465-467 (resizeLiveEmulator), hub.go:548-549 (EnsureLiveEmulator): same two lines
```
`const liveEmulatorScrollbackLines = 50` is at :107; the doc comment at :100-106 must be rewritten (drained after every write).

**Drain point:** `recordFrame` (:408-431) ends with
```go
clean := liveEmuQueryStripPattern.ReplaceAll(raw, nil)
h.liveEmu.Write(clean) //nolint:errcheck
```
Add the scrollback drain right after it (RESEARCH Pattern 4 code, including `clear(ls[:cap(ls)])` before `sb.Clear()`, Pitfall 2).

**Atomicity:** `Run` (:363-385) currently does
```go
h.recordFrame(frame, buf[:n])
h.broadcast(frame)
```
as two steps. Hold `emuMu` across both (lock order emuMu → hub.mu, as documented at :422-423 and :455-458), and add `Join(sub, resume, hasResume)` that subscribes and builds the replay under `emuMu`. It replaces `Subscribe` + `EnsureLiveEmulator` + `ReconnectPreamble` at the call sites.

**Replay contract to replace:** `ReconnectPreamble` (:626-650). Keep its order: raw ring intact → query-stripped raw; wrapped → rendered. The wrapped branch becomes history + screen + CUP + modes (Pattern 5), output as ≤16 KiB chunks. Keep the long "why" comment block style (:600-625) and keep the `liveEmuQueryStripPattern` strip on the delta path.

**Comment convention:** every change carries a "why" paragraph that cites the bug/phase ID (M-51, CR-03, #109). Follow that and cite SHAR-07 / Phase 5.

---

### `internal/relay/protocol.go` (wire constants)

```go
// protocol.go:11-22
MsgOutput  byte = 0x01 // PTY stdout/stderr → client
MsgResize  byte = 0x02
MsgTitle   byte = 0x03
MsgInput   byte = 0x10
MsgResize2 byte = 0x11
MsgPing    byte = 0x12 // Keep-alive ping
MsgMeta byte = 0x20
```
Add `MsgPong byte = 0x04` and a prebuilt `PongFrame = []byte{MsgPong}`. Extend `MetaPayload` (:62-64) with the existing pointer + omitempty convention:
```go
type MetaPayload struct {
	ViewerCount *int `json:"viewerCount,omitempty"`
}
```
→ add `Replay *string \`json:"replay,omitempty"\``, `Offset *uint64 \`json:"offset,omitempty"\``. Note the retired-range comment at :75-78 (0x30-0x37 never reused). The phase constants (PingEvery, PingTimeout, HistoryMaxLines, HistoryMaxBytes, ReplayChunk) go in one place, as package vars so tests can shorten them.

---

### `internal/relay/heartbeat.go` (NEW)

No exact analog. Use RESEARCH Pattern 1 code verbatim (`Ping` with timeout ctx, then `CloseNow`). The ticker/select/ctx.Done style matches `cmd_attach.go:302-319`:
```go
tick := time.NewTicker(time.Second)
defer tick.Stop()
for {
	select {
	case <-tick.C:
		...
	case <-ctx.Done():
		return
	}
}
```
Call it as `go relay.Heartbeat(ctx, conn, relay.PingEvery, relay.PingTimeout)` after the replay writes in relay/server.go, webserver/server.go, and on `upstream` in the proxy.

---

### `internal/relay/close_reasons.go`

Existing consts (:7-12) `CloseReasonShareStopped`, `CloseReasonSessionEnded`. Add `IsTerminalClose(err error) bool` using `websocket.CloseStatus(err) == StatusNormalClosure` and `errors.As(err, &websocket.CloseError)` reason match. That is the same extraction the proxy uses at `remote_ws_proxy.go:136-145`. Mirror test: `close_reasons_test.go` pins literals.

---

### `internal/relay/server.go` (local relay WS, controller)

**Join section to replace** (:97-135):
```go
hub.Subscribe(sub)
NotifyViewerCount(hub)
defer func() {
	hub.Unsubscribe(sub)
	NotifyViewerCount(hub)
}()
defer conn.CloseNow()
// VIEW-03: grid first
if c, r := hub.Cols(), hub.Rows(); c > 0 && r > 0 {
	if err := conn.Write(ctx, websocket.MessageBinary, MakeResizeFrame(uint16(c), uint16(r))); err != nil {
		return
	}
}
hub.EnsureLiveEmulator()
if snapshot := hub.ReconnectPreamble(); len(snapshot) > 0 {
	if err := conn.Write(ctx, websocket.MessageBinary, snapshot); err != nil {
		return
	}
}
```
→ `Join` + grid frame + `Meta{replay}` + chunks + `Meta{offset}` (RESEARCH "Server join" example). Keep the grid frame first.

**Ping arm** (:169-170) `case MsgPing: // Keep-alive — no-op.` → non-blocking enqueue of `PongFrame` onto `sub.Msgs` (Pattern 1 snippet).

**Write pump close convention** (:176-195): fixed reasons only, `conn.Close(websocket.StatusNormalClosure, CloseReasonSessionEnded)` on `hub.Done()`. Slow kick at :90-92 is `conn.Close(websocket.StatusPolicyViolation, "too slow")`, and clients retry on it.

---

### `internal/webserver/server.go` (share server WS)

Same edits at the mirrored lines in `handleWSSRelay` (:739):
- Subscriber with `Msgs: make(chan []byte, 256)` (:791).
- `hub.Subscribe(sub)` at :812. **The TOCTOU re-check at :820-827 must stay right after the subscribe/Join:**
```go
if !ws.isGrantActive(claims.SID, claims.GrantID) || !ws.IsSessionEnabled(sessionID) {
	conn.Close(websocket.StatusNormalClosure, relay.CloseReasonShareStopped)
	return
}
```
- Grid frame at :833, `EnsureLiveEmulator` + `ReconnectPreamble` at :846-847 → Join replay.
- `case relay.MsgPing:` no-op at :906 → pong.
- Test-only seam convention: `beforeSubscribeHookForTest` field (:131-132). Use the same pattern if a test needs a hook.
- Do not touch `web/assets/terminal.js` (deprecated, RESEARCH State of the Art).

---

### `internal/daemon/remote_ws_proxy.go` (proxy)

- :86 `u.RawQuery = "cap=" + url.QueryEscape(capToken)` → `url.Values` with validated `resume` (RESEARCH "Proxy resume pass-through"). Add `strconv` import. `redact` (:97-99) already strips the whole RawQuery.
- After :118 `upstream.SetReadLimit(relay.ViewerReadLimit)` add `go relay.Heartbeat(ctx, upstream, ...)`. Note `ctx := r.Context()` is declared at :126, so move it up or use `r.Context()`.
- Close forwarding (:143-146) stays. A heartbeat `CloseNow` gives code -1 → client sees 1006 → retry. Keep the viewer-diag log format (:132-139).
- Keep the no-cap JSON 404 (:56-60). The proxy owns no reconnect.

---

### `internal/attach/attach.go` + `cmd_attach.go` (CLI loop)

**Current single-shot shape** to restructure (cmd_attach.go:236-240):
```go
conn, _, err := websocket.Dial(ctx, wsURL, nil)
if err != nil {
	return fmt.Errorf("portty: couldn't reach the session: %w", err)
}
defer conn.CloseNow()
```
**Watcher to delete** (cmd_attach.go:291-321, SB-05 5 s no-frame ticker that calls `bar.SetConnectionState("reconnecting")`). This is UAT gap 1.

**Keep across reconnects:** `LockedWriter` stdout (:243), clear-screen `\x1b[2J\x1b[H` on TTY (:248-250; reuse it before a `replay:"full"`), status bar setup (:253-270), raw mode + deferred restore (:273-277), initial resize frame via `attach.MakeClientResizeFrame` (:280-284) re-sent on each connection, `remoteAttachEnd` exit message path (:323-330).

**Frame handling analog** `attach.go:120-150` `WsOutputPump`:
```go
case relay.MsgOutput:
	if _, werr := w.Write(payload); werr != nil { return werr }
case relay.MsgMeta:
	var meta relay.MetaPayload
	if err := json.Unmarshal(payload, &meta); err == nil && meta.ViewerCount != nil {
		bar.SetViewerCount(*meta.ViewerCount)
	}
```
Extend it with `MsgPong` (liveness), `meta.Replay` / `meta.Offset`, and an offset `+= len(payload)+1` per output frame. A meta parse failure must take an explicit path, never a silent ignore.

**Stdin:** `StdinPump` (:71) is per-connection today. Pattern 8 needs one long-lived stdin reader goroutine and a per-connection forwarder that still checks the detach key while reconnecting. Re-auth seam: `client.GetRelayPort()` + `client.ConnectPeer(...)` (inject as a func for tests). Local attach (`remote=false`) stays single-connection.

---

### `frontend/src/lib/relayClient.ts` (client loop)

Current structure: one `this.ws` created in the constructor (:98-152), `onopen` starts a 30 s `setInterval` ping (:118-126), `onclose` → `_clearPing` + `callbacks.onClose` (:144-147), `close()` (:169-172). Refactor into a private `dial()` that the constructor and the reconnect loop both call; `close()` sets `closedByUser`.

Frame parser to extend (:46-67):
```ts
switch (typeByte) {
  case MSG_OUTPUT: return { type: 'output', payload: data.slice(1) }
  case MSG_RESIZE: { ... }
  default: return { type: 'unknown' }
}
```
→ add `MSG_PONG = 0x04` and `MSG_META = 0x20` (consts at :2-6), returning `pong` / `meta`. Callback interface (:70-88): add optional `onReset`, `onStateChange(state)`, and an options field `reconnect?: { beforeReconnect: () => Promise<'retry'|'ok'|{stop:'share-stopped'|'ended'}> }`. Keep the existing doc-comment style that cites phase IDs. Reconnect stays opt-in, so host-path tabs are unchanged. Put the constants in one exported `RECONNECT` object (RESEARCH).

---

### `frontend/src/lib/closeReasons.ts`

```ts
export const CLOSE_REASON_SHARE_STOPPED = 'share stopped'
export const CLOSE_REASON_SESSION_ENDED = 'session ended'
```
Add `isTerminalClose(code, reason)` (1000 + one of these reasons). Literals are pinned in tests on both sides.

---

### `frontend/src/components/ReconnectingBanner.tsx` (NEW) or SessionEndedBanner `kind`

**Analog:** `SessionEndedBanner.tsx` (84 lines). Copy:
```tsx
const cls = ['banner', 'banner--info', 'session-ended-banner', className].filter(Boolean).join(' ')
return (
  <div className={cls} role="status" aria-live="polite">
    <InformationCircleIcon className="banner__icon" aria-hidden="true" />
    <span className="banner__message">{/* fixed copy only */}</span>
    ...
  </div>
)
```
Rules: fixed copy (never render wire `reason`, T-175-06-02), text + role for the colorblind user, `@heroicons/react/24/outline` icons. The simplest option is adding `kind: 'reconnecting'` to the existing union at :44 (no dismiss button for that kind). Update the doc comment at :17-20 ("do NOT wire any auto-reconnect") with the reason it changed (Chesterton). Test analog: `components/__tests__/SessionEndedBanner.test.tsx`.

---

### `frontend/src/components/TerminalPanel.tsx`

- State + stale comment at :227-235 (`sessionEnded` with `ended/reason/shareStopped`). Rewrite the "never wired to auto-reconnect" sentence and add a reconnect state.
- RelayClient construction :373-400. Today `onClose` sets `term.options.disableStdin = true` and treats every guest close as ended:
```tsx
onClose: (code, reason) => {
  if (!isGuest) return
  term.options.disableStdin = true
  const shareStopped = reason === CLOSE_REASON_SHARE_STOPPED
  setSessionEnded({ ended: true, reason, shareStopped })
  if (shareStopped) onShareStoppedRef.current?.()
},
```
→ terminal closes keep this; drops go through `onStateChange` (disable stdin, show the banner after 3 s, re-enable on open); `onReset: () => term.reset()`. Pass `{ remote, wsURL, reconnect }` only when `isGuest`.
- Banner render at :1002-1005 (`isGuestRef.current && sessionEnded.ended && <SessionEndedBanner .../>`). Add the reconnecting banner next to it.

---

### `WebShareSessionView.tsx` + `App.tsx` (beforeReconnect hooks)

- Browser: wsURL built at WebShareSessionView.tsx:90 (cap reusable). The probe analog is App.tsx:760-775:
```ts
fetch(`/api/sessions/${encodeURIComponent(sessionId)}/info?cap=${encodeURIComponent(capToken)}`)
  .then(async (res) => {
    if (res.status === 401 || res.status === 403) { setWebLinkDead(true); return }
    ...
```
Map it to 401/403 → share-stopped, 404 → ended, ok → redial, throw → retry.
- GUI peer tab: `ConnectPeerSession(session.peerDNSName || peerHost, session.id)` (App.tsx:791) and `GetRelayPort()` (App.tsx:327, :832). Map `errorCode` per RESEARCH Pattern 7. Thread the callback App → WebShareSessionView → TerminalPanel → RelayClient. Note: a new relay port means the GUI URL must be rebuilt, so `beforeReconnect` should be able to return the new port.

---

### Tests

- **Frontend mock pattern:** `frontend/src/lib/relayClient.test.ts:102-128` (`vi.fn` MockWebSocket + `vi.stubGlobal('WebSocket', ...)`, `vi.unstubAllGlobals` in afterEach). Further MockWS variants are at :196, :235, :325. Use `vi.useFakeTimers` for backoff/banner timing. `TerminalPanel.guestClose.test.tsx` must be rewritten (Pitfall 13).
- **Go integration:** `internal/daemon/attach_viewers_test.go` has `newAttachScene` (:187), `viewerTap` (:79, `state/outLen/has`), `waitUntil` (:122), `dialViewer` (:170), `dialCLI`/`attachCLI` (:274/:291). Extend it for kill-upstream → delta reconnect, viewing-daemon restart, and share-off during the gap. Also use `internal/testutil.WaitFor`.
- **Relay unit:** `internal/relay/hub_test.go`, `scrollback_altscreen_test.go`. The wrapped-ring tests use `bytes.Contains`; add exact-byte tests (CRLF, CUP, Height rows). Hub preamble tests pinning single-frame `ReconnectPreamble` will change.
- **TESTING.md:** add §2 manifest entries, §4 SHAR-07 rows (path-only), and §5 M-84..M-87; update the M-80/M-83 "no reconnect" wording; run `bash tests/check-traceability-paths.sh`.

## Shared Patterns

### Fixed close reasons, never wire text
**Source:** `internal/relay/close_reasons.go:3-12`, `frontend/src/lib/closeReasons.ts`, `SessionEndedBanner.tsx:25-29`
**Apply to:** all servers, proxy, both clients, banners. Close with `StatusNormalClosure` + constant; branch on code+reason+who-initiated; render fixed copy.

### Cap secrecy in logs
**Source:** `remote_ws_proxy.go:93-99` (`redact`), `:110-113` (fixed literal close reasons)
**Apply to:** proxy resume change and any new CLI/proxy logging. Never put the URL query or cap in a log or close reason.

### Lock order emuMu → hub.mu
**Source:** `hub.go:81-87`, `:422-423`, `:455-458`
**Apply to:** `Join`, `Run` atomic record+broadcast, history drain, resize push-to-history.

### viewer-diag logging
**Source:** `remote_ws_proxy.go:112,120,134,138`; `webserver/server.go:903`
**Apply to:** reconnect/heartbeat reaps (`viewer-diag: ... session=<DiagID>`), using `relay.DiagID(sid)`.

### Non-blocking enqueue to sub.Msgs
**Source:** `hub.broadcast` (hub.go:472) and the CloseSlow kick (relay/server.go:90-92)
**Apply to:** pong replies (select/default, never block the read pump).

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| Blackhole TCP forwarder (test helper, likely in `internal/relay` or `internal/daemon` test files) | test utility | streaming | No half-open simulation exists; build per RESEARCH Testing Strategy item 3 (accept, forward, `Freeze()`, `Kill()`) |
| Backoff/jitter logic (Go + TS) | utility | — | No existing backoff in repo. Use about 15 lines per client, no library |

## Metadata

**Analog search scope:** `internal/relay`, `internal/webserver`, `internal/daemon`, `internal/attach`, `internal/statusbar`, `cmd_attach.go`, `frontend/src/lib`, `frontend/src/components`, `frontend/src/App.tsx`
**Files scanned:** ~20
**All analog paths git-tracked** (no mirror paths).
**Pattern extraction date:** 2026-11-17
