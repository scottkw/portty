---
phase: quick-261123-kcr
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - cmd_share.go
  - cmd_share_test.go
  - TESTING.md
autonomous: true
requirements:
  - AUDIT-v1.2-W-2

estimate:
  tokens: 25000
  raw_tokens: 25000
  tasks: 2
  confidence: low

must_haves:
  truths:
    - "`portty unshare <arg>` where arg is a unique ID prefix shorter than 8 characters (and not an exact name or exact ID) prints the standard no-session error, and the session stays shared"
    - "`portty unshare` with an 8-character ID prefix, an exact name, or an exact ID still stops sharing"
    - "`portty share` and `portty attach` still resolve a short unique ID prefix (only the destructive unshare tightens)"
    - "unshare reuses the existing lookupSessionStrict helper; no second strict helper exists"
  artifacts:
    - path: cmd_share.go
      provides: "cmdUnshare resolving its argument through lookupSessionStrict"
      contains: "lookupSessionStrict"
    - path: cmd_share_test.go
      provides: "TestCmdUnshare_ShortPrefixRefused"
      contains: "func TestCmdUnshare_ShortPrefixRefused"
    - path: TESTING.md
      provides: "Section 2 note recording the in-place test edit"
      contains: "TestCmdUnshare_ShortPrefixRefused"
  key_links:
    - from: cmd_share.go
      to: cmd_share.go
      via: "cmdUnshare calls lookupSessionStrict, which enforces minDestructivePrefix"
      pattern: "lookupSessionStrict\\(client, args\\[0\\]\\)"
    - from: main.go
      to: cmd_share.go
      via: "case \"unshare\" dispatches to cmdUnshare (unchanged)"
      pattern: "cmdUnshare\\(client, cmdArgs"
---

<objective>
Make `portty unshare` require an ID prefix of at least 8 characters, like `kill` and `rename`, by switching cmdUnshare to the existing lookupSessionStrict helper in cmd_share.go, and pin it with a test.

Purpose: closes v1.2 milestone audit Integration W-2 (.planning/v1.2-MILESTONE-AUDIT.md). Unshare revokes the share link and disconnects remote viewers, so it is destructive in the same way kill and rename are. A short name abbreviation like "b" must not fall through to a hex ID prefix and stop sharing an unrelated session (for example, the one the author is watching from home). Per the "rock-solid" project priority, a mis-targeted unshare on the office Mac cuts off the remote viewer, and nobody is at the office Mac to turn sharing back on.

Output: a one-call change in cmdUnshare, updated doc comments, a new test function in the existing cmd_share_test.go, and a TESTING.md Section 2 note.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@CLAUDE.md
@cmd_share.go
@cmd_share_test.go

<interfaces>
<!-- Extracted from the codebase. The executor does not need to explore further. -->

cmd_share.go (package main):
- `func resolveSession(sessions []daemon.SessionInfo, arg string) (daemon.SessionInfo, error)`: exact name, then exact ID, then unique ID prefix.
- `type noSessionError struct{ arg string }`. Error() is `portty: no session named %q. Run 'portty list' to see your sessions.`
- `func lookupSession(client *daemon.DaemonClient, arg string) (daemon.SessionInfo, error)`: permissive; used by share and attach. Leave it unchanged.
- `const minDestructivePrefix = 8`, at lines 70-73. Its doc comment currently says only kill and rename use it.
- `func lookupSessionStrict(client *daemon.DaemonClient, arg string) (daemon.SessionInfo, error)`, at lines 75-86. It returns noSessionError{arg} when the match came from an ID prefix and `len(arg) < minDestructivePrefix`.
- `func cmdShare(...)`, at line 91. It uses the permissive helper and must stay that way.
- `func cmdUnshare(client *daemon.DaemonClient, args []string, out io.Writer) error`, at lines 124-149. Line 131 currently calls the permissive helper. The function then checks `s.WebEnabled`, prints `"%q isn't shared.\n"` when it is false, and otherwise calls `client.SetSessionShared(s.ID, false)` and prints `Stopped sharing %q. Remote viewers were disconnected.\n`.

