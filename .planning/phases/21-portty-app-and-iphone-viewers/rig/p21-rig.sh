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

cmd_build_probe() {
  [ $# -eq 1 ] || die "usage: build-probe OUTDIR"
  local out=$1 nm
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
  bash "$HERE/keyprobe/apply.sh" "$out/tree"
  # The package.json build script, run directly: pnpm run would re-verify (and try to write) node_modules.
  (cd "$out/tree/frontend" && ./node_modules/.bin/tsc && ./node_modules/.bin/vite build)
  (cd "$out/tree" && GOTOOLCHAIN="${GOTOOLCHAIN:-go1.26.11}" wails build -s -platform darwin/arm64 -tags wailsassets)
  cp -R "$out/tree/build/bin/Portty.app" "$out/Portty.app"
  codesign --deep --force --identifier app.portty.Portty --sign - "$out/Portty.app"
  codesign -dv "$out/Portty.app" 2>&1 | grep -E '^(Identifier|Signature|TeamIdentifier)=' || true
  echo "$out/Portty.app"
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

main() {
  local c=${1:-}
  [ -n "$c" ] || die "usage: p21-rig.sh COMMAND ... (see the header)"
  shift
  case "$c" in
    build-probe) cmd_build_probe "$@" ;;
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
