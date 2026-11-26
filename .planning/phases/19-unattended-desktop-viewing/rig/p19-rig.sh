#!/usr/bin/env bash
# p19-rig.sh: VM rig for the Phase 19 launchd start model (19-07), modelled on p18-rig.sh.
#
#   build VERSION OUTDIR [--tags TAGS] [--sign adhoc|devid] [--skip-frontend]
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
# 19-12 (real media path; office = portty-test captures, home = portty-test-2 views; vm2 is the
# ssh helper for the home VM):
#   enable-desktop         office: PUT /desktop on; the link goes to ~/p19/link.txt in both guests
#   safari-stats [SECS] [keep|reuse]   home: safaridriver, open the link#debug, print __desktopStats
#   safari-close           home: end a kept Safari session
#   chrome-stats [SECS]    home: Chrome (temporary profile, DevTools on loopback), print page titles
#   assertions             office: pmset assertions naming Portty
#   lsof-desktop           office: the daemon's UDP/TCP sockets, tailnet address shown generically
#   pcap START|STOP        office: tcpdump counts for UDP 3478, 19302, 5353 by process
#   perm-poll SECS         office: GET /desktop/permissions every 2 s, timestamped
#   tcc-log MINUTES        office: tccd log lines naming app.portty.Portty
#   notif-list             office: Portty's records in the notification database, with a count
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
  local ver=$1 out=$2 tags="wailsassets vmpreview" sign=adhoc skipfe=
  shift 2
  while [ $# -gt 0 ]; do
    case "$1" in
      --tags) tags=$2; shift 2 ;;
      --sign) sign=$2; shift 2 ;;
      --skip-frontend) skipfe=-s; shift ;; # reuse an existing frontend/dist (the build needs no pnpm)
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
  GOTOOLCHAIN=go1.26.11 wails build $skipfe -platform darwin/arm64 -tags "$tags" \
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
  launchctl print "gui/\$UIDN/\$L" 2>&1 | grep -E '^[[:space:]]*(state|pid|last exit code|last terminating signal|runs) = |Could not find service' | head -8
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
echo "--- daemon.log.1 / daemon.log (mtime, size) and daemon.log tail"
for f in daemon.log daemon.log.1; do stat -f "\$f %z bytes mtime %Sm" -t %T "\$CFG/\$f" 2>&1; done
tail -n 25 "\$CFG/daemon.log" 2>&1
echo "--- gui.log tail (open --stderr, from start-gui; update-check noise dropped)"
grep -v 'update: check failed' ~/p19/gui.log 2>&1 | tail -n 25
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

# ---- 19-12: the real media path, office (capture) to home (viewer) -------------------------------
# Everything below acts inside the two VMs. The share link goes office -> home through one ssh
# pipe and is only ever written to ~/p19/link.txt in each guest; no output carries it, a
# tailnet address or a tailnet name (scrub below).

VM2_NAME="${P19_VM2_NAME:-portty-test-2}"
# vm2 runs a command inside the home VM with the same key vm.sh uses (its address from tart).
vm2() {
  ssh -i "$HOME/.ssh/transom_demo_ed25519" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
    -o BatchMode=yes -o ConnectTimeout=10 -o LogLevel=ERROR "admin@$(tart ip "$VM2_NAME")" "$@"
}
vm2sh() { vm2 bash -s -- "$@"; }
require_vm2() {
  tart list | awk -v n="$VM2_NAME" '$2 == n && $NF == "running" {f = 1} END {exit !f}' ||
    die "$VM2_NAME is not running (tart list)"
}
# scrub: the VM home shows as ~; tailnet addresses, names and capability tokens never survive.
scrub() {
  sed -E -e 's#/Users/[A-Za-z0-9._-]*#~#g' -e 's#100\.[0-9]+\.[0-9]+\.[0-9]+#100.x.x.N#g' \
    -e 's#[A-Za-z0-9.-]+\.ts\.net#TAILNET-NAME#g' -e 's#cap=[^& "]*#CAP-REDACTED#g'
}

