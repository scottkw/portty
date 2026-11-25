# Phase 18: Self-Installing Updates - Context

**Gathered:** 2026-11-25
**Status:** Ready for planning

<domain>
## Phase Boundary

Clicking Update installs the new Portty by itself: the DMG that already passes the v1.0 checks (SHA-256, Team ID, signing identifier, notarization) replaces `/Applications/Portty.app`, Portty relaunches, the update DMG is ejected and deleted, and terminal sessions survive through the v1.2 daemon takeover. A new version that fails to start or never reports healthy is rolled back. A first launch from a mounted DMG moves Portty into `/Applications` (or `~/Applications`), relaunches from there, ejects the DMG and trashes the download. Every failure falls back to today's Reveal-in-Finder flow with a clear message. Desktop sharing is not part of this phase.

</domain>

<decisions>
## Implementation Decisions

### Install behaviour
- After a successful update, the old Portty.app is moved to the Finder Trash (recoverable by hand), not deleted; until the new version reports healthy, the old copy is kept aside for rollback (user choice).
- When Portty cannot write to its install location (for example a standard, non-admin account), the update falls back to today's Reveal-in-Finder flow with a clear message; no admin password prompt (Claude's discretion, consistent with UPD-09).

### First launch from a DMG
- If an older Portty is already in `/Applications` (and possibly running sessions), Portty asks first ("Replace Portty <old> in Applications?") before replacing it; on yes it follows the update path (swap, relaunch, sessions move to the new daemon via the v1.2 takeover) (user choice).
- With no existing copy, the move happens without asking: copy to `/Applications` (or `~/Applications` if not writable), relaunch from there, eject the DMG, move the downloaded DMG to the Trash (user's milestone request, 2026-11-24).

### Decided autonomously
- [Installer] Q: "Which process swaps the app?" → Selected: "A detached installer subcommand of the same signed binary, run from a copy outside the bundle being replaced; it swaps, relaunches, waits for a health signal and rolls back" — internal process shape, no persisted contract
- [Rollback] Q: "What counts as a failed update?" → Selected: "The new GUI must report healthy (daemon reachable at the new version) within about 60 s; otherwise the old copy is put back and relaunched and the user is told the update did not take" — a timeout and a local signal, easy to tune
- [Gatekeeper] Q: "How is a Gatekeeper stop avoided on relaunch?" → Selected: "Quarantine is removed from the installed copy only after the existing verify chain passes; relaunch via `open` of the installed path" — local install step
- [Assets] Q: "Which DMG names does the updater accept?" → Selected: "The existing universal name plus the Apple Silicon-only name REL-01 will publish, so builds from this phase on still find Phase 23 releases" — a lookup list in internal/updater
- [Mover] Q: "How is 'running from a mounted DMG' detected?" → Selected: "Bundle path under /Volumes on a read-only disk-image volume (hdiutil info), with the DMG's backing file found from hdiutil info for trashing" — detection helper, replaceable

### Claude's Discretion
- No-write fallback (above): Reveal-in-Finder rather than an admin prompt.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- internal/updater: Checker (GitHub latest release), Download (with progress), Verify (SHA-256, Team ID, signing identifier, notarization via an injectable Runner), Reveal, Clean, DMGAssetName; Service state machine (checking/downloading/verifying/ready).
- internal/daemon handoff/takeover (v1.2, api.go, handoff.go, client.go): a new daemon takes over sessions from the old one with PIDs and history.
- TransomWM AppMover (~/dev/transomwm/Sources/TransomMac/Onboarding/AppMover.swift): stage copy, trash existing, rename into place, restore on failed rename; relaunch helper `/bin/sh -c 'while kill -0 $1; do sleep 0.2; done; open $2'` with pid/path as argv.

### Established Patterns
- Runner seam for exec'd tools (hdiutil, codesign, spctl, ditto) keeps the updater testable without a real DMG.
- GUI and daemon are the same Mach-O (Portty.app/Contents/MacOS/portty, `portty daemon`), so an installer subcommand fits the existing dispatch.
- Two-build rig + clean VM (portty-test) for real install checks; office Mac once (ROADMAP SC1).

### Integration Points
- app.go update bindings (GetUpdateStatus, DownloadUpdate) and the UpdateBanner component: add install/relaunch and the failure message.
- Startup path of the GUI: first-launch-from-DMG detection before the window opens.

</code_context>

<specifics>
## Specific Ideas

- User's reference: "We have other app releases that do this" (GIFsmith, TransomWM); TransomWM's AppMover is the closest pattern.
- ROADMAP risk: v1.2 clients only look for `portty-v<ver>-darwin-universal.dmg`; this phase widens the lookup before Phase 23 changes the asset name.
- Research constraint carried from v2.0: never mount, copy or modify the user's real files in ~/Downloads or /Applications on the host during research or tests; install tests run in the VM.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>
