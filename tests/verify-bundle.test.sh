#!/usr/bin/env bash
# tests/verify-bundle.test.sh
# Fixture tests for scripts/verify-bundle.sh: tiny cc-built bundles, one case per gate.
# Also covers the --signed gate (real ad-hoc fixtures and a fake codesign on PATH) and
# scripts/sign-bundle.sh. Needs cc, lipo, vtool, plutil, codesign (Xcode command line tools) and
# jq. Touches only a temp dir; no keychain, no host signing identity.
#
# Run: bash tests/verify-bundle.test.sh    (from project root)

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GATE="$SCRIPT_DIR/../scripts/verify-bundle.sh"
PASS=0
FAIL=0

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

pass() {
  echo "  PASS: $1"
  PASS=$((PASS + 1))
}

fail() {
  echo "  FAIL: $1"
  [[ -n "${2:-}" ]] && echo "        $2"
  FAIL=$((FAIL + 1))
}

# build_macho FILE VERSION ARCH... compiles a one-line C main for the given archs and minos.
build_macho() {
  local file="$1" ver="$2"
  shift 2
  local archflags=()
  local a
  for a in "$@"; do archflags+=(-arch "$a"); done
  mkdir -p "$(dirname "$file")"
  # A file, not stdin: a multi-arch cc call reads the source once per arch.
  [[ -f "$WORK/main.c" ]] || echo 'int main(void){return 0;}' >"$WORK/main.c"
  cc "$WORK/main.c" "${archflags[@]}" "-mmacosx-version-min=$ver" -o "$file" || {
    echo "fixture build failed: $file" >&2
    exit 1
  }
}

# write_plist APP MINVER ID writes a minimal Contents/Info.plist naming executable Portty.
write_plist() {
  local plist="$1/Contents/Info.plist"
  mkdir -p "$1/Contents"
  plutil -create xml1 "$plist"
  plutil -insert CFBundleExecutable -string Portty "$plist"
  plutil -insert CFBundleIdentifier -string "$3" "$plist"
  plutil -insert LSMinimumSystemVersion -string "$2" "$plist"
}

# make_app NAME EXEC_VERSION PLIST_MIN BUNDLE_ID ARCH... builds $WORK/NAME/Portty.app and prints its path.
make_app() {
  local name="$1" ver="$2" plistmin="$3" id="$4"
  shift 4
  local app="$WORK/$name/Portty.app"
  build_macho "$app/Contents/MacOS/Portty" "$ver" "$@"
  write_plist "$app" "$plistmin" "$id"
  echo "$app"
}

# expect NAME WANT_EXIT WANT_TEXT_OR_EMPTY CMD... runs CMD and checks exit code and output text.
expect() {
  local name="$1" want_exit="$2" want_text="$3"
  shift 3
  local out rc
  out=$("$@" 2>&1)
  rc=$?
  if [[ $rc -ne $want_exit ]]; then
    fail "$name" "exit $rc, want $want_exit. output: $out"
  elif [[ -n "$want_text" ]] && ! grep -qF -- "$want_text" <<<"$out"; then
    fail "$name" "output lacks '$want_text'. output: $out"
  else
    pass "$name"
  fi
}

echo ""
echo "=== verify-bundle.sh fixture cases ==="

APP=$(make_app good 14.0 14.0 app.portty.Portty arm64)
expect "good bundle (arm64, minos 14.0) passes" 0 "OK: 1 Mach-O files" bash "$GATE" "$APP"

APP=$(make_app newer 15.0 14.0 app.portty.Portty arm64)
expect "minos 15.0 passes" 0 "OK:" bash "$GATE" "$APP"

APP=$(make_app lowminos 11.0 14.0 app.portty.Portty arm64)
expect "minos 11.0 fails with FAIL: minos" 1 "FAIL: minos" bash "$GATE" "$APP"

