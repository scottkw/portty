#!/usr/bin/env bash
# g03.sh: row G-03. Revoke mid-key: a controlling guest holds the A key; Disconnect on its row releases the
# key on the office (the office's key state is polled every 0.1 s inside the office), the guest page says
# You were disconnected, and the audit file holds revoked. Usage: g03.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"
row_begin G-03 "revoke while the guest holds a key"

step "pre: a new share with one admitted guest holding control"
stop_leftover
ensure_admitted
ensure_control
close_share_sheet
office_textedit
guest_click_text
ev "keystate before: $(r keystate)"
ev "key-state poller: $(r keylog-start)"

step "1: the guest holds A for 100 s (WebDriver key down, pause, key up), started in the background"
r guest-hold a 100 &
HP=$!
sleep 8
ev "document while held: $(doc_text)"
ev "poller transitions so far (epoch keystate):"
r keylog-transitions >> "$EV"

step "2: Disconnect pressed on the guest's row (AX), timed inside the office"
open_share_sheet
ev "$(r press-timed Disconnect | tr '\n' ' ')"
sleep 5
ev "poller transitions (epoch to 10 ms, held-key set changes):"
r keylog-transitions >> "$EV"
ev "guest page: $(wait_guest_has 'You were disconnected' 10)"
ev "guest-text: $(r guest-text | jq -r .text | cut -c 150-400)"
ev "share-status: $(r share-status | jq -c '{active, viewers: (.viewers | length), pending: (.pending | length)}')"
ev "keystate now (the guest's own key up has not run yet): $(r keystate)"
ev "audit (last 6 lines, ts event mode reason): "
r audit-dump 6 | tail -6 >> "$EV"
kill $HP 2>/dev/null
step "done"
cat "$EV"
