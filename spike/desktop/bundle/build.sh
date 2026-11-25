#!/usr/bin/env bash
# build.sh: assemble and sign build/Spike.app for the desktop feasibility spike.
#
# One Mach-O plays three roles, as in production: Contents/MacOS/spike is the
# registrar app and the daemon, Contents/Helpers/spike-helper is a signed copy
# with its own identifier (the SMAppService helper shape). Each Mach-O gets its
# own codesign call, inside out. This script deliberately signs nested code
# one binary at a time, as a trial run of the release signing rework; it never
# launches anything and never notarizes.
#
# Signing identity: SPIKE_SIGN_ID, default the Developer ID Application
# identity of team S2K7P43927. Ad-hoc ("-") and any other team are refused,
# because TCC behaviour must match a real release.
set -euo pipefail

HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
SPIKE_DIR=$(dirname "$HERE")
BUILD="$SPIKE_DIR/build"
APP="$BUILD/Spike.app"
TEAM=S2K7P43927
ID="${SPIKE_SIGN_ID:-Developer ID Application: Ken Scott ($TEAM)}"

if [[ "$ID" == "-" ]]; then
  echo "build.sh: refusing an ad-hoc signature; TCC rows must match a release signature" >&2
  exit 2
fi
if [[ "$ID" != *"($TEAM)" ]]; then
  echo "build.sh: signing identity '$ID' is not team $TEAM" >&2
  exit 2
fi

GOTOOLCHAIN=local CGO_ENABLED=1 GOARCH=arm64 go -C "$SPIKE_DIR" build -o build/spike .

rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Helpers" "$APP/Contents/Library/LaunchAgents"
cp "$BUILD/spike" "$APP/Contents/MacOS/spike"
cp "$BUILD/spike" "$APP/Contents/Helpers/spike-helper"
cp "$HERE/app.portty.spike.helper.plist" "$APP/Contents/Library/LaunchAgents/app.portty.spike.helper.plist"
cp "$HERE/Info.plist" "$APP/Contents/Info.plist"
# A fresh build number so a rebuilt bundle differs from the one it replaces (R10).
plutil -replace CFBundleVersion -string "$(date -u +%Y%m%d%H%M%S)" "$APP/Contents/Info.plist"
cp "$HERE/app.portty.spike.daemon.plist" "$BUILD/app.portty.spike.daemon.plist.template"

# Inside out: the helper first, then the app that contains it.
codesign --force --options runtime --timestamp \
  --identifier app.portty.spike.helper \
  --sign "$ID" "$APP/Contents/Helpers/spike-helper"
codesign --force --options runtime --timestamp \
  --entitlements "$HERE/entitlements.plist" \
  --sign "$ID" "$APP"

codesign --verify --strict --verbose=2 "$APP"
for target in "$APP" "$APP/Contents/Helpers/spike-helper"; do
  echo "== $target"
  codesign -dv "$target" 2>&1 | grep -E '^(Identifier|TeamIdentifier)=|^CodeDirectory .*flags='
done