APP=$(make_app twoslice 14.0 14.0 app.portty.Portty arm64 x86_64)
expect "arm64+x86_64 fails with FAIL: arch" 1 "FAIL: arch" bash "$GATE" "$APP"

APP=$(make_app intel 14.0 14.0 app.portty.Portty x86_64)
expect "x86_64 only fails with FAIL: arch" 1 "FAIL: arch" bash "$GATE" "$APP"

APP=$(make_app helper 14.0 14.0 app.portty.Portty arm64)
build_macho "$APP/Contents/Helpers/helper" 11.0 arm64
expect "nested helper at minos 11.0 fails and names Contents/Helpers/helper" 1 \
  "FAIL: minos Contents/Helpers/helper" bash "$GATE" "$APP"

APP=$(make_app nomacho 14.0 14.0 app.portty.Portty arm64)
echo "not a binary" >"$APP/Contents/MacOS/Portty"
expect "text file as the only executable fails with FAIL: no Mach-O" 1 "FAIL: no Mach-O" bash "$GATE" "$APP"

APP=$(make_app oldplist 14.0 10.13.0 app.portty.Portty arm64)
expect "plist LSMinimumSystemVersion 10.13.0 fails with FAIL: plist" 1 "FAIL: plist" bash "$GATE" "$APP"

APP=$(make_app otherid 14.0 14.0 app.portty.Other arm64)
expect "plist CFBundleIdentifier app.portty.Other fails with FAIL: plist" 1 "FAIL: plist" bash "$GATE" "$APP"

expect "no argument exits 2" 2 "Usage:" bash "$GATE"
APP=$(make_app extraarg 14.0 14.0 app.portty.Portty arm64)
expect "unknown second argument exits 2" 2 "Usage:" bash "$GATE" "$APP" --bogus
expect "APP that is not a directory exits 2" 2 "Usage:" bash "$GATE" "$WORK/missing.app"

echo ""
echo "=== verify-bundle.sh --signed (real ad-hoc fixtures) ==="

APP=$(make_app unsigned 14.0 14.0 app.portty.Portty arm64)
expect "unsigned fixture (linker-signed only) fails --signed with FAIL: team" 1 "FAIL: team" bash "$GATE" "$APP" --signed

APP=$(make_app adhoc 14.0 14.0 app.portty.Portty arm64)
codesign --force --options runtime --sign - "$APP" >/dev/null 2>&1 || { echo "ad-hoc fixture sign failed" >&2; exit 1; }
ADHOC_OUT=$(bash "$GATE" "$APP" --signed 2>&1)
ADHOC_RC=$?
for tag in "FAIL: team" "FAIL: timestamp" "FAIL: designated requirement"; do
  if [[ $ADHOC_RC -eq 1 ]] && grep -qF -- "$tag" <<<"$ADHOC_OUT"; then
    pass "ad-hoc runtime fixture fails --signed with $tag"
  else
    fail "ad-hoc runtime fixture fails --signed with $tag" "exit $ADHOC_RC. output: $ADHOC_OUT"
  fi
done

echo ""
echo "=== verify-bundle.sh --signed (fake codesign) ==="

# A fake codesign answers the verify, -dvv, -d -r- and entitlements queries from FAKE_* variables
# and logs every call to $FAKE_LOG; signing calls (--force) just succeed. No keychain is touched.
# It is per file: when the queried path (the last argument) contains FAKE_BAD_PATH, the
# FAKE_BAD_VERIFY_RC, FAKE_BAD_FLAGS and FAKE_BAD_TIMESTAMP values replace the normal ones for that
# path only, so a bad nested Mach-O can sit next to a good main executable and bundle.
mkdir -p "$WORK/bin"
cat >"$WORK/bin/codesign" <<'FAKE'
#!/usr/bin/env bash
echo "$*" >>"${FAKE_LOG:-/dev/null}"
target="${*: -1}"
if [[ -n "${FAKE_BAD_PATH:-}" && "$target" == *"$FAKE_BAD_PATH"* ]]; then
  FAKE_VERIFY_RC="${FAKE_BAD_VERIFY_RC:-${FAKE_VERIFY_RC:-0}}"
  FAKE_FLAGS="${FAKE_BAD_FLAGS:-${FAKE_FLAGS:-0x10000(runtime)}}"
  FAKE_TIMESTAMP="${FAKE_BAD_TIMESTAMP:-${FAKE_TIMESTAMP:-1}}"
