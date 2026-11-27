---
status: awaiting_human_verify
trigger: "O-22-06: during VM row G-10 (build 22.12.0, HEAD dfa1a208) the office daemon wrote `revoked` / `media_identity_mismatch` (evidence/G-10.txt:30, 18:28:49Z) and cut the legitimately admitted same-account guest (portty-test-2, Safari), which rejoined at 18:29:17Z without a knock. How can a legitimate guest fail the media identity check?"
created: 2026-11-27T23:00:00Z
updated: 2026-11-27T23:30:00Z
---

## Current Focus

hypothesis: the G-10 cut was a check with no verdict (most likely `lookup_failed`: tailscaled's WhoIs slow or failing on the guest's own tailnet address) that the dfa1a208 build reported as `media_identity_mismatch`. The exact `kind` cannot be recovered from the evidence.
next_action: VM re-check (below), then resolve.

reasoning_checkpoint:
  hypothesis: "On a live, legitimate guest, mediaMismatch asked tailscaled again about the guest's own tailnet IP, an address WhoIs had already named as the admitted user and node when the signalling socket connected. Any failure of that second lookup (3 s timeout under heavy host load, LocalBackend b.mu held during a netmap or link change) cut the guest. A re-fire of the check on a live viewer is expected: pion re-enters Connected after an ICE Disconnected (a stall), and a pair change also fires it."
  confirming_evidence:
    - "dfa1a208's checkMediaIdentity emitted AuditRevoked/ReasonMediaIdentityMismatch for EVERY non-empty kind (no_lookup, no_pair, lookup_failed, wrong_user, wrong_node). The audit line therefore does not mean wrong_user/wrong_node (git show dfa1a208:internal/desktop/media_binding.go). Quick 261127-lub (507ee5fc, after the build) stopped writing the line for lookup_failed/no_pair but still CUTS (1001)."
    - "The viewer was live for 39 minutes (join 17:49:54, leave 18:28:50 right after the cut). So the check was a re-fire on a running session, not the connect check."
    - "22-VM-CHECKS.md records heavy host load and a picture stall on a long-lived viewer in the same run. A stall is what re-fires the Connected check, and load is also what slows a LocalAPI WhoIs (LocalBackend.WhoIs takes b.mu, tailscale.com@v1.98.3 ipn/ipnlocal/local.go:1562). The two are correlated, an AND-gate."
    - "Unit reproduction: TestMediaBinding_OwnSignallingAddressNeedsNoLookup on the old code. A check on the guest's signalling IP with a failing lookup cut the guest (close 1001 'media failed', kind=lookup_failed)."
  falsification_test: "If the office daemon.log for 18:28:49Z shows kind=wrong_node or wrong_user, the selected pair came from another node's address. One way that could happen: the guest VM's non-tailnet (en0) path NATed through the host's own Tailscale. The fix does not cover that case, and the cut stays correct for it."
  fix_rationale: "The guest's signalling IP was resolved by WhoIs to (user ID, stable node ID) for this very connection (tailnetPeerStatus to Identity.TailnetIP), and a tailnet IP belongs to one node. A selected pair on that IP is the admitted node, with no second lookup. Any other address still goes to WhoIs and is cut on wrong_user, wrong_node, a lookup error or no pair, so the check against a real other node or user is not weakened."
  blind_spots: "The G-10 `kind` is not recoverable (the audit line had no kind, and no daemon.log was captured). If the real kind was wrong_node via a host-NAT path, this fix does not address it. A no_pair on a live viewer is still a retryable cut."
  candidate_causes:
    - "lookup_failed on the guest's own IP (WhoIs timeout/error under load) on a re-fired check: MOST LIKELY, reproduced in a unit test, fixed"
    - "no_pair (GetSelectedCandidatePair nil while the PeerConnection was closing). Possible on dfa1a208 because it had no v.cut/closed guard. Since fixed by IN-01 (closed PC = no verdict)"
    - "wrong_node via a non-tailnet guest path NATed by the host (the VMs share the host's vmnet, and the host runs Tailscale). Speculative and not checkable without the VMs. If true, the cut is correct behaviour"
    - "identity extraction mismatch (Node.User vs UserProfile.ID, StableID): ruled out by reading the code. Both the admission and LookupPeer use who.Node.User and who.Node.StableID"
    - "IPv4/IPv6 or v4-mapped address forms: ruled out. The media path is UDP4 only (peer.go) and the loopback test sees a plain 127.0.0.1. sameTailnetIP also Unmaps"

## Symptoms

expected: an admitted same-account guest is never cut by the media identity check
actual: G-10 audit line `revoked guest media_identity_mismatch` at 18:28:49Z; the guest rejoined 28 s later
reproduction: unit test `go test -race -run TestMediaBinding_OwnSignallingAddressNeedsNoLookup ./internal/desktop/`

## Resolution

root_cause: "A live re-check of the guest's own, already-verified tailnet address depended on a second tailscaled WhoIs. A slow or failing lookup cut a legitimate guest. The dfa1a208 build also labelled every no-verdict cut `media_identity_mismatch`."
fix: "mediaMismatch passes a selected pair whose remote IP equals Identity.TailnetIP (the WhoIs-verified signalling address) without a lookup. Every other address still goes to WhoIs, unchanged (internal/desktop/media_binding.go)."
verification: "TestMediaBinding_* (all, including the existing wrong-user/wrong-node/lookup-error/no-pair cuts) pass under -race; go test ./... passes."
files_changed: [internal/desktop/media_binding.go, internal/desktop/media_binding_test.go]

## VM re-check (to run later; do not run while Phase 23 holds the VMs)

1. Office portty-test on a build containing the fix. The guest portty-test-2 in Safari is admitted and streaming, view-only and then with control.
2. Keep the guest streaming for 40 min or more under the same rig load as G-10. Optionally force an ICE stall: on the guest, a pf block of the media UDP port for about 8 s (longer than the 5 s disconnected timeout, shorter than the failed timeout), then unblock.
3. Pass: no `desktop: guest media identity` line in the office daemon.log, no `revoked` audit line and no `media failed` close for that guest, and the guest never rejoins. If any `guest media identity` line appears, record its `kind`. A `wrong_node` kind means a non-tailnet path (the falsification test above).
4. Record the result under M-206 and close O-22-06.