cmd_enable_desktop() {
  require_vm
  require_vm2
  vmsh <<EOF | scrub
$VM_COMMON
mkdir -p ~/p19; chmod 700 ~/p19
resp=\$(curl -s -m 90 -X PUT --unix-socket "\$SOCK" -H 'Content-Type: application/json' -d '{"alwaysAvailable":true}' http://localhost/desktop)
printf '%s' "\$resp" | jq -r '.url // empty' > ~/p19/link.txt
chmod 600 ~/p19/link.txt
echo "enable-desktop: \$(printf '%s' "\$resp" | jq -c 'del(.url)' 2>&1) link_bytes=\$(wc -c < ~/p19/link.txt | tr -d ' ')"
exit 0
EOF
  [ "$(vm 'wc -c < ~/p19/link.txt | tr -d " "')" -gt 0 ] || die "enable-desktop: the daemon returned no link"
  vm 'cat ~/p19/link.txt' |
    vm2 'mkdir -p ~/p19 && chmod 700 ~/p19 && cat > ~/p19/link.txt && chmod 600 ~/p19/link.txt && echo "link_bytes_home=$(wc -c < ~/p19/link.txt | tr -d " ")"'
}

# safari-stats [SECS] [keep|reuse]: Safari in the home VM through safaridriver. Default opens the
# link with #debug, prints window.__desktopStats and the page title once a second for SECS (20)
# and closes the session; keep leaves the session (and the viewer) open, reuse polls it again.
cmd_safari_stats() {
  local secs=${1:-20} mode=${2:-}
  svreq
  svsh "$secs" "$mode" <<'EOF' | scrub
secs=$1; mode=$2
D=http://127.0.0.1:4444
# 19-15: with a second (terminal) window open, reuse reads the desktop window
if [ "$mode" = reuse ] && [ -f ~/p19/win-desktop.txt ]; then
  curl -s -m 10 -X POST $D/session/$(cat ~/p19/safari.sid)/window -d "$(jq -nc --arg h "$(cat ~/p19/win-desktop.txt)" '{handle:$h}')" >/dev/null
fi
if [ "$mode" != reuse ]; then
  echo admin | sudo -S -p '' safaridriver --enable >/dev/null 2>&1
  pkill -x safaridriver 2>/dev/null; sleep 1
  safaridriver -p 4444 >~/p19/safaridriver.log 2>&1 &
  sleep 2
  SID=$(curl -s -m 60 -X POST $D/session -d '{"capabilities":{"alwaysMatch":{"browserName":"safari"}}}' | jq -r '.value.sessionId // empty')
  [ -n "$SID" ] || { echo "safari-stats: no session"; tail -3 ~/p19/safaridriver.log; exit 1; }
  echo "$SID" > ~/p19/safari.sid
  curl -s -m 60 -X POST $D/session/$SID/url -d "$(jq -nc --arg u "$(cat ~/p19/link.txt)#debug" '{url:$u}')" >/dev/null
fi
SID=$(cat ~/p19/safari.sid)
for i in $(seq 1 $secs); do
  s=$(curl -s -m 10 -X POST $D/session/$SID/execute/sync -d '{"script":"return JSON.stringify(window.__desktopStats||null)","args":[]}' | jq -r '.value|tostring')
  t=$(curl -s -m 10 $D/session/$SID/title | jq -r '.value|tostring')
  echo "p19-safari t=$i stats=$s title=$t"
  sleep 1
done
if [ "$mode" != keep ] && [ "$mode" != reuse ]; then
  curl -s -m 10 -X DELETE $D/session/$SID >/dev/null; pkill -x safaridriver; rm -f ~/p19/safari.sid
fi
exit 0
EOF
}

cmd_safari_close() {
  svreq
  svsh <<'EOF'
[ -f ~/p19/safari.sid ] && curl -s -m 10 -X DELETE http://127.0.0.1:4444/session/$(cat ~/p19/safari.sid) >/dev/null
pkill -x safaridriver; rm -f ~/p19/safari.sid ~/p19/win-desktop.txt ~/p19/win-term.txt
echo "safari session closed"
exit 0
EOF
}

# chrome-stats [SECS]: Chrome with a temporary profile and DevTools on 127.0.0.1 in the home VM;
# prints the page titles (the viewer's debug title carries fd, fw, csp, st) once a second.
cmd_chrome_stats() {
  local secs=${1:-20}
  require_vm2
  vm2sh "$secs" <<'EOF' | scrub
secs=$1
pkill -f p19-chrome-profile 2>/dev/null; sleep 1
rm -rf /tmp/p19-chrome-profile
open -na "Google Chrome" --args --user-data-dir=/tmp/p19-chrome-profile --remote-debugging-port=9222 \
  --remote-debugging-address=127.0.0.1 --no-first-run --no-default-browser-check "$(cat ~/p19/link.txt)#debug"
for i in $(seq 1 $secs); do
  sleep 1
  echo "p19-chrome t=$i titles=$(curl -s -m 3 http://127.0.0.1:9222/json | jq -r '[.[]|select(.type=="page")|.title]|join(" | ")')"
done
pkill -f p19-chrome-profile
exit 0
EOF
}

# assertions: the Portty power assertions in the office VM (pmset), plus the power source.
cmd_assertions() {
  require_vm
  vmsh <<'EOF' | scrub
echo "p19-assertions vm_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
pmset -g batt | head -1
pmset -g assertions | grep -E 'Portty|PreventUserIdle(System|Display)Sleep +[0-9]'
exit 0
EOF
}

# lsof-desktop: the office daemon's UDP and TCP sockets, the VM's tailnet address shown as
# "tailnet address".
cmd_lsof_desktop() {
  require_vm
  vmsh <<EOF | scrub
$VM_COMMON
tip=\$(/usr/local/bin/tailscale ip -4 | head -1)
for p in \$(daemon_pids); do
  echo "p19-lsof daemon pid \$p"
  lsof -nP -a -p \$p -iUDP -iTCP 2>/dev/null | sed -e "s#\$tip#tailnet address#g"
done
exit 0
EOF
}

# pcap START|STOP: tcpdump in the office VM for the STUN, TURN-alt and mDNS ports, with the
# process behind each packet (pktap, -k NP). STOP prints counts only, never packet contents.
cmd_pcap() {
  require_vm
  case "${1:-}" in
    START)
      vmsh <<'EOF'
mkdir -p ~/p19
echo admin | sudo -S -p '' sh -c 'pkill tcpdump; rm -f '"$HOME"'/p19/b1.pcap'
echo admin | sudo -S -p '' sh -c 'nohup tcpdump -i pktap,all -k NP -n -w '"$HOME"'/p19/b1.pcap "udp port 3478 or udp port 19302 or udp port 5353" >'"$HOME"'/p19/tcpdump.log 2>&1 &'
sleep 2
echo "pcap started"
exit 0
EOF
      ;;
    STOP)
      vmsh <<'EOF' | scrub
