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
