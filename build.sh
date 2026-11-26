#!/usr/bin/env bash
set -euo pipefail

# build.sh — macOS build script for Portty (Wails v2)
# Usage: build.sh --platform macos [--sign]

PLATFORM=""
SIGN="false"

usage() {
  echo "Usage: build.sh --platform macos [--sign]"
  echo ""
  echo "  --platform macos     Build the macOS Apple Silicon (arm64) app bundle, macOS 14 or later"
  echo "  --sign               Code-sign and notarize the macOS build (requires credentials)"
  exit 1
}

# Argument parsing
while [[ $# -gt 0 ]]; do
  case "$1" in
    --platform)
      if [[ $# -lt 2 ]]; then
        echo "ERROR: --platform requires a value (macos)"
        usage
      fi
      PLATFORM="$2"
      shift 2
      ;;
    --sign)
      SIGN="true"
      shift
      ;;
    *)
      echo "ERROR: Unknown flag: $1"
      usage
      ;;
  esac
done

if [[ -z "$PLATFORM" ]]; then
  usage
fi

# Validate platform value
case "$PLATFORM" in
  macos)
    ;;
  *)
    echo "ERROR: Invalid platform: $PLATFORM"
    usage
    ;;
esac

# Build with go.mod's toolchain: tailscale.com v1.98.3's JSON decoding breaks on Go 1.27, and
# GOTOOLCHAIN=auto never downgrades a newer local Go. Remove with the tailscale.com upgrade.
GOMOD_TOOLCHAIN="$(awk '/^toolchain /{print $2}' "$(dirname "$0")/go.mod")"
if [[ -z "$GOMOD_TOOLCHAIN" ]]; then
  echo "ERROR: go.mod has no toolchain line (expected e.g. 'toolchain go1.26.11')"
  exit 1
fi
export GOTOOLCHAIN="${GOTOOLCHAIN:-$GOMOD_TOOLCHAIN}"

# Wails binary check
WAILS="$(go env GOPATH)/bin/wails"
WAILS_PINNED_VER="$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v2 2>/dev/null)"
if [[ -z "$WAILS_PINNED_VER" ]]; then
  echo "ERROR: wails not pinned in go.mod — Phase 90 tools.go setup missing"
  exit 1
fi
if [[ ! -x "$WAILS" ]]; then
  echo "ERROR: wails not found at $WAILS"
  echo "Install: go install github.com/wailsapp/wails/v2/cmd/wails@$WAILS_PINNED_VER"
  exit 1
fi
# Optional: verify installed version matches pinned version
if ! "$WAILS" version 2>/dev/null | grep -qF "$WAILS_PINNED_VER"; then
  echo "WARN: installed wails does not match pinned version ($WAILS_PINNED_VER)"
  echo "Reinstall: go install github.com/wailsapp/wails/v2/cmd/wails@$WAILS_PINNED_VER"
fi

# --- Build functions ---

build_macos() {
  local VERSION="${BUILD_VERSION:-dev}"
  # Wails v2.10.2 adds -mmacosx-version-min=10.13 only when the variable lacks one, and both
  # are needed: CFLAGS alone leaves the linked minos at 11.0 (23-RESEARCH Pattern 9).
  export CGO_CFLAGS="-mmacosx-version-min=14.0"
  export CGO_LDFLAGS="-mmacosx-version-min=14.0"
  echo "==> Building macOS (darwin/arm64, macOS 14+) version=${VERSION}"
  "$WAILS" build -platform darwin/arm64 -clean \
    -ldflags "-X main.Version=${VERSION}" \
    -tags wailsassets
  echo "==> macOS build complete: build/bin/Portty.app"
  bash "$(dirname "$0")/scripts/verify-bundle.sh" build/bin/Portty.app

  if [[ "$SIGN" == "true" ]]; then
    sign_and_notarize "build/bin/Portty.app"
  fi
}

sign_and_notarize() {
  local APP="$1"
  local ENTITLEMENTS="build/entitlements.plist"
  local IDENTITY="${MACOS_SIGNING_IDENTITY:-}"
  local APPLE_ID="${MACOS_APPLE_ID:-}"
  local TEAM_ID="${MACOS_TEAM_ID:-}"
  local APP_PASSWORD="${MACOS_APP_PASSWORD:-}"

  # Validate all four required env vars
  local MISSING=()
  [[ -z "$IDENTITY" ]] && MISSING+=("MACOS_SIGNING_IDENTITY")
  [[ -z "$APPLE_ID" ]] && MISSING+=("MACOS_APPLE_ID")
  [[ -z "$TEAM_ID" ]] && MISSING+=("MACOS_TEAM_ID")
  [[ -z "$APP_PASSWORD" ]] && MISSING+=("MACOS_APP_PASSWORD")

  if [[ ${#MISSING[@]} -gt 0 ]]; then
    echo "ERROR: Missing required environment variables for code signing:"
    for var in "${MISSING[@]}"; do
      echo "  - $var"
    done
    echo ""
    echo "Setup instructions:"
    echo "  MACOS_SIGNING_IDENTITY  Run: security find-identity -v -p codesigning"
    echo "  MACOS_APPLE_ID          Your Apple ID email"
    echo "  MACOS_TEAM_ID           Apple Developer portal -> Membership -> Team ID"
    echo "  MACOS_APP_PASSWORD      appleid.apple.com -> Security -> App-Specific Passwords"
    exit 1
  fi

  # Verify entitlements file exists
  if [[ ! -f "$ENTITLEMENTS" ]]; then
    echo "ERROR: Entitlements file not found: $ENTITLEMENTS"
    exit 1
  fi

  # Step 1: Sign every Mach-O inside out, then the bundle (hardened runtime, timestamp)
  echo "==> Signing $APP"
  SIGN_IDENTITY="$IDENTITY" ENTITLEMENTS="$ENTITLEMENTS" bash "$(dirname "$0")/scripts/sign-bundle.sh" "$APP"

  # Step 2: Verify the signature gates (team, runtime, timestamp, identifier, designated requirement)
  echo "==> Verifying signature"
  bash "$(dirname "$0")/scripts/verify-bundle.sh" "$APP" --signed

  # Step 3: Create zip for notarytool (ditto preserves macOS metadata; NOT zip -r)
  local ZIP="build/bin/portty-notarize.zip"
  echo "==> Creating notarization archive"
  ditto -c -k --keepParent "$APP" "$ZIP"

  # Step 4: Store credentials in keychain profile
  echo "==> Storing notarization credentials"
  xcrun notarytool store-credentials "portty-notarize" \
    --apple-id "$APPLE_ID" \
    --team-id "$TEAM_ID" \
    --password "$APP_PASSWORD"

  # Step 5: Submit and wait (--wait is CRITICAL: without it, exit 0 does NOT mean success)
  echo "==> Submitting for notarization (this may take several minutes)"
  xcrun notarytool submit "$ZIP" \
    --keychain-profile "portty-notarize" \
    --wait

  # Step 6: Staple notarization ticket to .app
  echo "==> Stapling notarization ticket"
  xcrun stapler staple "$APP"

  # Step 7: Final verification with spctl (exit 0 = Gatekeeper accepted)
  echo "==> Verifying with spctl"
  spctl --assess --verbose=2 --type install "$APP"

  # Cleanup
  rm -f "$ZIP"
  echo "==> macOS build signed and notarized: $APP"
}

# --- Dispatch ---

case "$PLATFORM" in
  macos)   build_macos ;;
  *)       usage ;;
esac