echo admin | sudo -S -p '' pkill -INT tcpdump
sleep 2
f=$HOME/p19/b1.pcap
echo "p19-pcap total_packets=$(tcpdump -r $f -n 2>/dev/null | wc -l | tr -d ' ')"
echo "p19-pcap portty_packets=$(tcpdump -r $f -k NP -n 2>/dev/null | grep -c '(proc Portty:')"
echo "p19-pcap by_process:"
tcpdump -r $f -k NP -n 2>/dev/null | sed -nE 's/.*\(proc ([^:)]*):.*/\1/p' | sort | uniq -c
exit 0
EOF
      ;;
    *) die "usage: pcap START|STOP" ;;
  esac
}

# perm-poll SECS: GET /desktop/permissions on the office daemon every 2 s, one line per answer
# (guest epoch seconds, so a flip can be timed against the click that caused it).
cmd_perm_poll() {
  local secs=${1:-60}
  require_vm
  vmsh "$secs" <<'EOF' | scrub
SOCK="$HOME/Library/Application Support/portty/daemon.sock"
end=$(( $(date +%s) + $1 ))
while [ "$(date +%s)" -lt "$end" ]; do
  echo "p19-perm epoch=$(date +%s) $(curl -s -m 5 --unix-socket "$SOCK" http://localhost/desktop/permissions)"
  sleep 2
done
exit 0
EOF
}