fi
case " $* " in
  *" --verify "*) exit "${FAKE_VERIFY_RC:-0}" ;;
  *" --entitlements - --xml "*) cat "$FAKE_ENT" ;;
  *" -dvv "*)
    echo "Identifier=${FAKE_ID:-app.portty.Portty}" >&2
    echo "CodeDirectory v=20500 size=1 flags=${FAKE_FLAGS:-0x10000(runtime)} hashes=1+7 location=embedded" >&2
    [[ "${FAKE_TIMESTAMP:-1}" == 1 ]] && echo "Timestamp=Nov 25, 2026 at 1:00:00 AM" >&2
    echo "TeamIdentifier=${FAKE_TEAM:-S2K7P43927}" >&2
    ;;
  *" -r- "*)
    echo "Executable=/fake/Portty.app/Contents/MacOS/Portty" >&2
    echo "$FAKE_DR" >&2
    ;;
  *" --force "*) exit 0 ;;
esac
FAKE
chmod +x "$WORK/bin/codesign"

# Fake xcrun (stapler validate exits FAKE_STAPLE_RC) and spctl (prints a Gatekeeper verdict built
# from FAKE_GK_VERDICT and FAKE_GK_SOURCE) for the --notarized branch.
cat >"$WORK/bin/xcrun" <<'FAKE'
#!/usr/bin/env bash
echo "xcrun $*" >>"${FAKE_LOG:-/dev/null}"
[[ "$1 $2" == "stapler validate" ]] && exit "${FAKE_STAPLE_RC:-0}"
exit 0
FAKE
cat >"$WORK/bin/spctl" <<'FAKE'
#!/usr/bin/env bash
echo "spctl $*" >>"${FAKE_LOG:-/dev/null}"
echo "${*: -1}: ${FAKE_GK_VERDICT:-accepted}" >&2
echo "source=${FAKE_GK_SOURCE:-Notarized Developer ID}" >&2
echo "origin=Developer ID Application: Test (S2K7P43927)" >&2
[[ "${FAKE_GK_VERDICT:-accepted}" == accepted ]]
FAKE
chmod +x "$WORK/bin/xcrun" "$WORK/bin/spctl"

# A PATH with every tool the gate needs except jq (macOS ships /usr/bin/jq, so jq cannot be
# hidden by trimming PATH; this links each needed tool into its own directory instead).
mkdir -p "$WORK/nojq"
for t in bash env plutil file lipo vtool awk sort head find grep cat dirname tr wc cut xcrun xcode-select; do
  p="$(command -v "$t")" && ln -sf "$p" "$WORK/nojq/$t"
done

ENT="$(cd "$SCRIPT_DIR/.." && pwd)/build/entitlements.plist"
V12_DR='designated => identifier "app.portty.Portty" and anchor apple generic and certificate 1[field.1.2.840.113635.100.6.2.6] /* exists */ and certificate leaf[field.1.2.840.113635.100.6.1.13] /* exists */ and certificate leaf[subject.OU] = S2K7P43927'
# (plutil key paths split on dots, so the variants are written from JSON.)
echo '{"com.apple.security.network.client":true,"com.apple.security.network.server":true,"com.apple.security.get-task-allow":true}' |
  plutil -convert xml1 -o "$WORK/ent-taskallow.plist" -
echo '{"com.apple.security.network.client":true}' | plutil -convert xml1 -o "$WORK/ent-noserver.plist" -

