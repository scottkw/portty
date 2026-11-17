---
created: 2026-11-17T20:32:19Z
title: Send the catch-up replay in chunks of 32 KiB or less
area: relay
severity: minor
files:
  - internal/relay/scrollback.go
---

## Problem

Phase 4 review WR-03: relay.ViewerReadLimit (1 MiB, 04-10) is enforced on the raw-ring path but only estimated on the wrapped-ring path (RenderSnapshot of the current screen). A very wide truecolor screen could exceed 1 MiB and bring back the gap-4 joiner drop.

## Solution

Send the preamble as a sequence of frames of at most 32 KiB so no reader needs a raised limit; then the 1 MiB limit can stay as a backstop or be removed.