# tcc-log MINUTES: what tccd logged about Portty in the last MINUTES in the office VM; the long
# hex identifiers are cut so the lines read as one decision each.
cmd_tcc_log() {
  local mins=${1:-5}
  require_vm
  vmsh "$mins" <<'EOF' | scrub
log show --last "${1}m" --predicate 'subsystem == "com.apple.TCC"' --style compact 2>/dev/null |
  grep -i 'app.portty.Portty' | sed -E 's/[0-9a-f]{16,}/HEX/g' | cut -c1-260
exit 0
EOF
}

# notif-list: Portty's records in the office user's notification database (sqlite3, read-only),
# title and body decoded; the count line is what V1 compares before and after a lapse.
cmd_notif_list() {
  require_vm
  vmsh <<'EOF' | scrub
D="$HOME/Library/Group Containers/group.com.apple.usernoted/db2/db"
sqlite3 -readonly "$D" "select r.rec_id, a.identifier, datetime(r.delivered_date + 978307200, 'unixepoch'), hex(r.data) from record r join app a on a.app_id = r.app_id where lower(a.identifier) like '%portty%' order by r.rec_id;" 2>&1 |
while IFS='|' read -r id app dt hex; do
  printf '%s' "$hex" | xxd -r -p > /tmp/p19-notif.plist
  echo "rec=$id app=$app delivered_utc=$dt id=[$(plutil -extract req.iden raw -o - /tmp/p19-notif.plist 2>/dev/null)] title=[$(plutil -extract req.titl raw -o - /tmp/p19-notif.plist 2>/dev/null)] body=[$(plutil -extract req.body raw -o - /tmp/p19-notif.plist 2>/dev/null)]"
done
rm -f /tmp/p19-notif.plist
echo "p19-notif-count=$(sqlite3 -readonly "$D" "select count(*) from record r join app a on a.app_id = r.app_id where lower(a.identifier) like '%portty%';" 2>&1)"
exit 0
EOF
}

# ---- 19-15: the final matrix -------------------------------------------------------------------
# Viewer Safari: home by default; P19_SAFARI_ON=office runs the same commands in the office VM
# (F2's third viewer opens its own link there).
svsh() { if [ "${P19_SAFARI_ON:-home}" = office ]; then vmsh "$@"; else vm2sh "$@"; fi; }
svreq() { if [ "${P19_SAFARI_ON:-home}" = office ]; then require_vm; else require_vm2; fi; }

# safari-wins: with a kept Safari session on the desktop link, open the terminal share link
# (~/p19/term-link.txt in the home VM) in a second window next to it, so neither window is
# occluded and Safari does not throttle either page. readterm.js is shipped to the guest.
cmd_safari_wins() {
  require_vm2
  vm2 'cat > ~/p19/readterm.js' < "$HERE/readterm.js"
  vm2sh <<'EOF' | scrub
D=http://127.0.0.1:4444
SID=$(cat ~/p19/safari.sid) || exit 1
sw() { curl -s -m 10 -X POST $D/session/$SID/window -d "$(jq -nc --arg h "$1" '{handle:$h}')" >/dev/null; }
curl -s -m 10 $D/session/$SID/window | jq -r '.value' > ~/p19/win-desktop.txt
curl -s -m 10 -X POST $D/session/$SID/window/rect -d '{"x":0,"y":30,"width":620,"height":600}' >/dev/null
NEW=$(curl -s -m 20 -X POST $D/session/$SID/window/new -d '{"type":"window"}' | jq -r '.value.handle')
echo "$NEW" > ~/p19/win-term.txt
sw "$NEW"
curl -s -m 10 -X POST $D/session/$SID/window/rect -d '{"x":640,"y":30,"width":620,"height":330}' >/dev/null
curl -s -m 60 -X POST $D/session/$SID/url -d "$(jq -nc --arg u "$(cat ~/p19/term-link.txt)" '{url:$u}')" >/dev/null
sleep 4
sw "$(cat ~/p19/win-desktop.txt)"
echo "safari-wins: desktop window and terminal window open"
exit 0
EOF
}