# fake_case NAME WANT_TAG [VAR=value ...] runs the gate (CASE_MODE, default --signed) on a good
# fixture with a nested helper at Contents/Helpers/helper, with the fakes on PATH (CASE_PATH
# replaces the real PATH behind them). WANT_TAG empty: expect exit 0. Otherwise exit 1 with at
# least one FAIL line, every FAIL line carrying exactly WANT_TAG (no unrelated tags).
fake_case() {
  local name="$1" tag="$2"
  shift 2
  local app out rc total tagged
  app=$(make_app "fake$PASS$FAIL" 14.0 14.0 app.portty.Portty arm64)
  build_macho "$app/Contents/Helpers/helper" 14.0 arm64
  out=$(env PATH="$WORK/bin:${CASE_PATH:-$PATH}" FAKE_DR="$V12_DR" FAKE_ENT="$ENT" "$@" \
    /bin/bash "$GATE" "$app" "${CASE_MODE:---signed}" 2>&1)
  rc=$?
  if [[ -z "$tag" ]]; then
    if [[ $rc -eq 0 && "$out" == OK:* ]]; then pass "$name"; else fail "$name" "exit $rc. output: $out"; fi
    return
  fi
  total=$(grep -c '^FAIL:' <<<"$out")
  tagged=$(grep -c "^FAIL: $tag" <<<"$out")
  if [[ $rc -eq 1 && $total -ge 1 && $total -eq $tagged ]]; then
    pass "$name"
  else
    fail "$name" "exit $rc, want only 'FAIL: $tag'. output: $out"
  fi
}

fake_case "Developer ID shape (team, runtime, timestamp, v1.2 designated line, real entitlements) passes" ""
fake_case "designated line with identifier app.portty.Other fails FAIL: designated requirement" "designated requirement" \
  FAKE_DR="${V12_DR//identifier \"app.portty.Portty\"/identifier \"app.portty.Other\"}"
fake_case "designated line with subject.OU ABCDE12345 fails FAIL: designated requirement" "designated requirement" \
  FAKE_DR="${V12_DR//= S2K7P43927/= ABCDE12345}"
fake_case "cdhash designated line fails FAIL: designated requirement" "designated requirement" \
  FAKE_DR='designated => cdhash H"0123456789abcdef0123456789abcdef01234567"'
fake_case "TeamIdentifier ABCDE12345 fails FAIL: team" "team" FAKE_TEAM=ABCDE12345
fake_case "flags=0x0(none) fails FAIL: runtime" "runtime" FAKE_FLAGS='0x0(none)'
fake_case "no Timestamp= line fails FAIL: timestamp" "timestamp" FAKE_TIMESTAMP=0
fake_case "entitlements plus com.apple.security.get-task-allow fail FAIL: entitlements" "entitlements" FAKE_ENT="$WORK/ent-taskallow.plist"
fake_case "entitlements without network.server fail FAIL: entitlements" "entitlements" FAKE_ENT="$WORK/ent-noserver.plist"
fake_case "bundle identifier app.portty.Other fails FAIL: identifier" "identifier" FAKE_ID=app.portty.Other
fake_case "codesign --verify exiting 1 fails FAIL: unsigned" "unsigned" FAKE_VERIFY_RC=1

# Per-file: only the nested helper is bad; the main executable and bundle stay good, so the
# per-Mach-O loop is the only thing that can catch it (WR-02).
fake_case "unsigned nested helper fails FAIL: unsigned Contents/Helpers/helper only" \
  "unsigned Contents/Helpers/helper" FAKE_BAD_PATH=Contents/Helpers/helper FAKE_BAD_VERIFY_RC=1
fake_case "nested helper without the runtime flag fails FAIL: runtime Contents/Helpers/helper only" \
  "runtime Contents/Helpers/helper" FAKE_BAD_PATH=Contents/Helpers/helper FAKE_BAD_FLAGS='0x0(none)'
