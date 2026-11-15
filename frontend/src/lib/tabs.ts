/**
 * Portty 02-08 (TERM-03): pure tab helpers.
 */

/**
 * The tab to activate after `closingId` closes. Closing a background tab keeps
 * the active one; closing the active tab moves right, else left, else null
 * (the empty state).
 */
export function nextActiveTabId(tabs: { id: string }[], closingId: string, activeId: string | null): string | null {
  if (closingId !== activeId) return activeId
  const idx = tabs.findIndex((t) => t.id === closingId)
  if (idx === -1) return activeId
  return (tabs[idx + 1] ?? tabs[idx - 1])?.id ?? null
}

type Named = { id: string; name: string }

/**
 * A tab's label, with the session list as the one source of truth for local
 * names: a renamed session relabels its tab in the same render. Peer and other
 * tabs keep their own name ('<host> › <session>' for a peer).
 */
export function tabLabel(tab: { name: string; sessionId: string; type?: string }, sessions: Named[]): string {
  if (tab.type && tab.type !== 'terminal') return tab.name
  return sessions.find((s) => s.id === tab.sessionId)?.name || tab.name
}

/** A copy of `sessions` with session `id` renamed. */
export function renameInList<T extends Named>(sessions: T[], id: string, name: string): T[] {
  return sessions.map((s) => (s.id === id ? { ...s, name } : s))
}
