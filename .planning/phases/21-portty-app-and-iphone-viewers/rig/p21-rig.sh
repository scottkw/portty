#!/usr/bin/env bash
# p21-rig.sh: Phase 21 VM rig for the 21-02 key-routing probe, modelled on p19-rig.sh.
#
#   build-probe OUTDIR     host: git archive of HEAD into OUTDIR/tree (scratch, never the repo), the
#                          repo's frontend/node_modules linked in, keyprobe/apply.sh run there, then
#                          frontend build + wails build (tags wailsassets), ad-hoc signed with
#                          identifier app.portty.Portty -> OUTDIR/Portty.app (OUTDIR/build-commit)
#   tools2                 host builds nothing; copies the key binary (P21_KEY_BIN) into portty-test-2
#   vm2-idle               exit 0 only when nothing else drives portty-test-2 (no safaridriver /
#                          chromedriver, no fresh Portty GUI that this rig did not start)
#   deploy2 APP            portty-test-2: quit GUIs, Quit All through the daemon socket, replace
#                          /Applications/Portty.app with the host app dir APP
#   start2 [SECS]          portty-test-2: open /Applications/Portty.app, wait for the probe's ready line
#   quit-all2              portty-test-2: GUI quit, then POST /shutdown on the daemon socket
#   remove2                portty-test-2: Quit All, delete /Applications/Portty.app and ~/p21
#   key2 NAME [cmd|ctrl|opt|shift|hold MS]...   portty-test-2: post one key (NAME: KeyA, Comma, Return,
#                          Backspace, Escape, Tab ... or a virtual key code)
#   mode2 page|blank|rebuild [textarea|div|none]   portty-test-2: set the probe mode (and focus target)
#   focus2 textarea|div|none   portty-test-2: re-apply a focus target
#   log2 [--from N] [ROW]  portty-test-2: print ~/p21/keyprobe.log (from line N), scrubbed; with ROW
#                          also save evidence/ROW.log
#   alive2                 portty-test-2: GUI pid or "none"
#   matrix2 ROW FOCUS      portty-test-2: run the 15-chord matrix in the current mode, restoring state
#                          between chords; output scrubbed, saved to evidence/ROW.log
#   click2 X Y             portty-test-2: left click at screen point X,Y (the VM's click tool)
#   winfo2                 portty-test-2: x y w h of the Portty window, or "none"
#   sh2 CMD...             portty-test-2: run CMD in the VM (open, osascript ... for the K4/K5 steps)
#
# 21-11 (the real app on the home guest viewing the office guest; office = portty-test, home = portty-test-2):
#   build-home OUTDIR      host: like build-probe but with keyprobe/apply.sh --stats (the stats probe only,
#                          no KeyProbe overlay) and a Developer ID signature (Local Network TCC follows the
#                          signing identity). The office build is p19-rig.sh build ... --tags "wailsassets
#                          vmdesktop" --sign devid (p20-rig.sh build20), then p19-rig.sh deploy/start-gui.
#   ax2 [ROW [BUNDLE]]     home: accessibility tree of BUNDLE (default app.portty.Portty), scrubbed; with
#                          ROW also saved as evidence/ROW.ax
#   wait-text2 TEXT SECS [BUNDLE]   home: poll the tree each second until TEXT shows; prints seconds
#   stats2 SECS            home: the STATS/ICE/PC lines the stats probe logs during the next SECS seconds
#   office-viewers         office: GET /desktop on the daemon socket: state, viewers, locked only
#   office-assertions      office: pmset assertions naming Portty (p19-rig.sh assertions)
#   safari2-front          home: Safari's front document path only (no host, no query, no fragment)
#   ln-prompt2             home: is a Local Network prompt for Portty open, and is it frontmost
#   tailnet-health         both guests: tailscale status run inside the guest, online yes or no only
#
# It lives under .planning on purpose (like p19-rig.sh). Every launch, key post, log read and removal
# runs inside portty-test-2 through ssh; the host side only builds a scratch copy, streams the bundle
# and writes scrubbed evidence. Nothing here posts input on, captures, or touches the host, contacts the
# host's tailscaled or a 100.x address, or stops either VM.
set -euo pipefail

VM2_NAME="${P21_VM2_NAME:-portty-test-2}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PHASE_DIR="$(dirname "$HERE")"
ROOT="$(git -C "$HERE" rev-parse --show-toplevel)"
KEY_BIN="${P21_KEY_BIN:-}"

