import React from 'react'
import type { SessionInfo } from '../../wailsjs/go/main/App'
import type { AdaptedRemoteSessionInfo } from '../../lib/remoteAdapter'
import type { ITheme } from '@xterm/xterm'
import { daemon } from '../../wailsjs/go/models'
import { SessionCard } from './SessionCard'
import { memberKey, type HubGroupDef } from '../../lib/hubGroups'

// ---- Helpers ----

/**
 * Group sessions by their working directory.
 * Sessions with an empty workDir are keyed under ''.
 * Does not sort: sessions keep the order they arrive in.
 *
 * @returns A Map preserving insertion order — one entry per distinct workDir.
 */
export function groupByWorkDir(sessions: SessionInfo[]): Map<string, SessionInfo[]> {
  const groups = new Map<string, SessionInfo[]>()
  for (const s of sessions) {
    const key = s.workDir || ''
    const group = groups.get(key) ?? []
    group.push(s)
    groups.set(key, group)
  }
  return groups
}

/**
 * Group sessions by named group definitions.
 * Sessions are placed into the first matching HubGroupDef (matched by memberKey).
 * Unmatched sessions fall into the '__other__' bucket labelled "Other".
 * Named groups appear in definition order, Other last.
 * GROUP-04: membership key = "${session.name}:::${session.workDir}" — survives session-id churn.
 * Does not sort: sessions keep the order they arrive in.
 */
export function groupByNamedGroups(
  sessions: SessionInfo[],
  groupDefs: HubGroupDef[],
): Map<string, { label: string; sessions: SessionInfo[] }> {
  // Build result map: named groups in definition order, then "Other"
  const result = new Map<string, { label: string; sessions: SessionInfo[] }>()
  for (const g of groupDefs) {
    result.set(g.id, { label: g.name, sessions: [] })
  }
  result.set('__other__', { label: 'Other', sessions: [] })

  for (const s of sessions) {
    const key = memberKey(s.name, s.workDir)
    const matchingGroup = groupDefs.find((g) => g.memberKeys.includes(key))
    if (matchingGroup) {
      result.get(matchingGroup.id)!.sessions.push(s)
    } else {
      result.get('__other__')!.sessions.push(s)
    }
  }
  return result
}

/**
 * basename — extract the last non-empty path segment.
 * Works with both Unix ('/', \\) and Windows ('\\') separators.
 * Do NOT import node:path in frontend code.
 */
function basename(path: string): string {
  // Normalise separators, then split
  const segments = path.replace(/\\/g, '/').split('/')
  // Take the last non-empty segment
  for (let i = segments.length - 1; i >= 0; i--) {
    if (segments[i] !== '') return segments[i]
  }
  return path
}

// ---- Props ----

export interface SessionCardGridProps {
  /** Full list of sessions to group and render. */
  sessions: SessionInfo[]
  /** Passed through to each SessionCard for inline rename. */
  onRename: (id: string, name: string) => void
  /** Passed through to each SessionCard's Open button. Phase 131 UAT follow-up. */
  onOpenSession?: (sessionId: string, name: string) => void
  /** Phase 132 — named group definitions; when non-empty overrides workDir grouping */
  groupDefs?: HubGroupDef[]
  /** Phase 132/139 — styled tail lines map from usePreviewPoller, keyed by session ID (CARD-05: StyledSpan[][] since Plan 04) */
  previewTails?: Map<string, daemon.StyledSpan[][]>
  /** Phase 139 / CARD-05 — active xterm ITheme for color resolution in MiniPreview */
  previewTheme?: ITheme
  /** Phase 132 — fires when user assigns via card overflow menu or DnD */
  onAssignGroup?: (memberKey: string, groupId: string) => void
  /** Phase 134 — threaded to each SessionCard for modal open trigger */
  onCardClick?: (session: SessionInfo, rect: DOMRect) => void
  /** Phase 137 / D-12 — threaded to each SessionCard Share button */
  onShare?: (session: SessionInfo) => void
  /** NET-01 — threaded to each SessionCard; disables local Share buttons */
  shareDisabledReason?: string
  /** Phase 138 — Set of session IDs with a cached cap (isConnected signal) */
  connectedRemoteIds?: Set<string>
  /** Phase 138 — Set of remote session IDs (provenance-based isRemote signal) */
  remoteIdSet?: Set<string>
  /** Phase 138 — Kill handler (threaded from HubPanel → App.handleCloseTab) */
  onKill?: (sessionId: string) => void
  /** Phase 138 — Open remote session in browser (Phase 146: receives session object for cap exchange) */
  onOpenInBrowser?: (session: AdaptedRemoteSessionInfo) => void
  /** NOTIF-01: unread badge data keyed by session ID.
   *  Threaded from HubPanel's unreadMap to both SessionCard render sites.
   *  Optional — absence is equivalent to an empty map (no badges shown). */
  unreadBySessionId?: Map<string, { count: number; hasMention: boolean }>
}