cmd_cli.go: cmdKill (line 320) and cmdRename (line 337) already call `lookupSessionStrict(client, args[0])`. That is the pattern to copy.

cmd_share_test.go test helpers:
- `testShareSetup(t) (*daemon.API, *daemon.DaemonClient)`: a real daemon on a loopback share server, so share and unshare actually run.
- `client.CreateSession(name, "/tmp", 0, 0) (string, error)` returns the session ID.
- `client.ListSessions() ([]daemon.SessionInfo, error)`. SessionInfo has ID, Name and WebEnabled.
- `cmdShare(client, []string{name}, &bytes.Buffer{})` turns sharing on.
- Imports already present: bytes, fmt, strings, testing, daemon. The `mustCreate` and `sessionNames` helpers live in cmd_cli_test.go (same package main), so they can be reused.

Analog test to mirror: `TestCmdKillRename_ShortPrefixRefused` in cmd_cli_test.go (lines 262-282). It builds wantNo with `fmt.Sprintf` from the noSessionError copy, using `id[:1]`, compares the exact error string, then checks that state is unchanged.
</interfaces>
</context>

<tasks>

<task type="tracer" tdd="true">
  <name>Task 1: unshare refuses a short ID prefix end-to-end (CLI to daemon), with a test</name>
  <files>cmd_share.go, cmd_share_test.go</files>
  <behavior>
    - Setup: testShareSetup, create session "build", capture its id, and share it with cmdShare. Guard: t.Fatalf unless len(id) > minDestructivePrefix, so the boundary cases mean something.
    - cmdUnshare with id[:1] returns an error whose text equals the noSessionError copy for id[:1] exactly. Nothing is written to out. ListSessions still shows "build" with WebEnabled true.
    - cmdUnshare with id[:minDestructivePrefix-1] (7 characters, the boundary) gives the same refusal (no-session copy for that arg), and the session stays shared.
    - lookupSession (the share/attach path) with id[:1] still resolves to id. Share stays permissive.
    - cmdUnshare with id[:minDestructivePrefix] (8 characters) succeeds and prints exactly `Stopped sharing "build". Remote viewers were disconnected.` followed by a newline, and afterwards ListSessions shows WebEnabled false.
    - The existing TestCmdUnshare (unshare by exact name) and TestCmdKillRename_ShortPrefixRefused keep passing without edits.
  </behavior>
  <action>
RED: add `TestCmdUnshare_ShortPrefixRefused` to cmd_share_test.go, directly after the existing TestCmdUnshare. Give it a doc comment in the style of TestCmdKillRename_ShortPrefixRefused: it pins that unshare, which revokes the link and disconnects viewers, needs an 8-character ID prefix like kill and rename (audit W-2), while share and attach still accept a short one. Implement the cases in <behavior>. Build the expected errors with fmt.Sprintf from the same no-session copy the kill/rename test uses. Use minDestructivePrefix for the boundary slices rather than the literals 7 and 8. Check WebEnabled by looping over client.ListSessions() and finding the session by ID. Run the verify command and confirm the new test FAILS, because the short prefix currently unshares the session. Commit it as `test(quick-261123-kcr): add failing test for unshare short-prefix refusal`.

GREEN: in cmdUnshare (cmd_share.go), replace the permissive lookupSession call on args[0] with `lookupSessionStrict(client, args[0])`. This reuses the existing helper exactly as cmdKill and cmdRename do. Do not add a new helper, and do not touch cmdShare, lookupSession or resolveSession: share and attach are not destructive, and TestCmdKillRename_ShortPrefixRefused pins that the permissive path still resolves short prefixes.