die() { echo "p21-rig: $*" >&2; exit 1; }

require_vm2() {
  tart list | awk -v n="$VM2_NAME" '$2 == n && $NF == "running" {f = 1} END {exit !f}' ||
    die "$VM2_NAME is not running (tart list)"
}
vm2() {
  ssh -i "$HOME/.ssh/transom_demo_ed25519" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
    -o BatchMode=yes -o ConnectTimeout=10 -o LogLevel=ERROR "admin@$(tart ip "$VM2_NAME")" "$@"
}
vm2sh() { vm2 bash -s -- "$@"; }
# scrub: the VM home shows as ~; tailnet addresses, names and capability tokens never survive.
scrub() {
  sed -E -e 's#/Users/[A-Za-z0-9._-]*#~#g' -e 's#100\.[0-9]+\.[0-9]+\.[0-9]+#100.x.x.N#g' \
    -e 's#[A-Za-z0-9.-]+\.ts\.net#TAILNET-NAME#g' -e 's#cap=[^& "]*#CAP-REDACTED#g'
}

VM_COMMON='
P=$HOME/p21
CFG="$HOME/Library/Application Support/portty"
SOCK="$CFG/daemon.sock"
APP=/Applications/Portty.app
EXE=$APP/Contents/MacOS/Portty
gui_pids() { ps -axww -o pid=,args= | awk '"'"'$2 ~ /\/Portty\.app\/Contents\/MacOS\/Portty$/ && NF == 2 {print $1}'"'"'; }
daemon_pids() { ps -axww -o pid=,args= | awk '"'"'$2 ~ /\/Portty\.app\/Contents\/MacOS\/Portty$/ && $3 == "daemon" {print $1}'"'"'; }
'

