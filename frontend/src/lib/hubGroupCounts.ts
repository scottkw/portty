/* hubGroupCounts.ts — shared count helpers extracted from GroupSidebar.tsx (POL-05) */
/* CONTEXT Amendment 2: counts are membership only; no session status is shown. */
import { memberKey } from './hubGroups'
import type { SessionInfo } from '../wailsjs/go/main/App'

export interface GroupCounts {
  total: number
}

export function computeCounts(sessions: SessionInfo[], memberKeys: Set<string>): GroupCounts {
  return computeGlobalCounts(sessions.filter((s) => memberKeys.has(memberKey(s.name, s.workDir))))
}

export function computeGlobalCounts(sessions: SessionInfo[]): GroupCounts {
  return { total: sessions.length }
}