// ---- Component ----

/**
 * SessionCardGrid — groups SessionCards by working directory with group headers.
 *
 * Layout per UI-SPEC Layout Contract:
 *   .hub__group
 *     .hub__group-header (h2, role="heading" aria-level=2)
 *       span[title={fullPath}]  ← basename shown; full path as tooltip
 *     .hub__card-row [role="list"]
 *       div [role="listitem"]
 *         SessionCard
 *
 * Phase 132 named-group branch:
 *   When groupDefs is non-empty, uses groupByNamedGroups instead of groupByWorkDir.
 *   Group headers show named group labels (in definition order), Other last (GROUP-04).
 *
 * GRID-02: groupByWorkDir drives the grouping logic when no named groups are defined.
 * Accessibility: role=list/listitem satisfies UI-SPEC §Accessibility rule 6.
 * No node:path import — basename helper is self-contained.
 */
export function SessionCardGrid({
  sessions,
  onRename,
  onOpenSession,
  groupDefs,
  previewTails,
  previewTheme,
  onAssignGroup,
  onCardClick,
  onShare,
  shareDisabledReason,
  connectedRemoteIds,
  remoteIdSet,
  onKill,
  onOpenInBrowser,
  unreadBySessionId,
}: SessionCardGridProps): React.ReactElement {
  // Phase 132: when groupDefs is non-empty, use named-group grouping; else fall back to workDir
  if (groupDefs && groupDefs.length > 0) {
    // Named-group render path (Phase 132)
    const namedGroups = groupByNamedGroups(sessions, groupDefs)

    return (
      <>
        {Array.from(namedGroups.entries()).map(([groupId, { label, sessions: groupSessions }]) => (
          <div key={groupId} className="hub__group">
            {/* Group header: 11px / 600 / uppercase — named group label */}
            <h2 className="hub__group-header" role="heading" aria-level={2}>
              <span>{label}</span>
            </h2>

            {/* Card grid — role=list per UI-SPEC §Accessibility rule 6 */}
            <div role="list" className="hub__card-row">
              {groupSessions.map((s) => (
                <div role="listitem" key={s.id}>
                  <SessionCard
                    session={s}
                    onRename={onRename}
                    onOpenSession={onOpenSession}
                    onCardClick={onCardClick}
                    onShare={onShare}
                    shareDisabledReason={shareDisabledReason}
                    previewLines={previewTails?.get(s.id)}
                    previewTheme={previewTheme}
                    groupDefs={groupDefs}
                    onAssignGroup={onAssignGroup}
                    isRemote={remoteIdSet?.has(s.id)}
                    isConnected={connectedRemoteIds?.has(s.id)}
                    onKill={onKill}
                    onOpenInBrowser={onOpenInBrowser}
                    unreadCount={unreadBySessionId?.get(s.id)?.count}
                    hasChatMention={unreadBySessionId?.get(s.id)?.hasMention}
                  />
                </div>
              ))}
            </div>
          </div>
        ))}
      </>
    )
  }

  // Phase 131 fallback: workDir-based grouping
  const groups = groupByWorkDir(sessions)

  return (
    <>
      {Array.from(groups.entries()).map(([workDir, groupSessions]) => {
        const headerLabel = workDir ? basename(workDir) : 'Other'
        const titleValue = workDir || undefined

        return (
          <div key={workDir || '__other__'} className="hub__group">
            {/* Group header: 11px / 600 / uppercase — mirrors .remote-panel__peer-header */}
            <h2 className="hub__group-header" role="heading" aria-level={2}>
              <span title={titleValue}>{headerLabel}</span>
            </h2>

            {/* Card grid — role=list per UI-SPEC §Accessibility rule 6 */}
            <div role="list" className="hub__card-row">
              {groupSessions.map((s) => (
                <div role="listitem" key={s.id}>
                  <SessionCard
                    session={s}
                    onRename={onRename}
                    onOpenSession={onOpenSession}
                    onCardClick={onCardClick}
                    onShare={onShare}
                    shareDisabledReason={shareDisabledReason}
                    previewLines={previewTails?.get(s.id)}
                    previewTheme={previewTheme}
                    groupDefs={groupDefs}
                    onAssignGroup={onAssignGroup}
                    isRemote={remoteIdSet?.has(s.id)}
                    isConnected={connectedRemoteIds?.has(s.id)}
                    onKill={onKill}
                    onOpenInBrowser={onOpenInBrowser}
                    unreadCount={unreadBySessionId?.get(s.id)?.count}
                    hasChatMention={unreadBySessionId?.get(s.id)?.hasMention}
                  />
                </div>
              ))}
            </div>
          </div>
        )
      })}
    </>
  )
}