# safari-both SECS: once a second, the desktop window's [status, state, viewers, framesDecoded]
# and the terminal window's buffer facts (readterm.js), with the guest epoch.
cmd_safari_both() {
  local secs=${1:-20}
  require_vm2
  vm2sh "$secs" <<'EOF' | scrub
secs=$1
D=http://127.0.0.1:4444
SID=$(cat ~/p19/safari.sid); DH=$(cat ~/p19/win-desktop.txt); TH=$(cat ~/p19/win-term.txt)
sw() { curl -s -m 10 -X POST $D/session/$SID/window -d "$(jq -nc --arg h "$1" '{handle:$h}')" >/dev/null; }
ex() { curl -s -m 20 -X POST $D/session/$SID/execute/sync -d "$1"; }
TERMJS=$(jq -nc --rawfile s ~/p19/readterm.js '{script:$s,args:[]}')
STATJS='{"script":"return JSON.stringify(window.__desktopStats||null)","args":[]}'
for i in $(seq 1 $secs); do
  e=$(date +%s)
  sw "$DH"; s=$(ex "$STATJS" | jq -r '.value|tostring' | jq -c '[.status,.state,.viewers,.framesDecoded]' 2>/dev/null)
  sw "$TH"; t=$(ex "$TERMJS" | jq -r '.value|tostring' 2>/dev/null)
  echo "p19-both t=$i epoch=$e desk=${s:-?} term=${t:-?}"
  sleep 1
done
sw "$DH"
exit 0
EOF
}

# safari-page: the desktop window's visible text and title (the static off page and the cards).
cmd_safari_page() {
  svreq
  svsh <<'EOF' | scrub
D=http://127.0.0.1:4444
SID=$(cat ~/p19/safari.sid)
[ -f ~/p19/win-desktop.txt ] && curl -s -m 10 -X POST $D/session/$SID/window -d "$(jq -nc --arg h "$(cat ~/p19/win-desktop.txt)" '{handle:$h}')" >/dev/null
[ "${1:-}" = reload ] && curl -s -m 30 -X POST $D/session/$SID/refresh -d '{}' >/dev/null && sleep 3
echo "p19-page title=$(curl -s -m 10 $D/session/$SID/title | jq -r '.value|tostring')"
echo "p19-page text=$(curl -s -m 10 -X POST $D/session/$SID/execute/sync -d '{"script":"return document.body.innerText.replace(/\\s+/g,\" \").slice(0,300)","args":[]}' | jq -r '.value|tostring')"
exit 0
EOF
}

# chrome-open / chrome-titles SECS / chrome-close: a Chrome viewer that stays open between reads.
cmd_chrome_open() {
  require_vm2
  vm2sh <<'EOF'
pkill -f p19-chrome-profile 2>/dev/null; sleep 1
rm -rf /tmp/p19-chrome-profile
open -na "Google Chrome" --args --user-data-dir=/tmp/p19-chrome-profile --remote-debugging-port=9222 \
  --remote-debugging-address=127.0.0.1 --no-first-run --no-default-browser-check \
  --window-position=640,380 --window-size=620,330 "$(cat ~/p19/link.txt)#debug"
echo "chrome-open: started at guest epoch $(date +%s)"
exit 0
EOF
}
cmd_chrome_titles() {
  local secs=${1:-10}
  require_vm2
  vm2sh "$secs" <<'EOF' | scrub
for i in $(seq 1 $1); do
  echo "p19-chrome t=$i epoch=$(date +%s) titles=$(curl -s -m 3 http://127.0.0.1:9222/json | jq -r '[.[]|select(.type=="page")|.title]|join(" | ")')"
  sleep 1
done
exit 0
EOF
}
cmd_chrome_close() {
  require_vm2
  vm2sh <<'EOF'
pkill -f p19-chrome-profile; rm -rf /tmp/p19-chrome-profile
echo "chrome closed"
exit 0
EOF
}

