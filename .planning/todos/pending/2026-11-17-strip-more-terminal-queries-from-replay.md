---
created: 2026-11-17T20:32:19Z
title: Strip remaining reply-eliciting terminal queries from the catch-up replay
area: relay
severity: minor
files:
  - internal/relay/hub.go
---

## Problem

Phase 4 review WR-02 (04-REVIEW.md, 2026-11-17): Hub.ReconnectPreamble's query strip (and its byte-identical copy in internal/daemon/engine.go) misses CSI 14/16/18 t, XTGETTCAP, DECRQSS (neovim sends both at startup), DA3, OSC 4 palette queries and OSC 52 clipboard reads, so a joining terminal still answers them into the shared shell. The hub.go doc comment overstates completeness, and 'mode sets stay' is wrong (CSI ? 2048 h is stripped).

## Solution

Extend both patterns identically, keep a test per added sequence, fix the doc comment. Known limit stays: a query split across two PTY reads is not stripped.
