# Phase 10: Input Path Reliability - Research

**Researched:** 2026-11-20
**Domain:** Go WebSocket relay (coder/websocket v1.8.15), PTY input path, hub grid arbitration, React/xterm.js viewer, Tart VM verification
**Confidence:** HIGH on the code facts (every file:line below was opened this session); MEDIUM on the INPT-06 client state machine and the INPT-05 root cause (derived from reading, not from a captured failure or a run)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Locked by the user and the roadmap**
- **INPT-06 (user, 2026-11-19: "Viewer takes over size"):** the viewer you are using sets the session's grid. Typing in it, or zooming/resizing it, makes the shell reflow to that viewer's grid. When another viewer (the host's local tab included) types or zooms/resizes, it takes the size back. Works for local tabs, peer tabs, browser viewers and `portty attach`.
- Keep the 32 KiB per-frame guard, share tokens, CSP and the tailnet-only listener unchanged (INPT-03 must not touch CSP or use innerHTML).

**Decided autonomously (reversible)**

| Question | Decision | Why |
|---|---|---|
| INPT-01 input path | A bounded per-connection input queue (about 1,024 frames / 32 MiB) between the read pump and `hub.WriteInput`, drained in order by a writer goroutine that outlives the socket, so nothing already read is lost. The pump blocks only when the queue is full. The daemon proxy (`internal/daemon/remote_ws_proxy.go`) gets the same treatment so its own upstream ping can still be written. | The todo's analysis: it is the only fix that covers all three liveness paths (server heartbeat, proxy ping, browser watchdog). |
| What a pong acknowledges after INPT-01 | "Read by the server and queued for the PTY in order". The writer outlives the socket, so queued bytes still reach the shell after a drop; the DEBT-05 notice keeps its "may have been cut short" copy for anything sent after the last pong. Update the comments and tests in `relayClient.ts` (`ackPing`). | Honest under the new lifetime; no copy change. |
| INPT-02 pong delivery | The pong must not be dropped when `sub.Msgs` is full: send it out of band (owed-pong flag resent on the next drain, or a blocking send with a short timeout). Planner picks after reading the write path; the test is a full `sub.Msgs` when the ping arrives and the client clearing the pending paste without a heartbeat-length wait. | Either satisfies SC2; the choice is internal. |
| INPT-03 browser notice | terminal.js sends `[MsgPing]` after a paste of 2+ frames, tracks the pong, and on close with a pending paste shows "Paste may have been cut short" in the status bar via textContent. Same copy as the app. | Matches the GUI's DEBT-05 design; CSP unchanged. |
| INPT-04 peer tab identity | Key `webSessionTabId` by host plus session everywhere (restore, close, snapshot keys), and `retryInit` drops the snapshot of every peer tab it removes. | The todo's solution; prevents two Macs' equal ids from sharing a tab. |
| INPT-05 flake | Reproduce with `-count=20` over the whole package, capture the message, then make the test hermetic (no live process racing SetState) or wait for the session's wait loop before SetState. | The todo's sketch. |
| INPT-06 grid authority model | The hub keeps a **grid owner**: the subscriber that most recently sent input or made a user-initiated size change. The grid is the owner's last reported size. Joining, reconnecting and passive output never claim (keeps Phase 9's delta resumes after a handoff). When the owner leaves, the grid stays as it is (D-01 freeze) until someone claims. This replaces D-02 min-among-local. | Directly implements the user's decision; "passive join never claims" keeps reconnects from moving the grid. |
| INPT-06 telling a claim from a size report | Each viewer keeps reporting its size (as today) so the hub knows it; a size change the user caused (window resize, Cmd+/Cmd−, attach's SIGWINCH) is sent as a **claim**. Input from a non-owner claims with that viewer's last reported size. Older viewers' plain resize frames are treated as reports only (no claim), and old hosts ignore claims, so mixed versions degrade to today's behavior. The planner picks the wire form (a new frame type or a flag) and whether `/health` `api` goes to 3. | Needed so the initial fit on join is not a claim. Backward compatible both ways. |
| INPT-06 other viewers | A viewer that is not the owner shows the session at the owner's grid: peer tabs and the browser use the existing guest scaling (`computeGuestScale` / readability floor); a local tab that loses ownership uses the same guest scaling instead of fitting the PTY to itself. `portty attach` shows it as a terminal shows any foreign-size program. | Reuses the existing guest path; no new UI. |
| INPT-06 safety | A guest already has full write access to the shell (one read,write capability per share), so letting it size the PTY is no escalation. Clamp claimed sizes to sane bounds (for example 20..500 cols, 5..200 rows) and ignore zero sizes. | Trust boundary validation. |
| INPT-06 vs the Phase 9 settle window | With "joins never claim", the post-handoff settle window may become redundant. The planner decides whether to keep it, simplify it or remove it; Phase 9's V-02 behavior (no grid change and delta resumes after a handoff) must still hold, pinned by its existing test. | Chesterton's Fence: it exists for reconnect order; the new model may cover that. |

### Claude's Discretion
- How to split the plans, file names, test names, queue sizes, timeouts.

### Deferred Ideas (OUT OF SCOPE)
- A visible "who has the size" indicator in the UI (not asked for; add only if the VM check shows it is confusing).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| INPT-01 | A long paste into a slow program never gets a healthy viewer dropped; heartbeats on server, daemon proxy and browser keep working under backpressure via a bounded input queue | Sections "INPT-01/02 design", Pitfalls 1-4. Both read pumps (`webserver/server.go:932-993`, `relay/server.go:154-192`) block in `hub.WriteInput`; a hub-owned FIFO writer fixes all three liveness paths. The proxy needs a test, not (provably) a queue |
| INPT-02 | The paste notice never appears for a paste that fully arrived | Owed-pong counter written by the write pump (never via `sub.Msgs`) |
| INPT-03 | Browser share viewer shows the same partial-paste notice as the app | **Conflict found:** `web/assets/terminal.js` is unreachable (dead code); the live browser viewer is the React SPA, which already shows the notice. See Open Question 1 |
| INPT-04 | Peer-tab snapshots keyed by host plus session everywhere, including Retry | `webSessionTabId` and its 7 call sites in `App.tsx`; `retryInit` only reachable with zero tabs |
| INPT-05 | `TestExitEvent_ListSessions_ExitCodePopulatedForStopped` deterministic | Root cause: spy session has no PTY, so the exit watcher stops it on its own within microseconds of `CreateSession` |
| INPT-06 | The viewer you're typing on sets the session's size | Owner model in `Hub`, claim bit on `MsgResize2`, typed-input classifier, unified follow-the-grid `TerminalPanel`, settle window removed |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **TESTING.md is the canonical regression home** (`CLAUDE.md`): every phase that adds/renames/removes tests updates Section 2 (suite manifest, no build tags, no file moves), Section 5 (new `M-NN` for anything not automatable: native GUI, remote peer, live PTY), Section 4 (a traceability row per new test file for a v4.0+ requirement; path column is a repo-relative `.go/.ts/.tsx/.sh` path only; run `bash tests/check-traceability-paths.sh`), Section 3 only if `build.yml` matrix changes. Next free manual id is **M-116**; the last category is **AY** (TESTING.md:1338, M-115 at :1402).
- **Chesterton's Fence** (both CLAUDE.md files): articulate why code exists before deleting it. Applied below to the settle window, `ResizeClient`'s origin gate and the min-among-local arbiter.
- **Silent fallbacks**: "`or {}` converts hard failures into silent corruption. Let it crash." The recommended designs return errors (dead input writer returns an error to the pump) instead of swallowing.
- **Premature abstraction**: need 3 real examples. Hence one small `InputConn` type and one `PongOwed` type, each used by exactly the two pumps that exist today (two examples, but they are the same code today, copy-pasted: extracting is removing a duplicate, not speculating).
- **Security (project constraint):** token-gated tailnet-only share links and the CSP stay intact; remote write access to a shell is powerful.
- **License MIT**, macOS only, Go/Wails/React/tsnet stack unchanged.
- **Go:** `go fmt`, golangci-lint, `ctx context.Context` on context-aware functions. **TS:** ESLint + Prettier. **Toolchain:** `export GOTOOLCHAIN=go1.26.11` (go.mod `toolchain go1.26.11`).
- **Testing:** 80%+ coverage in critical components; integration tests for FastAPI do not apply (Go/React).
- **Workflow:** edits happen through GSD commands only; browser UAT is run by the agent (dev-browser skill), and the user's memory says GUI click checks are driven by the agent inside the Tart VM `portty-test`, never on the host. Reliability first, radical simplicity second (user memory): prefer the smaller design when two work.
- **Phase rules (CONTEXT):** each full Go/vitest suite at most once per plan (every test binary raises a LuLu alert); save full output to files, never pipe through `tail`; ad-hoc signing only; fault injection only inside the VMs; never touch the user's running Portty; worktrees start at an old commit, fast-forward to `gsd/v1.2` first; do not commit planning files on `gsd/v1.2` while executors run.

## Summary

Phase 10 is six small problems that share two seams: the **read pumps** (`internal/webserver/server.go:932-993`, `internal/relay/server.go:154-192`) and the **grid arbiter** (`Hub.ResizeClient`/`applyGridLocked`, `internal/relay/hub.go:485-556`). INPT-01/02 fix the pump (it blocks inside `hub.WriteInput`, so coder/websocket never reads pongs and every liveness check downstream misfires). INPT-06 replaces the arbiter's rule (min among local viewers) with "the owner's last reported size", which also makes the Phase 9 settle window unnecessary.

Three findings change the plan and must be read before planning. (1) **INPT-03 targets dead code**: `handleTerminalPage` always 302-redirects `/sessions/{id}` to the React SPA (`webserver/server.go:787-795`), and TESTING.md:767 already records `web/assets/terminal.js` as dead. The browser share viewer that a person actually uses is `/app/` → `WebShareSessionView` → `TerminalPanel` → `RelayClient`, which already has the DEBT-05 notice (`TerminalPanel.tsx:1156`). (2) **The local relay pump (`relay/server.go`) has the same blocking `WriteInput` and must get the queue too**: SC1 names "a local tab", and the local tab's watchdog (`relayClient.ts` deadMs 40 s + probeGraceMs 5 s) abandons it exactly like the browser's. (3) **A per-connection writer that outlives the socket can interleave with a reconnect's fresh input** (the dropped viewer's backlog and the reconnected viewer's keystrokes become two concurrent writers into one PTY). A single hub-owned FIFO writer with per-connection quotas keeps the locked "bounded, per-connection, in-order, outlives the socket" semantics and adds global order.

**Primary recommendation:** One hub-owned input FIFO (`relay.InputConn` per connection, 1,024 frames / 32 MiB quota each) used by both pumps; an owed-pong counter written by each write pump; a claim bit as byte 5 of the existing `MsgResize2` frame (no new frame type, no `api` bump); an owner pointer in `Hub` that replaces min-among-local; remove the settle window; make `TerminalPanel` follow the server grid in every mode and only ever *report* its measured fit.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Reading viewer input without stalling liveness | API / Backend (relay + webserver read pumps) | Daemon proxy (opaque copy) | coder/websocket only processes pongs inside `Read` (heartbeat.go:21-22); the pump must never block on the PTY |
| Ordering and bounding PTY writes | API / Backend (`relay.Hub`) | — | One PTY, many writers; order and quota belong next to the writer |
| Pong delivery | API / Backend (write pump) | Browser/Client (ack accounting in `RelayClient`) | The pong is a write on the viewer's socket; it must not depend on `sub.Msgs` space |
| Paste-cut notice | Browser / Client (`RelayClient`, `TerminalPanel`) | — | Only the client knows what it sent; server only acks order |
| Grid ownership and clamping | API / Backend (`Hub`) | — | Trust boundary: viewer sizes are untrusted input; the hub holds the PTY size |
| Measuring a viewer's fit | Browser / Client (`TerminalPanel`) | `portty attach` (SIGWINCH) | Only the viewer knows its pixels/cells |
| Showing a foreign-size grid | Browser / Client (`recomputeScale`, `terminalScale.ts`) | `attach` (the user's terminal) | Existing guest scaling |
| Peer tab identity and snapshot keys | Browser / Client (`App.tsx`, `peerSnapshots.ts`) | Daemon (`RemoteCapStore`, keyed by session id alone; see Pitfall 12) | Tab ids are UI state |
| Surviving an upgrade handoff | API / Backend (`oldHandoff.finish`) | — | Queued input lives in the old daemon process |

## Standard Stack

No new packages. Everything is the inherited stack. Versions confirmed by reading the manifests this session.

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/coder/websocket` | v1.8.15 [VERIFIED: go.mod:14] | WebSocket server/client; `Read`, `Write`, `Ping`, `CloseNow` | Already used by both pumps, the proxy and attach; semantics read from source below |
| Go | 1.26.11 toolchain [VERIFIED: `go version go1.26.11 darwin/arm64`, go.mod:5] | `sync.Cond`, `sync/atomic` typed values, `testing` | Existing pin |
| xterm.js + FitAddon | via `frontend/package.json` | Terminal in `TerminalPanel` | Existing |
| vitest ^4.1.0, jsdom ^29.0.0, react ^19.2.4 [VERIFIED: frontend/package.json:15,26,36,39] | Frontend tests | Existing harness `appHarness.ts` mounts the real App with Wails faked |

### Supporting
| Tool | Purpose | When to Use |
|------|---------|-------------|
| `internal/testutil.WaitFor(t, timeout, cond, msg, args...)` | Poll for a condition | Every new Go test that waits (never `time.Sleep` for a state change) |
| `io.Pipe` as the hub writer | A writer that blocks until read | The "blocking fake hub" for SC1 (`testServerWithHub` already wires pipes: `capability_test_helpers.go:131-169`) |
| Tart VMs `portty-test` (office) and `portty-test-2` (home) | M-101 slow-reader paste, `tailscale down`, SC6 zoom checks | Both are running now [VERIFIED: `tart list`] |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hub-owned FIFO writer (recommended) | Per-connection queue and per-connection writer goroutine (as literally locked) | Per-connection writers interleave: a dropped viewer's backlog and the reconnected viewer's new input become concurrent writes into one PTY. Fallback if the user insists: a hub "baton" mutex a writer holds while its queue is non-empty (no frame interleave, but no arrival order and the reconnect starves behind the backlog) |
| Claim bit in `MsgResize2` byte 5 (recommended) | New frame type `MsgClaim 0x13` | A new type needs the viewer to send a plain report *and* a claim to keep old hosts working (two frames, not atomic). The flag is one atomic frame and old hosts already read only 4 bytes |
| Owed-pong counter (recommended) | Blocking send on `sub.Msgs` with timeout | A blocked read pump is the bug being fixed; a timeout still drops the pong when the viewer is genuinely slow |

**Installation:** none.

**Version verification:** `npm view`/`pip index` do not apply (no installs). coder/websocket v1.8.15 source is in the module cache at `~/go/pkg/mod/github.com/coder/websocket@v1.8.15` and was read directly.

## Package Legitimacy Audit

No external packages are installed in this phase.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| (none) | — | — | — | — | — | — |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
 viewers                        daemon process (one per Mac)                              shell
 ───────                        ─────────────────────────────                             ─────
 local GUI tab ──ws loopback──► relay/server.go handleSession ─┐
 portty attach ──ws loopback──►   (read pump, write pump)      │
 peer tab ─ws loopback► daemon proxy (opaque copyWS) ─wss tailnet─► webserver handleWSSRelay ─┤
 browser (SPA) ───────wss tailnet──────────────────────────────►   (read pump, write pump,   │
                                                                     relay.Heartbeat)        │
                                                                                             ▼
   read pump, per frame:                                                         ┌────────────────────┐
   MsgInput  ─► absorber.Filter ─► typed? ─► hub.NoteTyping(sub)  (claim) ─►     │ Hub                │
                                  └──────► InputConn.Write(frame) ── quota ──►   │  input FIFO (1 per │
   MsgResize2 ─► ParseResize ─► ResizeClient / ClaimGrid(sub,c,r) ──────────►    │  hub, ordered)     │
   MsgPing   ─► PongOwed.Owe()  (no sub.Msgs, no PTY)                            │  owner, ptyCols/   │
                                                                                 │  ptyRows, gridMu→mu│
   write pump, per loop:                                                         └─────────┬──────────┘
   select { sub.Msgs frame | pongKick → write owed pongs | hub.Done | readDone }           │ one writer goroutine
                                                                                           ▼ h.writer.Write (PTY master)
   grid change (owner moved or owner resized): applyGridLocked ─► broadcastResize(0x02) to ALL subs
                                              ─► resizeLiveEmulator (geomEnd) ─► resizeFn (TIOCSWINSZ)
   handoff finish: RestartViewers(1012) ─► WaitInputIdle(≤ bound) ─► exit
```

### Recommended Project Structure
```
internal/relay/
├── input.go          # NEW: Hub input FIFO: OpenInput, InputConn.Write/Close, WaitInputIdle
├── pong.go           # NEW (tiny): PongOwed (atomic count + 1-slot kick channel)
├── protocol.go       # + ResizeClaim, ParseResize, MakeClaimFrame
├── hub.go            # owner/ownerFast fields, ResizeClient/ClaimGrid/NoteTyping, applyGridLocked, remove settle
├── oscabsorb.go      # + IsUserInput(filtered []byte) bool (typed-input classifier)
├── server.go         # relay pump: queue, pong, claim
internal/webserver/server.go   # share pump: same three
internal/attach/                # + MakeClientClaimFrame; SIGWINCH sends it
frontend/src/lib/relayClient.ts # + encodeClaimFrame, sendClaim; ackPing comments
frontend/src/components/TerminalPanel.tsx, WebShareSessionView.tsx; App.tsx
```

### Pattern 1: Hub-owned input FIFO with per-connection quota
**What:** The pump hands each filtered frame to `InputConn.Write`, which appends to one hub-level FIFO and returns at once unless this connection already has 1,024 frames or 32 MiB queued. One goroutine per hub (started on first `OpenInput`) pops in order and calls `h.writer.Write` with no lock held. `InputConn.Close()` (pump exit) only marks the connection closed; its queued frames are still delivered. The writer dies, discarding the queue and waking blocked pumps with an error, when `h.writer.Write` fails or `h.done` closes.
**When to use:** both read pumps, for MsgInput only.
**Why hub-level rather than per-connection:** see Pitfall 3.
**Example (skeleton, planner finishes):**
```go
// internal/relay/input.go  (proposed)
const (
	InputMaxFrames = 1024     // per connection
	InputMaxBytes  = 32 << 20 // per connection; frames are <= MaxInputPayload (32767) so frames bind first
)

var ErrInputClosed = errors.New("relay: input writer stopped")

type inputReq struct{ c *InputConn; data []byte }

type InputConn struct {
	h             *Hub
	frames, bytes int  // queued + in flight; under h.inMu
	closed        bool // pump gone; queued frames still delivered
}

func (h *Hub) OpenInput() *InputConn { return &InputConn{h: h} }

func (c *InputConn) Write(b []byte) error { // b must not be reused by the caller (Filter allocates: oscabsorb.go:55)
	h := c.h
	h.inMu.Lock()
	defer h.inMu.Unlock()
	for !h.inDead && (c.frames >= InputMaxFrames || c.bytes+len(b) > InputMaxBytes) {
		h.inCond.Wait() // woken after each written frame and on death
	}
	if h.inDead {
		return ErrInputClosed
	}
	h.inq = append(h.inq, inputReq{c, b})
	c.frames++
	c.bytes += len(b)
	h.startInputWriterLocked() // once; also starts the "<-h.done => inDead" watcher
	h.inCond.Broadcast()
	return nil
}

func (h *Hub) inputWriter() {
	for {
		h.inMu.Lock()
		for len(h.inq) == 0 && !h.inDead {
			h.inCond.Wait()
		}
		if h.inDead {
			h.inMu.Unlock()
			return
		}
		r := h.pop() // O(1): ring or chunked slice, not inq[1:] (retains the backing array)
		h.inWriting = true
		h.inMu.Unlock()

		err := h.writer.Write(r.data) // may block for minutes (slow reader); no lock held

		h.inMu.Lock()
		h.inWriting = false
		r.c.frames--
		r.c.bytes -= len(r.data)
		if err != nil {
			h.inDead, h.inq = true, nil
		}
		h.inCond.Broadcast()
		h.inMu.Unlock()
	}
}
```
`WaitInputIdle(timeout)` polls `len(h.inq)==0 && !h.inWriting` (used by the handoff, Pitfall 7). `Hub.WriteInput` (hub.go:1427) stays as the direct writer for `engine_test.go:346` and `TestHubWriteInput`.

### Pattern 2: Out-of-band pong (owed counter)
**What:** `case MsgPing:` does `pongs.Owe()` (atomic increment plus a non-blocking kick on a 1-slot channel). The write pump's `select` gains `case <-pongs.Kick():` which writes `Take()` pongs with `conn.Write`. A counter, not a flag, because `relayClient.ts` matches pongs by count (`pongsReceived >= paste.ackPing`, relayClient.ts:374): coalescing two pings into one pong would leave a paste unacked.
**Example:**
```go
// internal/relay/pong.go (proposed)
type PongOwed struct {
	n    atomic.Int32
	kick chan struct{} // cap 1
}
func NewPongOwed() *PongOwed         { return &PongOwed{kick: make(chan struct{}, 1)} }
func (p *PongOwed) Owe()             { p.n.Add(1); select { case p.kick <- struct{}{}: default: } }
func (p *PongOwed) Kick() <-chan struct{} { return p.kick }
func (p *PongOwed) Take() int        { return int(p.n.Swap(0)) }

// write pump (both servers), added case:
case <-pongs.Kick():
	for i := pongs.Take(); i > 0; i-- {
		if err := conn.Write(ctx, websocket.MessageBinary, relay.PongFrame); err != nil {
			return
		}
	}
```
Go's `select` picks randomly among ready cases, so with `sub.Msgs` always ready the pong goes out within a couple of loop turns; check `pongs.Kick()` non-blockingly at the top of each iteration to make it first.

### Pattern 3: Claim as a flag on `MsgResize2`
**What:** `MsgResize2` payload stays `[cols_hi, cols_lo, rows_hi, rows_lo]` (report). Byte 5 of the frame (payload[4]) bit 0 = `ResizeClaim`. A frame with fewer than 5 payload bytes is a report. No other bits are defined; receivers ignore unknown bits.
```go
// internal/relay/protocol.go (proposed additions)
const ResizeClaim byte = 0x01 // MsgResize2 payload[4], bit 0: a user-initiated size change

func ParseResize(payload []byte) (cols, rows int, claim, ok bool) {
	if len(payload) < 4 {
		return 0, 0, false, false
	}
	cols = int(payload[0])<<8 | int(payload[1])
	rows = int(payload[2])<<8 | int(payload[3])
	return cols, rows, len(payload) >= 5 && payload[4]&ResizeClaim != 0, true
}
```
**Backward compatibility, all six directions** (each cell verified by reading the receiving code):

| Sender → receiver | Result | Evidence |
|---|---|---|
| old viewer (4 B) → new host | report only; typing still claims server-side, so an old GUI/attach can take the grid by typing | new code; old viewers never send byte 5 |
| new viewer (claim, 5 B payload) → old local host | old pump reads the first 4 bytes: behaves as today (min-among-local) | `relay/server.go:178` `if len(payload) >= 4 {` then reads `payload[0..3]` |
| new viewer → old web host | old pump drops resize (logs once) | `webserver/server.go:981` `if resizes == 1 && len(payload) >= 4 {` |
| new viewer → host through the daemon proxy | proxy copies bytes verbatim | `remote_ws_proxy.go:181-183` "the proxy is transport-only: it never parses or evaluates frames" |
| old host → new viewer | viewer follows the grid the old host pushes (as today); its claims are ignored | grid push exists since Phase 157 (`relay/server.go:114-121`) |
| new host's 0x02 frames → old viewer | unchanged frame | `MakeResizeFrame` (protocol.go:48-54) untouched |

**`/health` `api`:** do **not** bump (stay at `APIVersion = 2`, daemon/types.go:62). Nothing reads the peer `/info` `api` field today (grep of `frontend/src` finds no consumer), `MinDaemonAPI` is 1 (client.go:96), the claim degrades safely by construction, and `api` is documented as the *route set* (types.go:60-62 "v1.0 and v1.1 send no api field and count as 1"). Phase 11 needs its own bump for the preview message; bumping twice buys nothing. [ASSUMED: design judgment; see Assumptions Log A4]

### Pattern 4: Owner model in `Hub` (replaces min-among-local)
State (under `mu`): `owner *Subscriber`; mirrored in `ownerFast atomic.Pointer[Subscriber]` so `NoteTyping` can check "am I already the owner" without taking any lock (lock order is gridMu then mu, hub.go:92-96; a fast path that locked `mu` first and then wanted `gridMu` would invert it).

Rules (this is the whole model):
1. **Report** (`ResizeClient`, plain frame): store the clamped size on the sub. If the sub is the owner, the grid follows. Exception, **bootstrap**: a *local* sub reporting while `ptyCols == 0` (grid never known) becomes the owner. Needed so a fresh session adopts its first tab's fit (today's "first resize from the unknown grid is not genuine", hub.go:1343-1350) and so 39 existing helper call sites keep working (see Test impact).
2. **Claim** (`ClaimGrid`, flagged frame): store size, `owner = sub`, apply.
3. **Typing** (`NoteTyping`, a MsgInput whose filtered bytes pass `IsUserInput`): `owner = sub`; apply the sub's last reported size if it has one (if it has none yet, ownership is recorded and the next report applies).
4. **Never claim:** join, replay, reconnect, passive output, and reports from non-owners.
5. `Unsubscribe(owner)` clears `owner`; the grid stays (D-01).
6. Read-only subs (`sub.ReadOnly`) can neither report, claim nor type into ownership. **This is a gate CONTEXT's safety paragraph does not mention:** it says a guest "already has full write access", but `webserver/server.go:821` derives `readonly` from the cap and the code supports `"read"`-only caps (capability.go:21 `"read" or "read,write"`); today's minting always gives `"read,write"` (daemon/share.go:326), but a read-only cap must not move the shell's size.
7. Frozen hub (`h.freezing`): record owner and size, return; `unfreeze` already re-runs `applyGridLocked` (hub.go:690-701) so the claim lands at `Thaw`.
8. Clamp at intake: cols 20..500, rows 5..200 (CONTEXT's example values; planner's call) and ignore `<= 0`. Today a local viewer can request up to 65535x65535 (uint16, relay/server.go:179-180) and `resizeLiveEmulator` allocates a `newLiveEmulator(cols, rows)`; with web claims allowed this must be bounded.

`applyGridLocked` body becomes: compute `(cols, rows)` from `owner` if it is still in `h.subscribers` and has a valid size; the rest (`needResize`, log, `broadcastResize`, `resizeLiveEmulator(pc, pr, oldCols > 0 && oldRows > 0)`, `resizeFn`, unlock-before-IO) is unchanged (hub.go:531-555). Add `why=report|claim|input` to the `viewer-diag: grid session=%s %dx%d -> %dx%d by=%s client=%q` line (hub.go:542) by appending, so `daemon_log_test.go:116` still matches its three substrings and the VM checks can grep ownership changes.

### Pattern 5: Typed-input classifier (so auto-replies never claim)
xterm.js answers terminal queries with `MsgInput` bytes (cursor-position reports, DA2, DECRPM, XTVERSION, focus events) on *every* connected viewer, with no human involved; `webserver/server.go:968` already documents "Input within ms of joining ... is a terminal answering replayed queries". The `InputAbsorber` only removes OSC 10/11 and DA1 replies (oscabsorb.go:54-181). If those other replies counted as typing, every live query would make all viewers fight over the grid and a reconnect would claim and break V-02. `portty attach` has the same hazard (its real terminal answers queries; `attach/loop.go:171` forwards stdin raw).

`IsUserInput(filtered []byte) bool`: false when every byte is part of (a) a CSI whose final byte is one of `R c y t n I O M m` or whose introducer is `ESC [ <` / `ESC [ M` (reports, mouse), or (b) an OSC/DCS/APC string; true for any other byte (printable, CR/LF/TAB, control bytes, DEL, `ESC` + char, arrow/function keys `CSI A..D H F ~`, SS3). Bracketed-paste markers carry printable payload, so a paste claims. Conservative on purpose: a false non-claim costs one more keystroke; a false claim causes a reflow storm. Pin with a table test (CPR `\x1b[12;40R`, DA2 `\x1b[>0;276;0c`, DECRPM `\x1b[?2004;2$y`, focus `\x1b[I`, XTVERSION `\x1bP>|xterm.js\x1b\\` do not claim; `a`, `\r`, `\x03`, `\x1b[A`, `\x1bb` do) plus a fuzz seed run.

### Pattern 6: Client follows the server grid; it only ever reports its *fit*
Today (`TerminalPanel.tsx`): host path = `fitTerminal` resizes xterm and `term.onResize → sendResize` (line 519); guest path = never fits, `onResize → applyHostGrid` (line 471-473), no resize ever sent (lines 449-451, 517-519). The single invariant the new client must keep: **`sendResize`/`sendClaim` carry the container's *measured fit*, never `term.cols/rows`**, because in follow mode `term.cols/rows` is someone else's grid and reporting it would make a later typing-claim claim the foreign grid (a no-op) instead of this viewer's own.

State machine per panel (local tab, peer tab and browser use the same code):
- `fit` = `gridForContentBox(contentW, contentH, cellW, cellH)` measured from the container (extract the measuring half of `fitTerminal`, TerminalPanel.tsx:39-61, as `measureFit(term)`).
- `serverGrid` = last `MsgResize` (0x02) received; `null` before any.
- `serverGrid === null` (only a never-sized hub, see the server guard below): behave as today's host: `fitTerminal` + report; the hub bootstraps and broadcasts the grid.
- Otherwise: `applyHostGrid(term, serverGrid)` then `recomputeScale()` always (extend the `isGuestRef` gates at TerminalPanel.tsx:247 and :923, :990 to "grid differs from fit"). When grid equals fit the scale is 1, so the owner looks exactly as today.
- **Reports** (plain, never claim): on open (`onOpen`, now for all panels), on every `ResizeObserver` fire (dedupe: send only when `fit` changed). Hidden panels have zero cell metrics and send nothing (`fitTerminal` early-returns at :43).
- **Claims**: (a) `window` `resize` event, debounced 150 ms trailing; (b) font family/size change on an active panel (TerminalPanel.tsx:984-997): measure `fit` with the new cell metrics, `term.resize(fit)` optimistically, `sendClaim(fit)`. **Do not claim from `ResizeObserver`**: the reconnecting banner and the DEBT-05 paste notice change the container height (`TerminalPanel.tsx:1148-1161`), and a sidebar toggle changes its width; none of those is the user taking the size. `window.resize` fires for a real window resize, fullscreen and browser zoom (which is how a browser viewer "zooms").
- Remove `term.onResize → sendResize` (line 515-519).

**Server guard needed for this to work:** `relay/server.go:117` pushes `hub.Cols()/Rows()` at join, and those return the 220x50 / 50-row *fallback* when the grid is unknown (hub.go:1773-1792, "Returns 220 when no resize has been applied"). A local tab that followed that would flash 220x50 on every new session. Add `Hub.GridKnown()` (`ptyCols > 0 && ptyRows > 0`) and skip the join push on the *local* relay path when false (web guests keep today's fallback push). `TestRelayJoin_PushesResizeBeforeScrollback` (relay/server_test.go:353-362) stamps 120x30 first, so it is unaffected.

### Anti-Patterns to Avoid
- **Claiming from `ResizeObserver`** (banner/sidebar chrome would steal the grid).
- **`term.onResize → sendResize`** surviving the rewrite (reports a foreign grid).
- **Dropping the queue's frames when the socket dies** ("nothing already read is lost" is the locked semantic).
- **A blocking pong send** (re-creates the stalled pump).
- **Bumping `api`** to signal something nothing reads.
- **Editing `web/assets/terminal.js` for INPT-06** (unreachable; Open Question 1).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Waiting for a goroutine/queue to drain in tests | `time.Sleep` loops | `testutil.WaitFor` | Existing helper; deterministic under `-race` |
| Slow/blocking PTY writer in tests | A custom fake `io.Writer` with gates | `io.Pipe` (blocks until read) | Already how `testServerWithHub` captures input |
| WebSocket liveness | A second ping mechanism | `relay.Heartbeat` + app-level `MsgPing/MsgPong` | Both exist; the fix is to keep the pump reading, not to add probes |
| Frame split for large pastes | New chunking | `encodeInputFrames` (relayClient.ts:33-44) and `relay.MaxInputPayload` | Already splits at 32767 |
| Guest scaling for a foreign grid | New scale math | `computeGuestViewport` + `DEFAULT_GUEST_MIN_SCALE` (terminalScale.ts:48-62) | CONTEXT locks reuse |
| Delta-vs-full replay after a resize | Anything new | `geomEnd` in `resizeLiveEmulator` (hub.go:1349-1351) | Every genuine claim already bumps it correctly |

**Key insight:** every mechanism this phase needs already exists (heartbeat, ping/pong, split frames, grid broadcast, guest scaling, geomEnd). The bugs are in *who is allowed to block* and *who is allowed to decide the size*.

## Runtime State Inventory

Not a rename/refactor/migration phase. One adjacent runtime-state fact: the input FIFO holds bytes **in the old daemon's memory** during an upgrade handoff (Pitfall 7). No stored data, service config, OS registration, env var or build artifact carries a renamed string. None found in any of the five categories.

## Common Pitfalls

### Pitfall 1: The local relay pump has the same bug
**What goes wrong:** Fixing only `webserver/server.go` leaves `relay/server.go:170-176` blocking in `hub.WriteInput`. A local tab pasting into a slow reader stops answering `MsgPing`; `relayClient.ts` probes after `deadMs` 40_000 and abandons after `probeGraceMs` 5_000 (relayClient.ts:102-114, :393-405). `portty attach` does the same (`DeadAfter` 40 s, attach/loop.go:43,250).
**How to avoid:** apply Pattern 1 and 2 in both pumps; SC1 names the local tab.
**Warning signs:** a local tab shows "reconnecting" 45 s into a slow paste.

### Pitfall 2: Blocked read means no pong processed
**What goes wrong:** coder/websocket processes control frames only inside `Read` ("Ping must be called concurrently with Reader", conn.go:223-226 comment; heartbeat.go:21-22). `Heartbeat` closes the conn on *any* `Ping` error (heartbeat.go:35-40), and `writeControl` has its own 5 s cap: `ctx, cancel := context.WithTimeout(ctx, time.Second*5)` (write.go:277), after which a ping that cannot get `writeFrameMu` fails. So a stalled data write holding the frame lock for more than 5 s also kills a healthy peer.
**How to avoid:** the pump must always be in `conn.Read`.

### Pitfall 3: Writers that outlive the socket can interleave
**What goes wrong:** Per-connection writers: viewer A pastes 5 MB into `pv -L 50k`, drops; its writer keeps draining for ~100 s. A reconnects as a new connection with its own writer; anything typed or pasted now interleaves with A's remaining backlog at frame granularity (or at byte granularity across the pump/writer boundary). Before this phase the dropped pump's remainder was discarded.
**How to avoid:** hub-owned FIFO (Pattern 1): order is arrival order across connections. Consequence to document: Ctrl-C typed after a large backlog waits behind it (same as typing into any terminal mid-paste; the old design queued it behind the paste in the TCP stream too).

### Pitfall 4: Queue full still blocks the pump
**What goes wrong:** Past 1,024 frames / 32 MiB queued *per connection* the pump blocks again, TCP backs up, the server `Heartbeat` times out (15 s) and the proxy's `Ping` loses the frame lock after 5 s. That is the accepted limit (a 32 MiB+ backlog); the DEBT-05 notice shows afterwards. Keep M-101 payloads at or under ~4 MB at 50 KB/s (80 s, over 5x the 15 s timeout) so the VM check never hits the cap.
**Do not:** make `InputConn.Write` take the *handler* context: a pump blocked on quota whose socket died would drop a frame that was already read. Let it return only when the hub input is dead.

### Pitfall 5: Aliasing a read buffer into the queue
`conn.Read` returns a fresh slice per message and `absorber.Filter` allocates `out` per call (oscabsorb.go:55), so queuing the filtered slice is safe today. Any future "optimization" that reuses a buffer across frames would corrupt queued input. Keep the `// must not be reused` comment on `InputConn.Write`. The absorber itself stays in the pump (stateful per connection, order-sensitive).

### Pitfall 6: Claims and resizes while the PTY write is blocked
`resizeFn` is `TIOCSWINSZ` via `backend.Resize` (engine.go:242-244) and does not wait on the TTY input queue, so claims and `SIGWINCH` land immediately even with 30 MB of backlog. That is correct but means ordering between a claim and bytes already queued is not preserved on the PTY (the shell sees the resize first). Harmless.

### Pitfall 7: Upgrade handoff loses queued input (and the pong already said "queued")
**What goes wrong:** `Freeze` only stops PTY *reads* (hub.go:645-678); input writes continue through a freeze (fine: output sits in the PTY buffer for the takeover). The API `writeGate` drains HTTP writes, not WebSocket input (api.go:124-165). After COMMITTED, `oldHandoff.finish` sends 1012 to every viewer, waits `restartDrain` (1 s, handoff.go:84,767) and calls `handoffExit(0)` (handoff.go:782): bytes still in the FIFO die with the process, after a pong told the client they were queued, so no notice shows.
**How to avoid:** `hub.WaitInputIdle(d)` for every hub inside `finish`, sharing the `restartDrain` budget (typing and ordinary pastes are idle by then), log `input still queued: N bytes lost` otherwise. Never extend the 19 s `maxFrozen` budget (handoff.go:88-91). Record the residual (a multi-MB slow paste during an upgrade) as a `shortcut:` comment, as the file already does for other windows.

### Pitfall 8: Lock order and unlock-before-IO
Order is `emuMu → hub.mu` (Run, hub.go:603-606) and `gridMu → mu` (resize/freeze). `NoteTyping` must take `gridMu` first, so its "already owner" check uses the atomic mirror. `applyGridLocked` releases `mu` before `broadcastResize` (which re-takes `mu`), `resizeLiveEmulator` (takes `emuMu`) and `resizeFn`; keep exactly that. `Unsubscribe` clears `owner` under `mu` only (it never changes the grid, so no `gridMu`).

### Pitfall 9: Reflow churn when two viewers alternate typing
Each owner change with a different size runs the full emulator rebuild, bumps `geomEnd` (reconnecting viewers then get full replays, not deltas) and `resizeLiveEmulator` copies the visible rows into history, "a few prompt lines can appear twice" (hub.go:1352-1356). Two viewers with equal fit sizes cost nothing (`needResize` false). Mitigations already in the design: owner changes only on typing or an explicit resize; client debounce 150 ms. Accept; document in M-117.

### Pitfall 10: `portty attach` shows a foreign grid garbled
Attach ignores `0x02` and its terminal is whatever size the user has (cmd_attach.go:391-396). Under min-among-local the grid was never larger than any local viewer; under the owner model a GUI tab with a big window makes the grid wider than an attach terminal until the attach user types (typing claims). This is the user's decision ("shows it as a terminal shows any foreign-size program"); flag it in M-117 and the release note.

### Pitfall 11: Relaunch/rejoin shows the old grid until you type
Strict "joins never claim" plus D-01 means: quit the GUI (owner leaves, `owner = nil`), relaunch with a bigger window: the tab follows the old, smaller grid at scale ≤ 1 until typing, zooming or resizing the window. That is the literal consequence of CONTEXT's rules and is what keeps V-02 working. If the VM check finds it annoying, the cheap refinement is "a lone local viewer joining an *ownerless* hub adopts its fit", **disabled for the first seconds after a handoff import** (which is a smaller settle window with a real reason). Do not build it speculatively; see Open Question 3.

### Pitfall 12: Peer-tab key vs the daemon's cap store
`RemoteCapStore` and the proxy route `/api/relay/remote/{sessionID}/ws` are keyed by session id alone (`remote_caps.go` `entries map[string]remoteCapEntry`, `Put/Get(sessionID)`; `remote_ws_proxy.go:61`). INPT-04 keys *tabs* and snapshots by host; two Macs with literally equal ids would still share one cap entry (the second `ConnectPeerSession` overwrites the first; `beforeReconnect` re-puts the right one just before each redial, so only simultaneous redials could cross). Session ids are 16 random bytes in hex (pty/native.go:161-168), so an honest collision is not realistic; a *chosen* collision needs a hostile peer on the same Tailscale account. Out of scope for INPT-04 as decided; recorded as a residual risk, no code.

### Pitfall 13: `TerminalPanel` mocks list RelayClient members by hand
Five test files mock `../../lib/relayClient` with a literal class (`sendResize() {}` in contextLoss:138 and guestClose:111, a method list in scale/tabSwitch/reconnect). A new `sendClaim` called on the client, or a new exported helper imported by `TerminalPanel`, throws in every mock that lacks it (the comment at TerminalPanel.tsx:489 records the same trap for `RECONNECT`). Add `sendClaim() {}` to each mock in the same plan that changes `TerminalPanel`.

## Code Examples

### The current blocking pump (what changes)
```go
// internal/webserver/server.go:956-964 (current)
case relay.MsgInput:
	passed := 0
	if !sub.ReadOnly { // MC-03: discard input for read-only clients
		filtered := absorber.Filter(payload)
		passed = len(filtered)
		if len(filtered) > 0 {
			_ = hub.WriteInput(filtered)   // <-- blocks the pump for as long as the shell is slow
		}
	}
```
```go
// proposed (both pumps)
case relay.MsgInput:
	if !sub.ReadOnly {
		filtered := absorber.Filter(payload)
		if len(filtered) > 0 {
			if relay.IsUserInput(filtered) {
				_ = hub.NoteTyping(sub) // claim before queueing, in arrival order with resizes
			}
			if err := in.Write(filtered); err != nil { // in := hub.OpenInput(); defer in.Close() on pump exit
				return // hub input is dead: the session is ending
			}
		}
	}
case relay.MsgResize2:
	if cols, rows, claim, ok := relay.ParseResize(payload); ok && !sub.ReadOnly {
		if claim { _ = hub.ClaimGrid(sub, cols, rows) } else { _ = hub.ResizeClient(sub, cols, rows) }
	}
case relay.MsgPing:
	pongs.Owe() // was: select { case sub.Msgs <- relay.PongFrame: default: }  (server.go:987-990)
```
Keep the web pump's viewer-diag counters (`inFrames`, `inBytes`, `resizes`, server.go:938-943) and the `first input` log line, and never log input payloads.

### Hub grid owner
```go
func (h *Hub) sizeClient(sub *Subscriber, cols, rows int, claim bool) error {
	if sub.ReadOnly || cols <= 0 || rows <= 0 {
		return nil
	}
	cols, rows = clamp(cols, MinGridCols, MaxGridCols), clamp(rows, MinGridRows, MaxGridRows)
	h.gridMu.Lock()
	defer h.gridMu.Unlock()
	h.mu.Lock()
	sub.Cols, sub.Rows = cols, rows
	switch {
	case claim:
		h.setOwnerLocked(sub)
	case h.owner == nil && h.ptyCols == 0 && sub.Origin == "local": // never-sized grid: first local fit adopts
		h.setOwnerLocked(sub)
	}
	if h.freezing { // Thaw re-runs the arbiter (hub.go:690-701)
		h.mu.Unlock()
		return nil
	}
	return h.applyGridLocked(sub.Origin, sub.Name) // releases mu before any IO
}
func (h *Hub) ResizeClient(sub *Subscriber, cols, rows int) error { return h.sizeClient(sub, cols, rows, false) }
func (h *Hub) ClaimGrid(sub *Subscriber, cols, rows int) error     { return h.sizeClient(sub, cols, rows, true) }
```

### The one-line test-helper change
`resizeHub` (relay/history_test.go:28-37) subscribes a *fresh* sub, reports, unsubscribes, and is called 39 times in package relay to set a genuine grid. Under the owner model a report from a non-owner on a known grid does nothing, so change that helper to `hub.ClaimGrid(sub, cols, rows)` (a window resize is a claim). The other `ResizeClient` call sites set a first grid (bootstrap) and keep working unchanged: relay/server_test.go:361, webserver/server_test.go:871, daemon/daemon_log_test.go:108, relay/scrollback_altscreen_test.go:372 and :420, hub_test.go NoOp/Freeze/Broadcast/Rows tests.

### INPT-05 hermetic shape
```go
// Two tests instead of one racy one.
func TestExitEvent_ListSessions_RunningHasNoExitCode(t *testing.T) {
	e := newCatEngine() // real /bin/cat on a real PTY (engine_test.go:39-43): stays running
	id, err := e.CreateSession(context.Background(), "running", "", 80, 24, nil)
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _ = e.KillSession(id) })
	// find id in e.ListSessions(); ExitCode == nil && Duration == nil
}
func TestExitEvent_ListSessions_ExitCodePopulatedForStopped(t *testing.T) {
	e := newCatEngine()
	e.backend = &spyBackend{} // no PTY: the hub's first Read errors and the exit watcher stops the session by itself
	id, _ := e.CreateSession(context.Background(), "stopped", "", 80, 24, nil)
	sess, _ := e.registry.Get(id)
	testutil.WaitFor(t, 3*time.Second, func() bool { return sess.GetState() == pty.StateStopped }, "exit watcher never stopped the spy session")
	// no manual SetState; ListSessions now has ExitCode != nil && *ExitCode == 0, Duration != nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Grid = min over local viewers (D-02); web/peer viewers never size the PTY (VIEW-02) | Grid = last owner's reported size; any write-capable viewer can own it | This phase | Replaces `applyGridLocked`'s loop and `ResizeClient`'s origin gate (hub.go:488) |
| Settle window caps the arbiter for 3 s after a handoff import (50903bee) | Removed: joins cannot move a known grid | This phase | Deletes `settleCols/settleRows/settleTimer`, `handoffSettle`, `endSettle`, `Start`'s timer arm, `Shutdown`'s stop, `NewHubFromState`'s assignment (handoff_state.go:218) |
| Pong = non-blocking send into `sub.Msgs` | Owed counter written by the write pump | This phase | Pong survives a full outgoing queue |
| Browser SPA viewer is "guest, never sizes" (D-03) | Same component, reports fit, claims on zoom/resize | This phase | `TerminalPanel.scale.test.tsx` D-03 assertions change |

**Deprecated/outdated:** `web/assets/terminal.js` and `web/terminal.html` are dead (TESTING.md:767, 776); do not extend them for INPT-06.

### The Phase 9 settle window: remove it
**Why it existed (Chesterton):** after a handoff the imported hub's grid was known (85x29) but viewers rejoined one at a time; min-among-local let the first rejoiner (100x30) *grow* the grid and the second (85x29) *shrink* it back, each bumping `geomEnd` to the ring end, so every later resume was `resume == geomEnd` and got a full replay (09-07-FIX2.md "The cause").
**Why the cause is gone:** under the owner model a join or plain report never changes a known grid (rule 4), and an imported hub has `ptyCols > 0` so the bootstrap exception cannot fire; there is no arbiter loop left to grow or shrink. Only a real claim or typing moves it, which is a genuine resize.
**What pins it afterwards:** rewrite `TestHub_HandoffSettleKeepsGrid` (handoff_state_test.go:309-370) as `TestHub_HandoffRejoinKeepsGrid`: same fixture (`settleHub`, imported 85x29 with `geomEnd > 0`), same joins (attach 100x30, GUI tab 85x29, resume at the ring end), same assertions (both `ReplayDelta`, no grid frame, `geomEnd` unchanged, `resizeFn` never called); then drop the "inside the window / after endSettle" half and add: typing by the 100x30 viewer claims, grid becomes 100x30, one `resizeFn(100,30)`, one grid frame. Replace `TestHub_HandoffSettleSkipsFrozen` (:373-390) by a frozen-claim test: claim while frozen, no PTY resize or broadcast, `Thaw` applies once (`TestHub_FrozenResizeAppliedOnThaw`, handoff_state_test.go:199-230, already covers most of it). V-02 must be re-run in the VMs (09-VM-CHECKS.md) because the arbiter it exercised is gone.
**Risk:** the removal is only safe if no non-user bytes claim; the classifier (Pattern 5) is therefore part of this decision.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | INPT-05's cause is the race between `ListSessions()` and the exit watcher on a PTY-less spy session; the failing line was never captured and I did not run the package | INPT-05 | Fix might not remove the flake; the executor's `-count=20` repro (below) confirms or refutes in one run |
| A2 | `pv` exists in both VMs | Validation | SC1 VM run needs a different slow reader; fallback given (perl one-liner) |
| A3 | No server-side rate limit on claims is needed (a write-cap holder already has a shell; client debounces) | Pattern 3/4 | A hostile holder could burn daemon CPU with alternating claims; add a per-connection token bucket (30/s) if the security review objects |
| A4 | `/health` and `/info` `api` stay 2 | Pattern 3 | Phase 11 may want to key on a bump; costs one constant |
| A5 | The user accepts a hub-owned FIFO instead of literally per-connection writers | Pattern 1, Pitfall 3 | If not: baton fallback (Alternatives table) |
| A6 | Clamp 20..500 cols, 5..200 rows | Pattern 4 | A very narrow `attach` pane (<20 cols) gets a wider PTY than its terminal |
| A7 | The typed-input classifier's final-byte set (`R c y t n I O M m`, OSC/DCS/APC) covers xterm.js's and common terminals' auto-replies | Pattern 5 | A missed reply class claims the grid spuriously; found by the VM V-02 re-run and the `attach` check |
| A8 | xterm applies a font-size change to cell metrics synchronously (the existing host path relies on it: TerminalPanel.tsx:988-993) | Pattern 6 | A claim measured with stale metrics is one step off; remeasure after one `requestAnimationFrame` |
| A9 | Relaunch showing the old grid until you type is acceptable | Pitfall 11 | UX complaint in the VM check; refinement described |
| A10 | `retryInit`'s orphan-snapshot case is only reachable by a race (both entry points need zero tabs) | INPT-04 | None for correctness; changes how the test is built |

## Open Questions

1. **INPT-03 as written edits an unreachable file. Decision needed before planning that task.**
   - What we know: `handleTerminalPage` redirects every `/sessions/{id}` hit to `/app/?session=..&cap=..` (server.go:787-795, route at :544-545); no route serves `terminal.html`; `web/embed.go` embeds it but nothing renders it; TESTING.md:767 and :776 state `terminal.js` "is now dead code, unreachable by any live share URL". The browser viewer in practice is `WebShareSessionView` with `wsURL` → `TerminalPanel` → `RelayClient`, which already counts pongs, tracks the paste and renders `Paste may have been cut short (N KB)` (relayClient.ts:556-613, TerminalPanel.tsx:479-482 and :1156-1161, SessionEndedBanner.tsx:87). `terminal.js` has no reconnect at all (terminal.js:1073-1081).
   - What's unclear: whether the user wants the notice added to dead code anyway (cheap: ~30 lines in the prelude as a pure `PasteAck` plus a `webTerminalInput.test.ts` case), or SC3 satisfied by the SPA.
   - Recommendation: do **not** edit `terminal.js`. Prove SC3 where users are: (a) a vitest for the `wsURL` guest path of `TerminalPanel` (the existing paste-notice tests in `TerminalPanel.reconnect.test.tsx:260-290` use the default props; add a `wsURL` variant), and (b) the VM `tailscale down` check with a **browser** (Chromium/Safari) on the home VM opened on the share link, during a paste. The orchestrator should confirm with the user; if they say "do it in terminal.js anyway", it is an isolated Wave 1 plan (files: `web/assets/terminal.js`, `frontend/src/lib/webTerminalInput.test.ts`, optionally `web/terminal.html` for a notice element) with a `PasteAck` factory placed in the prelude (the test loads only the text before `// Extract session ID from URL path`, webTerminalInput.test.ts:7-8) and `textContent` only.

2. **Hub FIFO vs literal per-connection writers.** See A5 and Pitfall 3. Recommendation: hub FIFO, quotas stay per connection (1,024 frames, 32 MiB).

3. **Relaunch UX (Pitfall 11).** Ship strict; add the lone-local-adopts refinement only if the VM check shows it hurts.

4. **Proxy queue.** CONTEXT says the proxy "gets the same treatment". Source reading says a queue there cannot help: a blocked `upstream.Write` holds `writeFrameMu` regardless of whether a queue sits in front of it, and `Ping` gives up after 5 s (write.go:277). What actually keeps the proxy's ping alive is the *server* never blocking (Pattern 1): a slow link only slows each 32 KiB frame write (~tens to hundreds of ms), and `Ping` interleaves between frames (the lock is a FIFO channel send, conn.go:286-305). Recommendation: no proxy code change; add `TestRemoteWSProxy_PasteBackpressureStaysOpen` (fixture peer that keeps reading and answering pings while its "PTY" drains slowly, short `SetHeartbeatForTest`, 3x the timeout, all bytes forwarded in order) as the real acceptance of "its own upstream ping can still be written". Add a proxy-side queue only if that test fails.

5. **Peer cap store keyed by session id alone (Pitfall 12).** Record, do not fix in this phase.

6. **>32 MiB paste into a slow reader** still drops the viewer (Pitfall 4). Say so in the M-item.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | all Go work | ✓ | go1.26.11 darwin/arm64 (`GOTOOLCHAIN=go1.26.11`) | — |
| node / pnpm | vitest, tsc | ✓ | node v24.22.1, pnpm 12.6.0 | — |
| Tart + VMs `portty-test`, `portty-test-2` | M-101, SC3, SC6 | ✓ | both `running` | — |
| `tailscale` CLI (host) | not needed on host | ✓ | `/usr/local/bin/tailscale` | the scripted `down`/`up` runs inside the office VM only |
| `expect` (host) | VM GUI/CLI drivers from 09-VM-CHECKS | ✓ | `/usr/bin/expect` | — |
| `pv` in the VMs | `stty raw -echo; pv -L 50k > /tmp/f` (SC1) | not checked (not on the host) | — | `perl -e 'binmode STDIN; while(read(STDIN,$b,1024)){print $b; select(undef,undef,undef,0.02)}' > /tmp/f` is about 50 KB/s and ships with macOS [ASSUMED] |
| `tailscale up` without a key on the office VM | SC3 drop and rejoin | unknown | — | CONTEXT: confirm before relying on it |

**Missing dependencies with no fallback:** none known.
**Missing dependencies with fallback:** `pv` (perl reader).

## Validation Architecture

> `.planning/config.json` has `workflow.nyquist_validation: false`, so this section is not required; it is included because the request asked for it. Each full suite runs once per plan (LuLu alerts); everything below is a targeted `-run` except the single full run per plan.

### Test Framework
| Property | Value |
|----------|-------|
| Go | `go test` with `-race`, `-short`; `GOTOOLCHAIN=go1.26.11`; helpers `internal/testutil` |
| Frontend | vitest 4 + jsdom 29 (`cd frontend && pnpm vitest run <file>`), tsc via `pnpm exec tsc --noEmit` |
| Quick run | `go test -race -run '<names>' ./internal/relay/ ./internal/webserver/` ; `cd frontend && pnpm vitest run src/lib/relayClient.test.ts` |
| Full suite | `go test -race -short ./...` ; `cd frontend && pnpm vitest run` ; `bash tests/check-brand.sh`; `bash tests/check-traceability-paths.sh` (once per plan, output to a file, never piped through `tail`) |

### Phase Requirements → Test Map
| Req | Behavior | Test Type | Automated Command | File Exists? |
|-----|----------|-----------|-------------------|-------------|
| INPT-01 | Viewer stays connected through 3x heartbeat timeout while the PTY writer blocks (webserver) | integration | `go test -race -run TestShare_PasteBacklogKeepsViewer ./internal/webserver/` | ❌ Wave 0 `internal/webserver/input_queue_test.go` |
| INPT-01 | Silent viewer reaped while backlog stays queued; after release, bytes delivered are an in-order prefix of the payload and the writer goes idle | integration | `go test -race -run TestShare_DeadViewerReapedBacklogDelivered ./internal/webserver/` | ❌ same file |
| INPT-01 | Local relay pump: same two properties | integration | `go test -race -run TestRelay_PasteBacklog ./internal/relay/` | ❌ Wave 0 `internal/relay/server_input_queue_test.go` |
| INPT-01 | FIFO order across two connections, per-connection quota blocks only that pump, writer death returns `ErrInputClosed`, `WaitInputIdle` | unit | `go test -race -run TestHubInput ./internal/relay/` | ❌ Wave 0 `internal/relay/input_test.go` |
| INPT-01 | Proxy stays open through a slow-draining peer, frames in order | integration | `go test -race -run TestRemoteWSProxy_PasteBackpressure ./internal/daemon/` | ❌ add to `remote_ws_proxy_test.go` |
| INPT-01 | Handoff `finish` waits for idle input within `restartDrain`, logs loss otherwise | unit | `go test -race -run TestHandoff_FinishWaitsForInput ./internal/daemon/` | ❌ add to `handoff_test.go` |
| INPT-01 | 4 MB paste into `pv -L 50k` keeps home peer tab, a browser viewer and a local tab; file matches byte for byte | VM | M-116 (below) | n/a |
| INPT-02 | Ping while the outgoing queue is full still gets its pong without waiting for a heartbeat | unit | `go test -race -run TestPongOwed ./internal/relay/` | ❌ Wave 0 `internal/relay/pong_test.go` |
| INPT-02 | Client clears a pending paste on the pong (count-based) | vitest | `pnpm vitest run src/lib/relayClient.test.ts -t "partial paste"` | ✅ extend `relayClient.test.ts:1099+` (comment/semantics update) |
| INPT-03 | `wsURL` guest `TerminalPanel` shows the notice on a drop mid-paste | vitest | `pnpm vitest run src/components/__tests__/TerminalPanel.reconnect.test.tsx` | ✅ add a `wsURL` case |
| INPT-03 | Browser viewer (SPA in a real browser on the home VM) during `tailscale down` | VM | M-118 | n/a |
| INPT-04 | Two hosts, equal session id → two tabs, two snapshot keys | vitest (App harness) | `pnpm vitest run src/components/__tests__/App.open-remote.test.tsx` | ✅ extend (and fix `tabpanel-__websession__sess-remote-1` at :156 and the source-inspection slice at :468-480) |
| INPT-04 | Retry drops a removed peer tab's snapshot | vitest (App harness, deferred `RetryDaemon`) | `pnpm vitest run src/components/__tests__/App.daemon-recovered.test.tsx` | ✅ extend |
| INPT-05 | Deterministic | Go | `go test -race -count=20 -run TestExitEvent ./internal/daemon/` then the one full-package `-race -count=20 ./internal/daemon/` | ✅ edit `engine_exit_test.go` |
| INPT-06 | Owner model: report vs claim vs typing, bootstrap, freeze, owner leaves, read-only refused, clamp, unknown-grid guard | unit | `go test -race -run 'TestHub_Grid\|TestHub_Resize\|TestHub_HandoffRejoin' ./internal/relay/` | rewrite `hub_test.go:407-` MinAmongLocal; new `internal/relay/hub_grid_owner_test.go` |
| INPT-06 | `IsUserInput` table + fuzz seeds | unit | `go test -race -run 'TestIsUserInput\|FuzzIsUserInput' ./internal/relay/` | ❌ add to `oscabsorb_test.go` |
| INPT-06 | Wire: `ParseResize`/`MakeClaimFrame`; old 4-byte frame is a report; web claim with write cap applies; with a read-only cap does not; plain web resize still leaves a known grid alone | integration | `go test -race -run 'TestWeb.*Claim\|TestWebReadPump_DropsGuestResize' ./internal/webserver/` | `server_test.go:934` stays as the back-compat pin; new `internal/webserver/claim_test.go` |
| INPT-06 | Attach: connect = report frame, SIGWINCH = claim frame | unit | `go test -race -run TestMakeClientClaimFrame ./internal/attach/ .` | extend `internal/attach/attach_test.go`, `cmd_attach_test.go` |
| INPT-06 | `RelayClient.sendClaim` bytes (`[0x11,c,c,r,r,0x01]`), `encodeResizeFrame` unchanged | vitest | `pnpm vitest run src/lib/relayClient.test.ts` | extend |
| INPT-06 | Panel reports fit (never `term.cols`), claims on window resize and font change only, follows server grid, debounces | vitest | `pnpm vitest run src/components/__tests__/TerminalPanel.scale.test.tsx` | rewrite the D-03 / host-path assertions |
| INPT-06 | Peer-tab Cmd+ zoom changes font and claims | vitest | `pnpm vitest run src/components/__tests__/WebShareSessionView.test.tsx` | extend |
| INPT-06 | Home peer tab Cmd+ and window resize reflow the office shell; office typing hands it back; browser and `attach` the same; V-02 re-run | VM | M-117 and the V-02 re-run | n/a |

### Sampling Rate
- **Per task commit:** the targeted `-run` / `vitest <file>` for the files touched.
- **Per wave merge:** the package-level run of the packages touched (`./internal/relay/ ./internal/webserver/ ./internal/daemon/`), once.
- **Phase gate:** the full suite once, `go build ./... && go vet ./...`, tsc, `check-brand.sh`, `check-traceability-paths.sh`, then the VM rows.

### Wave 0 Gaps
- [ ] `internal/relay/input_test.go`, `internal/relay/pong_test.go`, `internal/relay/hub_grid_owner_test.go`, `internal/relay/server_input_queue_test.go`
- [ ] `internal/webserver/input_queue_test.go`, `internal/webserver/claim_test.go`
- [ ] Update `resizeHub` (relay/history_test.go:28-37) to `ClaimGrid`; rewrite the two settle tests and `TestHub_ResizeHostAuthority_MinAmongLocal`
- [ ] Add `sendClaim() {}` to the five RelayClient mocks (Pitfall 13)
- [ ] No framework install needed.

### How to build each hard test (so executors do not improvise)
- **Blocking fake hub (SC1, webserver):** `testServerWithHub` gives `inputCaptureR/W` pipes (capability_test_helpers.go:131-169). The pipe write blocks until the test reads, so a gated reader goroutine *is* the blocking fake. Use `ws.heartbeatEvery, ws.heartbeatTimeout = 50ms, 100ms` (as `joinViewer` does, liveness_test.go:14-24). Client must run a read loop (coder/websocket's client answers pings inside `Read`, TestWSS_KeepsReadingViewer, liveness_test.go:65-76). Send 8 MiB as 256 frames of 32767 bytes; hold the reader for 3x timeout; assert `hub.RemoteViewerCount() == 1` throughout; release; assert sha256 and order.
- **"A viewer whose network really dies is still dropped once its backlog drains" (SC1 wording):** the drop is by heartbeat and happens *while* the backlog is still queued (the pump keeps reading, `Heartbeat` pings, no pong, `CloseNow`); "once its backlog drains" is when the writer goroutine exits and memory is released. Test both halves: reaped while blocked; after release, delivered bytes are an in-order prefix and `WaitInputIdle` returns true.
- **INPT-02 full queue:** the 256-deep `sub.Msgs` is hard to fill deterministically through a real socket (TCP buffers absorb megabytes, and the hub kicks on the next broadcast once full). Unit-test `PongOwed` (N `Owe()` while no one drains, then N pongs written by a fake writer) and add one integration test through the real pump asserting the pong arrives while the client has stopped reading output.
- **Proxy:** fixture peer from `newFixtureRemotePeerWithWS` variant that reads continuously and drains to a slow sink; `proxyWithHeartbeat` (remote_ws_proxy_test.go:669-684) gives 50 ms / 100 ms.
- **App harness retry (INPT-04):** both `retryInit` entry points require zero tabs (`App.tsx:1240` banner, `:1005` recovered), so the orphan arises only if a peer tab appears while `retryInit` is awaiting. Use `mountApp` with a `RetryDaemon` returning a deferred promise, fire `wailsEvents.get('daemon:recovered')`, click a peer row, `putPeerSnapshot` under `peerSnapshotKey(host, id)`, resolve with one local session, assert `takePeerSnapshot` is undefined. Extract the "which peer tabs does this replace" computation into `lib/tabs.ts` so it also has a plain unit test.

### VM checks (Tart; agent-driven; ephemeral tailnet nodes, never stop them)
| Row | SC | What | Evidence |
|-----|----|------|----------|
| M-116 (new) | 1 | Office session: `stty raw -echo; pv -L 50k > /tmp/f` (or the perl reader), paste a 4 MB base64 payload (no newline, sha256 recorded) in the home peer tab, then a browser viewer on the share link (home VM), then the office local tab (three runs). Heartbeat timeout is 15 s; 4 MB at 50 KB/s is about 80 s. PASS: no viewer reconnects (`daemon.log` has no `heartbeat timeout`/`proxy heartbeat timeout`/`slow` for the session), `cmp`/sha256 of `/tmp/f` equals the payload | `daemon.log` viewer-diag lines, `sha256` |
| M-101 (edit) | 1 | Re-run as written; text changes: after a drop the host still delivers every byte it had read, so the file is an exact prefix of the payload that stops growing only once the queue drains (allow the drain time before the "stops growing" check) | existing rule + this note |
| M-118 (new) | 3 | Browser viewer on the home VM, paste 5 MB into `stty raw -echo; cat > /tmp/f`, `tailscale down` on the office VM within ~1 s, `tailscale up` after 15 s | the browser shows `Paste may have been cut short (M KB)` or `Paste interrupted: N of M KB sent` (a real browser socket can leave `bufferedAmount`, M-101's variant), no `innerHTML`/CSP console errors |
| M-117 (new) | 6 | Home peer tab on an office session: `tput cols` in the office shell before; Cmd+Shift+= in the home tab; `tput cols` smaller and text larger; resize the home window: changes again; type in the office local tab: back to the office size; browser viewer: window resize then typing; `portty attach` on each side: SIGWINCH then typing. Also: V-02 re-run (09-VM-CHECKS V-02), relaunch the office GUI (Pitfall 11), two viewers alternate typing (Pitfall 9) | `daemon.log` `viewer-diag: grid session=... a -> b by=... why=...` lines, `tput cols`, V-02's `seqcheck.sh TICK` |

Drive the GUI from inside the VM, never the host (user memory; CONTEXT). Zoom needs a real key event in the VM's WebView; resize with AppleScript window bounds.

## Security Domain

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no (unchanged) | Existing HMAC capability token (`capability.Sign`) |
| V3 Session Management | no (unchanged) | Existing grants and `isGrantActive` recheck (server.go:896) |
| V4 Access Control | **yes** | Resize reports/claims and typing-claims honor `sub.ReadOnly` (derived from the verified cap's `write` perm, server.go:821); read-only caps can never move the PTY grid. A write cap can, as decided |
| V5 Input Validation | **yes** | `ParseResize` length check, clamp 20..500 x 5..200, zero ignored, unknown flag bits ignored; keep the 32 KiB read limit (input_limit_test.go) |
| V6 Cryptography | no | none added |
| V11/V13 Business logic and API (DoS) | **yes** | Bounded queues (1,024 frames / 32 MiB per connection), clamped emulator allocation, debounced client claims |

### Known Threat Patterns for this stack
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Read-only viewer resizes the shell | Tampering / Elevation | `ReadOnly` gate in the pump **and** in `Hub.sizeClient` (defense in depth, as the code does for VIEW-02 today) |
| Huge size claim forces a giant emulator allocation | DoS | Clamp at intake; `resizeLiveEmulator` only ever sees clamped values |
| Auto-replies (not a human) stealing the grid | Tampering (integrity of ownership) | `IsUserInput` classifier; ownership changes only on typing, claim, or bootstrap |
| Claim storm (write-cap holder alternates sizes) | DoS | Debounce; a holder already has a shell (A3); optional token bucket |
| Unbounded memory from queued input | DoS | Per-connection frame and byte quotas; hub writer dies on PTY error |
| CSP / token leakage | Info disclosure | No change: no new route, no `innerHTML`, no URL in logs (keep `viewer-diag` counts-only, never payloads) |

## Sources

### Primary (HIGH confidence): files opened with Read or sed this session
- `internal/webserver/server.go` (pump 932-993, write pump 1007-1028, sub setup 855-875, `handleTerminalPage` 771-795, routes 491-648, `readonly` 821)
- `internal/relay/server.go` (pump 154-216, join push 114-121), `internal/relay/hub.go` (Subscriber 23-70, Hub 74-161, `ResizeClient` 485-504, `applyGridLocked` 510-556, `Freeze/Thaw` 645-701, settle 717-746, `WriteInput` 1427, `resizeLiveEmulator` 1343-1388, `Join` 1690-1767, `Cols/Rows` 1773-1792), `internal/relay/protocol.go`, `internal/relay/heartbeat.go`, `internal/relay/oscabsorb.go` (Filter)
- `internal/daemon/remote_ws_proxy.go`, `internal/daemon/handoff.go` (header, timeouts 84-91, `finish` 748-783), `internal/daemon/engine.go` (206-304, `ListSessions` 376-434), `internal/daemon/remote_caps.go`, `internal/daemon/types.go:60-62`, `internal/daemon/client.go:93-96`, `internal/daemon/share.go:326`, `internal/pty/native.go:161-168`, `internal/pty/session.go:131-175`, `internal/pty/reap_other.go`
- `internal/attach/loop.go`, `internal/attach/attach_unix.go`, `internal/attach/attach.go` (`MakeClientResizeFrame`), `cmd_attach.go:340-430`
- coder/websocket v1.8.15 `write.go:276-375`, `conn.go:215-312` (module cache)
- Frontend: `relayClient.ts` (whole), `TerminalPanel.tsx` (39-61, 196-256, 330-636, 900-1010, 1148-1161), `WebShareSessionView.tsx`, `terminalScale.ts`, `guestTerminal.ts`, `initialGrid.ts`, `App.tsx` (577-600, 659-665, 796-1009, 1136-1285), `Sidebar.tsx:70-100`, `peerSnapshots.ts`, `tabs.ts`, `web/assets/terminal.js` (1-100, 1030-1110)
- Tests read: `webserver/input_limit_test.go`, `liveness_test.go`, `resume_test.go` (helpers), `capability_test_helpers.go:100-220`, `server_test.go:862-973`, `relay/hub_test.go:395-640`, `handoff_state_test.go:190-420`, `history_test.go:20-45`, `daemon/remote_ws_proxy_test.go` (fixture + 669-730), `daemon/engine_exit_test.go`, `engine_test.go:183-250`, `frontend/.../appHarness.ts`, `App.open-remote.test.tsx`, `App.daemon-recovered.test.tsx`, `TerminalPanel.scale.test.tsx` (1-120), `TerminalPanel.reconnect.test.tsx:255-300`, `relayClient.test.ts:1095-1140`, `webTerminalInput.test.ts`
- Planning/docs: `10-CONTEXT.md`, `ROADMAP.md` Phase 10, `REQUIREMENTS.md`, the five todos, `09-07-FIX2.md`, `09-VM-CHECKS.md` (head), `TESTING.md` (headings, Section 6, M-101, 767-776), root and `.claude` CLAUDE.md
- Environment: `go version`, `tart list`, `node`, `pnpm`

### Secondary / Tertiary
- None. No web or Context7 lookups were needed: the work is internal code and a pinned dependency whose source was read directly. The `research-plan` seam and provider fetches were not used (nothing to fetch).

## Suggested Plan Split

Six plans in three waves. Disjoint file sets inside a wave, so they can run in parallel without conflicts; `TESTING.md` is touched only by the last plan.

| Wave | Plan | Reqs | Files (exclusive) | Depends on |
|------|------|------|-------------------|-----------|
| 1 | 10-01 Flake | INPT-05 | `internal/daemon/engine_exit_test.go` | none |
| 1 | 10-02 Peer tab keys | INPT-04 | `frontend/src/App.tsx`, `frontend/src/lib/tabs.ts`, `components/Sidebar.tsx` (optional host-aware `aria-current`), `App.open-remote.test.tsx`, `App.daemon-recovered.test.tsx`, `lib/__tests__/tabs.test.ts` | none |
| 1 | 10-03 Input queue + pong | INPT-01, INPT-02 | `internal/relay/input.go`, `pong.go`, `hub.go` (input fields only), `relay/server.go`, `webserver/server.go`, `daemon/handoff.go` (`finish`), `daemon/remote_ws_proxy_test.go`, new Go tests, `frontend/src/lib/relayClient.ts` (comments only: lines 157-163, 197-200, 370-374, 585-594) and `relayClient.test.ts` | none |
| 2 | 10-04 Grid owner (Go) | INPT-06 | `relay/protocol.go`, `hub.go` (owner, settle removal), `handoff_state.go:218`, `oscabsorb.go`, `relay/server.go` + `webserver/server.go` (the resize/typing cases and `GridKnown` push guard), `internal/attach/*`, `cmd_attach*.go`, Go tests incl. the settle rewrites | 10-03 (same pump code) |
| 2 | 10-05 Viewer (TS) | INPT-06 | `relayClient.ts` (`encodeClaimFrame`, `sendClaim`), `TerminalPanel.tsx`, `WebShareSessionView.tsx`, `lib/` fit helper, `App.tsx` (per-tab font sizes keyed by tab id, pass `onFontSizeChange` to peer tabs), the five RelayClient mocks, scale/reconnect tests | 10-02 (App.tsx), 10-03 (relayClient.ts comments); wire format fixed by this document, so it runs in parallel with 10-04 |
| 3 | 10-06 Browser notice, docs, VM | INPT-03, all | INPT-03 per Open Question 1 (default: the `wsURL` vitest only); `TESTING.md` (Section 2 note, Section 4 rows, Section 5 M-101 edit + M-116..M-118 in new Category AZ); VM rows in `10-VM-CHECKS.md`; single full-suite gate | 10-01..10-05 |

Parallel-safety notes: 10-03 and 10-04 both edit the two pumps, so they are sequential; 10-04 (Go) and 10-05 (TS) share no files; the two Tart VMs are one shared resource, so every VM row stays in 10-06. If the user chooses to edit `terminal.js` for INPT-03, make it an extra Wave 1 plan (`web/assets/terminal.js`, `webTerminalInput.test.ts`).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH, no new packages, versions read from manifests.
- Architecture (INPT-01/02, INPT-04, INPT-05, wire form, owner model, settle removal): HIGH on the code facts, MEDIUM on design choices flagged in the Assumptions Log.
- INPT-06 client state machine: MEDIUM, designed from reading `TerminalPanel`; needs the vitest rewrite and the VM check to confirm.
- Pitfalls: HIGH where tied to a file:line, MEDIUM for A7/A8.

**Research date:** 2026-11-20
**Valid until:** until `internal/relay/hub.go`, the two pumps or `TerminalPanel.tsx` change on another branch (code-bound research; re-check line numbers after any merge). Not time-sensitive otherwise.
