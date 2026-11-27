#!/usr/bin/env bash
# Brand asset check. macOS built-ins only (sips, cmp, stat). Run from the repo root.
set -e

PASS=0
FAIL=0
ERRORS=""

check() {
  local desc="$1"
  shift
  if eval "$@" >/dev/null 2>&1; then
    echo "  PASS: $desc"
    PASS=$((PASS+1))
  else
    echo "  FAIL: $desc"
    ERRORS="$ERRORS\n  - $desc"
    FAIL=$((FAIL+1))
  fi
}

# dims FILE W H — true when sips reports exactly W×H pixels.
dims() {
  local out
  out=$(sips -g pixelWidth -g pixelHeight "$1" 2>/dev/null) || return 1
  echo "$out" | grep -q "pixelWidth: $2\$" && echo "$out" | grep -q "pixelHeight: $3\$"
}

echo "=== Portty Icon Verification ==="
echo ""

# App icon
check "build/appicon.png is 1024x1024" "dims build/appicon.png 1024 1024"
check "assets/appicon.png is byte-identical to build/appicon.png" "cmp build/appicon.png assets/appicon.png"

# Iconset and icns
check "AppIcon.iconset holds exactly 10 PNGs" \
  "test \$(ls build/AppIcon.iconset/*.png 2>/dev/null | wc -l) -eq 10"
check "AppIcon.iconset has no -2x.png names" \
  "test \$(ls build/AppIcon.iconset/ | grep -c -- '-2x\.png\$') -eq 0"
check "iconfile.icns exists and is >100000 bytes (multi-entry)" \
  "test -f build/darwin/iconfile.icns && test \$(stat -f%z build/darwin/iconfile.icns) -gt 100000"

# Tray icons: 1x 18x18, @2x 36x36
for s in "" _error _sharing _progress_25 _progress_50 _progress_75 _progress_100; do
  check "assets/tray_icon${s}.png is 18x18" "dims assets/tray_icon${s}.png 18 18"
  check "assets/tray_icon${s}@2x.png is 36x36" "dims assets/tray_icon${s}@2x.png 36 36"
done

# Title logos
for d in docs frontend/public frontend/src/assets; do
  for f in portty-title-logo.png portty-title-logo-dark.png; do
    check "$d/$f is 1400x360" "dims $d/$f 1400 360"
  done
done
check "exactly six title-logo files, all Portty-named" \
  "test \$(find docs frontend/public frontend/src/assets -maxdepth 1 -name '*title-logo*.png' | wc -l) -eq 6 && \
   test \$(find docs frontend/public frontend/src/assets -maxdepth 1 -name '*title-logo*.png' \
     ! -name portty-title-logo.png ! -name portty-title-logo-dark.png | wc -l) -eq 0"

# One source per asset: no generator or brand drop folder left
check "portty-brand/ is gone" "test ! -e portty-brand"
check "build/gen_icon.go is gone" "test ! -e build/gen_icon.go"
check "build/gen_progress_icons.go is gone" "test ! -e build/gen_progress_icons.go"

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
if [ $FAIL -gt 0 ]; then
  echo -e "Failures:$ERRORS"
  exit 1
fi
echo "All checks passed."