# scratch_build OUT SIGN APPLY_ARGS...: a git archive of HEAD in OUT/tree (scratch, never the repo), the
# repo's frontend/node_modules linked in, keyprobe/apply.sh APPLY_ARGS run there, then frontend build and
# wails build (tags wailsassets), signed with identifier app.portty.Portty (adhoc or devid).
scratch_build() {
  local out=$1 sign=$2 nm
  shift 2
  [ ! -e "$out/Portty.app" ] || die "$out/Portty.app already exists; use a fresh OUTDIR"
  case "$(cd "$(dirname "$out")" && pwd -P)/$(basename "$out")" in
    "$(cd "$ROOT" && pwd -P)"/*) die "OUTDIR must be outside the repo (scratch directory)" ;;
  esac
  nm="$ROOT/frontend/node_modules"
  [ -d "$nm" ] || nm="$(git -C "$ROOT" worktree list --porcelain | awk '/^worktree /{print $2; exit}')/frontend/node_modules"
  [ -d "$nm" ] || die "no frontend/node_modules to link (no package install is done here)"
  mkdir -p "$out/tree"
  git -C "$ROOT" rev-parse HEAD > "$out/build-commit"
  git -C "$ROOT" archive HEAD | tar -x -C "$out/tree"
  touch "$out/tree/.p21-scratch"
  ln -s "$nm" "$out/tree/frontend/node_modules"
  bash "$HERE/keyprobe/apply.sh" "$@" "$out/tree"
  # The package.json build script, run directly: pnpm run would re-verify (and try to write) node_modules.
  (cd "$out/tree/frontend" && ./node_modules/.bin/tsc && ./node_modules/.bin/vite build)
  (cd "$out/tree" && GOTOOLCHAIN="${GOTOOLCHAIN:-go1.26.11}" wails build -s -platform darwin/arm64 -tags wailsassets)
  cp -R "$out/tree/build/bin/Portty.app" "$out/Portty.app"
  if [ "$sign" = devid ]; then
    local hash
    hash=$(security find-identity -v -p codesigning | awk '/"Developer ID Application: / {print $2; exit}')
    [ -n "$hash" ] || die "no Developer ID Application identity"
    codesign --deep --force --options runtime --timestamp --entitlements "$out/tree/build/entitlements.plist" \
      --identifier app.portty.Portty --sign "$hash" "$out/Portty.app"
  else
    codesign --deep --force --identifier app.portty.Portty --sign - "$out/Portty.app"
  fi
  codesign -dv "$out/Portty.app" 2>&1 | grep -E '^(Identifier|Signature|TeamIdentifier)=' || true
  echo "$out/Portty.app"
}

cmd_build_probe() {
  [ $# -eq 1 ] || die "usage: build-probe OUTDIR"
  scratch_build "$1" adhoc
}

cmd_build_home() {
  [ $# -eq 1 ] || die "usage: build-home OUTDIR"
  scratch_build "$1" devid --stats
}

cmd_tools2() {
  require_vm2
  [ -x "$KEY_BIN" ] || die "set P21_KEY_BIN to the host-built key binary (swiftc rig/key.swift)"
  vm2 'mkdir -p ~/p21 && cat > ~/p21/key && chmod +x ~/p21/key' < "$KEY_BIN"
  vm2 'test -x ~/ptest/click && echo click-present'
}

cmd_vm2_idle() {
  require_vm2
  vm2sh <<'EOF'
busy=0
if ps -axww -o args= | grep -E '(^|/)(safaridriver|chromedriver)( |$)' | grep -v grep >/dev/null; then echo "busy: a WebDriver process is running"; busy=1; fi
if ps -axww -o args= | grep -E 'remote-debugging-port' | grep -v grep >/dev/null; then echo "busy: a browser with a debugging port is running"; busy=1; fi
# A Portty GUI younger than 30 minutes that this rig did not start (no ~/p21/started marker) belongs to someone else.
for p in $(ps -axww -o pid=,args= | awk '$2 ~ /\/Portty\.app\/Contents\/MacOS\/Portty$/ && NF == 2 {print $1}'); do
  et=$(ps -o etime= -p "$p" | tr -d ' ')
  case "$et" in *-*) continue ;; esac
  mins=$(echo "$et" | awk -F: '{ if (NF == 3) print $1 * 60 + $2; else print $1 }')
  if [ "$mins" -lt 30 ] && [ ! -f "$HOME/p21/started" ]; then echo "busy: Portty GUI pid $p is ${et} old and not started by p21"; busy=1; fi
done
[ "$busy" -eq 0 ] && echo "vm2 idle"
exit $busy
EOF
}

cmd_quit_all2() {
  require_vm2
  vm2sh <<EOF
$VM_COMMON
pids=\$(gui_pids); [ -n "\$pids" ] && kill -TERM \$pids
sleep 1
curl -s -m 5 -X POST --unix-socket "\$SOCK" http://localhost/shutdown >/dev/null 2>&1
for i in \$(seq 1 30); do [ -z "\$(daemon_pids)" ] && break; sleep 1; done
echo "daemon_pids_after=\$(daemon_pids | tr '\n' ' ')"
exit 0
EOF
}

cmd_deploy2() {
  [ $# -eq 1 ] && [ -d "$1/Contents" ] || die "usage: deploy2 APP_DIR"
  require_vm2
  cmd_quit_all2
  # the old ~/ptest copy would also answer to gui_pids; it is gone once the GUIs are signalled above
  vm2 'rm -rf /Applications/Portty.app'
  COPYFILE_DISABLE=1 tar -C "$(dirname "$1")" -czf - "$(basename "$1")" | vm2 'tar xzf - -C /Applications'
  vm2 'plutil -extract CFBundleShortVersionString raw /Applications/Portty.app/Contents/Info.plist'
}

cmd_start2() {
  local secs=${1:-30}
  require_vm2
  vm2sh "$secs" <<EOF
$VM_COMMON
mkdir -p \$P; touch \$P/started
# an older run's "PROBE ready" line must not end the wait below
[ -f \$P/keyprobe.log ] && mv -f \$P/keyprobe.log \$P/keyprobe.log.prev
caffeinate -u -t 1
open --stdout \$P/gui.log --stderr \$P/gui.log "\$APP"
for i in \$(seq 1 \$1); do
  grep -q 'PROBE ready' \$P/keyprobe.log 2>/dev/null && break
  sleep 1
done
echo "gui_pids=\$(gui_pids | tr '\n' ' ') log_lines=\$(wc -l < \$P/keyprobe.log 2>/dev/null)"
exit 0
EOF
}

cmd_remove2() {
  require_vm2
  cmd_quit_all2
  vm2sh <<EOF
$VM_COMMON
pids=\$(gui_pids; daemon_pids); [ -n "\$pids" ] && kill -9 \$pids 2>/dev/null
rm -rf /Applications/Portty.app \$P
echo "gui=\$(gui_pids | tr '\n' ' ') app_present=\$([ -e /Applications/Portty.app ] && echo yes || echo no)"
exit 0
EOF
}

# key NAME table (US layout virtual key codes).
KEYNAMES='KeyA=0 KeyS=1 KeyZ=6 KeyF=3 KeyH=4 KeyX=7 KeyC=8 KeyV=9 KeyQ=12 KeyW=13 KeyT=17 KeyM=46 Comma=43 Return=36 Tab=48 Backspace=51 Escape=53'

cmd_key2() {
  [ $# -ge 1 ] || die "usage: key2 NAME [cmd|ctrl|opt|shift|hold MS]..."
  require_vm2
  local name=$1 code
  shift
  code=$(printf '%s\n' $KEYNAMES | awk -F= -v n="$name" '$1 == n {print $2}')
  [ -n "$code" ] || code=$name
  vm2 "\$HOME/p21/key key $code $*"
}

cmd_mode2() {
  [ $# -ge 1 ] || die "usage: mode2 page|blank|rebuild [textarea|div|none]"
  case "$1" in page | blank | rebuild) ;; *) die "mode2: page|blank|rebuild" ;; esac
  require_vm2
  vm2sh "$1" "${2:-textarea}" <<'EOF'
mkdir -p ~/p21
echo "$1 $2 $(date +%s)$RANDOM" > ~/p21/keyprobe.mode
sleep 1
EOF
}

cmd_focus2() {
  [ $# -eq 1 ] || die "usage: focus2 textarea|div|none"
  require_vm2
  vm2sh "$1" <<'EOF'
m=$(awk '{print $1}' ~/p21/keyprobe.mode 2>/dev/null); [ -n "$m" ] || m=page
echo "$m $1 $(date +%s)$RANDOM" > ~/p21/keyprobe.mode
sleep 0.8
EOF
}

cmd_log2() {
  local from=1 row=
  if [ "${1:-}" = "--from" ]; then from=$2; shift 2; fi
  row=${1:-}
  require_vm2
  if [ -n "$row" ]; then
    mkdir -p "$PHASE_DIR/evidence"
    vm2 "tail -n +$from ~/p21/keyprobe.log" | scrub | tee "$PHASE_DIR/evidence/$row.log"
  else
    vm2 "tail -n +$from ~/p21/keyprobe.log" | scrub
  fi
}

cmd_alive2() {
  require_vm2
  vm2sh <<EOF
$VM_COMMON
p=\$(gui_pids | tr '\n' ' '); echo "\${p:-none}"
exit 0
EOF
}

cmd_click2() {
  [ $# -eq 2 ] || die "usage: click2 X Y"
  require_vm2
  vm2 "~/ptest/click click $1 $2"
}

cmd_winfo2() {
  require_vm2
  vm2 '~/p21/key winfo Portty || echo none'
}

# matrix2 runs entirely inside the VM: per chord it records the log delta, the GUI pid and whether
# a Portty window is on screen, then restores the app (start if dead, reopen, Esc outside the
# textarea, focus target back). "menu fired" is read from the delta by the caller.
cmd_matrix2() {
  [ $# -eq 2 ] || die "usage: matrix2 ROW textarea|div"
  local row=$1 focus=$2
  require_vm2
  mkdir -p "$PHASE_DIR/evidence"
  vm2sh "$focus" <<EOF | scrub | tee "$PHASE_DIR/evidence/$row.log"
$VM_COMMON
FOCUS=\$1
K=\$P/key
setfocus() { m=\$(awk '{print \$1}' \$P/keyprobe.mode 2>/dev/null); [ -n "\$m" ] || m=page; echo "\$m \$1 \$(date +%s)\$RANDOM" > \$P/keyprobe.mode; sleep 0.8; }
win() { \$K winfo Portty >/dev/null 2>&1 && echo yes || echo no; }
restore() {
  [ -n "\$(gui_pids)" ] || { open "\$APP"; for i in \$(seq 1 30); do grep -q 'PROBE ready' <(tail -n 3 \$P/keyprobe.log) && break; sleep 1; done; sleep 2; }
  open "\$APP"; sleep 1.5
  [ "\$(win)" = yes ] || { open "\$APP"; sleep 2; }
  setfocus none
  \$K key 53; sleep 0.4; \$K key 53; sleep 0.4
  setfocus \$FOCUS
}
echo "matrix row=$row focus=\$FOCUS mode=\$(cat \$P/keyprobe.mode)"
restore
for spec in "Cmd+Q:12:cmd" "Cmd+W:13:cmd" "Cmd+C:8:cmd" "Cmd+V:9:cmd" "Cmd+X:7:cmd" "Cmd+Z:6:cmd" "Cmd+A:0:cmd" "Cmd+T:17:cmd" "Cmd+H:4:cmd" "Cmd+M:46:cmd" "Cmd+comma:43:cmd" "Ctrl+Cmd+S:1:ctrl cmd" "Ctrl+Cmd+F:3:ctrl cmd" "Cmd+Option+H:4:cmd opt" "Backspace:51:"; do
  name=\${spec%%:*}; rest=\${spec#*:}; code=\${rest%%:*}; mods=\${rest#*:}
  n=\$(wc -l < \$P/keyprobe.log)
  \$K key \$code \$mods; sleep 0.9
  echo "=== \$name code=\$code mods=[\$mods] gui=\$(gui_pids | tr '\n' ' ') window=\$(win)"
  tail -n +\$((n + 1)) \$P/keyprobe.log
  restore
done
echo "matrix done"
exit 0
EOF
}

# ---- 21-11: the real app in the home guest viewing the office guest -------------------------------

VM1_NAME="${P21_VM1_NAME:-portty-test}"
P19="$ROOT/.planning/phases/19-unattended-desktop-viewing/rig/p19-rig.sh"
SOCKQ='"$HOME/Library/Application Support/portty/daemon.sock"'

# vm_ssh NAME CMD...: ssh into a running guest by tart name (never anything else).
vm_ssh() {
  local n=$1
  shift
  tart list | awk -v n="$n" '$2 == n && $NF == "running" {f = 1} END {exit !f}' || die "$n is not running (tart list)"
  ssh -i "$HOME/.ssh/transom_demo_ed25519" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
    -o BatchMode=yes -o ConnectTimeout=10 -o LogLevel=ERROR "admin@$(tart ip "$n")" "$@"
}
# scrub_ax: scrub plus the guests' short names (a tab name carries the office's), so a name never
# lands in evidence as a tailnet hostname.
scrub_ax() { scrub | sed -E -e 's#portty-vm-office#OFFICE#g' -e 's#portty-vm-home#HOME#g'; }

cmd_ax2() {
  local row=${1:-} bundle=${2:-app.portty.Portty}
  require_vm2
  if [ -n "$row" ]; then
    mkdir -p "$PHASE_DIR/evidence"
    vm2 "~/ptest/axdump2 $bundle" | scrub_ax | tee "$PHASE_DIR/evidence/$row.ax"
  else
    vm2 "~/ptest/axdump2 $bundle" | scrub_ax
  fi
}

cmd_wait_text2() {
  [ $# -ge 2 ] || die "usage: wait-text2 TEXT SECS [BUNDLE]"
  require_vm2
  vm2sh "$(printf '%s' "$1" | base64)" "$2" "${3:-app.portty.Portty}" <<'EOF'
text=$(printf '%s' "$1" | base64 -D); secs=$2; b=$3
start=$SECONDS
while [ $((SECONDS - start)) -le "$secs" ]; do
  if ~/ptest/axdump2 "$b" 2>/dev/null | grep -qF -- "$text"; then echo "found after $((SECONDS - start))s"; exit 0; fi
  sleep 1
done
echo "not found in ${secs}s"
exit 1
EOF
}

# stats2 SECS: the probe's lines that arrive during the next SECS seconds. Each line carries the app's
# monotonic ms, so a guest clock step cannot fake a gap.
cmd_stats2() {
  local secs=${1:-10}
  require_vm2
  vm2sh "$secs" <<'EOF' | scrub
n=$(wc -l < ~/p21/keyprobe.log); sleep "$1"
tail -n +$((n + 1)) ~/p21/keyprobe.log | grep -E ' (STATS|ICE|ICEGATHER|PC|PCSTATE|PROBE) '
exit 0
EOF
}

cmd_office_viewers() {
  vm_ssh "$VM1_NAME" "curl -s -m 5 --unix-socket $SOCKQ http://localhost/desktop" |
    jq -c '{alwaysAvailable, state, viewers, locked}'
}

cmd_office_assertions() { bash "$P19" assertions | scrub; }

# safari2-front: Safari's front window through the accessibility tree (no Apple events: a consent
# prompt would be one more thing on screen). Only the path is kept: no host, no query, no fragment.
cmd_safari2_front() {
  require_vm2
  vm2 '~/ptest/axdump2 com.apple.Safari 2>&1' |
    awk '/AXTextField/ && /Value=/ {v=$0; sub(/.*Value=/, "", v); sub(/ DOMClassList=.*/, "", v); print v; exit}' |
    sed -E -e 's#^https?://##' -e 's#^[^/]*##' -e 's#[?\#].*$##' -e 's#^#safari2-front path=#'
}

