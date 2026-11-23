---
created: 2026-11-20
source: 09-SECURITY.md observations O-1, O-2, O-6 (security audit of Phase 9, verdict SECURED)
area: daemon
---

## Problem
Small hardening items found by the Phase 9 security audit; none is an open threat.
- O-2: the daemon socket is created with a plain `net.Listen` (`internal/daemon/ipc.go:11`) and ends up `srwxr-xr-x`; only the 0700 parent directories keep it private. Docs say 0600. Predates Phase 9.
- O-1: `fdpass.Recv` leaves fds open in control messages it couldn't parse (`internal/fdpass/fdpass.go:76-88`); bounded because the takeover exits on failure.
- O-6: the takeover trusts the old daemon's pid from its HELLO-OK reply (`internal/daemon/handoff.go:856`) instead of the kernel's `LOCAL_PEERPID`.

## Solution
1. chmod the socket 0600 right after listen (or umask around it); test the mode.
2. Close every received fd in unparsed control messages.
3. Read the old daemon's pid from `fdpass.PeerPID` on the takeover side and refuse a mismatch with the HELLO-OK value.

## Resolution

Resolved in Phase 15 plans 15-03 (commits 5c25a63a, 349b3784) and 15-08 (commits e08d1b90, d610b7c8).
15-03: O-2, the control socket is 0600 at listen and tightened after COMMIT on a takeover. 15-08: O-6, the takeover refuses unless HELLO-OK's pid equals the kernel's LOCAL_PEERPID (the old daemon keeps serving); O-1, `rightsFromOOB` lets `Recv` free descriptors that arrived before a malformed control message. Real-process check: TESTING.md M-155.
