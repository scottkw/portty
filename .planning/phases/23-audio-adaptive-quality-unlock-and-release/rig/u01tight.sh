#!/usr/bin/env bash
# runs inside portty-test: u01tight.sh VK DELAY [click]   VK: probe wake vk (or "none"); "click" uses the click tool instead of the probe
SOCK="$HOME/Library/Application Support/portty/daemon.sock"
vk=$1; delay=$2; how=${3:-probe}
~/ptest/click2 key 12 cmd ctrl
sleep "$delay"
T=$(date '+%Y-%m-%d %H:%M:%S')
if [ "$vk" != "none" ]; then
  if [ "$how" = click ]; then ~/ptest/click2 key "$vk"; else curl -s -m 10 -X POST --unix-socket "$SOCK" --data-binary "{\"op\":\"wake\",\"vk\":$vk}" http://localhost/desktop/debug/vm20 >/dev/null; fi
fi
sleep 0.8
~/ptest/click2 key 6
sleep 1
~/ptest/click2 key 51
sleep 1.2
E=$(date -v+1S '+%Y-%m-%d %H:%M:%S')
echo "window $T .. $E (vk=$vk delay=$delay how=$how)"
/usr/bin/log show --start "$T" --end "$E" --predicate 'process == "loginwindow"' --style compact 2>/dev/null | grep -E 'keyPressed:\] \||Clearing password|Event: (Will|Did) (Sleep|Wake)|showPasswordFieldMakingFirstResponder|hidePasswordField\] \|' | sed -E 's/^[0-9-]+ ([0-9:.]+) [A-Za-z]+ +loginwindow\[[^]]*\] \[[^]]*\] /\1 /' | cut -c1-150