# ln-prompt2: a Local Network prompt for Portty on screen? "open" is any on-screen window of the
# system's user-notification process whose tree names Portty and local networks; "frontmost" means no
# other app's window sits above it in the window list.
cmd_ln_prompt2() {
  require_vm2
  vm2sh <<'EOF'
cg=$(~/p20/ax20 cgwin x 2>/dev/null)
top=$(printf '%s\n' "$cg" | grep -v -E 'owner=(Window Server|Dock|Finder|Control Center|SystemUIServer|Notification Center|TextInputMenuAgent)' | head -1 | sed -E 's/.*owner=([^=]*) pid=.*/\1/')
unc=$(printf '%s\n' "$cg" | grep -c 'owner=UserNotificationCenter')
txt=$(~/ptest/axdump2 com.apple.UserNotificationCenter 2>/dev/null)
names=no; printf '%s' "$txt" | grep -qi 'portty' && printf '%s' "$txt" | grep -qi 'local network' && names=yes
open=no; [ "$names" = yes ] && open=yes
front=no; [ "$open" = yes ] && [ "$top" = UserNotificationCenter ] && front=yes
echo "ln-prompt open=$open frontmost=$front names_portty=$names notif_windows=$unc topmost_owner=$top"
printf '%s' "$txt" | grep -E 'AXStaticText|AXButton' | sed -E 's/ DOMClassList=.*//' | head -12
exit 0
EOF
}

