---
phase: "18"
slug: "self-installing-updates"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-11-26"
---

# Phase 18 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Downloaded DMG to installed app | A release DMG (cache or a browser download) becomes `/Applications/Portty.app` | Signed, notarized bundle; verified (SHA-256, Team ID, identifier, notarization) before quarantine is cleared |
| GUI to detached installer | The GUI writes a job file and spawns a private copy of itself to swap the bundle | Job JSON (0600, owner-only, strict schema), private run folder (0700) |
| Installer to the user's processes | The installer quits, signals and relaunches Portty GUIs | PIDs proven by exact argv and uid; the daemon is never signalled |
| Installer to the filesystem | Swap, rollback, Trash and removal of Portty's own staging folders | Only `.Portty-update-*` folders beside the target, `run-*` work dirs, and a validated DMG path |
| Result file to the next GUI | Outcome, versions, keepDaemon, DMG path | update-result.json (0600, owner-only, outcome whitelist) |
| VM preview build to release | The `vmpreview` tag's verify bypass and loopback identity | Build tag only; brand gate and release grep keep it out of releases |

---

## Threat Register

Mitigations are those declared in the 18-01..18-10 PLAN `<threat_model>` blocks; evidence was re-checked against the code after quick tasks 261126-dfn and 261126-e29.

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-18-01 | Tampering | Stage/RunInstaller verify-to-swap window | high | mitigate | 0700 staging dir beside target; VerifyApp on mounted app, staged copy, and again right before Swap (`appinstall.go`, `installer.go`) | closed |
| T-18-02 | Tampering | ReadJob | high | mitigate | O_NOFOLLOW, regular file, 0600, owner uid, 64 KiB cap, DisallowUnknownFields, path shape checks; Job.KeepDaemon is a known bool | closed |
| T-18-03 | Elevation | ClearQuarantine order | high | mitigate | Quarantine cleared only after verification; written back before every reveal | closed |
| T-18-04 | DoS | Swap / relaunch | high | mitigate | RENAME_SWAP; failed swap relaunches old copy; GUI quits only after the started marker; unprovable quit timeout now relaunches the target (e29) | closed |
| T-18-05 | Elevation | updates_vmpreview.go verify bypass | high | mitigate | `vmpreview` build tag, 127.0.0.1 only, release.yml binary grep, brand gate refuses the tag in CI/build scripts | closed |
| T-18-06 | Info disclosure | update.log, Status.Error | medium | mitigate | Logs and texts carry step names only, no URLs or paths (tests in service_test.go) | closed |
| T-18-07 | DoS | hdiutil/ditto/codesign hangs | medium | mitigate | Every external tool bounded by a context timeout | closed |
| T-18-08 | Tampering | Test binaries touching the host | high | mitigate | SafeRunner refuses hdiutil/open/ditto/osascript under test; temp HOME in TestMain | closed |
| T-18-09 | Tampering | App inside a genuine DMG | high | mitigate | VerifyApp (deep strict, Team, identifier) and CheckStagedApp (notarized, exact version, min OS, smoke run) | closed |
| T-18-10 | Elevation | Privilege escalation on permission failure | high | mitigate | EACCES/EPERM map to ErrNoWrite/ErrAppManagement and the reveal fallback; no sudo/Authorization anywhere | closed |
| T-18-11 | DoS | Smoke run | medium | mitigate | 10 s SmokeTimeout, only after VerifyApp; `--version` never starts the GUI | closed |
| T-18-12 | Tampering | Asset-name list | medium | mitigate | Names built from the version; first present name validated, never skipped; identifier = name | closed |
| T-18-13 | Info disclosure | Failure texts | low | mitigate | Tested texts carry no paths | closed |
| T-18-14 | DoS | Signals to the wrong process | high | mitigate | Signals only to GUIPids (exact argv + uid); SIGKILL only for the listed OldPID; the unprovable branch sends no signal | closed |
| T-18-15 | DoS | Rollback leaves no runnable Portty | high | mitigate | Swap back, result written before relaunch, OpenAttempts retry, failed relaunch recorded | closed |
| T-18-16 | Tampering | Forged result file | medium | mitigate | readOwnFile + outcome whitelist; put-back via WriteResult (temp + rename, 0600) | closed |
| T-18-17 | Elevation | vmFaultHook in a release build | high | mitigate | nil by default, assigned only under `vmpreview`; tag gate | closed |
| T-18-18 | Repudiation | Silent rollback | low | mitigate | Log line, banner and notification on every rollback | closed |
| T-18-19 | Spoofing | Banner text injection | low | accept | React renders text only; texts are constants plus validated versions | closed |
| T-18-20 | Elevation | Banner opening URLs/files | low | mitigate | Banner calls only onDownload/onDismiss; test asserts BrowserOpenURL never called | closed |
| T-18-21 | Tampering | Trash or removal of the wrong path | high | mitigate | Trash and RemoveAll limited to Portty's own `.Portty-update-*` folders (prefix + sibling of target, Lstat, no symlinks) and `run-*` dirs. RemoveAll on a staging folder happens only in the installer rollback, a failed mover first install, and (since e29, T-e29-01) the sweep when a real installed Portty.app sits beside it and the staged copy is strictly newer; sweep keeps the 1 h floor and fixed parents | closed |
| T-18-22 | DoS | Sweep interrupting a live install | medium | mitigate | 1 h age floor (also on the e29 RemoveAll path); sweep runs after startDaemon; mover returns first | closed |
| T-18-23 | Repudiation | Old app lost without trace | medium | mitigate | Old app to the Trash after health; refused Trash keeps it aside and logs. Edge: after a manual downgrade, a kept-aside copy newer than the installed one is deleted by the e29 sweep (Portty's own flows never lower the installed version) | closed |
| T-18-24 | Elevation | Automation prompt / Finder scripting | low | mitigate | NSFileManager trashItemAtURL, no AppleEvents; dfn's osascript `display notification` sends no event to another app | closed |
| T-18-25 | Tampering | Trash of an arbitrary file via image path | high | mitigate | DMG Trash requires a successful Detach and imageFileOK (name pattern, absolute, clean, not in cache, regular file, owner uid); checkMoveJob checks only a clean absolute path since CR-01 | closed |
| T-18-26 | Tampering | Untrusted DMG app installed | high | mitigate | Mover runs StageFrom (verify, copy, unquarantine, verify, staged checks) before the job; installer verifies again | closed |
| T-18-27 | DoS | Daemon/tray pinning the volume | medium | mitigate | Mover path returns before daemon/tray/window start (tested) | closed |
| T-18-28 | Spoofing | Downgrade via an old DMG | medium | mitigate | Same or newer installed copy is opened instead; replace only after the dialog; version check in checkMoveJob | closed |
| T-18-29 | DoS | Two GUIs / killed wrong process | medium | mitigate | DMG GUI never killed; stopGUIs only target's GUIPids; `open` without `-n` | closed |
| T-18-30 | Info disclosure | Probe fixture | low | mitigate | Fixtures use placeholder home paths | closed |
| T-18-31 | Info disclosure | 18-VM-CHECKS.md, evidence | medium | mitigate | Leak greps clean (no tailnet addresses, home paths or tokens) | closed |
| T-18-32 | Tampering | Host files during the matrix | high | mitigate | Every destructive rig step runs inside the VM | closed |
| T-18-33 | DoS | HealthTimeout too short | medium | mitigate | 60 s HealthTimeout vs measured 3.45 s (12x headroom) | closed |
| T-18-34 | Tampering | Host files during rig/probes | high | mitigate | Rig refuses to build over uncommitted generated files; effects in the VM | closed |
| T-18-35 | Info disclosure | R1-lite/P rows, captures | medium | mitigate | Leak greps clean | closed |
| T-18-36 | Tampering | skip-version file | low | accept | Read as a version string only, never as a path | closed |
| T-18-37 | Repudiation | Verdicts without proof | medium | mitigate | Every PASS row in scope cites an existing evidence file | closed |
| T-18-38 | Tampering | Host files during the mover matrix | high | mitigate | Same VM-only rig design | closed |
| T-18-39 | Info disclosure | R4 rows, TESTING.md items | medium | mitigate | Leak greps clean | closed |
| T-18-40 | Repudiation | SC1 claimed as fully met | medium | mitigate | TESTING.md M-169 records SC1's office-Mac half as human_needed | closed |
| T-18-41 | Tampering | Installer copy in RUN | medium | mitigate | RUN via MkdirTemp under a 0700 work dir; copy verified before spawn | closed |
| T-18-42 | Elevation | Quarantine strip on the copy | medium | mitigate | ditto, VerifyApp, then ClearQuarantine, on the private copy only | closed |
| T-18-43 | DoS | Installer killed at exec / slow | high | mitigate | Started-marker wait (3 s vs 417 ms measured); failures remove RUN and reveal | closed |
| T-18-44 | Info disclosure | GUI update.log lines, log captures | medium | mitigate | Durations only; leak greps clean | closed |
| T-18-45 | Tampering | Host files and signing identity | high | mitigate | Rig refuses paths outside scratch; identity printed only as found | closed |
| T-18-46 | DoS | Leftover run folders | low | mitigate | Removed by installer, GUI failure path and sweep | closed |
| T-18-SC | Tampering | npm/pip/cargo installs | low | accept | No dependency change during Phase 18 or its quick tasks | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-18-01 | T-18-19 | Banner text comes from Go constants and validated versions and is rendered as text by React; injection has no input path | Phase 18 plan threat model (18-04); premise re-checked by security audit | 2026-11-26 |
| AR-18-02 | T-18-36 | The skip-version file is parsed as a version only; a same-user writer can at most suppress an offer, which a manual check overrides | Phase 18 plan threat model; premise re-checked by security audit | 2026-11-26 |
| AR-18-03 | T-18-SC | No packages added during the phase | Phase 18 plan threat models (all plans); premise re-checked by security audit | 2026-11-26 |

---

## Unregistered Flags (non-blocking)

- First install from a DMG in Downloads triggers macOS's one-time Downloads-folder access prompt (product decision in 18-CONTEXT, option A; human check M-170).
- `vmpreview` builds treat any 127.0.0.1 caller as this Mac's user (`internal/daemon/share_vmpreview.go`); kept out of releases by the build tag, `tests/check-brand.sh` and `TestNoFakeWhoIsInRelease`. release.yml's binary grep checks only the main-package marker (18-REVIEW IN-09).

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-11-26 | 47 | 47 | 0 | gsd-security-auditor (ASVS L1, block_on high), after quick tasks 261126-dfn and 261126-e29 |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-11-26
