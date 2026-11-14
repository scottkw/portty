/* hubGroupCounts.ts — shared count helpers extracted from GroupSidebar.tsx (POL-05) */
/* ATTN-06: attention means errored */
import { memberKey } from './hubGroups'
import { deriveHubStatus, isAttentionStatus } from './hubStatus'
import type { SessionInfo } from '../wailsjs/go/main/App'

export interface GroupCounts {
  running: number
  total: number
  attention: number  // ATTN-06: errored sessions
}

export function computeCounts(sessions: SessionInfo[], memberKeys: Set<string>): GroupCounts {
  return computeGlobalCounts(sessions.filter((s) => memberKeys.has(memberKey(s.name, s.workDir))))
}

export function computeGlobalCounts(sessions: SessionInfo[]): GroupCounts {
  let running = 0
  let attention = 0
  for (const s of sessions) {
    const st = deriveHubStatus(s)
    if (st === 'running') running++
    if (isAttentionStatus(st)) attention++
  }
  return { running, total: sessions.length, attention }
}
