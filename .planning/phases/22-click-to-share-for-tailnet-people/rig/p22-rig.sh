#!/usr/bin/env bash
# p22-rig.sh: VM rig for the Phase 22 click-to-share rows (22-12), built on p20-rig.sh and p19-rig.sh.
#
# Roles: office = portty-test (the shared Mac: Portty GUI, daemon, TextEdit, Terminal, the probe route);
# guest = portty-test-2 (the person who opens the guest link: Safari through safaridriver).
# Every capture, input, AX call and launchctl call runs inside one of the two VMs through ssh;
# the host only builds a scratch copy (compile only), copies it over and writes scrubbed evidence.
# Nothing here contacts a 100.x address or the host's tailscaled, and nothing stops a VM.
#
# The guest link travels office -> guest through one ssh pipe into ~/p19/link.txt (mode 0600) on the
# guest VM, where p19/p20's Safari helpers read it. It is never printed and never reaches a committed
# file: scrub removes cap= values, tailnet names and addresses, e-mail-shaped login names, and
# whatever sed expressions the optional file P22_SCRUB_EXTRA lists (names of real people, kept out of
# the repo).
#
# Host, build and install:
#   build22 VERSION OUTDIR     scratch build of HEAD, tags "wailsassets vmdesktop", Developer ID signature
#   install APP                office: p19 deploy of APP (quit GUI and daemon, replace)
#   start                      office: p19 start-gui
#   tools22                    host compiles rig/axp22.swift and rig/../21 key.swift; installs ~/p22/axp22
#                              and ~/p22/key in the office
# Office, the shared Mac:
#   ax-gui [ROW]               Portty's accessibility tree, scrubbed; with ROW also saved as evidence/ROW.ax
#   ax-find WORDS              tree lines naming WORDS
#   ax-has WORDS               yes|no
#   press WORDS                AXPress the first button/switch/checkbox naming WORDS (Portty)
#   tray items|press PREFIX|icon   the tray menu through AX
#   share-api METHOD PATH [JSON]   a daemon-socket call; any url is removed from the answer
#   share-link                 GET /desktop/share, link piped to the guest VM (never printed)
#   share-status               GET /desktop/share, link removed
#   audit-dump                 desktop-audit.jsonl: mode, event counts, the lines; fails on cap= or a token
#   stand-in-input X Y         one untagged ~/ptest/click at a point (software stand-in for hardware)
#   local-key CODE [mods...]   one untagged key post through ~/p22/key (a software stand-in)
#   keystate                   which keys the OS holds down
#   secure-input on|off        Secure Keyboard Entry through the Terminal menu (AX), Terminal frontmost
#   gui-quit                   SIGTERM the Portty GUI process only (the daemon stays)
#   clock                      the office's epoch seconds with milliseconds
#   dlog [N]                   last N daemon.log lines (scrubbed)
#   menubar-shot OUT           screencapture of the menu-bar corner holding Portty's status item
#   probe OP [JSON]            p20 probe passthrough (counters etc.)
#   counters                   the Manager's input counters (probe control), compact
# Guest, the person with the link:
#   guest-open                 safaridriver session at ~/p19/link.txt#debug (window 1, session kept)
#   guest-window2              a second Safari window on the same link (window 2)
#   guest-window N             make window N current (1 or 2)
#   guest-text                 title, visible text and debug stats of the current window, scrubbed
#   guest-close                end the Safari session
#   guest-hold KEY SECS        WebDriver key down, hold SECS seconds, key up (blocks that long)
#   p20 ARGS...                p20-rig.sh passthrough (safari-js, safari-click, safari-type, textedit-new, ...)
#   p19 ARGS...                p19-rig.sh passthrough
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PHASE_DIR="$(dirname "$HERE")"
PLANNING="$(dirname "$(dirname "$PHASE_DIR")")"
ROOT="$(git -C "$HERE" rev-parse --show-toplevel)"
P19="$ROOT/.planning/phases/19-unattended-desktop-viewing/rig/p19-rig.sh"
P20="$ROOT/.planning/phases/20-remote-control/rig/p20-rig.sh"
KEY_SRC="$ROOT/.planning/phases/21-portty-app-and-iphone-viewers/rig/key.swift"
VM_SH="${P22_VM_SH:-$HOME/.local/share/portty-vm-tools/vm.sh}"
VM_NAME="${P22_VM_NAME:-portty-test}"
VM2_NAME="${P22_VM2_NAME:-portty-test-2}"
SOCKQ='"$HOME/Library/Application Support/portty/daemon.sock"'
BUNDLE=app.portty.Portty

