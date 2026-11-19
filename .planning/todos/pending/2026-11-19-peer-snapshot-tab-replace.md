---
created: 2026-11-19T22:00:00Z
title: Drop peer snapshots when retryInit replaces tabs, and key peer tabs by host
area: frontend
severity: minor
source: .planning/phases/08-debt-cleanup/08-REVIEW.md IN-07 (deferred in 08-REVIEW-FIX.md)
files:
  - frontend/src/App.tsx
---

## Problem

Snapshot host keying does not cover the paths that replace tabs:

1. `retryInit` (`frontend/src/App.tsx`, its `setTabs(restoredTabs)`) replaces every tab and drops peer tabs without calling `dropPeerSnapshot`, so their snapshots are orphaned.
2. Peer tab ids are keyed by session id alone (`webSessionTabId(sessionId)`). Two Macs that hand out equal session ids share one tab (`openWebSessionTab` replaces its peer fields), and the old host's snapshot is orphaned under its old key.

The cost is bounded memory, not correctness:
- Orphaned entries are capped by `PEER_SNAPSHOT_MAX` and `PEER_SNAPSHOT_MAX_BYTES`.
- A stale entry stays self-consistent.
- `retryInit` only runs after a daemon error.

## Why not fixed in 08-REVIEW-FIX

The `retryInit` drop is a few lines but needs an App-harness test. The equal-session-id-across-hosts half needs `webSessionTabId` keyed by host, which is a tab-identity change outside a review-fix iteration.

## Solution

1. In `retryInit`, call `dropPeerSnapshot` for every peer tab that `restoredTabs` removes. Test it in an App harness: a peer tab with a stored snapshot, a daemon error, `retryInit`, and the snapshot is gone.
2. If equal session ids across hosts are real, key `webSessionTabId` by host as well, and check every caller of the tab id (tab restore, close, snapshot keys).
