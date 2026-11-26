#!/usr/bin/env bash
# p18-rig.sh: two-build VM rig for the Phase 18 self-installing update (18-08).
#
#   build VERSION OUTDIR   host: wails build a vmpreview app of VERSION into OUTDIR/Portty.app
#   sign APP_DIR           host: Developer ID sign APP_DIR/Portty.app like release.yml (scratch copies only)
#   deploy-n APP           VM: wipe Portty and its update leftovers, install the host app dir APP
#   dmg APP VERSION        VM: make ~/p18/portty-vVERSION-darwin-universal.dmg from the host app dir APP
#   feed VERSION           VM: (re)start fakegh serving that DMG as the newer release on 127.0.0.1:8788
#   state [ROW]            VM: print the p18-state capture; with ROW also save evidence/ROW.state
#   quit-gui               VM: SIGTERM the Portty GUIs only (never the daemon)
#   clean                  VM: stop fakegh and remove ~/p18
#
# It lives under .planning on purpose: tests/check-brand.sh keeps the vmpreview tag out of
# scripts/, build.sh and .github. Every command that removes, mounts, creates DMGs or touches
# /Applications runs inside the Tart VM through vm.sh; the host side only builds, makes tar
# streams and writes evidence files. Nothing here contacts a 100.x address or the host's
# tailscaled.
set -euo pipefail

VM_SH="${P18_VM_SH:-$HOME/.local/share/portty-vm-tools/vm.sh}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PHASE_DIR="$(dirname "$HERE")"
ROOT="$(git -C "$HERE" rev-parse --show-toplevel)"
SCRATCH="${P18_SCRATCH:-${TMPDIR:-/tmp}/p18-rig}"

vm() { "$VM_SH" "$@"; }
# vmsh runs a script on stdin inside the VM (bash 3.2) with the given arguments.
vmsh() { "$VM_SH" bash -s -- "$@"; }

die() { echo "p18-rig: $*" >&2; exit 1; }

# The Portty GUI process: a Portty.app executable (any install location) started with no
# argument. daemon, install-update and every CLI command (list, new, attach ...) have one.
GUI_PIDS='ps -axww -o pid=,args= | awk '"'"'$2 ~ /\/Portty\.app\/Contents\/MacOS\/Portty$/ && NF == 2 {print $1}'"'"