fake_case "nested helper without a timestamp fails FAIL: timestamp Contents/Helpers/helper only" \
  "timestamp Contents/Helpers/helper" FAKE_BAD_PATH=Contents/Helpers/helper FAKE_BAD_TIMESTAMP=0
fake_case "bad main executable fails FAIL: runtime Contents/MacOS/Portty only" \
  "runtime Contents/MacOS/Portty" FAKE_BAD_PATH=Contents/MacOS/Portty FAKE_BAD_FLAGS='0x0(none)'

# WR-01: no jq on PATH must fail closed, not compare two identical error strings.
CASE_PATH="$WORK/nojq" fake_case "jq missing from PATH fails FAIL: entitlements gate needs jq" \
  "entitlements gate needs jq"

echo ""
echo "=== verify-bundle.sh --notarized (fake codesign, xcrun, spctl) ==="

CASE_MODE=--notarized fake_case "stapled and accepted as Notarized Developer ID passes --notarized" ""
CASE_MODE=--notarized fake_case "no stapled ticket fails FAIL: notarization has no valid stapled ticket" \
  "notarization has no valid stapled ticket" FAKE_STAPLE_RC=65
CASE_MODE=--notarized fake_case "Gatekeeper source=Unnotarized Developer ID fails FAIL: notarization Gatekeeper" \
  "notarization Gatekeeper" FAKE_GK_VERDICT=rejected FAKE_GK_SOURCE='Unnotarized Developer ID'
CASE_MODE=--notarized fake_case "accepted but source=Developer ID (not notarized) fails FAIL: notarization Gatekeeper" \
  "notarization Gatekeeper" FAKE_GK_SOURCE='Developer ID'
CASE_MODE=--notarized fake_case "--notarized still runs the signature gates (bad team fails FAIL: team)" \
  "team" FAKE_TEAM=ABCDE12345

echo ""
echo "=== sign-bundle.sh (fake codesign) ==="

SIGNER="$SCRIPT_DIR/../scripts/sign-bundle.sh"