Update the doc comments so they stay true. On minDestructivePrefix, the first sentence should say it is the shortest ID prefix that kill, rename and unshare accept. Keep the rest of that comment. On cmdUnshare, add one sentence after the existing first line saying the argument resolves like kill (exact name, exact ID, unique ID prefix of 8+ characters), so an unknown or short argument unshares nothing. Leave the usage text in cmd_cli.go alone: the kill and rename usage lines do not mention the 8-character rule either.

Run gofmt on both files. Commit as `fix(quick-261123-kcr): unshare requires an 8-character ID prefix like kill/rename`.
  </action>
  <verify>
    <automated>GOTOOLCHAIN=go1.26.11 go test -race -short -count=1 -run 'TestCmdUnshare|TestCmdKillRename_ShortPrefixRefused|TestResolveSession|TestCmdShare' . && test "$(awk '/^func cmdUnshare/,/^}/' cmd_share.go | grep -c 'lookupSessionStrict(')" -ge 1 && test "$(awk '/^func cmdUnshare/,/^}/' cmd_share.go | grep -c 'lookupSession(client')" -eq 0 && test "$(awk '/^func cmdShare/,/^}/' cmd_share.go | grep -c 'lookupSession(client')" -ge 1 && test "$(grep -c '^func TestCmdUnshare_ShortPrefixRefused' cmd_share_test.go)" -ge 1 && test "$(grep -c '^func ' cmd_share.go)" -le 6 && test -z "$(gofmt -l cmd_share.go cmd_share_test.go)"</automated>
    <!-- Gate baselines, one per count gate, in order:
         cmdUnshare strict call: [HEAD=0 EXPECT>=1 @d19f64f0]
         cmdUnshare permissive call: [HEAD=1 EXPECT==0 @d19f64f0]
         cmdShare permissive call: [HEAD=1 EXPECT>=1 @d19f64f0 GUARD MUTATION: switching cmdShare to lookupSessionStrict makes it 0]
         new test func: [HEAD=0 EXPECT>=1 @d19f64f0]
         func count in cmd_share.go: [HEAD=6 EXPECT<=6 @d19f64f0 GUARD MUTATION: adding any new helper func to cmd_share.go instead of reusing lookupSessionStrict makes it 7]
         gofmt -l prints nothing at HEAD and must print nothing after the change. The -run command passed at HEAD in 1.3 s. -->
  </verify>
  <done>The new test failed before the fix and passes after it. cmdUnshare calls lookupSessionStrict. cmdShare still calls the permissive lookupSession. cmd_share.go still has 6 top-level funcs (no new helper). The doc comments name unshare. Both files are gofmt-clean. Two commits (test, then fix).</done>
</task>

<task type="auto">
  <name>Task 2: Record the in-place test edit in TESTING.md and run the root package suite</name>
  <files>TESTING.md</files>
  <action>
Per the repo CLAUDE.md regression-test convention, and following the precedent of earlier quick tasks (for example "Quick 261116-c9f" at TESTING.md Section 2), add one note paragraph to TESTING.md Section 2. Insert it immediately above the existing "> Note: Portty Phase 16 (v1.2 cleanup, ..." paragraph and below the "> **Toolchain:**" paragraph; notes run newest first. Keep one blank line between paragraphs.

Wording: `> Note: Quick 261123-kcr (unshare 8-character ID prefix minimum, v1.2 audit Integration W-2, no new requirement IDs): counts from `git ls-files` unchanged — Go **174**, vitest **122**, Playwright **10**, build-script **1**, total **307** (no new test files). Edited in place: cmd_share_test.go (`TestCmdUnshare_ShortPrefixRefused`: unshare refuses an ID prefix under 8 characters (1 and 7) with the no-session copy and leaves the session shared; an 8-character prefix stops sharing; share/attach still resolve the short prefix).`

