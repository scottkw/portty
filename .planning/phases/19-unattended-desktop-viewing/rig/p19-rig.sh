#!/usr/bin/env bash
# p19-rig.sh: VM rig for the Phase 19 launchd start model (19-07), modelled on p18-rig.sh.
#
#   build VERSION OUTDIR [--tags TAGS] [--sign adhoc|devid]
#                          host: wails build of the current tree, version patched like p18-rig.sh
#   deploy APP [--keep-daemon]
#                          VM: quit the GUIs, Quit All the daemon through its socket (unless
#                          --keep-daemon), replace /Applications/Portty.app with the host app dir
#                          APP. Never touches launchd state.
#   start-gui [SECS]       VM: open /Applications/Portty.app, wait up to SECS (20) for /health
#   sessions N             VM: start N ticking shells through the installed CLI
#   state [ROW]            VM: print a p19-state capture; with ROW also save evidence/ROW.state
#   quit-gui               VM: SIGTERM the GUI processes only
#   quit-all               VM: GUI quit, then POST /shutdown on the daemon socket (exit 0)
#   legacy-plist           VM: write and bootstrap the old kardianos portty-daemon agent
#   clean                  VM: Quit All, bootout and remove both plists, remove the app and ~/p19
#
# It lives under .planning on purpose (like p18-rig.sh): tests/check-brand.sh keeps the
# vmpreview tag out of scripts/, build.sh and .github. Every launchctl call, kill, rm and
# /Applications write runs inside the Tart VM through vm.sh; the host side only builds, signs
# a scratch copy, makes tar streams and writes evidence files. Nothing here contacts a 100.x
# address or the host's tailscaled.
set -euo pipefail

VM_SH="${P19_VM_SH:-$HOME/.local/share/portty-vm-tools/vm.sh}"
VM_NAME="${P19_VM_NAME:-portty-test}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PHASE_DIR="$(dirname "$HERE")"
ROOT="$(git -C "$HERE" rev-parse --show-toplevel)"

vm() { "$VM_SH" "$@"; }
# vmsh runs a script on stdin inside the VM (bash 3.2) with the given arguments.
vmsh() { "$VM_SH" bash -s -- "$@"; }

die() { echo "p19-rig: $*" >&2; exit 1; }

# Refuse to act unless the VM is up: nothing here may fall through to the host.
require_vm() {
  tart list | awk -v n="$VM_NAME" '$2 == n && $NF == "running" {f = 1} END {exit !f}' ||
    die "$VM_NAME is not running (tart list)"
}

# Shell fragments shared by the VM-side scripts (bash 3.2).
VM_COMMON='
CFG="$HOME/Library/Application Support/portty"
SOCK="$CFG/daemon.sock"
APP=/Applications/Portty.app
EXE=$APP/Contents/MacOS/Portty
UIDN=$(id -u)
LABEL=app.portty.Portty.daemon
gui_pids() { ps -axww -o pid=,args= | awk '"'"'$2 ~ /\/Portty\.app\/Contents\/MacOS\/Portty$/ && NF == 2 {print $1}'"'"'; }
daemon_pids() { ps -axww -o pid=,args= | awk '"'"'$2 ~ /\/Portty\.app\/Contents\/MacOS\/Portty$/ && $3 == "daemon" {print $1}'"'"'; }
health() { curl -s -m 3 --unix-socket "$SOCK" http://localhost/health 2>/dev/null; }
'

cmd_build() {
  [ $# -ge 2 ] || die "usage: build VERSION OUTDIR [--tags TAGS] [--sign adhoc|devid]"
  local ver=$1 out=$2 tags="wailsassets vmpreview" sign=adhoc
  shift 2
  while [ $# -gt 0 ]; do
    case "$1" in
      --tags) tags=$2; shift 2 ;;
      --sign) sign=$2; shift 2 ;;
      *) die "build: unknown option $1" ;;
    esac
  done
  case "$sign" in adhoc | devid) ;; *) die "build: --sign adhoc|devid" ;; esac
  cd "$ROOT"
  # Refuse to run over someone's edit: the restore below is a git checkout of these paths.
  if [ -n "$(git status --porcelain -- wails.json frontend/src/wailsjs)" ]; then
    die "wails.json or frontend/src/wailsjs has uncommitted changes; refusing to build"
  fi
  [ ! -e "$out/Portty.app" ] || die "$out/Portty.app already exists; use a fresh OUTDIR"
  trap 'git -C "$ROOT" checkout -- wails.json frontend/src/wailsjs' EXIT
  # Without this, two rig builds share Info.plist's 0.1.0.
  jq --arg v "$ver" '.info.productVersion = $v' wails.json > wails.json.p19tmp
  mv wails.json.p19tmp wails.json
  GOTOOLCHAIN=go1.26.11 wails build -platform darwin/arm64 -tags "$tags" \
    -ldflags "-X main.Version=v$ver"
  mkdir -p "$out"
  cp -R build/bin/Portty.app "$out/Portty.app"
  if [ "$sign" = devid ]; then
    local hash
    hash=$(security find-identity -v -p codesigning | awk '/"Developer ID Application: / {print $2; exit}')
    [ -n "$hash" ] || die "no Developer ID Application identity"
    codesign --deep --force --options runtime --timestamp --entitlements build/entitlements.plist \
      --identifier app.portty.Portty --sign "$hash" "$out/Portty.app"
  else
    codesign --deep --force --identifier app.portty.Portty --sign - "$out/Portty.app"
  fi
  codesign -dv "$out/Portty.app" 2>&1 | grep -E '^(Identifier|Signature|TeamIdentifier)=' || true
}

