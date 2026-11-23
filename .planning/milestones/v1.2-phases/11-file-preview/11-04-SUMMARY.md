---
phase: 11-file-preview
plan: 04
subsystem: daemon
status: complete
tags: [daemon, preview, unix-socket, api-version, client]
requires:
  - "11-02: relay.Hub.SendPreview, PreviewMIMEs, PreviewMaxBytes, ErrPreviewBusy/Invalid"
provides:
  - "POST /sessions/{id}/preview on the daemon unix socket mux only"
  - "APIVersion 3 and PreviewAPI 3 (MinDaemonAPI stays 1)"
  - "DaemonClient.SendPreview with typed errors and a 429 wait"
affects: [11-08, 11-10]
tech-stack:
  added: []
  patterns:
    - "Buffer the body, answer 202, stream after the response, so the freeze gate's write lock covers only the local copy"
    - "go/parser import gate as a source-level security test"
key-files:
  created:
    - internal/daemon/preview.go
    - internal/daemon/preview_test.go
  modified:
    - internal/daemon/api.go
    - internal/daemon/types.go
    - internal/daemon/client.go
    - internal/daemon/client_test.go
    - internal/daemon/api_test.go
    - internal/webserver/files_removed_test.go
decisions:
  - "Key validation is in the handler, before the body is read, so a bad key never makes the daemon buffer 20 MB"
  - "doRetrying treats 429 with Retry-After like the freeze's 503 (only the preview route sends 429)"
  - "GET /app/* on the share server is the single-page-app fallback, not a preview route; the absence test pins its test-time 503"
metrics:
  duration: "~60 min"
  completed: 2026-11-21
actuals:
  tokens: 7400     # chars/4 over the realized diff (29699 chars)
  tasks: 3
  commits: 3       # measured: git rev-list --count 8e07f421..HEAD before the SUMMARY commit
plan_head_before: 8e07f421a44029934ff9a9fd4e846e54ea2f2885
plan_head_after: 6c7411e15858eec93f10036523a67e7b67e410e9
---

# Phase 11 Plan 04: Daemon preview route, API 3 and DaemonClient.SendPreview Summary

A same-user local process can now hand the daemon a file's bytes over the owner-only unix socket. The daemon checks them, buffers them, answers 202 with the viewer count, and streams them to the session's viewers through `relay.Hub.SendPreview`. No route anywhere takes a path. The relay loopback and the share server have no preview route, and the preview source imports no file package.

## Route (normative for 11-08)

`POST /sessions/{id}/preview`, registered in `registerRoutes` beside `POST /handoff` on `a.mux` only. It sits behind the freeze gate, so a frozen daemon answers 503 with Retry-After. The handler is `handlePreview` in `internal/daemon/preview.go`.

- **Body:** the file's bytes. **Headers:** `X-Portty-Name` (base name, `url.QueryEscape`d), `X-Portty-Mime` (one of `relay.PreviewMIMEs`), `X-Portty-Key` (32 lowercase hex). There is no path field, header or query parameter.
- **Checks, in order** (each answers before any frame is sent):

| Check | Status |
|-------|--------|
| unknown session (`Manager().Get`) | 404 |
| hub done (`hub.Done()` closed; also `ErrHubEnded` from SendPreview) | 409 |
| MIME not in `relay.PreviewMIMEs` | 415 |
| key not 32 lowercase hex | 400 |
| name fails to percent-decode, or is empty after cleaning | 400 |
| `Content-Length` over `relay.PreviewMaxBytes` (checked before reading) | 413 |
| body overflows `http.MaxBytesReader(w, r.Body, relay.PreviewMaxBytes)` | 413 |
| `relay.ErrPreviewBusy` | 429, `Retry-After: 1` |
| `relay.ErrPreviewInvalid` | 400 |
| accepted | 202 `{"viewers":n}` |

- **Name cleaning (`cleanPreviewName`):** keep only what follows the last `/`; remove C0/C1 controls (`unicode.IsControl`), U+FFFD (invalid UTF-8), U+202A–U+202E and U+2066–U+2069; trim spaces; cap at 255 bytes on a rune boundary.
- **Log line on accept:** `viewer-diag: preview accepted session=<DiagID> bytes=<n> mime=<mime> viewers=<n>` (no file name). The relay logs per-viewer start and end (11-02).

## Client (for 11-08)

```go
const PreviewAPI = 3 // CLI requires DaemonAPI(h) >= PreviewAPI for `portty open`
type PreviewResponse struct{ Viewers int `json:"viewers"` }
func (c *DaemonClient) SendPreview(sessionID, name, mime, key string, data []byte) (PreviewResponse, error)
```

