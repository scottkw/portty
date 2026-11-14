#!/usr/bin/env bash
# tests/check-traceability-paths.sh
# Verify every test path in TESTING.md traceability table still exists on disk.
# Fails loudly if any path is missing — prevents renamed/deleted tests from
# silently making the traceability map lie.
#
# Parsing convention: traceability table rows have format:
#   | REQ-ID | path/to/test_file.go | group |
# Any table cell of the form "| path.ext |" (.go/.ts/.tsx/.sh) is a path.
# POSIX tools only, so BSD (macOS) and GNU grep behave the same.
#
# Run: bash tests/check-traceability-paths.sh    (from project root)

set -euo pipefail

if [[ ! -f "TESTING.md" ]]; then
  echo "TESTING.md not present yet — skipping path check"
  exit 0
fi

# grep exits 1 on no match; tolerate that so the empty check below reports it.
PATHS=$( { grep -oE '\| [^|]+\.(go|ts|tsx|sh) \|' TESTING.md || true; } \
  | sed -E 's/^\| //; s/ \|$//' | tr -d ' ' | sort -u)

if [[ -z "$PATHS" ]]; then
  echo "ERROR: extracted zero traceability paths from TESTING.md — parser or table format is broken"
  exit 1
fi

COUNT=0
FAIL=0
while IFS= read -r path; do
  COUNT=$((COUNT + 1))
  if [[ ! -e "$path" ]]; then
    echo "MISSING traceability path: $path"
    FAIL=$((FAIL + 1))
  fi
done <<< "$PATHS"

if [[ $FAIL -gt 0 ]]; then
  echo "ERROR: $FAIL traceability path(s) missing — update TESTING.md or restore the test file"
  exit 1
fi
echo "OK: $COUNT traceability paths exist"
