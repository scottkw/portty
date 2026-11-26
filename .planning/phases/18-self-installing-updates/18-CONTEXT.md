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
- Downloads privacy prompt (user decision 2026-11-25, option A; found by 18-06 row R4-smoke): trashing a DMG that sits in `~/Downloads` makes macOS ask once "“Portty” would like to access files in your Downloads folder." Portty keeps trashing the DMG and accepts this one-time system prompt on a first install from a DMG (a Developer ID build remembers the answer per Mac). The installer writes its result and removes its run folder BEFORE the Trash step, so an unanswered prompt never leaves the install hanging or unrecorded; Don't Allow leaves the DMG in place and the install still counts as updated. Rejected: (B) eject only for DMGs in protected folders (the Trash step would almost never run); (C) trash via Finder Apple Events (still a prompt, plus an Automation usage key). Updates from Portty's own cache are unaffected (no prompt).
  - Refinement 2026-11-26 (made by the orchestrator, a technical refinement within option A): the run folder is now removed AFTER the Trash call returns or its 60 s limit runs out, not before it. The result is still written before the Trash. The run folder holds the installer's own executable. When it was removed first, tccd could not resolve the requesting process and refused without showing the prompt, so a DMG in Downloads was never trashed (18-06 row R4-smoke-rundir-first). The goal is unchanged: an unanswered prompt never leaves the install hanging or unrecorded.

### Decided autonomously
- [Installer] Q: "Which process swaps the app?" → Selected: "A detached installer subcommand of the same signed binary, run from a copy outside the bundle being replaced; it swaps, relaunches, waits for a health signal and rolls back" — internal process shape, no persisted contract
  - Revised 2026-11-25 after probe P2d (18-VM-CHECKS.md row P2d, `evidence/P2d-amfi.log`): on a Developer ID build, a lone copy of the executable outside its bundle is killed by AMFI at exec (`Code=-420`, "The code contains a Team ID, but validating its signature failed"), because the signature binds the bundle's Info.plist; a `ditto` copy of the whole bundle in another folder runs. Selected: "(b) The GUI copies the WHOLE currently running Portty.app (ditto, signature preserved) into the job's run folder and starts `install-update` from that copy's Contents/MacOS/Portty". Why: the installer is then the same version as the GUI that wrote the job (no cross-version job-file contract), it is known-good running code, and rollback does not depend on the new version's code working. Rejected: (a) run the installer from the staged new bundle (a broken new build could not roll itself back); (c) embed Info.plist in the Mach-O (untried, changes the release build). A signed re-run of P2d in the VM (row P2d-bundle) must pass before rollback work relies on it — technical, reversible, decided autonomously by the orchestrator.
  - Relaunch retry (18-VM-CHECKS.md row A9, `evidence/A9-open-procNotFound.log`): `open` right after the old GUI exits fails with -600 procNotFound in about half the runs. The installer retries `open` several times about 1 s apart; a persistent failure is a rollback case, and no path leaves the old daemon orphaned with no GUI.
- [Rollback] Q: "What counts as a failed update?" → Selected: "The new GUI must report healthy (daemon reachable at the new version) within about 60 s; otherwise the old copy is put back and relaunched and the user is told the update did not take" — a timeout and a local signal, easy to tune
- [Gatekeeper] Q: "How is a Gatekeeper stop avoided on relaunch?" → Selected: "Quarantine is removed from the installed copy only after the existing verify chain passes; relaunch via `open` of the installed path" — local install step
- [Assets] Q: "Which DMG names does the updater accept?" → Selected: "The existing universal name plus the Apple Silicon-only name REL-01 will publish, so builds from this phase on still find Phase 23 releases" — a lookup list in internal/updater
  - User-confirmed 2026-11-25: the Apple Silicon-only name is `portty-vX.Y.Z-darwin-arm64.dmg`, accepted alongside the universal name (user answer "portty-vX.Y.Z-darwin-arm64.dmg (Recommended)", 2026-11-25). This name is now human-settled, so no planning checkpoint asks for it again.
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
