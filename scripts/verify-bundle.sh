#!/usr/bin/env bash
# Bundle gate: proves a built Portty.app is Apple Silicon only with minimum macOS 14.
# Usage: scripts/verify-bundle.sh APP
# Checks the bundle plist (identifier and minimum system version), then enumerates every
# Mach-O under APP (find + file, never a path list) and requires exactly one arm64 slice and
# a minos of MIN_MACOS or later in each. Run by build.sh and by build.yml.
# Exit: 0 ok, 1 gate failed, 2 usage.
set -uo pipefail

BUNDLE_ID="app.portty.Portty"
MIN_MACOS="14.0"

usage() {
  echo "Usage: scripts/verify-bundle.sh APP" >&2
  exit 2
}

[[ $# -eq 1 ]] || usage
APP="${1%/}"
[[ -d "$APP" ]] || usage

FAILED=0
fail() {
  echo "FAIL: $1"
  FAILED=1
}

# --- Plist ---
PLIST="$APP/Contents/Info.plist"
plist_get() { plutil -extract "$1" raw -o - "$PLIST" 2>/dev/null; }

if [[ ! -f "$PLIST" ]]; then
  fail "plist Contents/Info.plist is missing"
  EXEC_NAME=""
else
  got="$(plist_get CFBundleIdentifier)"
  [[ "$got" == "$BUNDLE_ID" ]] || fail "plist CFBundleIdentifier is '$got', want $BUNDLE_ID"
  got="$(plist_get LSMinimumSystemVersion)"
  [[ "$got" == "$MIN_MACOS" ]] || fail "plist LSMinimumSystemVersion is '$got', want $MIN_MACOS"
  EXEC_NAME="$(plist_get CFBundleExecutable)"
fi

# --- Mach-O enumeration ---
COUNT=0
MAIN_SEEN=0
while IFS= read -r -d '' f; do
  case "$(file -b "$f")" in
    Mach-O*) ;;
    *) continue ;;
  esac
  rel="${f#"$APP"/}"
  COUNT=$((COUNT + 1))
  [[ -n "$EXEC_NAME" && "$rel" == "Contents/MacOS/$EXEC_NAME" ]] && MAIN_SEEN=1

  archs="$(lipo -archs "$f" 2>&1)"
  [[ "$archs" == "arm64" ]] || fail "arch $rel has '$archs', want arm64 only"

  minos="$(vtool -show-build "$f" 2>/dev/null | awk '/minos/{print $2; exit}')"
  if [[ -z "$minos" ]]; then
    fail "minos $rel has no minos (vtool -show-build)"
  elif [[ "$(printf '%s\n%s\n' "$MIN_MACOS" "$minos" | sort -V | head -1)" != "$MIN_MACOS" ]]; then
    fail "minos $rel is $minos, want $MIN_MACOS or later"
  fi
done < <(find "$APP" -type f -print0)

if [[ $COUNT -eq 0 ]]; then
  fail "no Mach-O files found under $APP"
elif [[ -n "$EXEC_NAME" && $MAIN_SEEN -eq 0 ]]; then
  fail "no Mach-O main executable Contents/MacOS/$EXEC_NAME"
fi

if [[ $FAILED -ne 0 ]]; then
  exit 1
fi
echo "OK: $COUNT Mach-O files, arm64 only, minos $MIN_MACOS or later"
