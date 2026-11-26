---
phase: 19-unattended-desktop-viewing
plan: 06
subsystem: desktop-viewer-frontend
tags: [react, webrtc, vitest, chromedp, csp, reconnect, letterbox, ios-autoplay]
status: complete

requires:
  - phase: 19-01
    provides: signalling protocol v1 (hello, offer, answer, state, size, ping, pong), close codes 4401, 4410, 1012
  - phase: 19-03
    provides: per-viewer tracks, close code 4429 for a full share
  - phase: 19-04
    provides: GET /desktop/check, X-Portty marker on the other-account refusal, static first-load pages
provides:
  - "detectMode 'desktopview' for /desktop and /desktop/...; main.tsx mounts DesktopView and never App"
  - "frontend/src/lib/desktopClient.ts: createDesktopClient with injectable transport, check, beforeRedial, peer-connection factory, clock, doc and win; webSocketTransport, defaultDesktopWsUrl, fetchCheck, RECONNECT"
  - "frontend/src/components/DesktopView.tsx: every Surface 5 state and error, Tap to Start, videoContentRect and onVideoRect"
  - "--color-viewer-bg token (light and dark) and the .desktop-* styles"
  - "TestDesktopCSP_E2E: headless Chrome under the real header CSP receives RTP with zero violations"
affects: [19-12, 19-15, 19-16, 20, 21]

actuals:
  tokens: 32500
  tasks: 3
  commits: 5
plan_head_before: f8f4953b09919cdec0c652e1ac65d926a3beee79
plan_head_after: 866c3bea9e789d8ee9e05aef7a0f7d7424631b1b

tech-stack:
  added: []
  patterns:
    - "Generation counter: every connect takes the next gen; callbacks from a superseded socket or peer connection drop out"
    - "Classification by fixed codes only: 4401/4410 close codes, or GET /desktop/check after a handshake that never produced hello; never message text"
    - "One overlay at a time: the first of errors, state cards, tap card, Reconnecting chip, Connecting card, Locked chip, count chip wins"

key-files:
  created:
    - frontend/src/lib/desktopClient.ts
    - frontend/src/lib/__tests__/desktopClient.test.ts
    - frontend/src/components/DesktopView.tsx
    - frontend/src/components/__tests__/DesktopView.test.tsx
    - internal/webserver/desktop_csp_e2e_test.go
  modified:
    - frontend/src/lib/webMode.ts
    - frontend/src/lib/__tests__/webMode.test.ts
    - frontend/src/main.tsx
    - frontend/src/style.css

key-decisions:
  - "A missing pong after a wake rebuilds at once (backoff reset), not after the 1 s step: the 3 s wait has already been spent and the user just came back"
  - "A wake during the very first connect is ignored (pageshow also fires at load); a wake with a connection that already failed redials at once"
  - "Hello timeout (10 s) is a plain retry, not a /desktop/check classification: a hung network would hang the check too"
  - "DesktopView takes an optional onReload prop (default location.reload) because jsdom cannot replace location.reload; the plan's props shape is otherwise unchanged"
  - "Unknown state values are dropped, not rendered: only an enum value and integers are read from the wire (T-19-22)"

requirements-completed: [DVWR-02, DESK-01, DESK-03]

