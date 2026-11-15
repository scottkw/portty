---
phase: "01"
slug: "plain-macos-terminal"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-11-15"
---

# Phase 01 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Tailnet / web guest → webserver + relay | Capability-token-gated HTTP and WebSocket routes for shared sessions | Terminal I/O, chat, session metadata (high: remote shell write) |
| Local GUI / CLI → daemon Unix socket | Session create/kill and settings routes | Spawn requests, settings (local user only) |
| Peer metadata → frontend | Remote peers' session lists rendered in the GUI | Session names and status strings (untrusted) |
| settings.json on disk → daemon | Legacy AgentHub keys loaded at startup | Shell path, CLI paths, toggles (local) |
| CI / supply chain → release artifacts | GitHub Actions and package lockfiles | Build inputs, signed DMG |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-01-01 | Repudiation | tests/check-traceability-paths.sh | medium | mitigate | Portable extraction plus "zero paths = fail" and "missing path = fail", proven by the two negative runs in Task 2 verify | closed |
| T-01-02 | Tampering | .github/workflows/*.yml | high | mitigate | scripts/grep-gate.sh must pass after edits (every action stays SHA-pinned); no new actions added | closed |
| T-01-03 | Denial of Service | release.yml validate job | medium | mitigate | validate moves to macos-latest so the root package (now darwin-only) still compiles at tag time (Pitfall 3) | closed |
| T-01-04 | Tampering | e2e fixture build on ubuntu | medium | mitigate | Untag reap_other.go and run the linux cross-compile guard in Task 1 verify (Pitfall 1) | closed |
| T-01-05 | Information Disclosure | committed binaries bin/*, playwright-fixture | low | mitigate | git rm --cached + .gitignore entries | closed |
| T-01-06 | Elevation of Privilege | GitHub branch protection required contexts | low | accept | CI is disabled by user request; the executor must not call the GitHub API. TESTING.md Section 3 records the new context list for the user to re-apply | closed |
| T-01-07 | Elevation of Privilege | App.tsx web-share bootstrap, TabBar guest menu | medium | mitigate | File-browser tab type and guest menu item deleted; Playwright asserts owner and viewer caps get no chevron and no file tab | closed |
| T-01-08 | Information Disclosure | SessionShareModal browse toggle | medium | mitigate | SetSessionBrowse call and binding removed so the GUI can no longer widen minted perms; plan 01-03 removes the server route and perm minting | closed |
| T-01-09 | Tampering | frontend/pnpm-lock.yaml | medium | mitigate | Removal-only change; lockfileVersion '9.0' asserted and `pnpm install --frozen-lockfile` must pass; no new packages fetched | closed |
| T-01-10 | Elevation of Privilege | stale caps carrying legacy files perms | high | mitigate | All files routes deleted on daemon, relay and webserver muxes; files_removed_test.go proves 404/405 on each | closed |
| T-01-11 | Elevation of Privilege | webserver/relay read-pump RO derivation | high | mitigate | Keep `!HasPerm(claims.Perms, "write")`; test that a "read,files.read" token stays read-only | closed |
| T-01-12 | Tampering | relay CORS helpers and origin allowlist | medium | mitigate | FilesCORS/FilesPreflight/isAllowedFilesOrigin kept; server_cors_test.go retargeted to the chat-route usage; security_regression tests must stay green | closed |
| T-01-13 | Denial of Service | remote session attach (RemoteCapStore, remote WS proxy) | medium | mitigate | Kept per Coupling F1/F2; remote_ws_proxy_test.go and open_remote_session_url_test.go run in Task 1 verify | closed |
| T-01-14 | Information Disclosure | file sandbox path traversal surface | high | mitigate | internal/files deleted; `go list ./...` gate proves the package is gone and no route reaches it (ASVS V12) | closed |
| T-01-15 | Tampering | handleCreateSession / CreateRequest | high | mitigate | CreateRequest has no CLI/Args; spawn argv is fixed to ["-l"]; api test posts a legacy body with "args":["--evil"] and asserts the spy saw exactly ["-l"] | closed |
| T-01-16 | Elevation of Privilege | Settings shellPath | medium | accept | Unchanged existing control: SetShellPath rejects non-existent, directory and non-executable paths; only the local user can set it through the daemon socket | closed |
| T-01-17 | Elevation of Privilege | SetSpawnCommandForTest seam | low | accept | In-process Go method only; no HTTP route or binding exposes it; mirrors existing ForTest helpers on the same types | closed |
| T-01-18 | Tampering | resolveDefaultShellPath ($SHELL) | medium | mitigate | Accept $SHELL only when absolute, existing, non-directory and executable; otherwise /bin/zsh; tests cover relative, nonexistent and unset values | closed |
| T-01-19 | Tampering | cliPaths stale-path filter | low | mitigate | Filter kept (minus Windows names) so stale shell-path overrides for non-shell keys, including tailscale, are still discarded; regression test added | closed (superseded: cliPaths and $SHELL resolution removed in 01-11; spawn is constant /bin/zsh -l, see T-01-36/T-01-38) |
| T-01-20 | Elevation of Privilege | daemon PATH augmentation | low | accept | Removing agent dirs only narrows the daemon PATH; login shells rebuild PATH from the user's own profile (RESEARCH A3) | closed |
| T-01-21 | Tampering | status rendering of untrusted peer/old-daemon values | low | mitigate | Single normalizer maps any unknown value to running; class names come only from the three-value union, never from raw input; vitest covers 'bogus' | closed |
| T-01-22 | Repudiation | exit reporting | low | mitigate | pollSessionStatus still emits session:status then session:exit; App.shellExit test guards auto-close and toast behavior | closed |
| T-01-23 | Elevation of Privilege | web-share warning gate (SessionShareModal) | medium | mitigate | Gate becomes enabled && !warned for every session (all are shells); vitest covers first share, repeat share, disabled and re-armed cases; ordered before the Go wire removal so the gate never silently stops firing | closed |
| T-01-24 | Information Disclosure | webserver handleSessionInfo | medium | mitigate | Resolver returns an explicit ok flag; unknown ids get 404; TestSessionInfo_UnknownID404KnownID200 covers unknown and known ids (Pitfall 7) | closed |
| T-01-25 | Spoofing | peer metadata with legacy cli_type/status | low | accept | Fields are ignored by plain json decoding; status values pass through the frontend normalizer from plan 01-06; tailnet test decodes legacy JSON | closed |
| T-01-26 | Elevation of Privilege | residual chat→PTY inject path | high | mitigate | Both read-pump cases deleted; retargeted tests send raw 0x35 from RO and RW caps on both paths and assert zero PTY bytes | closed |
| T-01-27 | Denial of Service | read pump on unknown frame types | medium | mitigate | No default arm; tests assert the socket stays open and chat still works after a 0x35 frame; FuzzParseFrame covers arbitrary frames | closed |
| T-01-28 | Tampering | webserver capability middleware and routing | high | mitigate | FuzzRequireCapability asserts next runs only for a verifying cap whose SID matches the path; FuzzWebServerRoutes asserts no panic/500 | closed |
| T-01-29 | Tampering | chat content sanitization | medium | mitigate | FuzzSanitizeChatContent asserts no control/bidi runes survive and the transform is idempotent | closed |
| T-01-30 | Denial of Service | legacy chat history | low | mitigate | Plain json.Unmarshal ignores sessionInject; chat_test.go loads a legacy line | closed |
| T-01-31 | Repudiation | exit detection (app.go checkExits) | medium | mitigate | One watcher with no deadline, an exitEmitted dedupe, no cold-start baseline, and the frontend restore filter for exits the GUI missed; app_exit_test.go covers emit-once, long-lived, first-sight, prune and relaunch | closed |
| T-01-32 | Tampering | KillSession triggered by session:exit | low | mitigate | checkExits emits only for sessions the daemon reports as State "stopped", so a running session is never removed by the watcher; killing a stopped session only deletes its record | closed |
| T-01-33 | Denial of Service | 500 ms ListSessions poll | low | accept | Local Unix-socket call; one poller replaces one 500 ms poller per created session, so load does not grow with session count | closed |
| T-01-34 | Tampering | legacy autoCloseSession key in settings.json | low | mitigate | json.Unmarshal ignores the unknown key and the next save drops it; TestSettingsMigration_LegacyAgentHubV42 loads the AgentHub fixture and checks the saved JSON | closed |
| T-01-35 | Elevation of Privilege | removed /settings/auto-close-session routes | low | mitigate | settings_removed_test.go asserts 404/405 for GET and PATCH on the daemon socket mux | closed |
| T-01-36 | Tampering | session spawn binary (engine spawnCommand) | medium | mitigate | Spawn is a compile-time constant /bin/zsh with argv [-l]; TestCreateSession_IgnoresShellEnvAndLegacySetting proves neither $SHELL nor a legacy shellPath changes it | closed |
| T-01-37 | Elevation of Privilege | removed shell-path, cli-paths and notify-on-waiting routes | low | mitigate | settings_removed_test.go asserts 404/405 for each removed method and path on the socket mux | closed |
| T-01-38 | Tampering | legacy settings.json keys (shellPath, cliPaths, notifyOnWaiting) | low | mitigate | Unknown keys are ignored by json.Unmarshal and dropped on save; the AgentHub v4.2 fixture test loads and re-saves | closed |
| T-01-39 | Denial of Service | ObjC/Go notification bridge edits | low | mitigate | go build links the cgo bridge; bundle-guard tests (TestHasAppBundleID_*, TestSendNotification_*) still pass; sendNotification keeps its guard so unbundled runs never call UNUserNotificationCenter | closed |
| T-01-40 | Spoofing | checkHealth connected decision | medium | mitigate | Fail closed: any LocalAPI error or non-Running state reads as not connected; TestCheckHealth_LocalAPIUnreachable and TestCheckHealth_BackendState cover it; the share controls key off the same flag | closed |
| T-01-41 | Elevation of Privilege | AutoInstallTailscale (brew install --cask from a Wails binding) | medium | mitigate | Binding, Go method, findBrew and TS declarations deleted; Task 2's count check proves no install-flow reference remains in code | closed |
| T-01-42 | Information Disclosure | sessions web-enabled before Tailscale dropped, now served by the daemon's local-mode LAN fallback | medium | accept | Unchanged daemon behavior outside NET-01's scope. The Share controls are disabled while disconnected, so no new session becomes shared; LAN access stays password-gated; Phase 4's Share redesign owns the fallback (logged in the SUMMARY) | closed |
| T-01-43 | Denial of Service | focus/visibility rechecks against the LocalAPI | low | accept | Event-driven with a 5 s timeout per check; the result is a small local call | closed |
| T-01-44 | Repudiation | the gate silently passing on stale TESTING.md rows | low | mitigate | The traceability script fails on missing paths and on zero extracted rows; Section 5 is checked with a region-scoped grep | closed |
| T-01-SC | Tampering | npm/pip/cargo installs (all plans) | high | accept | No new packages installed by any plan; removals and frozen-lockfile verification only | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

Evidence (L1 grep, 2026-11-15): files routes 404 (`internal/daemon/files_removed_test.go`, `internal/webserver/files_removed_test.go`); `internal/files` gone; RO derivation `!HasPerm(claims.Perms, "write")`; fuzz harnesses `FuzzParseFrame`, `FuzzRequireCapability`, `FuzzWebServerRoutes`, `FuzzSanitizeChatContent`; `TestSessionInfo_UnknownID404KnownID200`; `TestCreateSession_IgnoresShellEnvAndLegacySetting`; `TestCheckHealth_LocalAPIUnreachable`; exit dedupe via `seenSessions` in `app.go` checkExits with `TestCheckExits_EmitsOnceWhenShellExits`; removed settings routes in `internal/daemon/settings_removed_test.go`; no `AutoInstallTailscale`/`findBrew` in code; all workflow actions SHA-pinned.

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-01 | T-01-06 | CI disabled by user request; TESTING.md Section 3 lists contexts to re-apply | plan 01-01 | 2026-11-14 |
| AR-02 | T-01-16 | Superseded: shellPath setting removed in 01-11 | plan 01-04 | 2026-11-14 |
| AR-03 | T-01-17 | Test seam is in-process only, no route or binding | plan 01-04 | 2026-11-14 |
| AR-04 | T-01-20 | Removing agent dirs only narrows daemon PATH | plan 01-05 | 2026-11-14 |
| AR-05 | T-01-25 | Legacy peer fields ignored by json decoding; status normalized | plan 01-08 | 2026-11-14 |
| AR-06 | T-01-33 | One local-socket poller, load independent of session count | plan 01-10 | 2026-11-14 |
| AR-07 | T-01-42 | LAN fallback unchanged, password-gated; Phase 4 Share redesign owns it | plan 01-12 | 2026-11-15 |
| AR-08 | T-01-43 | Event-driven LocalAPI rechecks with 5 s timeout | plan 01-12 | 2026-11-15 |
| AR-09 | T-01-SC | No packages installed in this phase | all plans | 2026-11-15 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-11-15 | 45 | 45 | 0 | gsd-secure-phase (L1 grep, orchestrator) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-11-15
