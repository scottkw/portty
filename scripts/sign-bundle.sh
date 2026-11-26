#!/usr/bin/env bash
# Inside-out signer: signs every Mach-O in a Portty.app, then the bundle itself.
# Usage: SIGN_IDENTITY=<identity> [ENTITLEMENTS=<plist>] scripts/sign-bundle.sh APP
# Called by build.sh (--sign) and release.yml. Never signs with the deep option: one codesign
# call per item, nested code first, so every Mach-O carries its own hardened-runtime signature
# and timestamp. Verify the result with scripts/verify-bundle.sh APP --signed.
# Exit: 0 signed, 1 refused or codesign failed, 2 usage or SIGN_IDENTITY missing.
set -euo pipefail

BUNDLE_ID="app.portty.Portty"

if [[ $# -ne 1 || ! -d "${1%/}" ]]; then
  echo "Usage: SIGN_IDENTITY=<identity> [ENTITLEMENTS=<plist>] scripts/sign-bundle.sh APP" >&2
  exit 2
fi
APP="${1%/}"

if [[ -z "${SIGN_IDENTITY:-}" ]]; then
  echo "ERROR: SIGN_IDENTITY is not set (the codesigning identity to sign with)" >&2
  exit 2
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENTITLEMENTS="${ENTITLEMENTS:-$ROOT/build/entitlements.plist}"
if [[ ! -f "$ENTITLEMENTS" ]]; then
  echo "ERROR: entitlements file not found: $ENTITLEMENTS" >&2
  exit 1
fi

PLIST="$APP/Contents/Info.plist"
plist_get() { plutil -extract "$1" raw -o - "$PLIST" 2>/dev/null; }
[[ -f "$PLIST" ]] || { echo "ERROR: $PLIST is missing" >&2; exit 1; }
GOT_ID="$(plist_get CFBundleIdentifier)"
if [[ "$GOT_ID" != "$BUNDLE_ID" ]]; then
  echo "ERROR: CFBundleIdentifier is '$GOT_ID', want $BUNDLE_ID; nothing signed" >&2
  exit 1
fi
EXEC_NAME="$(plist_get CFBundleExecutable)"
MAIN="Contents/MacOS/$EXEC_NAME"

# Enumerate Mach-O files the way verify-bundle.sh does (find + file, never a path list). Nested
# ones sort deepest first (slash count descending, then path order); the main executable is
# signed separately.
macho_depths() { # prints "SLASHES<TAB>RELPATH" for every Mach-O under APP
  local f rel
  while IFS= read -r -d '' f; do
    [[ "$(file -b "$f")" == Mach-O* ]] || continue
    rel="${f#"$APP"/}"
    printf '%s\t%s\n' "$(tr -cd '/' <<<"$rel" | wc -c | tr -d ' ')" "$rel"
  done < <(find "$APP" -type f -print0)
}
NESTED=()
MAIN_SEEN=0
while IFS= read -r rel; do
  if [[ "$rel" == "$MAIN" ]]; then MAIN_SEEN=1; else NESTED+=("$rel"); fi
done < <(macho_depths | LC_ALL=C sort -t "$(printf '\t')" -k1,1nr -k2,2 | cut -f2-)
if [[ $MAIN_SEEN -eq 0 ]]; then
  echo "ERROR: no Mach-O main executable $MAIN under $APP; nothing signed" >&2
  exit 1
fi

sign() { # sign PATH [extra codesign args...]
  local path="$1"
  shift
  echo "==> Signing ${path#"$APP"/}"
  codesign --force --options runtime --timestamp --sign "$SIGN_IDENTITY" "$@" "$path"
}

# R-3: once Apple grants request T558S9Q5B3, copy the provisioning profile to
# Contents/embedded.provisionprofile here and add the one entitlement to build/entitlements.plist;
# nothing else changes.
for rel in ${NESTED[@]+"${NESTED[@]}"}; do
  sign "$APP/$rel"
done
sign "$APP/$MAIN" --identifier "$BUNDLE_ID" --entitlements "$ENTITLEMENTS"
sign "$APP" --identifier "$BUNDLE_ID" --entitlements "$ENTITLEMENTS"
