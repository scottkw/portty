#!/usr/bin/env bash
# Bundle gate: proves a built Portty.app is Apple Silicon only with minimum macOS 14.
# Usage: scripts/verify-bundle.sh APP [--signed | --notarized]
# Checks the bundle plist (identifier and minimum system version), then enumerates every
# Mach-O under APP (find + file, never a path list) and requires exactly one arm64 slice and
# a minos of MIN_MACOS or later in each. --signed adds the signature gates (every Mach-O: team,
# hardened runtime, secure timestamp; the bundle: identifier, designated requirement pinned to
# v1.2.0's, entitlements equal to build/entitlements.plist, overridable with ENTITLEMENTS).
# --notarized adds the stapled ticket and Gatekeeper check. Run by build.sh, build.yml and
# release.yml.
# Exit: 0 ok, 1 gate failed, 2 usage.
set -uo pipefail

BUNDLE_ID="app.portty.Portty"
MIN_MACOS="14.0"
TEAM_ID="S2K7P43927" # equals updater.TeamID
# v1.2.0's designated requirement, byte for byte. A changed one silently drops the office Mac's
# Screen Recording and Accessibility grants, so never edit this to make a failure go away.
DESIGNATED_REQUIREMENT='designated => identifier "app.portty.Portty" and anchor apple generic and certificate 1[field.1.2.840.113635.100.6.2.6] /* exists */ and certificate leaf[field.1.2.840.113635.100.6.1.13] /* exists */ and certificate leaf[subject.OU] = S2K7P43927'
ENTITLEMENTS="${ENTITLEMENTS:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/build/entitlements.plist}"

usage() {
  echo "Usage: scripts/verify-bundle.sh APP [--signed | --notarized]" >&2
  exit 2
}

[[ $# -ge 1 && $# -le 2 ]] || usage
APP="${1%/}"
[[ -d "$APP" ]] || usage
MODE="${2:-}"
case "$MODE" in
  "" | --signed | --notarized) ;;
  *) usage ;;
esac

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

  if [[ -n "$MODE" ]]; then
    codesign --verify --strict "$f" >/dev/null 2>&1 || fail "unsigned $rel (codesign --verify --strict)"
    info="$(codesign -dvv "$f" 2>&1)"
    grep -qxF "TeamIdentifier=$TEAM_ID" <<<"$info" || fail "team $rel is not $TEAM_ID"
    grep -Eq '^CodeDirectory .*flags=0x[0-9a-f]+\([^)]*runtime' <<<"$info" || fail "runtime $rel lacks the hardened runtime flag"
    grep -q '^Timestamp=' <<<"$info" || fail "timestamp $rel has no secure timestamp"
  fi
done < <(find "$APP" -type f -print0)

if [[ $COUNT -eq 0 ]]; then
  fail "no Mach-O files found under $APP"
elif [[ -n "$EXEC_NAME" && $MAIN_SEEN -eq 0 ]]; then
  fail "no Mach-O main executable Contents/MacOS/$EXEC_NAME"
fi

if [[ -n "$MODE" ]]; then
  # Deep verification here is a check only; signing never uses it (scripts/sign-bundle.sh).
  codesign --verify --deep --strict --verbose=2 "$APP" >/dev/null 2>&1 || fail "unsigned bundle (codesign --verify --deep --strict)"
  info="$(codesign -dvv "$APP" 2>&1)"
  grep -qxF "Identifier=$BUNDLE_ID" <<<"$info" || fail "identifier of the bundle is not $BUNDLE_ID"
  dr="$(codesign -d -r- "$APP" 2>&1 | grep '^designated => ')"
  [[ "$dr" == "$DESIGNATED_REQUIREMENT" ]] || fail "designated requirement differs from v1.2's: $dr"
  norm() { plutil -convert json -o - - | jq -S .; }
  want="$(norm <"$ENTITLEMENTS" 2>&1)"
  got="$(codesign -d --entitlements - --xml "$APP" 2>/dev/null | norm 2>&1)"
  [[ -n "$want" && "$got" == "$want" ]] || fail "entitlements of the bundle differ from $ENTITLEMENTS"
fi

if [[ "$MODE" == "--notarized" ]]; then
  if ! xcrun stapler validate "$APP" >/dev/null 2>&1; then
    fail "notarization has no valid stapled ticket (xcrun stapler validate)"
  else
    gk="$(spctl -a -t exec -vv "$APP" 2>&1)"
    { grep -q 'accepted' <<<"$gk" && grep -qxF 'source=Notarized Developer ID' <<<"$gk"; } ||
      fail "notarization Gatekeeper did not accept as Notarized Developer ID"
  fi
fi

if [[ $FAILED -ne 0 ]]; then
  exit 1
fi
echo "OK: $COUNT Mach-O files, arm64 only, minos $MIN_MACOS or later${MODE:+ ($MODE)}"