coverage:
  - id: F1
    description: "/desktop and /desktop/... select the 'desktopview' mode; /desktops and /foo/desktop/ stay 'desktop'; /app/ stays 'web'; main.tsx mounts DesktopView for it and adds no is-desktop class"
    requirement: "DVWR-02"
    verification:
      - kind: unit
        ref: "frontend/src/lib/__tests__/webMode.test.ts#detectMode desktopview (Phase 19)"
        status: pass
    human_judgment: false
  - id: F2
    description: "In headless Chrome under the real share-server header CSP, the built /desktop/ page connects, negotiates a PeerConnection and receives video RTP (bytesReceived > 0) with zero securitypolicyviolation events; the page CSP equals the /app/ CSP"
    requirement: "DVWR-02"
    verification:
      - kind: e2e
        ref: "internal/webserver/desktop_csp_e2e_test.go#TestDesktopCSP_E2E"
        status: pass
    human_judgment: false
  - id: F3
    description: "The client reconnects by itself: capped backoff 1, 2, 4, 8, 10, 10 s, unreachable after 30 s while retries continue, a returning track restarts the backoff, a hello timeout is retried, a rejecting beforeRedial is a failed attempt, 'stop' ends the loop, and only 4401, 4410 or a check verdict of token, off or other_account end it"
    requirement: "DESK-03"
    verification:
      - kind: unit
        ref: "frontend/src/lib/__tests__/desktopClient.test.ts#backoff and unreachable"
        status: pass
      - kind: unit
        ref: "frontend/src/lib/__tests__/desktopClient.test.ts#beforeRedial hook"
        status: pass
      - kind: unit
        ref: "frontend/src/lib/__tests__/desktopClient.test.ts#classification by fixed codes"
        status: pass
    human_judgment: false
  - id: F4
    description: "Wake: visibilitychange to visible, pageshow, online and wake('external') emit typed wake events; while streaming a ping with a 3 s deadline decides between keep and rebuild; while reconnecting a wake redials at once and restarts the backoff; a 1012 close redials at once; ICE failed, or disconnected held 2 s, rebuilds socket and peer connection and closes the old one"
    requirement: "DESK-03"
    verification:
      - kind: unit
        ref: "frontend/src/lib/__tests__/desktopClient.test.ts#wake"
        status: pass
      - kind: unit
        ref: "frontend/src/lib/__tests__/desktopClient.test.ts#ICE"
        status: pass
    human_judgment: false
  - id: F5
    description: "The viewer shows the Surface 5 copy exactly: Connecting card, Streaming to N viewers chip for 3 s and on count change, Locked chip that stays, the four state cards over a scrim, a static Reconnecting chip, Can't reach this Mac with Reload Page, the three distinct error cards, UserRefused for another account, and a focused Tap to Start card when play() is rejected (button or scrim starts it)"
    requirement: "DESK-03"
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/DesktopView.test.tsx"
        status: pass
    human_judgment: false
  - id: F6
    description: "One muted playsinline autoplay video with object-fit contain and no transform, zoom, image-rendering, size attribute or inline style; no key, pointer or context-menu listener; videoContentRect math and onVideoRect after the video resize and window resize; --color-viewer-bg once per token block; 150 ms chip fade, none under reduced motion"
    requirement: "DESK-01"
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/DesktopView.test.tsx#layout and scope"
        status: pass
      - kind: unit
        ref: "frontend/src/components/__tests__/DesktopView.test.tsx#videoContentRect"
        status: pass
      - kind: unit
        ref: "frontend/src/components/__tests__/DesktopView.test.tsx#style.css"
        status: pass
    human_judgment: false
  - id: F7
    description: "Chip and card read correctly over the black letterbox in light and dark appearance; the picture is near 1:1 sharp on a Retina window at the 2560 px stream size; a link saved to the iPhone Home Screen opens as Portty Desktop and keeps its ?cap= token; a first load while the Mac is unreachable shows the browser's own error page"
    requirement: "DESK-01"
    verification: []
    human_judgment: true

duration: 70min
completed: 2026-11-26
---

# Phase 19 Plan 06: Desktop Viewer Summary

**A `/desktop/` link now boots its own viewer (never the Wails shell): a seam-complete reconnecting client (injectable transport, pre-redial hook, wake events, capped backoff, ping liveness, fixed-code error branching) and a `DesktopView` with every Surface 5 state, proven to receive video under the real header CSP in headless Chrome with no policy violation.**

## Accomplishments

