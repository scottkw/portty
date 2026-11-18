/**
 * Portty 02-08 (TERM-03): pure tab helpers.
 */

/**
 * The tab to activate after `closingId` closes. Closing a background tab keeps
 * the active one; closing the active tab moves right, else left, else null
 * (the empty state). It never returns closingId: with duplicate ids it once
 * picked the other copy, which was removed too, and that blanked the window.
 */
export function nextActiveTabId(tabs: { id: string }[], closingId: string, activeId: string | null): string | null {
  if (closingId !== activeId) return activeId
  const idx = tabs.findIndex((t) => t.id === closingId)
  if (idx === -1) return activeId
  const right = tabs.slice(idx + 1).find((t) => t.id !== closingId)
  const left = tabs.slice(0, idx).reverse().find((t) => t.id !== closingId)
  return (right ?? left)?.id ?? null
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

/**
 * A peer tab's '<host> › <session>' name split for display ("session · host"),
 * or null for any other tab. The host is a single DNS label, so the first
 * separator is the right one even if the session name contains one.
 */
export function peerTabParts(tab: { name: string; type?: string; peerHost?: string }): { host: string; session: string } | null {
  if (tab.type !== 'web-session' || !tab.peerHost) return null
  const i = tab.name.indexOf(' › ')
  return i === -1 ? null : { host: tab.name.slice(0, i), session: tab.name.slice(i + 3) }
}

/** A copy of `sessions` with session `id` renamed. */
export function renameInList<T extends Named>(sessions: T[], id: string, name: string): T[] {
  return sessions.map((s) => (s.id === id ? { ...s, name } : s))
}
