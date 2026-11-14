import type { SessionInfo } from '../wailsjs/go/main/App'

/** Wire status the tabs render. Anything not exited/errored (legacy idle/waiting, '', unknown) is running. */
export type SessionStatus = 'running' | 'exited' | 'errored'

/**
 * HubStatus — display status for Hub session cards (same three states as the tabs).
 * WR-01: single source of truth for SessionCard, HubPanel and group counts.
 */
export type HubStatus = SessionStatus

export function normalizeStatus(raw: string | undefined): SessionStatus {
  return raw === 'exited' || raw === 'errored' ? raw : 'running'
}

const STATUS_LABELS: Record<SessionStatus, string> = { running: 'Running', exited: 'Exited', errored: 'Error' }

export function statusLabel(status: SessionStatus): string {
  return STATUS_LABELS[status]
}

/**
 * Derive the Hub display status from a SessionInfo.
 * - state 'stopped' → 'errored' for a non-zero exitCode, else 'exited'
 * - otherwise the normalized daemon status (errored stays errored; legacy values → running)
 */
export function deriveHubStatus(s: SessionInfo): HubStatus {
  if (s.state === 'stopped') {
    return (s.exitCode ?? 0) !== 0 ? 'errored' : 'exited'
  }
  return normalizeStatus(s.status)
}

/* ATTN-01: canonical attention predicate — only errored sessions need attention */
export function isAttentionStatus(status: HubStatus): boolean {
  return status === 'errored'
}