- **Mode and boot:** `detectMode` returns `'desktopview'` for `/desktop` and `/desktop/...` with the same boundary rule as `/app`; `main.tsx` computes the mode once, mounts `DesktopView` inside the root `ErrorBoundary`, and adds `is-desktop` only for `'desktop'`.
- **Client seams (1, 2, 3, 7):** `createDesktopClient` takes a transport factory, a check function, a `beforeRedial` hook, a peer-connection factory, a clock, a document and a window. The only `new WebSocket` is inside `webSocketTransport`. A generation counter drops callbacks from a superseded socket or peer connection. Every rebuild closes the previous peer connection and transport.
- **Reconnect (DESK-03):** backoff 1, 2, 4, 8, 10, 10 s, `unreachable` after 30 s without a track (retries continue), a returning track restarts the backoff. Wake sources are `visibilitychange`, `pageshow`, `online`, a 1012 close and `wake('external')`; while streaming a wake pings with a 3 s deadline. ICE `failed`, or `disconnected` held 2 s, rebuilds both halves.
- **Classification (seam 7):** an open socket closed with 4401 or 4410 is final; a handshake that never produced `hello` asks `GET /desktop/check` (204 retry, 401 token, 410 off, 403 with `X-Portty` other account, anything else or a thrown fetch retry, 5 s abort). Message text is never read.
- **Viewer (Surface 5):** one overlay at a time, copy word for word from the Copywriting Contract, plus the planner-added Desktop paused card. Tap to Start takes focus on mount; the scrim tap also starts it. The one `<video>` is never re-created.
- **Seam 13:** `videoContentRect` (pure) and `onVideoRect` (video `resize` and window `resize`) for Phase 20 pointer normalisation.
- **CSP e2e (seam 5):** `TestDesktopCSP_E2E` serves the built `frontend/dist` through the real share server with a synthetic source, asserts the `/desktop/` CSP equals the `/app/` CSP, then loads the link with `#debug` in headless Chrome and waits for `bytesReceived > 0` with an empty violation list. Passed 5 of 5 runs.
- **Debug hook:** opt-in by `#debug` or localStorage `portty:desktop-debug=1`; publishes counts only (status, state, viewers, framesDecoded, frame size, bytesReceived, cspViolations) to `window.__desktopStats` and the page title. No token, no SDP.

## Task Commits

1. **Task 1 (tracer): mode, client, view, CSP e2e** - `bdde4173` (feat)
2. **Task 2 RED: client seam tests** - `3765f9dc` (test)
3. **Task 2 GREEN: reconnect, wake, ping, classification** - `a9642a3d` (feat)
4. **Task 3 RED: viewer state and look tests** - `1e68d08d` (test)
5. **Task 3 GREEN: view, styles, token** - `866c3bea` (feat)

## TDD Gate Compliance

Tasks 2 and 3 each have a `test(19-06)` commit before their `feat(19-06)` commit. Task 1 is a tracer and has no RED, as the plan intends.