cmd_tailnet_health() {
  local n out
  for n in "$VM1_NAME" "$VM2_NAME"; do
    out=$(vm_ssh "$n" '/usr/local/bin/tailscale status --json 2>/dev/null' | jq -r 'if (.Self.Online // false) then "yes" else "no" end' 2>/dev/null) || out=unreachable
    echo "$n online=${out:-unreachable}"
  done
}

main() {
  local c=${1:-}
  [ -n "$c" ] || die "usage: p21-rig.sh COMMAND ... (see the header)"
  shift
  case "$c" in
    build-probe) cmd_build_probe "$@" ;;
    build-home) cmd_build_home "$@" ;;
    ax2) cmd_ax2 "$@" ;;
    wait-text2) cmd_wait_text2 "$@" ;;
    stats2) cmd_stats2 "$@" ;;
    office-viewers) cmd_office_viewers "$@" ;;
    office-assertions) cmd_office_assertions "$@" ;;
    safari2-front) cmd_safari2_front "$@" ;;
    ln-prompt2) cmd_ln_prompt2 "$@" ;;
    tailnet-health) cmd_tailnet_health "$@" ;;
    tools2) cmd_tools2 "$@" ;;
    vm2-idle) cmd_vm2_idle "$@" ;;
    deploy2) cmd_deploy2 "$@" ;;
    start2) cmd_start2 "$@" ;;
    quit-all2) cmd_quit_all2 "$@" ;;
    remove2) cmd_remove2 "$@" ;;
    key2) cmd_key2 "$@" ;;
    mode2) cmd_mode2 "$@" ;;
    focus2) cmd_focus2 "$@" ;;
    log2) cmd_log2 "$@" ;;
    alive2) cmd_alive2 "$@" ;;
    click2) cmd_click2 "$@" ;;
    winfo2) cmd_winfo2 "$@" ;;
    matrix2) cmd_matrix2 "$@" ;;
    sh2) require_vm2; vm2 "$@" ;;
    *) die "unknown command $c" ;;
  esac
}
main "$@"
