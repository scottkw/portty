---
phase: 13-remote-localhost-urls
plan: 01
subsystem: relay
tags: [go, hmac, hkdf, capability, anti-scan, port-grant]
requires:
  - phase: 12
    provides: pathSnapshot and the two path-click search passes
provides:
  - Hub.PortPrinted, the anti-scan printed check for the port grant
  - capability.PortClaims, PortKey, SignPort, VerifyPort, ErrWrongKind
affects: [13-03 share server grant and proxy routes]
status: complete
actuals:
  tokens: 5400
  tasks: 3
  commits: 3
plan_head_before: a30debc93f63b638b71c50eee573f44a8725218d
plan_head_after: 3716dbe7ef4500f1af534f7f0ef4d7f10c3feabc
tech-stack:
  added: []
  patterns:
    - "predicate-taking search passes (findMatch) shared by path clicks and the port check"
    - "port token signed under an HKDF-derived key so it cannot pass as a session capability"
key-files:
  created:
    - internal/relay/portprinted.go
    - internal/relay/portprinted_test.go
    - internal/capability/porttoken.go
    - internal/capability/porttoken_test.go
  modified:
    - internal/relay/pathclick.go
    - internal/relay/hub.go
key-decisions:
  - "firstPass/fallbackPass kept as thin text wrappers over new firstMatch/fallbackMatch because the untouched pathclick_test.go calls them directly"
  - "PortKey rejects an empty session key explicitly"
requirements-completed: []  # URL-02 is only partly delivered here (host primitives, nothing calls them yet); the plan forbids requirements.mark-complete
---

# Phase 13 Plan 01: Host gate primitives Summary

Two pure, tested primitives for the port grant: `Hub.PortPrinted` (a loopback http URL the session itself printed, found through the Phase 12 snapshot with URL token boundaries) and a port token signed under an HKDF-derived key that no session route can accept.

## Accomplishments

- `Hub.PortPrinted(port)`: false outside 1..65535, false for a hub with no output, false while another search runs on the hub (`portPrintedBusy` CAS). Otherwise searches history plus screen (ANSI stripped, soft wraps joined, alt-screen seam respected) with `hasURLPort`.
- `hasURLPort`: needles `http://{localhost,127.0.0.1,[::1],0.0.0.0,[::]}:<port>`, every occurrence scanned, with the before and after boundary rules from the plan. Refuses `:80` inside `:8000`, userinfo (`@evil.com`), suffix domains, https, upper case, leading zero, `:12` suffix and glued prefixes.
- `pathclick.go`: `findMatch(match, needleLen)` with `firstMatch`/`fallbackMatch`; `findPrinted` delegates with `hasToken` and `len(text)`. pathclick_test.go is byte-identical to 9574fd49 and passes.
- `porttoken.go`: `PortClaims{SID, Port, IAT, GrantID, Kind, V}`, `PortKey` (stdlib `crypto/hkdf`, info `portty/port-token/v1`, 32 bytes), `SignPort`, `VerifyPort` (Kind "port" and V 1 checked after the signature, `ErrWrongKind`). capability.go untouched.

## Tests added

- relay: TestPortPrinted_PythonBanner, _RangeRefused, _NoOutput, _History, _SoftWrap, _ANSIPort, _AltScreen, _Busy; TestHasURLPort_Table (18 rows), TestHasURLPort_Banners (8 dev-server banners).
- capability: TestPortToken_RoundTrip, _NotASessionCapability, _SessionCapNotAPortToken, _Tamper, _KeyRotation; TestPortKey_Derived; FuzzVerifyPort (seed corpus run only, not a fuzzing session).

No banner needed a rule change: every banner in the plan's list matched with the boundary rules as written.

## Verification

- Log directories (private `mktemp -d`): `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.0Eh86HFbr3` (Task 1), `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.Y33KHfa82I` (Task 2), `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.fYMGyf3PAa` (Task 3 full suite).
- `go build ./...`, `go vet ./...` clean.
- `go test -race -short -count=1 ./...` (GOTOOLCHAIN=go1.26.11): rc 0, no FAIL line; capability ok (1.0s), relay ok (120.8s).
- Tracer gate (Task 1): PortPrinted and PathClick tests re-run after the commit-ready state, passing; no expansion started on a failing slice.

## Commits

- 5cad573e feat(13-01): Hub.PortPrinted over the Phase 12 snapshot with URL boundaries
- f9d2f69e test(13-01): URL boundary table, dev-server banners and hub edge cases for PortPrinted
- 3716dbe7 feat(13-01): port token under an HKDF-derived key no session route accepts

## Deviations from Plan

**1. [Rule 3 - Blocking] Pass wrappers kept.** The plan said to turn `firstPass`/`fallbackPass` into predicate-taking forms, but the must-have "pathclick_test.go is unchanged" conflicts: TestPathClick_WrapFallback calls `snap.firstPass(text)` and `snap.fallbackPass(text)`. Fix: predicate forms are `firstMatch`/`fallbackMatch`; the old names remain as two-line wrappers. Files: internal/relay/pathclick.go. Commit 5cad573e.

**2. Test hub construction.** The hub cases use `cwdHub` + `feedHubs` rather than `pcHub` + `pcPrint` (which needs a PTY writer and a click seam this check does not use). Same hub, same recording path as Run.

**3. TestPortToken_Tamper.** An initial expectation that the token "." gives ErrMalformedToken was wrong (it decodes to empty payload and signature, so it fails the HMAC, same as `Verify`); the test case was dropped, not the code.

TDD note: the implementation file was written before the Task 1 test run, so there was no separate observed RED step for Tasks 1 and 2; tests then ran green first time. The boundary table rows (including look-alikes) are the evidence for refusal behaviour.

## Known Stubs

None.

## Threat Flags

None. No new endpoint or file access; nothing calls the primitives yet.

## Not verified

- FuzzVerifyPort was run on its seed corpus only.
- No GUI or VM checks (not in scope for this plan).

## Self-Check: PASSED

Files present: portprinted.go, portprinted_test.go, porttoken.go, porttoken_test.go; commits 5cad573e, f9d2f69e, 3716dbe7 are ancestors of HEAD.