- **Task 2 RED:** `RED_EVIDENCE_OK` from `gsd check tdd-red-evidence` on a Vitest `tap-flat` run of the target test "backoff and unreachable > redials after 1000, 2000, 4000, 8000 ms then every 10000 ms". Semantic assessment: the target executed and failed on `redial at 1000 ms: expected 1 to be 2`, the planned assertion, because the Task 1 client had no reconnect loop. 22 of 35 cases failed on assertions; 13 passed against the Task 1 client because it already did hello, offer, answer, events, `fetchCheck` and `stop` (investigated, not trusted: those cases exercise behaviour Task 1 built, not vacuous stubs). The classifier rejected the full-file run (`Malformed TAP`: Vitest's multi-line assertion messages are not strict TAP), so the record is for the single target test, whose message is one line.
- **Task 3 RED:** `RED_EVIDENCE_OK` for "streaming chip and live region > keeps a Locked chip ...", failing on `expected undefined to be 'Locked'` (no chip rendered by the Task 1 view). 24 of 28 cases failed on assertions; 4 passed (stop on unmount, title and meta, no-listener check and the one-video check hold for the minimal view too).
- **Test-first corrections inside GREEN commits (not a flaky-test cover-up):** two Task 2 expectations assumed the first reconnect status sequence was `['unreachable']` and that a never-opened socket keeps redialling past 30 s; hello timeouts also move the status to `reconnecting`, and the redial timer was ten seconds out. Two Task 3 assertions expected no card after a successful tap; with no `playing` event in jsdom the Connecting card correctly follows the tap card. All four corrections change the test's expectation to the real, specified behaviour, with the product code unchanged by them.

## Deviations from Plan

**1. [Rule 1 - Test expectations] Four assertions written against a wrong model**
- See TDD Gate Compliance. Files: `desktopClient.test.ts` (in `a9642a3d`), `DesktopView.test.tsx` (in `866c3bea`).

**2. [Plan interpretation] `onReload` prop added**
- jsdom's `location.reload` is unforgeable, so the Reload Page click cannot be observed through `window.location`. `DesktopView` takes an optional `onReload` defaulting to `window.location.reload()`. The plan's `{ onVideoRect? }` shape is a subset.

**3. [Plan interpretation] A missing pong redials at once**
- The plan says a failed ping "rebuilds the connection from scratch with capped backoff"; I reset the backoff and redial immediately (the 3 s deadline was already spent, and a wake means the user is looking now). Ordinary failures (close, ICE, hello timeout) use the 1, 2, 4, 8, 10 s steps.

**4. [Added bound] Safeguards beyond the plan text**
- `gatherTimeoutMs` (3 s) so a stuck ICE gathering still sends the offer, `iceDisconnectedMs` (2 s) as a named constant, a 5 s abort on `fetchCheck`, and a `makeTransport` throw treated as a failed attempt. All are inside `RECONNECT` or `fetchCheck`; none changes a contract.

**Total deviations:** 4 (1 test fix, 3 interpretations or bounds). No change to the contracts 19-12, 19-15, 20 and 21 build on.

## Known Stubs

None. No placeholder text, empty data flows or TODOs in the files this plan touched. Nothing added to `.planning/WINDOWS.md`.

## Threat Flags

None beyond the plan's register. T-19-21 (same-origin socket and fetch only, token never logged, debug hook counts only), T-19-22 (only enum and integers read from the wire; unknown state dropped; no server text rendered), T-19-23 (no CSP change; `TestDesktopCSP_E2E` asserts zero violations and an unchanged header) are implemented and tested. T-19-SC: no package added (icons already installed, vanilla `RTCPeerConnection`).

## Verification

- `pnpm --dir frontend exec tsc --noEmit -p tsconfig.json` clean; `pnpm --dir frontend build` ok.
- `pnpm --dir frontend test`: 124 files, 1901 tests pass (includes the colour-token source gate and the new viewer tests).
- Counts: `desktopClient.test.ts` 35 cases, `DesktopView.test.tsx` 28, `webMode.test.ts` 17.
- `GOTOOLCHAIN=go1.26.11 go test -tags=e2e -count=3 -run TestDesktopCSP_E2E ./internal/webserver/` pass (headless Chrome on the host against a loopback synthetic source; no Portty app or daemon, no capture, no tailnet). `go vet -tags=e2e` and `gofmt -l` clean.
- `bash tests/check-share-removal.sh` clean, `bash tests/check-brand.sh` clean.
- Acceptance greps: `desktopview` in webMode.ts 3 (>=2), `DesktopView` in main.tsx 3 (>=2), `Portty Desktop` in DesktopView.tsx 2, `--color-viewer-bg: #000000` 2, `Can't reach this Mac` 1, `Tap to Start` 1, `export function videoContentRect` 1, `beforeRedial` in desktopClient.ts 3, `'server-restart'` 2, `new WebSocket` 1.
- Not run: no VM and no real-media decode (the synthetic fixture is not decodable; real frames in Chrome and Safari are 19-12's V7 rows and the real-Mac M-rows). The four `verification: backstop` truths (dark and light legibility, 1:1 sharpness on Retina, Home Screen title and token, the browser's own first-load error page) are human or VM checks.

## Hand-off notes

- **19-16 (TESTING.md):** add `frontend/src/lib/__tests__/desktopClient.test.ts`, `frontend/src/components/__tests__/DesktopView.test.tsx` and `internal/webserver/desktop_csp_e2e_test.go` to the suite manifest and traceability map (DVWR-02, DESK-01, DESK-03). The e2e file uses `//go:build e2e`, the same exception as `browser_csp_e2e_test.go` (needs installed Chrome and a built `frontend/dist`; it skips with a clear message if `frontend/dist/index.html` is missing). TESTING.md was not touched here. Add manual rows for the four backstop truths above.
- **19-12 and 19-15 (VM rows):** open the link with `#debug` and read `window.__desktopStats` or the page title (`Portty Desktop (debug fd=... fw=... csp=... st=...)`) from Chrome DevTools titles or safaridriver; WebKit screenshots in the VMs are blank.
- **Phase 20:** consume `videoContentRect` and `onVideoRect`; the client already ignores unknown `caps` and the wire has room for `"input":1`.
- **Phase 21:** pass `beforeRedial`, a `transport` factory and call `wake('external')`; `portty:wake` is not listened for here.

## Self-Check: PASSED

- Files exist: `desktopClient.ts`, `desktopClient.test.ts`, `DesktopView.tsx`, `DesktopView.test.tsx`, `desktop_csp_e2e_test.go`, and the edited `webMode.ts`, `main.tsx`, `style.css`.
- Commits `bdde4173`, `3765f9dc`, `a9642a3d`, `1e68d08d`, `866c3bea` are ancestors of HEAD (`git rev-list --count` from the plan base is 5).
- No `STATE.md`, `ROADMAP.md`, `TESTING.md` changes; `.gsd/tdd-evidence/` is ignored and not committed.