APP=$(make_app order 14.0 14.0 app.portty.Portty arm64)
build_macho "$APP/Contents/Helpers/tool" 14.0 arm64
build_macho "$APP/Contents/Frameworks/Lib.framework/Versions/A/Lib" 14.0 arm64
mkdir -p "$APP/Contents/Resources"
echo "just text" >"$APP/Contents/Resources/readme.txt"
LOG="$WORK/order.log"
: >"$LOG"
env PATH="$WORK/bin:$PATH" FAKE_LOG="$LOG" SIGN_IDENTITY="Test Identity" bash "$SIGNER" "$APP" >/dev/null 2>&1
RC=$?
LINES=()
while IFS= read -r l; do LINES+=("$l"); done <"$LOG"
if [[ $RC -eq 0 && ${#LINES[@]} -eq 4 &&
  "${LINES[0]}" == *"$APP/Contents/Frameworks/Lib.framework/Versions/A/Lib" &&
  "${LINES[1]}" == *"$APP/Contents/Helpers/tool" &&
  "${LINES[2]}" == *"$APP/Contents/MacOS/Portty" &&
  "${LINES[3]}" == *"$APP" ]]; then
  pass "signs Lib, tool, main executable, then the bundle (four calls, inside out)"
else
  fail "signs Lib, tool, main executable, then the bundle" "rc $RC, log: $(cat "$LOG")"
fi

ALL_FLAGS=1
for l in "${LINES[@]}"; do
  [[ "$l" == *"--options runtime"* && "$l" == *"--timestamp"* && "$l" == *"--sign Test Identity"* ]] || ALL_FLAGS=0
done
[[ $ALL_FLAGS -eq 1 ]] && pass "every call carries --options runtime, --timestamp and the identity" ||
  fail "every call carries --options runtime, --timestamp and the identity" "log: $(cat "$LOG")"

if [[ "${LINES[2]}" == *"--identifier app.portty.Portty --entitlements $ENT "* &&
  "${LINES[3]}" == *"--identifier app.portty.Portty --entitlements $ENT "* &&
  "${LINES[0]}" != *"--identifier"* && "${LINES[0]}" != *"--entitlements"* &&
  "${LINES[1]}" != *"--identifier"* && "${LINES[1]}" != *"--entitlements"* ]]; then
  pass "main executable and bundle carry the identifier and entitlements; nested code carries neither"
else
  fail "identifier and entitlements flags only on the main executable and bundle" "log: $(cat "$LOG")"
fi

if grep -qF -- '--deep' "$LOG" || grep -qF 'readme.txt' "$LOG"; then
  fail "no deep flag and the text resource is never signed" "log: $(cat "$LOG")"
else
  pass "no deep flag and the text resource is never signed"
fi

APP=$(make_app noident 14.0 14.0 app.portty.Portty arm64)
LOG="$WORK/noident.log"
: >"$LOG"
OUT=$(env PATH="$WORK/bin:$PATH" FAKE_LOG="$LOG" SIGN_IDENTITY= bash "$SIGNER" "$APP" 2>&1)
RC=$?
if [[ $RC -ne 0 && "$OUT" == *SIGN_IDENTITY* && ! -s "$LOG" ]]; then
  pass "SIGN_IDENTITY unset exits non-zero naming SIGN_IDENTITY and signs nothing"
else
  fail "SIGN_IDENTITY unset exits non-zero naming SIGN_IDENTITY and signs nothing" "rc $RC, out: $OUT, log: $(cat "$LOG")"
fi

APP=$(make_app wrongid 14.0 14.0 app.portty.Other arm64)
LOG="$WORK/wrongid.log"
: >"$LOG"
OUT=$(env PATH="$WORK/bin:$PATH" FAKE_LOG="$LOG" SIGN_IDENTITY="Test Identity" bash "$SIGNER" "$APP" 2>&1)
RC=$?
if [[ $RC -ne 0 && ! -s "$LOG" ]]; then
  pass "CFBundleIdentifier app.portty.Other exits non-zero and signs nothing"
else
  fail "CFBundleIdentifier app.portty.Other exits non-zero and signs nothing" "rc $RC, out: $OUT, log: $(cat "$LOG")"
fi

for key in CFBundleIdentifier CFBundleExecutable; do
  APP=$(make_app "no$key" 14.0 14.0 app.portty.Portty arm64)
  plutil -remove "$key" "$APP/Contents/Info.plist"
  LOG="$WORK/no$key.log"
  : >"$LOG"
  OUT=$(env PATH="$WORK/bin:$PATH" FAKE_LOG="$LOG" SIGN_IDENTITY="Test Identity" bash "$SIGNER" "$APP" 2>&1)
  RC=$?
  if [[ $RC -eq 1 && "$OUT" == *"ERROR: $key"* && ! -s "$LOG" ]]; then
    pass "plist without $key exits 1 with a clear ERROR naming $key and signs nothing (WR-03)"
  else
    fail "plist without $key exits 1 with a clear ERROR naming $key and signs nothing (WR-03)" "rc $RC, out: $OUT, log: $(cat "$LOG")"
  fi
done

GOT_ENT=$(plutil -convert json -o - "$ENT" | jq -S -c .)
if [[ "$GOT_ENT" == '{"com.apple.security.network.client":true,"com.apple.security.network.server":true}' ]]; then
  pass "build/entitlements.plist is exactly network.client and network.server"
else
  fail "build/entitlements.plist is exactly network.client and network.server" "got: $GOT_ENT"
fi

echo ""
echo "================================================"
echo "Results: $PASS passed, $FAIL failed"
echo "================================================"

if [[ $FAIL -gt 0 ]]; then
  exit 1
fi
exit 0
