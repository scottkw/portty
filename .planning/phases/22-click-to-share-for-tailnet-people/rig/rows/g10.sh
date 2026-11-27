#!/usr/bin/env bash
# g10.sh: row G-10. The office's desktop-audit.jsonl after every row above: mode 0600, the expected events and
# reasons, and no cap= or token-shaped string. Usage: g10.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"
row_begin G-10 "audit file: mode, events, no secrets"

step "audit file (names scrubbed; the line list is the whole file, ts event mode reason share_id-length device)"
if r audit-dump 400 >> "$EV" 2>&1; then ev "secret check exit: 0"; else ev "secret check exit: NONZERO"; fi
OUT=$(r audit-dump 400)
for need in share_start admit_request admitted join control_granted control_revoked revoked leave share_stop; do
  ev "event $need present: $(grep -c " $need " <<<"$OUT" | tr -d ' ') line(s)"
done
ev "share_stop reasons seen: $(grep ' share_stop ' <<<"$OUT" | awk '{print $4}' | sort | uniq -c | tr '\n' ';')"
ev "leave reasons seen: $(grep ' leave ' <<<"$OUT" | awk '{print $4}' | sort | uniq -c | tr '\n' ';')"
step "done"
cat "$EV" | head -20