cmd_build() {
  [ $# -eq 2 ] || die "usage: build VERSION OUTDIR"
  local ver=$1 out=$2
  cd "$ROOT"
  # Refuse to run over someone's edit: the restore below is a git checkout of these paths.
  if [ -n "$(git status --porcelain -- wails.json frontend/src/wailsjs)" ]; then
    die "wails.json or frontend/src/wailsjs has uncommitted changes; refusing to build"
  fi
  [ ! -e "$out/Portty.app" ] || die "$out/Portty.app already exists; use a fresh OUTDIR"
  trap 'git -C "$ROOT" checkout -- wails.json frontend/src/wailsjs' EXIT
  # Pitfall 7: without this, two rig builds share Info.plist's 0.1.0.
  jq --arg v "$ver" '.info.productVersion = $v' wails.json > wails.json.p18tmp
  mv wails.json.p18tmp wails.json
  GOTOOLCHAIN=go1.26.11 wails build -platform darwin/arm64 -tags "wailsassets vmpreview" \
    -ldflags "-X main.Version=v$ver"
  mkdir -p "$out"
  cp -R build/bin/Portty.app "$out/Portty.app"
}

cmd_sign() {
  [ $# -eq 1 ] && [ -d "$1/Portty.app/Contents" ] || die "usage: sign APP_DIR (signs APP_DIR/Portty.app)"
  local dir scratch hash
  dir=$(cd "$1" && pwd -P)
  scratch=$(mkdir -p "$SCRATCH" && cd "$SCRATCH" && pwd -P)
  case "$dir/" in "$scratch"/*) ;; *) die "sign: APP_DIR must be under the rig scratch dir $SCRATCH" ;; esac
  # Read-only lookup; the identity is picked by team ID and never printed.
  hash=$(security find-identity -v -p codesigning | awk '/"Developer ID Application: .*\(S2K7P43927\)"$/ {print $2; exit}')
  if [ -z "$hash" ]; then
    echo "identity=none"
    return 1
  fi
  echo "identity=found"
  # release.yml's "Sign .app with hardened runtime" step, verbatim apart from the identity.
  cd "$ROOT"
  codesign --deep --force --verbose --options runtime --timestamp --entitlements build/entitlements.plist --sign "$hash" "$dir/Portty.app"
}

cmd_deploy_n() {
  [ $# -eq 1 ] && [ -d "$1/Contents" ] || die "usage: deploy-n APP_DIR"
  vmsh <<'EOF'
set -u
pkill -x Portty 2>/dev/null; sleep 2; pkill -9 -x Portty 2>/dev/null
CFG="$HOME/Library/Application Support/portty"
CACHE="$HOME/Library/Caches/app.portty.Portty/updates"
for m in "$CACHE"/mnt-*; do [ -e "$m" ] && hdiutil detach -force "$m" >/dev/null 2>&1; done
rm -rf /Applications/Portty.app /Applications/.Portty-update-* "$HOME/Applications/Portty.app" "$HOME"/Applications/.Portty-update-*
rm -rf "$CACHE"/* "$CFG/update" "$CFG/update-result.json" "$CFG"/update.log "$CFG"/update.log.1
exit 0
EOF
  COPYFILE_DISABLE=1 tar -C "$(dirname "$1")" -czf - "$(basename "$1")" | vm 'tar xzf - -C /Applications'
}

cmd_dmg() {
  [ $# -eq 2 ] && [ -d "$1/Contents" ] || die "usage: dmg APP_DIR VERSION"
  local ver=$2
  vm 'mkdir -p ~/p18 && rm -rf ~/p18/dmgsrc && mkdir ~/p18/dmgsrc'
  COPYFILE_DISABLE=1 tar -C "$(dirname "$1")" -czf - "$(basename "$1")" | vm 'tar xzf - -C ~/p18/dmgsrc'
  vm "rm -f ~/p18/portty-v$ver-darwin-universal.dmg && hdiutil create -quiet -volname Portty -srcfolder ~/p18/dmgsrc -format UDZO ~/p18/portty-v$ver-darwin-universal.dmg && rm -rf ~/p18/dmgsrc && ls -l ~/p18/portty-v$ver-darwin-universal.dmg"
}

cmd_feed() {
  [ $# -eq 1 ] || die "usage: feed VERSION"
  local ver=$1
  mkdir -p "$SCRATCH"
  if [ ! -x "$SCRATCH/fakegh" ]; then
    (cd "$ROOT" && GOTOOLCHAIN=go1.26.11 GOOS=darwin GOARCH=arm64 go build -o "$SCRATCH/fakegh" ./cmd/fakegh)
  fi
  vm 'mkdir -p ~/p18 && pkill -x fakegh; sleep 1; rm -f ~/p18/fakegh'
  vm 'cat > ~/p18/fakegh && chmod 755 ~/p18/fakegh' < "$SCRATCH/fakegh"
  vm "nohup ~/p18/fakegh -scenario newer -version $ver -dmg ~/p18/portty-v$ver-darwin-universal.dmg > ~/p18/fakegh.log 2>&1 < /dev/null &"
  sleep 1
  vm 'curl -s http://127.0.0.1:8788/repos/scottkw/portty/releases/latest | head -c 120; echo'
}

cmd_state() {
  local row=${1:--}
  local tmp
  mkdir -p "$SCRATCH"
  tmp=$(mktemp "$SCRATCH/state.XXXXXX")
  # The VM home shows as ~ in everything we keep.
  vmsh "$row" <<'EOF' | sed 's#/Users/[A-Za-z0-9._-]*#~#g' > "$tmp"
row=${1:--}
CFG="$HOME/Library/Application Support/portty"
CACHE="$HOME/Library/Caches/app.portty.Portty/updates"
APP=/Applications/Portty.app
count() { ls -d "$@" 2>/dev/null | wc -l | tr -d ' '; }
echo "p18-state row=$row vm_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "installed_version=$(plutil -extract CFBundleShortVersionString raw "$APP/Contents/Info.plist" 2>/dev/null || true)"
guis=$(ps -axww -o pid=,args= | awk '$2 ~ /\/Portty\.app\/Contents\/MacOS\/Portty$/ && NF == 2 {print $1}')
echo "gui_pid_count=$(printf '%s' "$guis" | grep -c . || true)"
dpids=$(ps -axww -o pid=,args= | awk '$2 ~ /\/Portty\.app\/Contents\/MacOS\/Portty$/ && $3 == "daemon" {print $1}')
dv=$(curl -s -m 3 --unix-socket "$CFG/daemon.sock" http://localhost/health 2>/dev/null | sed -n 's/.*"version" *: *"\([^"]*\)".*/\1/p')
echo "daemon_version=$dv"
imgs=$(hdiutil info | grep '^image-path' || true)
echo "attached_images=$(printf '%s' "$imgs" | grep -c . || true)"
echo "cache_dmgs=$(count "$CACHE"/*.dmg)"
echo "staging_dirs=$(( $(count /Applications/.Portty-update-*) + $(count "$HOME"/Applications/.Portty-update-*) ))"
echo "run_dirs=$(count "$CFG"/update/run-*)"
echo "trash_portty=$(count "$HOME"/.Trash/Portty*.app)"
echo "--- gui pids and executables"
for p in $guis; do echo "$p $(ps -p "$p" -o args= | cut -d' ' -f1)"; done
echo "--- daemon pids and executables"
for p in $dpids; do echo "$p $(ps -p "$p" -o args= | cut -d' ' -f1)"; done
echo "--- attached images"
printf '%s\n' "$imgs"
echo "--- update cache"
ls -la "$CACHE" 2>&1
echo "--- staging dirs"
ls -d /Applications/.Portty-update-* "$HOME"/Applications/.Portty-update-* 2>/dev/null
echo "--- run dirs"
ls -d "$CFG"/update/run-* 2>/dev/null
echo "--- trash: Portty bundles, version, ctime (the move into the Trash; 18-05)"
for t in "$HOME"/.Trash/Portty*.app; do
  [ -d "$t" ] && echo "$(basename "$t") $(plutil -extract CFBundleShortVersionString raw "$t/Contents/Info.plist" 2>/dev/null) $(stat -f %Sc -t %Y-%m-%dT%H:%M:%S "$t")"
done
echo "--- update.log tail"
tail -n 30 "$CFG/update.log" 2>&1
exit 0
EOF
  cat "$tmp"
  if [ "$row" != "-" ]; then
    mkdir -p "$PHASE_DIR/evidence"
    cp "$tmp" "$PHASE_DIR/evidence/$row.state"
  fi
  rm -f "$tmp"
}

cmd_quit_gui() {
  vm "pids=\$($GUI_PIDS); [ -n \"\$pids\" ] && kill -TERM \$pids; echo \"sent SIGTERM to: \$pids\"; exit 0"
}

cmd_clean() {
  vm 'pkill -x fakegh; rm -rf ~/p18; exit 0'
}

sub=${1:-}
[ $# -gt 0 ] && shift
case "$sub" in
  build) cmd_build "$@" ;;
  sign) cmd_sign "$@" ;;
  deploy-n) cmd_deploy_n "$@" ;;
  dmg) cmd_dmg "$@" ;;
  feed) cmd_feed "$@" ;;
  state) cmd_state "$@" ;;
  quit-gui) cmd_quit_gui ;;
  clean) cmd_clean ;;
  *) die "usage: $0 build|sign|deploy-n|dmg|feed|state|quit-gui|clean ..." ;;
esac
