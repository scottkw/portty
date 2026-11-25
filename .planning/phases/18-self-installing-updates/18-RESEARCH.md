# Phase 18: Self-Installing Updates - Research

**Researched:** 2026-11-25
**Domain:** macOS app self-replacement (hdiutil / ditto / atomic bundle swap / relaunch / rollback) inside an existing Go + Wails app with a detached daemon
**Confidence:** MEDIUM-HIGH. The in-repo mechanics (verify chain, daemon takeover, process shape) are HIGH, read this session. The macOS-behaviour unknowns (App Management, quarantined-DMG mount side effects, translocation of a DMG-run app, Downloads TCC) are LOW and are listed as VM probes (P1-P8) that must run before the install code is trusted.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
Copied verbatim from `18-CONTEXT.md` `## Implementation Decisions`:

**Install behaviour**
- After a successful update, the old Portty.app is moved to the Finder Trash (recoverable by hand), not deleted; until the new version reports healthy, the old copy is kept aside for rollback (user choice).
- When Portty cannot write to its install location (for example a standard, non-admin account), the update falls back to today's Reveal-in-Finder flow with a clear message; no admin password prompt (Claude's discretion, consistent with UPD-09).

**First launch from a DMG**
- If an older Portty is already in `/Applications` (and possibly running sessions), Portty asks first ("Replace Portty <old> in Applications?") before replacing it; on yes it follows the update path (swap, relaunch, sessions move to the new daemon via the v1.2 takeover) (user choice).
- With no existing copy, the move happens without asking: copy to `/Applications` (or `~/Applications` if not writable), relaunch from there, eject the DMG, move the downloaded DMG to the Trash (user's milestone request, 2026-11-24).

**Decided autonomously**
- [Installer] Q: "Which process swaps the app?" -> Selected: "A detached installer subcommand of the same signed binary, run from a copy outside the bundle being replaced; it swaps, relaunches, waits for a health signal and rolls back" — internal process shape, no persisted contract
- [Rollback] Q: "What counts as a failed update?" -> Selected: "The new GUI must report healthy (daemon reachable at the new version) within about 60 s; otherwise the old copy is put back and relaunched and the user is told the update did not take" — a timeout and a local signal, easy to tune
- [Gatekeeper] Q: "How is a Gatekeeper stop avoided on relaunch?" -> Selected: "Quarantine is removed from the installed copy only after the existing verify chain passes; relaunch via `open` of the installed path" — local install step
- [Assets] Q: "Which DMG names does the updater accept?" -> Selected: "The existing universal name plus the Apple Silicon-only name REL-01 will publish, so builds from this phase on still find Phase 23 releases" — a lookup list in internal/updater
- [Mover] Q: "How is 'running from a mounted DMG' detected?" -> Selected: "Bundle path under /Volumes on a read-only disk-image volume (hdiutil info), with the DMG's backing file found from hdiutil info for trashing" — detection helper, replaceable

### Claude's Discretion
- No-write fallback (above): Reveal-in-Finder rather than an admin prompt.

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope. (CONTEXT `<domain>`: "Desktop sharing is not part of this phase.")
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| UPD-05 | After the user clicks Update and the DMG passes the existing checks, Portty replaces itself in `/Applications` and relaunches; terminal sessions survive (v1.2 takeover) | Patterns 1-5: mount + stage + verify app in the GUI, detached installer swaps with `RENAME_SWAP`, relaunch with `open`; the new GUI's `EnsureDaemon` runs the existing takeover (verified in `process.go`), no daemon change needed |
| UPD-06 | The update DMG is ejected and deleted after a successful install | Detach right after staging (before the point of no return); delete the cache DMG only after the health signal passes, so every failure still has the DMG for the Reveal fallback |
| UPD-07 | If the new version fails to start, Portty rolls back to the previous version by itself | Pattern 6: health = new GUI alive + daemon `/health` reports the new version; rollback = swap back + relaunch with keep-daemon, because a downgrade handoff is refused by the daemon (Pitfall 1, verified in `handoff.go`) |
| UPD-08 | Opening Portty from a mounted DMG moves it to `/Applications` (or `~/Applications`), relaunches, ejects the DMG, moves the downloaded DMG to the Trash | Pattern 7: mover branch at GUI startup before any daemon/tray work; same installer with a `mover` job; DMG located via `hdiutil info -plist` (verified), trashed with `NSFileManager trashItem` |
| UPD-09 | Any install failure falls back to Reveal-in-Finder with a clear message; a relaunch never stops at a Gatekeeper dialog | Pattern 1 (all fallible steps before the point of no return, so most failures reveal immediately), Pattern 8 (quarantine removal, verified), failure-text table, Pitfalls 4-6 |
</phase_requirements>

## Summary

The phase is mostly orchestration of system tools plus one new hidden subcommand, and it needs no new Go module. [VERIFIED: golang.org/x/sys v0.46.0 is already in go.mod, `unix.RenamexNp` is at `syscall_darwin.go:407` and `RENAME_SWAP = 0x2` at `zerrors_darwin_arm64.go:1179` in the module cache] The existing code already provides the hard parts: the verify chain (`updater.Verify`), the `Runner` seam, GUI-only quit semantics (`onTerminate`), and the v1.2 fd-passing takeover, which the *new* GUI runs automatically on startup when it sees a stale daemon. Swap-and-relaunch alone therefore carries sessions across; the research found no daemon change is required.

Three findings change the shape of the plan versus the CONTEXT sketch. (1) **A downgrade handoff is refused** (`an older release cannot take the sessions`), so a rolled-back build N relaunched against an already-committed N+1 daemon cannot pull sessions back; rollback must relaunch N in a keep-daemon mode or N shows the destructive "Restart Now" question. (2) **`ditto --noqtn` does not strip quarantine** on this macOS (probe below); only `xattr -dr com.apple.quarantine` does. (3) **The user's own earlier incident** (memory: a researcher's `ditto` from a quarantined, mounted real DMG made macOS pop a "move disk image to Trash" dialog) means mounting a quarantined DMG may raise a system dialog on an unattended Mac; the plan must probe it first and carry the mitigation (clear the DMG's quarantine after Verify, before attach, and re-set it before any Reveal).

Design in one line: do every fallible step (mount, verify the app inside, stage, smoke-run, space and permission checks, detach) inside the GUI **before** a single point of no return; after that point a small detached installer (copy of the signed binary, outside the bundle) waits for the GUI to exit, swaps atomically, relaunches with `open`, waits for health, then trashes the old copy and deletes the DMG, or swaps back. Failures before the point of no return become today's Reveal flow immediately; failures after it are reported by a result file the relaunched GUI reads.

**Primary recommendation:** Build `internal/updater` install logic against injected seams (Runner, Swap, Trash, Spawn, Health, Clock), add a hidden `portty install-update <job.json>` subcommand beside `daemon --takeover`, branch GUI startup into a mover mode before any daemon work, and gate trust in the macOS behaviour behind Wave 0 VM probes P1-P8.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Download, SHA-256, Team ID / identifier / notarization verify of the DMG | GUI process (`internal/updater` Service) | — | Already GUI-owned (`app_update.go`); unchanged |
| Mount DMG, verify app inside, stage on target volume, space/permission checks, detach | GUI process | — | Every fallible step stays where it can show the Reveal fallback and a message immediately |
| Quit the old GUI without ending sessions | GUI process (`onTerminate`) | Installer (SIGTERM, then SIGKILL, of the GUI pid only) | `onTerminate` is the existing GUI-only quit; the daemon is Setsid-detached and unaffected |
| Atomic swap, relaunch, health wait, rollback, old-copy trash, DMG delete | Detached installer subcommand (same binary, copied out of the bundle) | — | Cannot run inside the bundle being replaced or the GUI that is exiting |
| Session survival across the swap | Daemon (existing v1.2 takeover, run by the new GUI's `EnsureDaemon`) | — | No new daemon code; the new `os.Executable()` is the new bundle |
| Health signal | Daemon `/health` (version) + process table (GUI alive) | New GUI (optional cooperative marker) | Observable from outside, works even when N+1 predates this phase |
| First launch from a mounted DMG (detect, ask, install) | GUI process in a "mover mode" startup branch | Installer (eject, trash, relaunch) | The DMG-run GUI must not start the daemon, tray or pollers (it would pin the volume) |
| Result / failure message after a swap-phase failure or rollback | Result file in the config dir, read by the relaunched GUI | Update banner / native alert | The installer has no UI; the next GUI says "the update did not take" |
| Trash of old bundle and downloaded DMG | Installer via an ObjC `NSFileManager trashItem` function in package main | — | No Automation (Finder AppleEvent) prompt; same call TransomWM ships |

## Standard Stack

### Core
| Library / tool | Version | Purpose | Why Standard |
|----------------|---------|---------|--------------|
| `golang.org/x/sys/unix` `RenamexNp` + `RENAME_SWAP` | v0.46.0 (already in go.mod) | Atomic exchange of staged and installed bundle | No window with no app at the target path; both inodes stay valid for the still-running old daemon. [VERIFIED: module cache, see above] |
| `/usr/bin/hdiutil` | system (macOS 27.2.1 on host) | `attach -nobrowse -readonly -noverify -noautoopen -plist -mountpoint`, `info -plist`, `detach` | The sanctioned route; `info -plist` gives `image-path`, `writeable`, `system-entities[].mount-point` [VERIFIED: scratch DMG probe this session] |
| `/usr/bin/ditto` | system | Copy the app to the staging dir (keeps symlinks, xattrs, signature) | Never `cp -r` (breaks the seal). `--noqtn` is NOT reliable, see Pitfall 3 |
| `/usr/bin/xattr -dr com.apple.quarantine` | system | Clear quarantine on the staged copy | [VERIFIED: scratch probe, removed it from every file] |
| `/usr/bin/codesign`, `/usr/sbin/spctl`, `/usr/bin/plutil` | system | Verify the app inside (`--verify --deep --strict`, `-dvv`, `spctl -a -t exec -vv`), read Info.plist, convert hdiutil plist to JSON | Same tools and Runner seam as `Verify`; no plist Go dependency |
| `/usr/bin/open` | system | Relaunch the installed path | LaunchServices launch; `--env`, `-n`, `-F`, `-j` exist [VERIFIED: `man open`] |
| `NSFileManager trashItemAtURL:resultingItemURL:error:` via a small cgo `.m` file | AppKit/Foundation | Trash the old bundle and the downloaded DMG | Synchronous, no AppKit runloop, no Finder AppleEvent. Same call as TransomWM's `AppMover.moveToTrash`. [CITED: ~/dev/transomwm/Sources/TransomMac/Onboarding/AppMover.swift] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `/bin/ps -axww -o pid=,args=` | system | Find the GUI pid of the installed bundle (and exclude `daemon` processes) | Health wait, quit the installed GUI before a replace-from-DMG swap |
| `internal/daemon` `DaemonClient.HealthInfo()` | in repo | Health signal (`Version`) | Injected into the installer as a `Healthy` func from package main |
| `cmd/fakegh` + `updates_vmpreview.go` | in repo | VM update feed | Two-build rig (Wave 0 extends them) |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-written installer | Sparkle | Separate appcast + EdDSA scheme would duplicate the existing SHA-256 + Team ID + identifier + notarization chain; ObjC framework. Not for this milestone. [CITED: .planning/research/STACK.md "Alternatives Considered"] |
| `/bin/sh` relaunch helper (TransomWM) | Go installer subcommand | The shell helper cannot health-check or roll back; CONTEXT locked the Go subcommand. The shell loop `while kill -0 $1` idea is still the model for the wait step |
| `NSWorkspace recycleURLs` | `NSFileManager trashItem` | recycleURLs gives Put Back but its completion handler needs a running runloop; the installer has no AppKit. Choose trashItem (probe P3 confirms) |
| Admin prompt (`osascript with administrator privileges`) | Reveal fallback | Locked: no admin prompt (UPD-09) |

**Installation:** none. No new Go module, no npm package, no Homebrew tool.

**Version verification:** `golang.org/x/sys v0.46.0` read from `go.mod` line `golang.org/x/sys v0.46.0`; symbols confirmed in the module cache path `~/go/pkg/mod/golang.org/x/sys@v0.46.0/unix/`. [VERIFIED: this session]

## Package Legitimacy Audit

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| (none new) | — | — | — | — | — | No external package is added by this phase; `golang.org/x/sys` is an existing direct dependency |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
                      UPDATE PATH (UPD-05/06/07/09)
 banner "Update" click
        |
        v
 GUI: Service.Download  --digest/codesign/Team/identifier/spctl (existing Verify)-->
        |  ok                                   | fail -> delete file, error (existing)
        v
 GUI: install steps (all reversible, all in-process, Runner seam)
   1 pre-flight: target = running bundle dir (not translocated, not /Volumes), parent writable,
                 free space >= 2x bundle, release.Version > running
   2 [P1] clear DMG quarantine (re-set before any Reveal)
   3 hdiutil attach -plist (stdout only) -> mount point
   4 verify app inside: codesign --verify --deep --strict, Team S2K7P43927, Identifier app.portty.Portty
   5 ditto -> <target parent>/.Portty-update-XXXX/Portty.app ; xattr -dr quarantine
   6 verify staged: codesign + spctl -a -t exec ("Notarized Developer ID"), LSMinimumSystemVersion <= host,
                 run staged "--version" == "v<release>" (launch smoke test, 10 s cap)
   7 hdiutil detach (retry)
        | any failure above: cleanup staging+mount, re-set DMG quarantine, Reveal(existing),
        |                    state=revealed + Error text  (UPD-09)      ==== POINT OF NO RETURN ====
        v
 GUI: write job.json (0600), copy own binary -> <configDir>/update/<rand>/portty-installer,
      spawn `portty-installer install-update job.json` (Setsid, cwd /, log to update.log),
      state=relaunching, a.onTerminate()  (GUI-only quit; daemon + sessions untouched)
        |
        v                         DETACHED INSTALLER (outside any bundle)
 wait old GUI pid exit (<=20 s, then SIGKILL the GUI pid only)
        |   timeout/fail -> nothing swapped: cleanup staging, `open` old path, write result(failed)
        v
 re-verify staged (codesign --verify --deep --strict + Team) ; RENAME_SWAP(staged, target)
        |   fail -> result(failed), open old path (still in place)
        v
 /usr/bin/open <target>  --> NEW GUI starts --> EnsureDaemon sees StaleVersion
                                                  --> takeover (`portty daemon --takeover`, new exe)
        |
        v
 poll: new GUI pid alive  AND  daemon /health Version == "v<release>"  (stable ~3 s)  within ~60 s
   healthy  --> trash old bundle (staged dir now holds it), rm staging dir, delete cache DMG,
                write result(updated)
   deadline --> kill new GUI pid (SIGTERM, SIGKILL), RENAME_SWAP back, write result(rolled-back,
                keepDaemon), `open <target>`  -> old GUI relaunches in keep-daemon mode, shows
                "update did not take"

                      FIRST LAUNCH FROM DMG (UPD-08)
 GUI starts from /Volumes/... --> [mover mode: NO daemon, NO tray, NO pollers, window stays hidden]
   detect (path + hdiutil info -plist: read-only image volume, image-path)
   existing copy older? --dialog "Replace Portty <old> in Applications?" --no--> open existing, quit
   same steps 1,4-6 (source is the mounted volume the GUI runs from; target /Applications,
   else ~/Applications when not writable), write job(kind=mover, dmg image-path, mount point)
   --> spawn installer, a.onTerminate()
 installer: wait DMG-GUI exit, (replace: quit installed GUI), swap or plain rename (no existing copy),
   open <installed>, hdiutil detach <mount> (retry, then -force), trash DMG image-path
   (pattern-checked), wait health (same rule), on failure restore/leave state and result(failed)
```

### Recommended Project Structure
```
internal/updater/
├── update.go            # + assetNames list (universal, arm64) replacing the single DMGAssetName use
├── install.go           # Download/Verify/Reveal/Clean unchanged; Clean globs widened
├── service.go           # + states installing, relaunching; Install hooks; busy() widened
├── appinstall.go        # NEW: Mount, VerifyApp, Stage, Preflight, Detach (Runner seam)
├── installer.go         # NEW: RunInstaller(job, env): wait, swap, open, health, rollback, cleanup
├── mover.go             # NEW: DetectDMGRun, installed-version read, job builder for kind=mover
├── job.go               # NEW: Job / Result structs, atomic write, path validation
└── *_test.go            # new tests (see Test Strategy)
cmd_install_update.go    # NEW: hidden `install-update <job>` subcommand wiring (like daemon --takeover)
app_install.go           # NEW: Service hooks (Quit=a.onTerminate, Spawn), result-file reader, mover boot branch
trash_darwin.go/.m       # NEW: trashItem cgo (pattern of terminate_darwin.go/.m)
frontend/.../UpdateBanner.tsx, App.tsx, wailsjs/go/main/App.d.ts   # states + copy ("Update")
```

### Pattern 1: Everything fallible happens before the point of no return
**What:** The GUI does mount, verify, stage, smoke test and detach itself. Only then does it spawn the installer and quit. **When to use:** always. **Why:** UPD-09 wants every failure to look like today's Reveal flow; the GUI is the only process that can show that message and the revealed DMG. The installer's remaining work (wait, swap, open, health) has few failure modes, and those report through the result file.
**Failure mapping (texts must hold no path or URL, per the existing `TestService_ErrorTextsHaveNoURLOrPath` rule):**

| Failure | Detect | Text direction |
|---------|--------|----------------|
| Parent dir not writable / staging `MkdirTemp` EACCES | `unix.Access(W_OK)` + the real mkdir | "Portty can't change the Applications folder from this account." |
| App Management refusal | EPERM from mkdir/ditto/rename while the parent is writable [ASSUMED mapping, P2] | "macOS didn't allow Portty to replace itself." |
| Mount failure | `hdiutil attach` rc != 0 or no mount-point in plist | "Portty couldn't open the download." |
| Copy failure / disk full | `ditto` rc != 0, `ENOSPC` text, free-space pre-check | "Portty couldn't copy the new version (is the disk full?)." |
| App inside fails verify / version / smoke | codesign, Team, Identifier, `--version` mismatch | "The new version didn't pass the safety check, so it wasn't installed." |

Every one ends in: cleanup, re-set DMG quarantine (if cleared), existing `Reveal`, `state=revealed` with `Error` set (the banner must render revealed+error as the warning variant; today `failed` is only `available`+error).

### Pattern 2: Detached installer subcommand, binary copied out of the bundle
**What:** `portty install-update <job.json>` dispatches in `runCLI` before the daemon-ensure step (add it to the `cliNeedsDaemon` skip list, not in `usage`), like `daemon --takeover`: "internal, so not in the usage". The GUI copies `os.Executable()` to `<configDir>/update/<rand>/portty-installer` (0700, new file, never overwritten in place), spawns it with `Setsid: true`, cwd `/`, stdout/stderr to `update.log`. **Why a copy:** the bundle at the target is renamed away; a process whose exec path lives in it must not be the one doing the work. The copy is a fresh file with the original signature bytes. [ASSUMED: runs without a signature kill, App Management attribution still team S2K7P43927, probe P2]
**Test guard:** like `spawnTakeover`, the spawn seam must refuse under `testing.Testing()` ("the child would take the user's real daemon", `process.go`); inject it.

### Pattern 3: Job file in, result file out (no sockets, no argv flags)
Job (JSON, 0600, owned by the user, written by the GUI): kind (`update`|`mover`), old GUI pid, target path, staging path, old version, new version, DMG cache path (update) or DMG image-path + mount point (mover), timeouts. Installer validates: file owner and mode, `filepath.Base(target)=="Portty.app"`, staging is a sibling of target, mover DMG basename matches the asset pattern. Result (JSON in the config dir, read once by the next GUI and deleted): `updated` | `rolled-back` | `failed`, versions, reason code, `keepDaemon`. **Why files:** `launchMode` makes any unknown leading flag a usage error by design (verified: `unknown flag %q`), so no new GUI argv flag; `open --env` is an alternative but a file survives a relaunch by LaunchServices from anywhere. [CITED: .planning/research/ARCHITECTURE.md section 8 step 5]
Logs follow the existing rule: step names and file base names only, never a URL or path; the installer log is `update.log` in the config dir.

### Pattern 4: Swap with `RENAME_SWAP`; plain rename when there is no existing copy
`unix.RenamexNp(staged, target, unix.RENAME_SWAP)`. After it, the staging path holds the **old** bundle, which is exactly what rollback and trash need. With no existing target (mover, first install) `renamex_np` returns ENOENT [VERIFIED: probe returned rc -1 errno 2], so use `os.Rename(staged, target)` there. Never `ditto` over a live bundle (in-place writes invalidate code pages of the still-running old daemon, which macOS may kill, ending sessions). [VERIFIED: RENAME_SWAP on directories works on APFS, scratch probe: `a/Portty.app` and `b/Portty.app` exchanged contents, rc 0]

### Pattern 5: Health = new GUI alive AND daemon reports the new version
The installer polls (a) the GUI pid for the target (from `ps`, args without `daemon`) and (b) `daemon.NewDaemonClient(socket).HealthInfo()` for `Version == "v"+newVersion`, stable for about 3 s, within the deadline (CONTEXT: about 60 s; make it one constant). This needs **no cooperation from N+1**, so it also works when N+1 is a release that predates this phase (for example the published v1.2.0, used in the R3 rig row below). `daemon.BuildVersion = Version` and `/health` returns `Version: BuildVersion`, so the string is the exact ldflags tag. Takeover worst case is bounded (about 19 s freeze plus a 15 s ready wait, from `handoff.go`/`process.go` comments), so 60 s covers a normal takeover; tune from VM evidence.
Optional belt: a new GUI that finds a pending job for its own version may also write a marker after `startup` and `domReady`; the installer must not require it.

### Pattern 6: Rollback, including the downgrade-handoff wall
Rollback steps: kill the new GUI pid (SIGTERM, then SIGKILL; GUI only, never the daemon), `RENAME_SWAP` back, write result `rolled-back` with `keepDaemon`, `open <target>`.
**Keep-daemon relaunch is mandatory.** Case A (N+1 never reached takeover): the daemon is still N, relaunched N sees the same version, nothing special. Case B (takeover committed, GUI N+1 then unhealthy): the daemon is N+1; N's `EnsureDaemon` finds `StaleVersion`, spawns a takeover that the daemon refuses (`olderRelease`), and falls to the DEBT-08 question whose "Restart Now" ends every session. So the relaunched N, on seeing `keepDaemon` in the result file, must call `ensureDaemon` with `NoHandoff: true` and a `Restart` policy that returns false (a one-line change in `App.ensure()`), exactly the existing `--keep-daemon` semantics. N+1's API is at least N's, so N's GUI works against it. [VERIFIED: handoff.go:473-474 and process.go:272-292, quoted below]

### Pattern 7: Mover mode is decided before any daemon work
In `startup()` (the first thing after `a.ctx = ctx`) branch to mover mode when the bundle runs from a disk image: skip `initTray`, `startDaemon`, pollers, wake watcher and `domReady`'s window show. Reason (prior research, confirmed by the code path): `EnsureDaemon` spawns `exe daemon` from `os.Executable()`, and a daemon running off `/Volumes/...` pins the volume so eject fails with "disk in use" and later dangles. [CITED: .planning/research/PITFALLS.md Pitfall 14]
Detection (CONTEXT): bundle path under `/Volumes` AND the volume is a read-only disk-image volume per `hdiutil info -plist` (entry whose `system-entities[].mount-point` is a prefix of the bundle path; `writeable` false; `image-path` gives the backing DMG). A plain external SSD at `/Volumes/X` is not a disk image, so no mover. Harden: if the path contains `/AppTranslocation/`, resolve the origin with `statfs` (`f_mntfromname` of the nullfs mount) [ASSUMED mechanism, P5] before deciding; if it cannot be resolved, do nothing and log (do not guess).
Run the check only for GUI launches (it lives in `startup()`, so CLI and daemon invocations never reach it).
Existing copy handling: older -> ask via the existing `a.dialog` seam ("Replace Portty <old> in Applications?"), yes = update-style job, no = `open` the existing copy and quit leaving the DMG mounted and untrashed; no copy -> no question; same or newer installed version -> recommend: do not replace, `open` the installed copy, eject (see Open Question O3). Read the installed version from its `Info.plist` with `plutil`, **never** by executing it (Pitfall 7).

### Pattern 8: Quarantine
Order per CONTEXT: remove quarantine from the installed copy only after the verify chain passed (`xattr -dr com.apple.quarantine <staged>`, then verify the staged copy, then swap). `Verify` still writes quarantine on the DMG (`xattr -w com.apple.quarantine "0081;<unix hex>;Portty;"`, `install.go:146-147`); that is a Chesterton's Fence for the Reveal path. Recommended refinement pending probe P1: clear it on the DMG after Verify and before attach (the DMG just passed SHA-256, Team ID, identifier and notarization), and re-write the same flag before any Reveal fallback, so the mounted volume carries no quarantine and the OS cannot raise its own dialog during our unattended copy.

### Pattern 9: Asset-name lookup list
Replace the single `DMGAssetName(v)` use in `Checker.parse` with an ordered list of accepted names built from the validated version (never from JSON, as today). `Verify` already derives the required identifier from the chosen asset name (`strings.TrimSuffix(assetName, ".dmg")`) and `release.yml` signs with `--identifier "portty-${VERSION}-darwin-universal"`, so any new name needs the same CI identifier rule (`release_workflow_test.go` pins it). `Clean` globs only `portty-v*-darwin-universal.dmg` and `.part`; widen to the list. The arm64 name is not defined anywhere in the repo yet (grep found no `darwin-arm64` outside archived milestone files), see Open Question O1.

### Anti-Patterns to Avoid
- **`ditto` or `cp` over `/Applications/Portty.app`:** invalidates code pages of running processes. Use stage + swap.
- **`open -n`:** can start a second GUI if the old one has not fully exited. Plain `open <path>` after the old pid is gone lets LaunchServices dedupe.
- **`pkill -f "Portty.app/Contents/MacOS/"`:** matches the daemon too (it is the same executable). The repo's own `vdeploy.sh` does this for tests; the installer must filter on args and pid.
- **Executing an installed older Portty to read its version:** v1.1.0's `--version` starts the GUI (todo `2026-11-23-version-flag-hangs`, fixed in Phase 16 per the `launchMode` comment). Read Info.plist instead; execute only the *staged* new build, under a timeout.
- **Combined stdout+stderr for `hdiutil -plist`:** `ExecRunner` uses `CombinedOutput()`; hdiutil writes a deprecation warning to stderr that corrupts the plist. [VERIFIED: probe stderr: `hdiutil: WARNING: 'hdiutil attach -nobrowse -readonly -mountpoint ...' is deprecated...`] Add a stdout-only runner.
- **Trashing the old bundle before health:** keep it aside (it is the rollback copy) until healthy.
- **AppleScript `tell Finder` to trash:** Automation consent prompt on an unattended Mac.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Atomic bundle exchange | rename-aside-then-rename-in (window with no app) | `unix.RenamexNp(..., RENAME_SWAP)` | One syscall, no missing-app window, swap-back is the same call |
| Plist parsing | a plist library | `plutil -convert json -o - -` / `plutil -extract KEY raw` via Runner | No dependency; matches the Runner seam |
| Copying a signed bundle | `cp -r`, Go tree walk | `ditto` | Preserves symlinks, xattrs, signature seal |
| Signature / notarization checks | custom Mach-O parsing | existing `codesign` + `spctl` Runner calls | Same trust chain as `Verify`; stapled ticket works offline |
| Trash | `os.Rename` into `~/.Trash`, Finder AppleScript | `NSFileManager trashItem` (cgo) | `~/.Trash` is TCC-sensitive [ASSUMED]; AppleScript prompts for Automation |
| Waiting for the old GUI | sleeping a fixed time | poll `kill(pid, 0)` with a deadline (TransomWM's loop) | Exits as soon as the pid is gone; deadline protects against a stuck quit |
| Session carry-over | any new daemon code | existing takeover run by the new GUI's `EnsureDaemon` | Verified path; adding a second mechanism risks two PTY readers |
| Single-instance guard | a new lock | wait for the old pid + plain `open` | `SingleInstanceLock` is not in use; LaunchServices dedupes after the old instance is gone [ASSUMED, P4] |

**Key insight:** every hard sub-problem here already has an OS or repo primitive; the work is sequencing, fault handling and test seams, not new mechanisms.

## Process and Runtime State Across the Swap

(This is not a rename phase; this table covers what stays live after the bundle at the target path changes.)

| Item | State after swap | Action |
|------|------------------|--------|
| Old daemon (version N) | Keeps running from the old inode; its exe path now names the new binary | None; the new GUI's takeover replaces it. Rename keeps old inodes valid [VERIFIED by Pattern 4 semantics; M-105 precedent used `rm -rf` + `ditto`] |
| `<configDir>/bin/portty` link and `/usr/local/bin/portty` | Point at the same path, which now holds the new binary | None for the update path. A link made from a DMG or translocated path dangles: mover mode must not create CLI links and the installed copy re-links on its first daemon start (`clilink.go` comment: "heals itself after app moves and updates") |
| LaunchAgent from `portty daemon install` | `Executable` is the resolved path of the binary at install time | Same path keeps working. Opt-in, KeepAlive false (`service.go`, `handoff.go` shortcut note) |
| Stale staging dirs, attached images, copied installer | Left behind if the GUI or installer dies mid-way | Sweep at GUI startup: detach images whose `image-path` is under the update cache, remove `.Portty-update-*` older than a day with no live installer, remove old installer copies |
| Update cache DMG | Deleted only after health | `Clean` (existing) also runs at the start of the next Download |

## Common Pitfalls

### Pitfall 1: Rollback cannot pull sessions back from a committed N+1 daemon
**What goes wrong:** N relaunches after rollback, sees a "stale" daemon (N+1), its takeover is refused, and the user gets "Restart Now" (ends all sessions).
**Why:** `handoff.go:473-474` refuses `an older release cannot take the sessions`; the comment at lines 327-329 says a downgrade handoff is unsupported.
**How to avoid:** `keepDaemon` result flag consumed by `App.ensure()` (Pattern 6). Test it in the unit suite with the `ensureDaemon` var seam, and in the VM with a committed takeover then a forced rollback.
**Warning signs:** "Update Portty's background service?" dialog right after a rollback.

### Pitfall 2: A mounted quarantined DMG may raise a system dialog during an unattended copy
**What goes wrong:** On 2026-11-24 a researcher attached the user's real quarantined DMG and ran `ditto` from it; macOS showed a "move disk image to Trash" dialog and the image stayed attached. [CITED: user memory file `portty-research-no-host-side-effects.md`] I found no public source describing that dialog; mechanism unknown.
**How to avoid:** Probe P1 in the VM first. Mitigation ready in Pattern 8 (clear the DMG's quarantine after Verify, before attach). For mover mode the user's DMG is quarantined by the browser and the OS may show its own dialog; ours must tolerate the user answering it either way (our trash call must handle "already gone").

### Pitfall 3: `ditto --noqtn` does not remove quarantine
**What goes wrong:** relying on `--noqtn` leaves `com.apple.quarantine` on the staged app and the relaunch stops at a Gatekeeper dialog nobody can click.
**Evidence:** [VERIFIED: scratch probe, macOS 27.2.1] source dir and file both carried `com.apple.quarantine`; `ditto src dst1.app` and `ditto --noqtn src dst2.app` both left it on `dst*.app` and `dst*.app/Contents/MacOS/Portty`; `xattr -dr com.apple.quarantine dst1.app` then listed nothing.
**How to avoid:** always `xattr -dr com.apple.quarantine <staged>`, then assert `xattr -lr` has no quarantine before the swap (and in the VM after).

### Pitfall 4: App Management (macOS 13+) can refuse the modification
**What goes wrong:** same-Team-ID updates are documented as allowed, but field reports show intermittent refusals, and refusal looks like EPERM plus a system notification the user sees.
**Evidence:** [CITED: developer.apple.com/documentation/bundleresources/information-property-list/nsupdatesecuritypolicy] "For signed apps, macOS allows apps from the same developer — those sharing the same Team ID — to modify the app's bundle." Counter-evidence: [CITED: developer.apple.com/forums/thread/742720] a same-Team updater on Sonoma 14.1+ failed on about 1 in 7 machines creating directories inside a freshly extracted `*.bundle` (`kTCCServiceSystemPolicyAppBundles`), worked around by extracting without the `.bundle` extension; [CITED: developer.apple.com/forums/thread/721002] same-Team installer still notified when extracting an app to a USB drive root. Apple offers `NSUpdateSecurityPolicy` `AllowProcesses` (Team ID to signing IDs) for finer control. A mjtsai excerpt claims unsandboxed apps get the permission anyway because they can run commands via Terminal, which conflicts with the Apple statement; treat the matter as unproven.
**How to avoid:** Probe P2 and P8 with a Developer-ID-signed build (ad-hoc builds have no Team ID, so an ad-hoc rig does not test the real rule). Map EPERM to the App Management fallback text. If P8 shows staging inside a new `.app` directory is refused, stage under a name without the `.app` extension and rename at swap time. Consider adding `NSUpdateSecurityPolicy` to `build/darwin/Info.plist` only if P2 fails; it needs a release to take effect, so decide before the first Phase-18 release.

### Pitfall 5: Gatekeeper / translocation on relaunch
**What goes wrong:** a quarantined app launched from where it landed runs from a randomized read-only `AppTranslocation` path; eclecticlight reports the only reliable way to prevent translocation is stripping `com.apple.quarantine`, and a move alone may not suffice. [CITED: https://eclecticlight.co/2023/05/09/what-causes-app-translocation (search summary)] Whether an app launched from a mounted quarantined DMG is translocated is not established by any source I found (Objective-See describes translocation as a read-only DMG of the app; no source exempts DMG launches).
**How to avoid:** strip quarantine on the installed copy (Pitfall 3), relaunch by path with `open`, assert no quarantine xattr and no dialog in the VM. For mover detection handle both `/Volumes/...` and `/AppTranslocation/...` bundle paths (Pattern 7). Probe P5.

### Pitfall 6: Two copies running
**What goes wrong:** `open` while the old process lives only activates it; or in replace-from-DMG both the DMG-run GUI and an already-running installed GUI exist; or a double relaunch starts two GUIs (no `SingleInstanceLock`, confirmed by grep: no `SingleInstance` in the root package).
**How to avoid:** the installer is the only relauncher and launches exactly once, after every Portty GUI pid for the source and the target is gone (SIGTERM, which `app.go` routes to the GUI-only quit, then SIGKILL after a deadline; never touch `daemon` processes). Plain `open`, no `-n`. VM assertion: exactly one GUI pid and one tray icon after every flow.

### Pitfall 7: Running the wrong thing to learn a version
`Portty --version` from v1.1.0 starts the GUI. [CITED: `main.go` `launchMode` comment: "a CLI-only call such as `Portty --version` over SSH never starts the GUI (todo 2026-11-23-version-flag-hangs; 16-CONTEXT D-01)"] Read installed versions from `Info.plist` (`CFBundleShortVersionString`, set from `wails.json` productVersion by `release.yml`); run only the staged new build with `--version`, 10 s cap, and compare to `"v"+release.Version`. Caveat for the rig: two ad-hoc rig builds that differ only in `-X main.Version=` share Info.plist version `0.1.0` (`wails.json`), so the rig must also patch `productVersion` per build with `jq` as `release.yml` does, or the mover's "older?" comparison reads equal versions.

### Pitfall 8: Release builds can silently lose the health path
The health signal depends on `daemon` `/health` `Version` and on the takeover running at GUI start. A change that skips `ensure()` at startup, or a future mover-mode edge that leaks into normal launches, makes every update roll back. Pin with a unit test that normal GUI startup calls the ensure seam, and that mover mode does not.

### Pitfall 9: Hidden state in /Applications
A killed installer or GUI leaves `/Applications/.Portty-update-*` (tens of MB each) and possibly an attached image. Sweep at startup (Process and Runtime State table). If `trashItem` of the old bundle fails, keep it in place and let the sweep delete it after a day rather than leak (log it; this departs from "Trash, not deleted" only for the failure case, see Open Question O4).

### Pitfall 10: LSUIElement app, no Dock presence in mover mode
Portty is `LSUIElement` true (`build/darwin/Info.plist`), so a double-click from the DMG shows nothing until the window; mover mode keeps the window hidden. The "Replace?" dialog and any failure alert must appear in front: `NSAlert` for an agent app is not guaranteed to activate. Verify in the VM (P4) that the dialog is frontmost; if not, activate the app before `a.dialog`.

### Pitfall 11: Test binaries must not spawn, mount or relaunch
`TestMain` already points HOME at a temp dir; every new seam (spawn installer, Swap, Trash, Open, Health, hdiutil) must be injected and the real spawn must refuse under `testing.Testing()`, like `spawnTakeover`. The real-tool test must use only `t.TempDir()` scratch images, never the user's DMG, `~/Downloads` or `/Applications`.

## Code Examples

Illustrative skeletons; the discrete in-repo values they depend on are quoted in the next section. Anything else is `[ASSUMED]` until the plan pins it.

### Stdout-only runner for `hdiutil -plist` (Runner-compatible)
```go
// Source: probe this session, hdiutil writes its deprecation WARNING to stderr.
func StdoutRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &out // stderr deliberately dropped
	err := cmd.Run()
	return out.Bytes(), err
}
// attach: /usr/bin/hdiutil attach -nobrowse -readonly -noverify -noautoopen -plist -mountpoint <dir> <dmg>
// then:   /usr/bin/plutil -convert json -o - -  (stdin) ; find system-entities[].mount-point
```

### Atomic swap and swap-back
```go
// Source: golang.org/x/sys v0.46.0 unix.RenamexNp, probed on APFS in scratch.
func swap(staged, target string) error {
	if _, err := os.Lstat(target); errors.Is(err, os.ErrNotExist) {
		return os.Rename(staged, target) // first install: renamex_np RENAME_SWAP returns ENOENT
	}
	return unix.RenamexNp(staged, target, unix.RENAME_SWAP) // staged now holds the OLD bundle
}
```

### Installer step order (pseudo-Go)
```go
// RunInstaller(job, env) - every effect behind a seam in env (Run, Swap, Trash, Open, Health, Procs, Now)
waitGone(job.OldPID, 20*time.Second)            // then SIGKILL that pid only
if job.Kind == "mover" { quitInstalledGUI(job.Target) } // never a daemon pid
reverify(job.Staged)                            // codesign --verify --deep --strict + Team (shrinks TOCTOU)
if err := swap(job.Staged, job.Target); err != nil { result("failed", "swap"); open(job.Target); return }
open(job.Target)                                // plain `open`, no -n
if healthy(ctx, job, deadline) {
	trash(job.Staged /* holds the old bundle */); os.RemoveAll(stagingDir)
	os.Remove(job.DMGCache) // update kind
	if job.Kind == "mover" { detach(job.Mount); trashDMG(job.ImagePath) }
	result("updated"); return
}
killGUI(job.Target); _ = swap(job.Staged, job.Target) // back
result("rolled-back", keepDaemon: true); open(job.Target)
```
(Mover kind detaches the volume earlier, right after the DMG-run GUI exits and before the swap, so a later failure does not leave the image mounted; the pseudo-code shows the success cleanup only.)

### GUI hooks (package main)
```go
// quit seam: a.onTerminate sets quitting then runtime.Quit -> beforeClose allows it (GUI only)
svc.Quit = a.onTerminate
// keep-daemon relaunch: in App.ensure(), when the pending result says keepDaemon:
//   ensureDaemon(ctx, sock, daemon.EnsureOptions{Restart: func(daemon.RestartQuestion) bool { return false }, NoHandoff: true})
```

### Verified in-repo values (read this session)
| Value | Source (path:lines) | Verbatim |
|-------|---------------------|----------|
| Team ID constant | `internal/updater/update.go:23` | `TeamID      = "S2K7P43927"` |
| Cache dir | `app_update.go:41-48` | `~/Library/Caches/app.portty.Portty/updates` (comment) and `filepath.Join(base, "app.portty.Portty", "updates")` |
| Update states | `internal/updater/service.go:16-17` | `State: idle \| checking \| current \| available \| downloading \| verifying \| revealed` |
| Busy states | `internal/updater/service.go:73` | `func busy(state string) bool { return state == "downloading" \|\| state == "verifying" }` |
| DMG quarantine written by Verify | `internal/updater/install.go:146` | `flag := fmt.Sprintf("0081;%x;Portty;", now.Unix())` |
| Clean globs | `internal/updater/install.go:174-175` | `"portty-v*-darwin-universal.dmg"` and `"portty-v*-darwin-universal.dmg.part"` |
| Single asset name | `internal/updater/update.go:78` | `func DMGAssetName(v string) string { return "portty-v" + v + "-darwin-universal.dmg" }` |
| GUI-only quit | `app.go:659-666` | `// onTerminate is the SIGTERM handler (UPGR-05): it quits the GUI only.` / `a.quitting.Store(true) // beforeClose now allows the quit` |
| Downgrade handoff refused | `internal/daemon/handoff.go:473-474` | `if olderRelease(hello.Build, BuildVersion) {` / `refuse(takeoverRefused, "an older release cannot take the sessions")` |
| Takeover trigger | `internal/daemon/process.go:272-276` | `if StaleVersion(ver) {` / `n := liveSessions(client)` / `if n != 0 && !opts.NoHandoff && health.Handoff == HandoffVersion {` |
| Takeover uses this build's exe | `internal/daemon/process.go:356` and `process_unix.go:65` | `exe, err := os.Executable()` and `cmd := exec.Command(exe, "daemon", "--takeover")` |
| Stale rule | `internal/daemon/process.go:475-477` | `return BuildVersion != "" && BuildVersion != "dev" && ver != BuildVersion` |
| Bundle id | `build/darwin/Info.plist:13` | `<string>app.portty.Portty</string>` |
| Output / binary name | `wails.json:4` | `"outputfilename": "Portty",` |
| DMG contents | `.github/workflows/release.yml:229` | `hdiutil create -volname "Portty" -srcfolder build/bin/Portty.app -ov -format UDZO "build/bin/${DMG_NAME}"` (the volume holds only `Portty.app`, no Applications symlink) |
| Release signing | `release.yml:206-211` | `codesign --deep --force --verbose --options runtime --timestamp --entitlements build/entitlements.plist --sign "$MACOS_CERTIFICATE_NAME" build/bin/Portty.app` |
| Agent app | `build/darwin/Info.plist` | `<key>LSUIElement</key>` / `<true/>` |
| No hidden CLI usage entry | `cmd_daemon.go:32` | `case "--takeover": // internal, so not in the usage: only EnsureDaemon spawns it (09-05)` |
| cliNeedsDaemon skip list | `cmd_cli.go:76` | `if args[0] == "health" \|\| args[0] == "version" \|\| args[0] == "open" {` |

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Updaters write into any app bundle | App Management (`kTCCServiceSystemPolicyAppBundles`) gates modification of other teams' bundles; same Team ID allowed, `NSUpdateSecurityPolicy` to widen | macOS 13 (Ventura) [CITED: Apple docs above] | Rig must use Developer ID signing to test the real rule |
| `hdiutil attach/detach` | Prints deprecation warnings to stderr; `diskutil image attach` is Apple's replacement but absent on older macOS | macOS 27 host shows the warning [VERIFIED: probe]; replacement availability on macOS 13/14 is from prior research [CITED: .planning/research/STACK.md] | Keep hdiutil, stdout-only runner, tolerate the warning; revisit if a future macOS removes it |
| Release DMG format | `hdiutil create -srcfolder` now yields an APFS image on this host [VERIFIED: probe entity content-hint `Apple_APFS`] | macOS 27 default | Attach works the same; do not assume HFS+ |
| Universal Intel+arm64 release | Apple Silicon only, macOS 14+ (REL-01, Phase 23) | planned | The pre-swap `--version` smoke test and an `LSMinimumSystemVersion` check protect a user whose Mac cannot run the new build |

**Deprecated/outdated:** `ditto --noqtn` as a quarantine remover (does not work here); `open -n` for relaunch (second-instance risk).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The installer copy (fresh file, original signature bytes) runs from the config dir without a code-signature kill, and its App Management attribution is still Team S2K7P43927 | Pattern 2 | Installer cannot start or every swap is refused; fall back to running the installer from the staged new bundle (N+1 code does the swap) |
| A2 | App Management denials surface as EPERM and plain permission failures as EACCES | Pattern 1 table | Wrong fallback text only; both still reveal |
| A3 | Mounting a quarantined DMG with `hdiutil attach` (and copying out) can raise an OS dialog; clearing the DMG's quarantine before attach prevents it | Pitfall 2, Pattern 8 | If clearing the DMG quarantine does not help, the install flow needs a different mount approach; if it is unneeded, drop the extra step |
| A4 | `NSFileManager trashItem` works from the non-AppKit installer process with no TCC prompt for the old bundle in `/Applications` | Standard Stack | Old bundle stays aside; sweep deletes it later (Open Question O4) |
| A5 | Trashing a DMG in `~/Downloads` may trigger a one-time Downloads-folder TCC prompt; refusal is non-fatal | Pattern 7 | Mover leaves the DMG; message says so |
| A6 | An app launched from a mounted quarantined DMG is not translocated; if it is, `statfs` `f_mntfromname` of the nullfs mount resolves the origin | Pattern 7, Pitfall 5 | Mover detection misses the DMG case; add the translocation branch |
| A7 | Double-clicking Portty in a mounted DMG while Portty runs from /Applications either launches a second GUI or just activates the running one | Pitfall 6 | Mover scenario differs; the installer's quit-installed-GUI step covers the first, the second needs no code |
| A8 | `codesign -dvv` of the shipped app reports `Identifier=app.portty.Portty` (identifier equals the bundle id because the bundle is signed with `--deep` from `Info.plist`) | Pattern 1 step 4 | Identifier check must use the real value; read it from a built app in P7 |
| A9 | LaunchServices dedupes a plain `open <path>` after the old instance exited | Pitfall 6 | Rare second GUI; the installer already waits for pid exit |
| A10 | Detach succeeds on the first or second try once the DMG-run GUI is gone (Finder does not block it) | Pattern 7 | Retry/backoff then `-force` covers it; VM measures |
| A11 | The arm64-only asset name will be `portty-v<ver>-darwin-arm64.dmg` | Pattern 9 | Updater cannot find Phase 23 releases until the list matches; confirm with the user (O1) |
| A12 | `~/.Trash` is TCC-protected so a plain rename into it is unreliable | Don't Hand-Roll | Only affects the rejected alternative |
| A13 | LaunchServices refuses with a modal dialog when a build needs a newer macOS than the host; the pre-swap `--version` run and `LSMinimumSystemVersion` check avoid reaching it | State of the Art | A newer-OS-only build could still be installed; the explicit plist check closes it |

## Open Questions (RESOLVED)

1. **O1: Arm64-only asset name.**
   - Known: REL-01 (Phase 23) says Apple Silicon only, macOS 14+; no name exists in the repo; `release.yml` and `Verify` bind the identifier to the asset name.
   - Unclear: the exact filename and whether Phase 23 drops the universal asset at once.
   - Recommendation: lookup list `[universal, arm64]` as one `var`, arm64 name per A11; confirm with the user before Phase 23; Phase 23 must keep `--identifier` equal to the name (the existing workflow test already enforces this).
   - RESOLVED (18-02), user-confirmed 2026-11-25: `portty-v<ver>-darwin-arm64.dmg` is accepted alongside the universal name (user answer "portty-vX.Y.Z-darwin-arm64.dmg (Recommended)", 2026-11-25, recorded under [Assets] in 18-CONTEXT.md). The one-way name is human-settled, so 18-02 has no checkpoint and its Task 2 compiles the two-name lookup list directly.

2. **O2: What is "healthy" when the takeover is refused and the DEBT-08 question is pending?**
   - Known: the daemon stays N, so the signal (daemon reports N+1) never fires; the user is looking at the restart dialog.
   - Recommendation: accept the rollback at the deadline (relaunched N shows "update did not take", sessions untouched); do not extend the deadline for a human answer. Confirm in P-row "forced takeover refusal".
   - RESOLVED (18-03, flagged decision O2): adopted. The deadline still rolls back, the relaunched N runs its first ensure with NoHandoff and a restart policy that answers no, so it asks nothing, and the deadline is not extended for a human answer. Covered by TestInstaller_HealthDeadlineRollsBack (daemon version stays at N while the N+1 GUI is alive); no separate VM row is planned, R2-A and R2-B cover the relaunch side.

3. **O3: Mover when the installed copy is the same version or newer.**
   - Recommendation: no replace, `open` the installed copy, eject the DMG, leave the DMG file alone (no Trash), message once. Needs user confirmation because the milestone request says "trash the download".
   - RESOLVED (18-06): the recommendation is not adopted, so no user confirmation is needed. Nothing is replaced and the installed copy is opened, and after the volume is ejected the downloaded DMG is moved to the Trash exactly as on the other install paths, which matches UPD-08 literally (the Trash is recoverable). 18-07 rows R4-d and R4-g expect the DMG in the Trash. REQUIREMENTS.md is not amended.

4. **O4: Old bundle when `trashItem` fails.**
   - CONTEXT: Trash, not delete. Failure case is unspecified. Recommendation: keep it aside and delete it in the startup sweep after a day, with a log line; confirm this single departure.
   - RESOLVED (18-05): stricter than the recommendation, so there is no departure to confirm. The old bundle is never deleted: it stays aside in its staging folder and every startup sweep retries the Trash, logging each refusal. The locked "Trash, not deleted" rule holds on every path.

5. **O5: Rig feed seam.** `updates_vmpreview.go` sits behind `vmpreview`, which also fakes Tailscale and sharing (`tailscale_vmpreview.go` returns `127.0.0.1`), so the existing "share link on a real tailnet" check cannot be combined with the fake update feed. Options: a second tag that only repoints the update feed (precedent: the `vmwhois` tag), or accept the loopback fake share for the update rows and verify the token with `curl` inside the VM. Pick before Wave 0; whichever is chosen must be added to `tests/check-brand.sh` (it greps for VM-only tags in `.github`, `build.sh`, scripts) and to the `release.yml` binary check that already greps for `127.0.0.1:8788`.
   - RESOLVED (18-08): the existing `vmpreview` tag is reused, with no new tag. VM rows check the share link with `curl` on loopback inside the VM (R1 in 18-07), and the real tailnet share link is checked in the office-Mac item M-169 (18-07). The verify-mode bypass lives in `updates_vmpreview.go` itself, so the existing `release.yml` `127.0.0.1:8788` binary check and `tests/check-brand.sh` already cover it.

6. **O6: Health deadline value.** CONTEXT says about 60 s; take the VM measurement of a takeover with two ticking sessions plus install time and set the constant (and the daemon-version stable window) from it.
   - RESOLVED (18-01 and 18-07): 18-01 sets `HealthTimeout = 60 s` and `HealthStable = 3 s` as single constants. 18-07 Task 1 measures click-to-healthy in R1 and R2-B and raises HealthTimeout to twice the larger time (rounded up to the next 10 s) when the 2x headroom rule fails, then re-runs R1.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| hdiutil, ditto, xattr, codesign, spctl, plutil, open, ps, pgrep, lipo | install steps | yes (host) | macOS 27.2.1 | — |
| Go toolchain | build/tests | yes | go1.26.11 (`GOTOOLCHAIN=go1.26.11`) | — |
| `gh` CLI | fetch real notarized release DMGs for rig row R3 | yes | 2.105.0 | `curl` the public asset URL |
| Tart VM `portty-test` (macOS 27.2) and tools in `~/.local/share/portty-vm-tools` (`vm.sh`, `vdeploy.sh`, `click`, `axmenu`, `vshot.sh`; `axdump` is in the VM's `~/ptest`) | all install/relaunch/rollback checks | tools present on host; VM not started in research | — | None for end-to-end; do not start/stop VMs in research. `vdeploy.sh` pkills with `-f "Portty.app/Contents/MacOS/"`, which also kills the daemon, so write a rig script that does not |
| Developer ID signing identity on the host (used by `spike/desktop/bundle/build.sh`) | rig builds that must carry Team S2K7P43927 for the App Management rows | unverified here (not probed; no host signing performed in research) | — | Ad-hoc builds exercise swap/relaunch/health/rollback but not App Management or the real Team check |
| Published real DMGs | R3 (real chain) | v1.2.0 `portty-v1.2.0-darwin-universal.dmg` 16,255,466 bytes is on GitHub; v1.1.0 also exists [VERIFIED: `gh release view`] | v1.2.0 Latest | — |
| macOS versions older than 27 | App Management behaviour on 13/14/15 | no | — | Record as untested; office Mac is macOS 27.2.x per TESTING.md |

**Missing dependencies with no fallback:** none for planning. **Execution-time blockers:** VM boot (portty-test must be up, tailnet state per the ephemeral-VM note) and a Developer ID identity for the faithful App Management row.

## Test Strategy and TESTING.md Obligations

(`workflow.nyquist_validation` is `false` in `.planning/config.json`, so the formal Validation Architecture section is omitted. The planner still needs this.)

**Automated (CI, `go test -race -short ./...`, vitest):** pure logic behind seams, no real mount or spawn.
- `internal/updater`: asset-name list (`TestLatestAcceptsArm64Name`-style), `Clean` globs, install pre-flight (writability, space, translocated/`/Volumes` target, downgrade), plist-JSON mount-point parse (feed a captured `hdiutil attach -plist` JSON), VerifyApp steps (each failure names its step), Stage cleanup on every failure, quarantine re-set before Reveal, Service state machine with a fake `Install` (installing, relaunching, revealed+Error), `busy()` includes the new states, error texts contain no URL or path (extend `TestService_ErrorTextsHaveNoURLOrPath`).
- Installer logic (`RunInstaller`) with fake Runner/Swap/Trash/Open/Health/Procs: success order, old GUI stuck then SIGKILL, swap failure leaves target untouched and relaunches old, health deadline rolls back and sets `keepDaemon`, mover with and without an existing copy, DMG detach retry then force, trash failure keeps old aside, job validation rejects wrong owner/mode/paths.
- Package main: `cmd_install_update` dispatch (skips daemon ensure, absent from usage, `launchMode` unchanged), `App.ensure()` keep-daemon policy under a result file, mover branch skips tray/daemon/pollers and `domReady` window (use the existing `ensureDaemonFunc`/`showWindowFunc` seams), startup sweep, spawn refuses under `testing.Testing()`.
- Real-tool test (darwin, env-gated like `real_darwin_test.go`, e.g. `PORTTY_UPDATER_REAL=1`): `t.TempDir()` scratch DMG built with `hdiutil create`, real attach/detach, `ditto`, `RenamexNp` on a fake ad-hoc `Portty.app`. Verified safe to run on the host this session (a nobrowse scratch attach shows no UI); never point it at a user DMG.
- vitest: `UpdateBanner` and `App.update` for the new states, "Update" copy, revealed+error warning variant, dismiss rules.

**VM rig (two Tart builds, evidence in a new `18-VM-CHECKS.md`):**
- R1 (swap/relaunch/health/rollback logic, ad-hoc): builds A and B from one commit differing in `-X main.Version=` and in `wails.json` productVersion (Pitfall 7), a `vmpreview`-gated fake Runner that returns success for codesign/spctl/xattr-write checks while real hdiutil, ditto, rename, open and health run, DMG of B served by `cmd/fakegh`. Two ticking sessions (one full-screen app) and a share link; assert same session ids and shell pids, ticks gapless, link token works, DMG ejected and deleted, old bundle in Trash.
- R2 (rollback): same rig with fault injection in the VM build only (installer ignores health or the new GUI is blocked), once with the takeover not started (case A) and once after a committed takeover (case B, keep-daemon relaunch, no "Restart Now" dialog).
- R3 (real chain, Developer ID N): fakegh serves the real v1.2.0 DMG under its real name; N is a Developer-ID-signed Phase-18 build with a lower version. Exercises the real Verify, Gatekeeper, notarization offline staple, App Management and P1. Because v1.2.0 predates this phase it also proves the health signal needs no N+1 cooperation. Fetch the DMG with `gh release download v1.2.0 --repo scottkw/portty --pattern '*.dmg'` into a scratch dir and check it against `checksums.txt`.
- R4 (mover): quarantined DMG (set `com.apple.quarantine` on the copy, then mount inside the VM only), no existing copy; existing older copy with the dialog; ask-No; non-writable `/Applications`; run twice to prove idempotence; one GUI pid and one tray icon at the end.
- Office Mac: one M-NN manual row for SC1 on the real release path, run by the owner.

**TESTING.md obligations (repo `CLAUDE.md` standing rule):** add new test files to the Section 2 manifest (counts: Go 174, vitest 122, total 307 today), add Section 4 traceability rows (UPD-05..UPD-09 with repo-relative paths ending `.go`/`.ts`/`.tsx`/`.sh`, no test names in the path column), add Section 5 items starting at **M-165** (highest existing is M-164) in a new Category BI, update existing UPD rows whose texts change (the "Download" copy, `revealed` state), run `bash tests/check-traceability-paths.sh` before committing, and note any `build.yml` matrix change (none expected).

## Security Domain

### Applicable ASVS Categories (level 1, `security_block_on: high`)

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no (unchanged) | share tokens and CSP untouched; the takeover keeps them |
| V4 Access Control | yes | Installer job file owner/mode checks; installer reachable only as a local subcommand; no admin escalation, no `osascript` privileges |
| V5 Input Validation | yes | Job and result JSON validated (types, path shape, `Portty.app` basename, staging is a sibling of target); asset names built from the validated version, never from JSON; DMG trash only for `portty-v*-darwin-*.dmg` regular files owned by the user; bundle id / Team / identifier compared exactly |
| V6 Cryptography | yes | SHA-256 + codesign + notarization via system tools; never hand-rolled |
| V12 Files / V14 Config | yes | 0700/0600 dirs and files; no path or URL in logs or user text (existing rule) |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Substituted or downgraded app inside a genuine DMG | Tampering | Verify the app inside, not only the DMG: Team S2K7P43927, `Identifier=app.portty.Portty` (A8), staged `--version` equals the release, version strictly newer than running |
| TOCTOU between verify and swap in a group-writable `/Applications` | Tampering | Staging dir created 0700 by the user, re-verify (codesign `--verify --deep --strict` + Team) in the installer immediately before the swap; same-user malware is out of scope |
| Symlink or path trick in the job file or staging path | Tampering | `Lstat`, reject symlinked targets/staging, sibling-only staging, base-name checks |
| Replay of an older genuine release | Tampering / Elevation | Existing identifier-bound-to-version rule plus the newer-than-running check |
| Arbitrary file trash/delete via crafted DMG backing path | Tampering | Mover trashes `image-path` only if it is a regular file whose basename matches the asset pattern, never a directory, never the update cache (that one is `os.Remove`d) |
| Killing the wrong process | DoS | Kill only pids parsed from `ps` whose args are exactly the bundle executable with no `daemon` argument |
| Hung or malicious `hdiutil`/`ditto` | DoS | Context deadlines on every Runner call; stdin is /dev/null so a DMG license prompt cannot block |
| Remote shell exposure through the update window | Info disclosure / Elevation | Nothing here changes tokens, CSP or share scope; the sessions-survive requirement is verified in the VM including `curl` of the share link before and after |

## Sources

### Primary (HIGH confidence)
- Repo files read this session: `internal/updater/{update,install,service}.go`, `app_update.go`, `app.go` (startup, ensure, quit paths), `main.go`, `cmd_cli.go`, `cmd_daemon.go`, `internal/daemon/{process,process_unix,handoff,client,service,clilink}.go`, `build/darwin/Info.plist`, `.github/workflows/release.yml`, `wails.json`, `TESTING.md`, frontend `UpdateBanner.tsx` and `wailsjs` bindings, `cmd/fakegh/main.go`, `updates_vmpreview.go`, `.planning/config.json`.
- Local probes (scratchpad only, no user files): `renamex_np(RENAME_SWAP)` on directories; ENOENT on a missing target; `hdiutil create/attach -plist/info -plist/detach` of a scratch DMG (stderr warning, plist keys, APFS content); `ditto`, `ditto --noqtn`, `xattr -dr` quarantine behaviour; `man open`, `man hdiutil`, `man ditto`.
- [Apple: NSUpdateSecurityPolicy](https://developer.apple.com/documentation/bundleresources/information-property-list/nsupdatesecuritypolicy) - same-Team-ID statement, `AllowPackages`, `AllowProcesses`.
- `gh release view v1.2.0` - asset names and sizes.

### Secondary (MEDIUM confidence)
- [Apple forum 742720](https://developer.apple.com/forums/thread/742720) - same-Team updater refused on Sonoma 14.1+ (original post only, no DTS reply on that page).
- [Apple forum 721002](https://developer.apple.com/forums/thread/721002) and [mjtsai: Ventura App Management](https://mjtsai.com/blog/2022/11/07/ventura-app-management/) - same-team behaviour reports and the WWDC 2022 quote "Apps validly signed by the same developer account or team will continue to be able to update each other."
- [Eclectic Light: What causes App Translocation](https://eclecticlight.co/2023/05/09/what-causes-app-translocation) - stripping quarantine is the only reliable prevention (via search summary).
- Prior milestone research (read, cited by section): `.planning/research/STACK.md` (DMG self-install mechanics), `ARCHITECTURE.md` section 8, `PITFALLS.md` pitfalls 14-16 and 18, `SUMMARY.md`, `FEATURES.md`.
- TransomWM `AppMover.swift` (user's own shipped pattern).
- User memory notes on VM testing and host-safety (`portty-research-no-host-side-effects.md`, `portty-gui-testing-in-tart-vm.md`, `portty-vms-ephemeral-tailnet.md`).

### Tertiary (LOW confidence, marked ASSUMED above)
- Cocoa-dev archive posts on detach failing while an app runs from the image (confirms only that the process must exit first).
- Everything in the Assumptions Log.

## Wave 0 VM probes (run before trusting the install code)

| # | Probe (VM `portty-test` only, scratch images) | Decides |
|---|-----------------------------------------------|---------|
| P1 | Attach a DMG carrying `com.apple.quarantine` with `hdiutil attach -nobrowse` and `ditto` the app out; repeat with the DMG's quarantine cleared. Watch the AX tree for any system dialog; record `xattr -lr` of the staged copy | A3, Pattern 8 |
| P2 | Developer-ID-signed N replaces `/Applications/Portty.app` using the installer copy run from the config dir; also an ad-hoc N; record the errno, any notification, `tccd` lines | A1, A2, Pitfall 4 |
| P3 | `trashItem` of the old bundle and of a DMG in `~/Downloads` from the non-AppKit process; any TCC prompt | A4, A5 |
| P4 | Double-click Portty in a mounted DMG while it runs from /Applications; `open` after a swap; is the replace dialog frontmost for an LSUIElement app | A7, A9, Pitfall 10 |
| P5 | `os.Executable()` and `statfs` of a Portty launched from a mounted quarantined DMG | A6 |
| P6 | Time from DMG-run GUI exit to a successful plain `hdiutil detach` | A10 |
| P7 | `codesign -dvv` of a Developer-ID-signed build: `Identifier=` and `TeamIdentifier=`; same for the installer copy | A8, A1 |
| P8 | Stage with `ditto` into `/Applications/.Portty-update-X/Portty.app` and into a name without `.app`, then rename; refused? | Pitfall 4 |

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - no new dependency; every tool and symbol was probed or read this session.
- Architecture: HIGH for the in-repo seams (verify chain, takeover, quit, dispatch), MEDIUM for the installer/job protocol (design, untested).
- Pitfalls: MEDIUM - the downgrade-handoff wall, `--noqtn`, stderr-in-plist and `--version` hazards are verified; App Management, the quarantined-DMG dialog and translocation are LOW until P1-P8 run.

**Research date:** 2026-11-25
**Valid until:** 2026-12-25 (macOS behaviour claims should be re-probed if the VM or office Mac moves past macOS 27.2.x)