# pf-block / pf-unblock: drop traffic between the home VM and the office VM's tailnet address,
# with pf in the home VM (anchor com.apple/p19, which /etc/pf.conf already evaluates). The
# address is resolved inside the VM and never printed. Never `tailscale down`.
cmd_pf_block() {
  require_vm2
  vm2sh <<'EOF'
su() { echo admin | sudo -S -p '' "$@"; }
ip=$(/usr/local/bin/tailscale ip -4 portty-vm-office | head -1)
[ -n "$ip" ] || { echo "pf-block: could not resolve the office VM"; exit 1; }
printf 'block drop quick from any to %s\nblock drop quick from %s to any\n' "$ip" "$ip" > /tmp/p19.rules
su pfctl -q -f /etc/pf.conf 2>/dev/null
su pfctl -q -a com.apple/p19 -f /tmp/p19.rules
su pfctl -E 2>&1 | awk '/Token/{print $NF}' >> /tmp/p19.tokens
su pfctl -q -k 0.0.0.0/0 -k "$ip" 2>/dev/null; su pfctl -q -k "$ip" 2>/dev/null
echo "pf-block: anchor com.apple/p19 holds $(su pfctl -a com.apple/p19 -s rules 2>/dev/null | wc -l | tr -d ' ') rules at guest epoch $(date +%s)"
exit 0
EOF
}
cmd_pf_unblock() {
  require_vm2
  vm2sh <<'EOF'
su() { echo admin | sudo -S -p '' "$@"; }
su pfctl -q -a com.apple/p19 -F all 2>/dev/null
for t in $(cat /tmp/p19.tokens 2>/dev/null); do su pfctl -q -X "$t" 2>/dev/null; done
rm -f /tmp/p19.tokens /tmp/p19.rules
echo "pf-unblock: anchor rules=$(su pfctl -a com.apple/p19 -s rules 2>/dev/null | wc -l | tr -d ' ') at guest epoch $(date +%s); $(su pfctl -s info 2>/dev/null | head -1)"
exit 0
EOF
}

# lock / unlock (office, the 17-06 method): Ctrl+Cmd+Q through the click tool; unlock types the
# VM's public password as key codes (a d m i n, then Return) after a user-activity wake.
cmd_lock() {
  require_vm
  vmsh <<'EOF'
~/ptest/click2 key 12 cmd ctrl
sleep 3
echo "lock: pressed at guest epoch $(date +%s); GET /desktop locked=$(curl -s -m 3 --unix-socket "$HOME/Library/Application Support/portty/daemon.sock" http://localhost/desktop | jq -r .locked)"
exit 0
EOF
}
cmd_unlock() {
  require_vm
  vmsh <<'EOF'
caffeinate -u -t 2; sleep 1
for k in 0 2 46 34 45 36; do ~/ptest/click2 key $k; sleep 0.2; done
sleep 3
echo "unlock: typed at guest epoch $(date +%s); GET /desktop locked=$(curl -s -m 3 --unix-socket "$HOME/Library/Application Support/portty/daemon.sock" http://localhost/desktop | jq -r .locked)"
exit 0
EOF
}