die() { echo "p22-rig: $*" >&2; exit 1; }
vm() { "$VM_SH" "$@"; }
vmsh() { "$VM_SH" bash -s -- "$@"; }
vm2() {
  ssh -i "$HOME/.ssh/transom_demo_ed25519" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
    -o BatchMode=yes -o ConnectTimeout=10 -o LogLevel=ERROR "admin@$(tart ip "$VM2_NAME")" "$@"
}
vm2sh() { vm2 bash -s -- "$@"; }
require_vms() {
  tart list | awk -v n="$VM_NAME" '$2 == n && $NF == "running" {f = 1} END {exit !f}' || die "$VM_NAME is not running"
  tart list | awk -v n="$VM2_NAME" '$2 == n && $NF == "running" {f = 1} END {exit !f}' || die "$VM2_NAME is not running"
}
# scrub: VM home shows as ~; tailnet addresses and names, capability tokens, e-mail-shaped login
# names and the VMs' short names never survive. P22_SCRUB_EXTRA names a file of extra sed -E
# expressions (real people's display names); it is never committed.
scrub() {
  sed -E -e 's#/Users/[A-Za-z0-9._-]*#~#g' -e 's#100\.[0-9]+\.[0-9]+\.[0-9]+#100.x.x.N#g' \
    -e 's#[A-Za-z0-9.-]+\.ts\.net#TAILNET-NAME#g' -e 's#cap=[^& "]*#CAP-REDACTED#g' \
    -e 's#[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+#LOGIN@REDACTED#g' \
    -e 's#portty-vm-office#OFFICE#g' -e 's#portty-vm-home#HOME#g' |
    sed -E -f "${P22_SCRUB_EXTRA:-/dev/null}"
}

