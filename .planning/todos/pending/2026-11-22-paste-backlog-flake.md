---
created: 2026-11-22
area: webserver
due: later
---
`internal/webserver` `TestShare_PasteBacklogKeepsViewer` (INPT-01) failed once in the v1.2 audit's full `go test -race ./...` run: `input_queue_test.go:116: viewer dropped while the paste is queued (heartbeat timeout)`. It passed 30 of 30 runs on its own and in a package-only run. The likely cause is timing under load: the test's 500 ms heartbeat limit under -race while `internal/relay` ran for 281 s alongside it. Fix: make the heartbeat limit in the test robust to load (scale it, or wait on an event rather than a wall-clock deadline). Do not loosen the product behaviour.
