#!/usr/bin/env bash
# tests/verify-bundle.test.sh
# Fixture tests for scripts/verify-bundle.sh: tiny cc-built bundles, one case per gate.
# Needs cc, lipo, vtool and plutil (Xcode command line tools). Touches only a temp dir.
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
echo "================================================"
echo "Results: $PASS passed, $FAIL failed"
echo "================================================"

if [[ $FAIL -gt 0 ]]; then
  exit 1
fi
exit 0