# tick-gaps START END (guest epochs): the ticking shells' gaps over 2 s inside the window, so a
# gap is attributed to the right moment (state's max_gap covers the whole file). Also reports the
# tty-less 0.2 s builtin monitor (~/p19/mon.txt) and the fork+exec latency monitor
# (~/p19/exec.txt) if they run; both are started by hand in the guest, see 19-VM-CHECKS.md F5.
cmd_tick_gaps() {
  [ $# -eq 2 ] || die "usage: tick-gaps START_EPOCH END_EPOCH"
  require_vm
  vmsh "$1" "$2" <<'EOF'
s=$1; e=$2
for f in ~/p19/tick-*.txt; do
  n=$(awk -v s=$s -v e=$e '$3>=s && $3<=e' "$f" | wc -l | tr -d ' ')
  [ "$n" -gt 0 ] || continue
  echo "session $(basename "$f" .txt | sed -E 's/tick-(.{8}).*/\1/') ticks_in_window=$n"
  awk -v s=$s -v e=$e '$3>=s && $3<=e && NR>1 && $3-prev>2 {print "  tick gap " $3-prev " s ending at +" $3-s} {prev=$3}' "$f"
done
if [ -f ~/p19/mon.txt ]; then
  echo "builtin monitor: samples=$(awk -v s=$s -v e=$e '$1>=s && $1<=e' ~/p19/mon.txt | wc -l | tr -d ' ') max_gap_s=$(awk -v s=$s -v e=$e '$1>=s && $1<=e && NR>1 {if ($1-prev>m) m=$1-prev} {prev=$1} END {printf "%.2f", m}' ~/p19/mon.txt)"
fi
if [ -f ~/p19/exec.txt ]; then
  echo "exec monitor: samples=$(awk -v s=$s -v e=$e '$1>=s && $1<=e' ~/p19/exec.txt | wc -l | tr -d ' ') max_exec_latency_s=$(awk -v s=$s -v e=$e '$1>=s && $1<=e {if ($2>m) m=$2} END {printf "%.2f", m}' ~/p19/exec.txt)"
  awk -v s=$s -v e=$e '$1>=s && $1<=e && $2>0.5 {printf "  slow exec %.2f s at +%.1f\n", $2, $1-s}' ~/p19/exec.txt | head -12
fi
exit 0
EOF
}

# pbpaste-check (office): says only whether the clipboard holds the desktop link, its length and
# whether it equals the daemon's own link (compared in the guest); then clears the clipboard.
cmd_pbpaste_check() {
  require_vm
  vmsh <<'EOF'
SOCK="$HOME/Library/Application Support/portty/daemon.sock"
c=$(pbpaste)
if printf '%s' "$c" | grep -q '/desktop/?cap='; then yes=yes; else yes=no; fi
cur=$(curl -s -m 5 --unix-socket "$SOCK" http://localhost/desktop/link | jq -r '.url // empty' 2>/dev/null)
same=no; [ -n "$cur" ] && [ "$c" = "$cur" ] && same=yes
echo "desktop link on clipboard: $yes (length ${#c}); same as the daemon's link: $same"
pbcopy < /dev/null
exit 0
EOF
}

# tray-items (office): the GUI's status item menu, read through Accessibility with the axtray
# helper (tray.swift, built on the host and copied to ~/p19/axtray on first use). It opens the
# menu, prints every titled item with enabled=yes/no, and closes it again.
cmd_tray_items() {
  require_vm
  if ! vm '[ -x ~/p19/axtray ]'; then
    local tmp
    tmp=$(mktemp -d "${TMPDIR:-/tmp}/p19-axtray.XXXXXX")
    swiftc -O "$HERE/tray.swift" -o "$tmp/axtray" 2>&1 | grep -v warning || true
    vm 'mkdir -p ~/p19 && cat > ~/p19/axtray && chmod +x ~/p19/axtray' < "$tmp/axtray"
    rm -rf "$tmp"
  fi
  vm '~/p19/axtray app.portty.Portty' | scrub
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
  enable-desktop) cmd_enable_desktop ;;
  safari-stats) cmd_safari_stats "$@" ;;
  safari-close) cmd_safari_close ;;
  chrome-stats) cmd_chrome_stats "$@" ;;
  assertions) cmd_assertions ;;
  lsof-desktop) cmd_lsof_desktop ;;
  pcap) cmd_pcap "$@" ;;
  perm-poll) cmd_perm_poll "$@" ;;
  tcc-log) cmd_tcc_log "$@" ;;
  notif-list) cmd_notif_list ;;
  safari-wins) cmd_safari_wins ;;
  safari-both) cmd_safari_both "$@" ;;
  safari-page) cmd_safari_page "$@" ;;
  chrome-open) cmd_chrome_open ;;
  chrome-titles) cmd_chrome_titles "$@" ;;
  chrome-close) cmd_chrome_close ;;
  pf-block) cmd_pf_block ;;
  pf-unblock) cmd_pf_unblock ;;
  lock) cmd_lock ;;
  unlock) cmd_unlock ;;
  pbpaste-check) cmd_pbpaste_check ;;
  tray-items) cmd_tray_items ;;
  tick-gaps) cmd_tick_gaps "$@" ;;
  *) die "usage: $0 build|deploy|start-gui|sessions|state|quit-gui|quit-all|legacy-plist|clean|enable-desktop|safari-stats|safari-close|chrome-stats|assertions|lsof-desktop|pcap|perm-poll|tcc-log|notif-list|safari-wins|safari-both|safari-page|chrome-open|chrome-titles|chrome-close|pf-block|pf-unblock|lock|unlock|pbpaste-check|tray-items ..." ;;
esac