cmd_quit_gui() {
  require_vm
  vmsh <<EOF
$VM_COMMON
pids=\$(gui_pids); [ -n "\$pids" ] && kill -TERM \$pids
echo "sent SIGTERM to: \$pids"
exit 0
EOF
}

cmd_quit_all() {
  require_vm
  vmsh <<EOF
$VM_COMMON
pids=\$(gui_pids); [ -n "\$pids" ] && kill -TERM \$pids
sleep 1
curl -s -m 5 -X POST --unix-socket "\$SOCK" http://localhost/shutdown >/dev/null 2>&1
for i in \$(seq 1 30); do [ -z "\$(daemon_pids)" ] && break; sleep 1; done
echo "daemon_pids_after=\$(daemon_pids | tr '\n' ' ')"
exit 0
EOF
}

cmd_deploy() {
  [ $# -ge 1 ] && [ -d "$1/Contents" ] || die "usage: deploy APP_DIR [--keep-daemon]"
  local app=$1 keep=${2:-}
  require_vm
  if [ "$keep" = "--keep-daemon" ]; then
    cmd_quit_gui
  else
    cmd_quit_all
  fi
  vm 'rm -rf /Applications/Portty.app'
  COPYFILE_DISABLE=1 tar -C "$(dirname "$app")" -czf - "$(basename "$app")" | vm 'tar xzf - -C /Applications'
  vm 'plutil -extract CFBundleShortVersionString raw /Applications/Portty.app/Contents/Info.plist'
}

cmd_start_gui() {
  local secs=${1:-20}
  require_vm
  vmsh "$secs" <<EOF
$VM_COMMON
mkdir -p ~/p19
caffeinate -u -t 1
open --stdout ~/p19/gui.log --stderr ~/p19/gui.log "\$APP"
for i in \$(seq 1 \$1); do
  health | grep -q '"status"' && break
  sleep 1
done
health; echo
exit 0
EOF
}

cmd_sessions() {
  [ $# -eq 1 ] || die "usage: sessions N"
  require_vm
  vmsh "$1" <<EOF
$VM_COMMON
mkdir -p ~/p19
cat > ~/p19/attach.exp <<'EXP'
set id [lindex \$argv 0]; set cmd [lindex \$argv 1]
log_user 0
set timeout 30
set stty_init "rows 30 cols 100"
spawn -noecho /Applications/Portty.app/Contents/MacOS/Portty attach \$id
sleep 3
send -- "\$cmd\r"
sleep 3
send -- "\034"
expect eof
EXP
for n in \$(seq 1 \$1); do
  id=\$("\$EXE" new </dev/null | tail -1)
  rm -f ~/p19/tick-\$id.txt
  # \$\$ in zsh is the session shell's pid even inside the subshell; the file proves ticks,
  # the TICK line proves the pty output still reaches the daemon.
  tickcmd="zmodload zsh/datetime; ( i=0; while :; do i=\\\$((i+1)); print \"TICK \\\$i \\\$EPOCHSECONDS\"; print \"\\\$\\\$ \\\$i \\\$EPOCHSECONDS\" >> \$HOME/p19/tick-\$id.txt; sleep 1; done ) &!"
  /usr/bin/expect -f ~/p19/attach.exp "\$id" "\$tickcmd" </dev/null >/dev/null 2>&1
  echo "session \$id started"
done
exit 0
EOF
}

cmd_legacy_plist() {
  require_vm
  vmsh <<EOF
$VM_COMMON
P="\$HOME/Library/LaunchAgents/portty-daemon.plist"
mkdir -p "\$HOME/Library/LaunchAgents"
cat > "\$P" <<PL
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>portty-daemon</string>
  <key>ProgramArguments</key><array><string>\$EXE</string><string>daemon</string></array>
  <key>KeepAlive</key><true/>
  <key>RunAtLoad</key><true/>
</dict>
</plist>
PL
launchctl bootstrap "gui/\$UIDN" "\$P"
echo "bootstrap rc=\$?"
exit 0
EOF
}

cmd_state() {
  local row=${1:--}
  local tmp
  require_vm
  tmp=$(mktemp "${TMPDIR:-/tmp}/p19-state.XXXXXX")
  # The VM home shows as ~ in everything we keep; tailnet addresses, names and capability
  # tokens never reach the file.
  vmsh "$row" <<EOF | sed -E -e 's#/Users/[A-Za-z0-9._-]*#~#g' \
    -e 's#100\.[0-9]+\.[0-9]+\.[0-9]+#100.x.x.N#g' \
    -e 's#[A-Za-z0-9.-]+\.ts\.net#TAILNET-NAME#g' \
    -e 's#cap=[^& "]*#CAP-REDACTED#g' > "$tmp"
$VM_COMMON
row=\${1:--}
echo "p19-state row=\$row vm_utc=\$(date -u +%Y-%m-%dT%H:%M:%SZ) epoch=\$(date +%s)"
echo "installed_version=\$(plutil -extract CFBundleShortVersionString raw "\$APP/Contents/Info.plist" 2>/dev/null || true)"
for L in \$LABEL portty-daemon; do
  echo "--- launchd \$L"
  echo "plist_exists=\$([ -f "\$HOME/Library/LaunchAgents/\$L.plist" ] && echo yes || echo no)"
  [ -f "\$HOME/Library/LaunchAgents/\$L.plist" ] && plutil -p "\$HOME/Library/LaunchAgents/\$L.plist" | tr '\n' ' ' && echo
  launchctl print "gui/\$UIDN/\$L" 2>&1 | grep -E '^[[:space:]]*(state|pid|last exit code|runs) = |Could not find service' | head -8
done
echo "--- launchctl print-disabled (portty labels)"
launchctl print-disabled "gui/\$UIDN" 2>&1 | grep -i 'portty' | grep -v spike
echo "--- processes (pid ppid etime args)"
ps -axww -o pid=,ppid=,etime=,args= | grep '/Portty\.app/Contents/MacOS/Portty' | grep -v grep
echo "--- health"
health; echo
dpids=\$(daemon_pids)
echo "daemon_pids=\$(echo \$dpids)"
for p in \$dpids; do
  echo "daemon \$p ppid=\$(ps -o ppid= -p \$p | tr -d ' ') xpc_service_name=\$(ps eww -p \$p 2>/dev/null | grep -o 'XPC_SERVICE_NAME=[^ ]*' | head -1 | cut -d= -f2)"
done
echo "--- sessions (id, shell pid, alive, ticks, last file tick age, max gap s, last pty tick age)"
now=\$(date +%s)
ids=\$(curl -s -m 3 --unix-socket "\$SOCK" http://localhost/sessions 2>/dev/null | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
for id in \$ids; do
  f="\$HOME/p19/tick-\$id.txt"
  if [ -f "\$f" ]; then
    spid=\$(head -1 "\$f" | cut -d' ' -f1)
    alive=no; kill -0 "\$spid" 2>/dev/null && alive=yes
    stats=\$(awk -v now="\$now" 'NR==1 {prev=\$3} {if (NR>1 && \$3-prev>mx) mx=\$3-prev; prev=\$3; n=NR; last=\$3} END {printf "ticks=%d last_age=%d max_gap=%d", n, now-last, mx}' "\$f")
  else
    spid=-; alive=-; stats="ticks=- (no tick file)"
  fi
  ptick=\$(curl -s -m 3 --unix-socket "\$SOCK" "http://localhost/sessions/\$id/tail" 2>/dev/null | grep -aoE 'TICK [0-9]+ [0-9]+' | tail -1)
  page=-; [ -n "\$ptick" ] && page=\$((now - \$(echo \$ptick | cut -d' ' -f3)))
  echo "session \$id shell_pid=\$spid alive=\$alive \$stats pty_last_tick=\"\$ptick\" pty_age=\$page"
done
echo "--- daemon.log tail"
tail -n 25 "\$CFG/daemon.log" 2>&1
echo "--- gui.log tail (open --stderr, from start-gui)"
tail -n 25 ~/p19/gui.log 2>&1
exit 0
EOF
  cat "$tmp"
  if [ "$row" != "-" ]; then
    mkdir -p "$PHASE_DIR/evidence"
    cp "$tmp" "$PHASE_DIR/evidence/$row.state"
  fi
  rm -f "$tmp"
}

cmd_clean() {
  require_vm
  cmd_quit_all
  vmsh <<EOF
$VM_COMMON
for L in \$LABEL portty-daemon; do
  launchctl bootout "gui/\$UIDN/\$L" 2>/dev/null
  launchctl enable "gui/\$UIDN/\$L" 2>/dev/null
  rm -f "\$HOME/Library/LaunchAgents/\$L.plist"
done
pids=\$(gui_pids; daemon_pids); [ -n "\$pids" ] && kill -9 \$pids 2>/dev/null
rm -rf /Applications/Portty.app ~/p19
exit 0
EOF
}

sub=${1:-}
[ $# -gt 0 ] && shift
case "$sub" in
  build) cmd_build "$@" ;;
  deploy) cmd_deploy "$@" ;;
  start-gui) cmd_start_gui "$@" ;;
  sessions) cmd_sessions "$@" ;;
  state) cmd_state "$@" ;;
  quit-gui) cmd_quit_gui ;;
  quit-all) cmd_quit_all ;;
  legacy-plist) cmd_legacy_plist ;;
  clean) cmd_clean ;;
  *) die "usage: $0 build|deploy|start-gui|sessions|state|quit-gui|quit-all|legacy-plist|clean ..." ;;
esac