Do not change the Suite Manifest counts: no test file was added, and `git ls-files '*_test.go' | wc -l` is 174 at HEAD. Do not add a Section 4 row: the path column is per file, and cmd_share_test.go already has rows. Do not add a Section 5 manual item: the behavior is fully automated.

Then run the full root package suite and the traceability path check, both of which must pass. Commit as `docs(quick-261123-kcr): record unshare short-prefix test in TESTING.md`.
  </action>
  <verify>
    <automated>test "$(grep -c 'TestCmdUnshare_ShortPrefixRefused' TESTING.md)" -ge 1 && files=$(git ls-files -- '*_test.go') && test "$(echo "$files" | grep -c .)" -le 174 && bash tests/check-traceability-paths.sh && GOTOOLCHAIN=go1.26.11 go test -race -short -count=1 .</automated>
    <!-- Gate baselines, one per count gate, in order:
         TESTING.md mentions of the new test: [HEAD=0 EXPECT>=1 @d19f64f0]
         tracked Go test files: [HEAD=174 EXPECT<=174 @d19f64f0 GUARD MUTATION: adding a new *_test.go file makes it 175, which would then need a Suite Manifest count update and a Section 4 row]
         The traceability check printed "OK: 228 traceability paths exist" at HEAD. The root package go test passed at HEAD in about 52 s. -->
  </verify>
  <done>TESTING.md Section 2 has the Quick 261123-kcr note above the Phase 16 note. Counts are unchanged. The traceability check prints OK. The full root package suite passes with -race.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| CLI argument to daemon (local Unix socket, same user) | The user's typed session argument chooses which session's share is revoked. The daemon trusts the CLI to have resolved the right session ID. |
| Daemon to remote viewers (tailnet) | Unshare revokes grants and disconnects viewers on other devices. A wrong target cuts off a viewer who cannot turn sharing back on remotely. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-261123-kcr-01 | D (Denial of Service) | cmdUnshare argument resolution (cmd_share.go) | medium | mitigate | A short name abbreviation that happens to be a unique hex ID prefix could revoke an unrelated session's share and disconnect its remote viewers. cmdUnshare now resolves through lookupSessionStrict (minDestructivePrefix = 8). TestCmdUnshare_ShortPrefixRefused pins refusal at 1 and 7 characters, acceptance at 8, and that the session stays shared after a refusal. |
| T-261123-kcr-02 | T (Tampering) | Share permissiveness (cmdShare, lookupSession) | low | accept | Share keeps accepting short prefixes. Turning a share on is not destructive, and the share warning plus the token- and tailnet-gated link are unchanged. The test asserts lookupSession still resolves a short prefix, so this choice is deliberate and pinned. |
| T-261123-kcr-SC | Tampering | npm/pip/cargo installs | high | accept | No package installs in this plan (no go.mod, package.json or lockfile changes). The package-legitimacy gate does not apply. |
</threat_model>

<verification>
- `GOTOOLCHAIN=go1.26.11 go test -race -short -count=1 .` passes (root package, includes cmd_share_test.go and cmd_cli_test.go).
- `bash tests/check-traceability-paths.sh` prints OK.
- `awk '/^func cmdUnshare/,/^}/' cmd_share.go | grep -c 'lookupSessionStrict('` equals 1.
- `git log --oneline -3` shows the test commit before the fix commit (RED then GREEN), then the docs commit.
</verification>

<success_criteria>
- `portty unshare <1..7-char unique ID prefix>` returns `portty: no session named "<arg>". Run 'portty list' to see your sessions.` and leaves the share on.
- `portty unshare <8-char prefix | exact name | exact ID>` stops sharing as before.
- `portty share` and `portty attach` behavior is unchanged.
- No new helper. lookupSessionStrict is reused.
- TESTING.md records the in-place edit; counts are unchanged.
</success_criteria>

<output>
Create `.planning/quick/261123-kcr-unshare-requires-an-8-character-minimum-/261123-kcr-SUMMARY.md` when done
</output>
