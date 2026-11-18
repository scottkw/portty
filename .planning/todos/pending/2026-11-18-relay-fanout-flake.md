---
created: 2026-11-18
area: tests
source: Phase 6 verification
---
# TestHub_TwoClientsFanOut flaked once under -race

`internal/relay` TestHub_TwoClientsFanOut failed once in the full `go test -race -short ./...` run (payload "\x01hello world"); it passed 3 times alone and on a package rerun. Phase 6 didn't touch internal/relay. Add to the known-flake list or find the race.