- A fresh `bytes.Reader` per attempt. Through `doRetrying`, which now waits out both 503 and 429 that carry Retry-After (1 s each, at most 2 s) within `frozenRetryBudget` (20 s).
- **Typed errors** (`errors.Is`): `ErrPreviewNoSession` (404), `ErrPreviewEnded` (409), `ErrPreviewTooLarge` (413), `ErrPreviewUnsupported` (415), `ErrPreviewBusy` (429 still there when the budget ends). Any other status returns `daemon API POST preview: status <n>: <body>`.
- `APIVersion = 3`; `MinDaemonAPI` stays 1, so an older daemon kept with Not Now keeps serving everything else.

## Tests (for 11-10's TESTING.md rows)

- `internal/daemon/preview_test.go`: TestPreview_EndToEnd, TestPreview_NameIsPercentDecodedAndCleaned, TestPreview_Refusals, TestPreview_BusyIs429, TestPreview_ZeroViewers, TestPreview_HandoffDrainNotBlocked, TestPreview_NotOnRelayHandler, TestPreview_SourceGate
- `internal/daemon/client_test.go`: TestSendPreview_RetriesFrozenWithFreshBody, TestSendPreview_WaitsOutBusy, TestSendPreview_TypedErrors (plus the TestHealthInfo pin now 3)
- `internal/daemon/api_test.go`: TestAPIHealth pin now `"api":3`
- `internal/webserver/files_removed_test.go`: TestPreviewRoute_NotOnShareServer

**Mutation check:** I made the handler wait for `sent.Done` before writing 202 (streaming inside the request). TestPreview_HandoffDrainNotBlocked failed with "drainWrites failed while the preview streamed". I reverted it before committing. The drain test itself takes about 0.4 s.

## Gate (Task 3)

- `go build ./...` and `go vet ./...` passed. `tests/check-brand.sh` and `tests/check-traceability-paths.sh` passed (`/tmp/p11-04-brand.log`, `/tmp/p11-04-trace.log`). `csp_mw.go`, `capability_mw.go` and `engine.go` are unchanged since 8e07f421.
- One `go test -race -short -count=1 ./...`, saved to `/tmp/p11-04-gate-go.log`. Every package was `ok` except two, each with one failing test. The coordinator accepted the gate with both as exceptions:
  1. **`TestShare_DeadViewerReapedBacklogDelivered` (internal/webserver):** the failure that was already there before this plan. It is fixed on gsd/v1.2 in bf0747e5 (a race in the test only, no product bug). This plan doesn't touch the webserver input queue.
  2. **`TestRemoteWSProxy_PasteBackpressureStaysOpen` (internal/daemon):** `remote_ws_proxy_test.go:819: write frame 79: ... write: broken pipe`. It is a heartbeat timing test (200 ms ping, 500 ms timeout during an 8 MiB paste) whose timing 9d7c2922 already widened for slow hosts under -race. It failed only under heavy parallel full-suite load (relay ran at the same time and took 442 s). Alone it passed 5 of 5 (`-count=5`, `/tmp/p11-04-proxy-loop.log`). This plan's diff doesn't touch the proxy path. The only shared code changed is `doRetrying`, which the proxy doesn't use. Treated as a load flake.
- No `--- SKIP` lines, no DATA RACE and no panic lines in the gate log. Per the coordinator, the full suite was not rerun.

## Deviations from Plan

1. **Share server GET `/app/sessions/<id>/preview`:** it reaches the `/app/*` single-page-app fallback. In the test it answers 503 because no app bundle is wired; in production it answers index.html with 200, the app's own page, never preview bytes. The test pins the 503 for that one case, with a comment. The plan said "never 2xx", which production can't meet for that GET. Every POST, and every other path, must answer 404 or 405.
2. **Order of work:** I wrote the tests together with the implementation, not tests first. The mutation check above shows the key test catches a handler that streams inside the request.
3. **Plan-head ledger:** the worktree sandbox refused writing the ledger file inside `.git`. The plan head before this plan was 8e07f421; `commits:` above is measured from it.

## Commits

- `e95d76e3` feat(11-04): POST /sessions/{id}/preview on the daemon socket
- `ae5fbf15` test(11-04): preview refusals, handoff gate and route absence
- `6c7411e1` feat(11-04): API 3 and DaemonClient.SendPreview

## Threat Flags

None beyond the plan's threat model. The one new route is on the 0700-directory unix socket; T-11-11 to T-11-15 are covered by the tests above.

## Known Stubs

None.

## Self-Check: PASSED

- FOUND: internal/daemon/preview.go, internal/daemon/preview_test.go
- FOUND: e95d76e3, ae5fbf15, 6c7411e1 (ancestors of HEAD)