# build22 VERSION OUTDIR: a git archive of HEAD in OUTDIR/tree (scratch, outside the repo), the main
# checkout's frontend/node_modules linked in, frontend build, wails build (tags wailsassets vmdesktop:
# the VM-only probe route; NOT vmpreview, which fakes Tailscale), Developer ID signature.
cmd_build22() {
  [ $# -eq 2 ] || die "usage: build22 VERSION OUTDIR"
  local ver=$1 out=$2 nm hash
  [ ! -e "$out/Portty.app" ] || die "$out/Portty.app already exists; use a fresh OUTDIR"
  mkdir -p "$out"
  case "$(cd "$out" && pwd -P)" in
    "$(cd "$ROOT" && pwd -P)"/*) die "OUTDIR must be outside the repo (scratch directory)" ;;
  esac
  nm="$ROOT/frontend/node_modules"
  [ -d "$nm" ] || nm="$(git -C "$ROOT" worktree list --porcelain | awk '/^worktree /{print $2; exit}')/frontend/node_modules"
  [ -d "$nm" ] || die "no frontend/node_modules to link (no package install is done here)"
  mkdir -p "$out/tree"
  git -C "$ROOT" rev-parse HEAD > "$out/build-commit"
  git -C "$ROOT" archive HEAD | tar -x -C "$out/tree"
  ln -s "$nm" "$out/tree/frontend/node_modules"
  jq --arg v "$ver" '.info.productVersion = $v' "$out/tree/wails.json" > "$out/tree/wails.json.tmp"
  mv "$out/tree/wails.json.tmp" "$out/tree/wails.json"
  (cd "$out/tree/frontend" && ./node_modules/.bin/tsc && ./node_modules/.bin/vite build)
  (cd "$out/tree" && GOTOOLCHAIN="${GOTOOLCHAIN:-go1.26.11}" wails build -s -platform darwin/arm64 \
    -tags "wailsassets vmdesktop" -ldflags "-X main.Version=v$ver")
  cp -R "$out/tree/build/bin/Portty.app" "$out/Portty.app"
  hash=$(security find-identity -v -p codesigning | awk '/"Developer ID Application: / {print $2; exit}')
  [ -n "$hash" ] || die "no Developer ID Application identity"
  codesign --deep --force --options runtime --timestamp --entitlements "$out/tree/build/entitlements.plist" \
    --identifier app.portty.Portty --sign "$hash" "$out/Portty.app"
  codesign -dv "$out/Portty.app" 2>&1 | grep -E '^(Identifier|Signature|TeamIdentifier)=' || true
  echo "$out/Portty.app (commit $(cat "$out/build-commit"))"
}

cmd_install() { [ $# -eq 1 ] || die "usage: install APP"; bash "$P19" deploy "$1"; }
cmd_start() { bash "$P19" start-gui "${1:-30}"; }

cmd_tools22() {
  require_vms
  local tmp
  tmp=$(mktemp -d "${TMPDIR:-/tmp}/p22-tools.XXXXXX")
  swiftc -O "$HERE/axp22.swift" -o "$tmp/axp22" 2>&1 | grep -v warning || true
  swiftc -O "$KEY_SRC" -o "$tmp/key" 2>&1 | grep -v warning || true
  vm 'mkdir -p ~/p22 && cat > ~/p22/axp22 && chmod +x ~/p22/axp22' < "$tmp/axp22"
  vm 'cat > ~/p22/key && chmod +x ~/p22/key' < "$tmp/key"
  rm -rf "$tmp"
  echo "tools22: axp22 and key installed in the office"
}

# ---- office -------------------------------------------------------------------------------------

cmd_ax_gui() {
  require_vms
  mkdir -p "$PHASE_DIR/evidence"
  if [ -n "${1:-}" ]; then
    vm "~/ptest/axdump2 $BUNDLE" | scrub | tee "$PHASE_DIR/evidence/$1.ax"
  else
    vm "~/ptest/axdump2 $BUNDLE" | scrub
  fi
}
cmd_ax_find() { vm "~/p22/axp22 find $BUNDLE $*" | scrub; }
cmd_ax_has() { vm "~/p22/axp22 has $BUNDLE $*" || true; }
cmd_press() { vm "~/p22/axp22 press $BUNDLE $*" | scrub; }
cmd_tray() { vm "~/p22/axp22 tray $BUNDLE $*" | scrub; }

cmd_share_api() {
  [ $# -ge 2 ] || die "usage: share-api METHOD PATH [JSON]"
  local method=$1 path=$2 body=${3:-}
  if [ -n "$body" ]; then
    printf '%s' "$body" |
      vm "curl -s -m 30 -X $method --unix-socket $SOCKQ -H 'Content-Type: application/json' --data-binary @- http://localhost$path"
  else
    vm "curl -s -m 30 -X $method --unix-socket $SOCKQ http://localhost$path"
  fi | jq -c 'del(.url)' 2>/dev/null | scrub || true
}

cmd_share_status() { cmd_share_api GET /desktop/share; }

# share-link: GET /desktop/share on the office; the url goes over one ssh pipe to ~/p19/link.txt on the
# guest VM (0600) and is never printed.
cmd_share_link() {
  require_vms
  vm "curl -s -m 10 --unix-socket $SOCKQ http://localhost/desktop/share | jq -r '.url // empty'" |
    vm2 'mkdir -p ~/p19 && chmod 700 ~/p19 && cat > ~/p19/link.txt && chmod 600 ~/p19/link.txt && echo "link_bytes_guest=$(wc -c < ~/p19/link.txt | tr -d " ")"'
}

# audit-dump: the file's mode, a count per event, the lines (scrubbed) and a hard check that no cap= and
# no token-shaped string is in them. Exit 1 when the check fails.
cmd_audit_dump() {
  require_vms
  local out
  out=$(vmsh <<'EOF'
F="$HOME/Library/Application Support/portty/desktop-audit.jsonl"
[ -f "$F" ] || { echo "audit: no file"; exit 3; }
echo "audit: mode=$(stat -f %Lp "$F") bytes=$(wc -c < "$F" | tr -d ' ') lines=$(wc -l < "$F" | tr -d ' ')"
echo "audit: events: $(jq -r '.event' "$F" | sort | uniq -c | awk '{printf "%s=%s ", $2, $1}')"
echo "audit: leave/stop reasons: $(jq -r 'select(.event=="leave" or .event=="share_stop" or .event=="revoked") | .event+":"+(.reason//"")' "$F" | sort | uniq -c | awk '{printf "%s x%s  ", $2, $1}')"
if grep -qE 'cap=|[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}|[A-Za-z0-9_-]{40,}' "$F"; then echo "audit: SECRET-SHAPED STRING PRESENT"; else echo "audit: no cap= and no token-shaped string"; fi
echo "audit: lines follow (ts event mode reason share_id-length viewer)"
jq -r '[.ts, .event, .mode, (.reason//""), ((.share_id//"")|length|tostring), (.node_name//"")] | join(" ")' "$F" | tail -${1:-60}
exit 0
EOF
)
  printf '%s\n' "$out" | scrub
  printf '%s' "$out" | grep -q 'SECRET-SHAPED' && return 1
  return 0
}

cmd_stand_in_input() {
  [ $# -eq 2 ] || die "usage: stand-in-input X Y"
  require_vms
  vm "~/ptest/click $1 $2"
}
cmd_local_key() { require_vms; vm "~/p22/key key $*"; }
cmd_keystate() { require_vms; vm '~/p21/keystate'; }

cmd_secure_input() {
  require_vms
  case "${1:-}" in on | off) ;; *) die "usage: secure-input on|off" ;; esac
  vmsh "$1" <<'EOF'
want=$1
open -a Terminal; sleep 2
# The system-wide fact the daemon reads (SecureEventInputIsEnabled) shows in the IO registry as the pid that holds it.
si() { ioreg -l -w 0 2>/dev/null | grep -c kCGSSessionSecureInputPID; }
mark() { ~/p22/axp22 find com.apple.Terminal "Secure Keyboard Entry" | grep -o 'mark=\[[^]]*\]' | head -1; }
on=off; [ "$(si)" -gt 0 ] && on=on
echo "secure-input: before=$on menu-item $(mark) want=$want"
[ "$on" != "$want" ] && ~/ptest/axmenu com.apple.Terminal Terminal "Secure Keyboard Entry"
sleep 1
on=off; [ "$(si)" -gt 0 ] && on=on
echo "secure-input: after=$on menu-item $(mark) frontmost=$(~/p20/ax20 front x)"
exit 0
EOF
}

# notif: Portty's notification records (p19 notif-list) and Notification Center's banners by AX.
cmd_notif() {
  bash "$P19" notif-list | tail -"${1:-6}"
  vm "~/ptest/axdump2 com.apple.notificationcenterui 2>&1 | grep -E 'AXStaticText|AXButton' | sed -E 's/ DOMClassList=.*//' | head -12" | scrub
}

cmd_gui_quit() { bash "$P19" quit-gui; }
cmd_clock() { require_vms; vm "perl -MTime::HiRes=time -e 'printf q(%.3f), time'"; echo; }
cmd_dlog() { bash "$P20" dlog "${1:-40}"; }

# menubar-shot OUT.png: the menu-bar corner (right 400 points, 26 high) inside the office; the file is a
# plain image of the VM's own menu bar and is copied back through the ssh pipe.
cmd_menubar_shot() {
  [ $# -eq 1 ] || die "usage: menubar-shot OUT.png"
  require_vms
  vm 'screencapture -x -R 900,0,160,26 /tmp/p22-bar.png && cat /tmp/p22-bar.png && rm -f /tmp/p22-bar.png' > "$1"
  echo "menubar-shot: $(wc -c < "$1" | tr -d ' ') bytes"
}

cmd_counters() {
  bash "$P20" probe control | jq -c '{counters, pseudo_counters}' 2>/dev/null | scrub
}

# ---- guest --------------------------------------------------------------------------------------

cmd_guest_open() { bash "$P20" safari-open-viewer | tail -1; }

cmd_guest_window2() {
  require_vms
  vm2sh <<'EOF'
D=http://127.0.0.1:4444; SID=$(cat ~/p19/safari.sid)
mkdir -p ~/p22
curl -s -m 10 $D/session/$SID/window | jq -r '.value' > ~/p22/win1.txt
h2=$(curl -s -m 20 -X POST $D/session/$SID/window/new -d '{"type":"window"}' | jq -r '.value.handle')
echo "$h2" > ~/p22/win2.txt
curl -s -m 20 -X POST $D/session/$SID/window -d "$(jq -nc --arg h "$h2" '{handle:$h}')" >/dev/null
curl -s -m 60 -X POST $D/session/$SID/url -d "$(jq -nc --arg u "$(cat ~/p19/link.txt)#debug" '{url:$u}')" >/dev/null
echo "guest-window2: opened (handles $(wc -c < ~/p22/win1.txt | tr -d ' ') and $(wc -c < ~/p22/win2.txt | tr -d ' ') bytes)"
exit 0
EOF
}

cmd_guest_window() {
  [ $# -eq 1 ] || die "usage: guest-window 1|2"
  vm2sh "$1" <<'EOF'
D=http://127.0.0.1:4444; SID=$(cat ~/p19/safari.sid)
curl -s -m 20 -X POST $D/session/$SID/window -d "$(jq -nc --arg h "$(cat ~/p22/win$1.txt)" '{handle:$h}')" | jq -c '.value'
EOF
}

# guest-close-window2: close Safari window 2 (its viewer leaves) and make window 1 current again.
cmd_guest_close_window2() {
  vm2sh <<'EOF'
D=http://127.0.0.1:4444; SID=$(cat ~/p19/safari.sid)
curl -s -m 20 -X POST $D/session/$SID/window -d "$(jq -nc --arg h "$(cat ~/p22/win2.txt)" '{handle:$h}')" >/dev/null
curl -s -m 20 -X DELETE $D/session/$SID/window >/dev/null
curl -s -m 20 -X POST $D/session/$SID/window -d "$(jq -nc --arg h "$(cat ~/p22/win1.txt)" '{handle:$h}')" >/dev/null
echo "guest-close-window2: window 2 closed, window 1 current"
EOF
}

cmd_guest_text() {
  bash "$P20" safari-js 'return JSON.stringify({title: document.title, text: document.body.innerText.replace(/\s+/g, " ").slice(0, 600), stats: window.__desktopStats || null})' | scrub
}

cmd_guest_close() {
  require_vms
  vm2 'D=http://127.0.0.1:4444; [ -f ~/p19/safari.sid ] && curl -s -m 10 -X DELETE $D/session/$(cat ~/p19/safari.sid) >/dev/null; pkill -x safaridriver; rm -f ~/p19/safari.sid ~/p19/win-desktop.txt ~/p19/win-term.txt ~/p22/win1.txt ~/p22/win2.txt; echo guest-close: done'
}

# guest-hold KEY SECS: WebDriver key down on KEY, SECS of pause, key up. Blocks that long.
cmd_guest_hold() {
  [ $# -eq 2 ] || die "usage: guest-hold KEY SECS"
  bash "$P20" safari-actions "$(jq -nc --arg k "$1" --argjson ms "$(( $2 * 1000 ))" \
    '{actions:[{type:"key",id:"kb",actions:[{type:"keyDown",value:$k},{type:"pause",duration:$ms},{type:"keyUp",value:$k}]}]}')" >/dev/null
}

sub=${1:-}
[ $# -gt 0 ] && shift
case "$sub" in
  build22) cmd_build22 "$@" ;;
  install) cmd_install "$@" ;;
  start) cmd_start "$@" ;;
  tools22) cmd_tools22 ;;
  ax-gui) cmd_ax_gui "$@" ;;
  ax-find) cmd_ax_find "$@" ;;
  ax-has) cmd_ax_has "$@" ;;
  press) cmd_press "$@" ;;
  tray) cmd_tray "$@" ;;
  share-api) cmd_share_api "$@" ;;
  share-link) cmd_share_link ;;
  share-status) cmd_share_status ;;
  audit-dump) cmd_audit_dump "$@" ;;
  stand-in-input) cmd_stand_in_input "$@" ;;
  local-key) cmd_local_key "$@" ;;
  keystate) cmd_keystate ;;
  secure-input) cmd_secure_input "$@" ;;
  gui-quit) cmd_gui_quit ;;
  notif) cmd_notif "$@" ;;
  clock) cmd_clock ;;
  dlog) cmd_dlog "$@" ;;
  menubar-shot) cmd_menubar_shot "$@" ;;
  counters) cmd_counters ;;
  probe) bash "$P20" probe "$@" ;;
  guest-open) cmd_guest_open ;;
  guest-window2) cmd_guest_window2 ;;
  guest-window) cmd_guest_window "$@" ;;
  guest-close-window2) cmd_guest_close_window2 ;;
  guest-text) cmd_guest_text ;;
  guest-close) cmd_guest_close ;;
  guest-hold) cmd_guest_hold "$@" ;;
  p20) bash "$P20" "$@" ;;
  p19) bash "$P19" "$@" ;;
  *) die "usage: $0 build22|install|start|tools22|ax-gui|ax-find|ax-has|press|tray|share-api|share-link|share-status|audit-dump|stand-in-input|local-key|keystate|secure-input|gui-quit|clock|dlog|menubar-shot|counters|probe|guest-open|guest-window2|guest-window|guest-text|guest-close|guest-hold|p20|p19" ;;
esac
