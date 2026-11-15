/**
 * TERM-05 — App.tsx close-on-exit contract (CONTEXT Amendment 2).
 *
 * Every session:exit (any exit code, any session age) closes its tab and
 * removes the session; sessions already stopped at restore time are removed,
 * not restored as tabs. Source inspection (`App.tsx?raw`), the same pattern as
 * App.shellWebShare.test.tsx and App.nav.test.tsx, avoids mounting the full
 * tree with Wails-runtime mocks.
 */
import { describe, it, expect } from 'vitest'
import raw from '../../App.tsx?raw'

/** Slice from `marker` to the first `end` after it. */
function slice(marker: string, end: string): string {
  const start = raw.indexOf(marker)
  if (start === -1) throw new Error(`${marker} not found in App.tsx`)
  const stop = raw.indexOf(end, start + marker.length)
  if (stop === -1) throw new Error(`${end} not found after ${marker}`)
  return raw.slice(start, stop)
}

const exitHandler = () => slice("'session:exit'", '\n    )')
const initBody = () => slice('async function init()', 'void init()')
const retryInitBody = () => slice('const retryInit = useCallback(', '}, [])')
const closeTabBody = () => slice('const handleCloseTab = useCallback(', 'handleCloseTabRef.current = handleCloseTab')

describe('App.tsx TERM-05: session:exit closes the tab', () => {
  it('calls handleCloseTabRef.current with data.sessionId', () => {
    expect(exitHandler()).toContain('handleCloseTabRef.current?.(data.sessionId)')
  })

  it('has no exit-code branch and records no exit state', () => {
    const h = exitHandler()
    expect(h).not.toContain('exitCode')
    expect(h).not.toContain('setSessionExits')
    expect(h).not.toMatch(/\bif\s*\(/)
  })

  // WR-03: a shell can exit (and be reported, once) before CreateSession
  // resolves; createTab must not then add a tab for the removed session.
  it('records exited ids and createTab skips one that already exited', () => {
    expect(exitHandler()).toContain('exitedIdsRef.current.add(data.sessionId)')
    const body = slice('const createTab = useCallback(', '}, [tabCounter')
    const created = body.indexOf('await CreateSession(')
    const guard = body.indexOf('if (exitedIdsRef.current.has(sessionId)) return')
    expect(created).toBeGreaterThan(-1)
    expect(guard).toBeGreaterThan(created)
    expect(guard).toBeLessThan(body.indexOf('setTabs('))
    expect(guard).toBeLessThan(body.indexOf('setHubSessions('))
  })

  // WR-04: the Hub reflects the exit at once (and its open modal closes).
  it('removes the exited session from hubSessions', () => {
    expect(exitHandler()).toContain('setHubSessions((prev) => prev.filter((s) => s.id !== data.sessionId))')
  })

  it('handleCloseTabRef stays in sync with handleCloseTab', () => {
    expect(raw).toContain('handleCloseTabRef.current = handleCloseTab')
  })

  // Plan-checker advisory: the exit cleanup must not depend on a tab existing.
  // Hub-only or CLI-created sessions have no tab, yet their exit must still
  // remove the session from the daemon.
  it('handleCloseTab calls KillSession(id) unconditionally, before touching tabs', () => {
    const body = closeTabBody()
    const killIdx = body.indexOf('await KillSession(id)')
    const setTabsIdx = body.indexOf('setTabs(')
    expect(killIdx).toBeGreaterThan(-1)
    expect(setTabsIdx).toBeGreaterThan(killIdx)
    const beforeKill = body.slice(0, killIdx)
    expect(beforeKill).not.toMatch(/\btabs\b/)
    expect(beforeKill).not.toMatch(/^\s*return\b/m) // no early return before the kill
  })

  it('handleCloseTab still shifts focus to the adjacent tab', () => {
    const body = closeTabBody()
    expect(body).toContain('activeId === id')
    expect(body).toContain('setActiveId(next?.id ?? null)')
  })
})

describe.each([
  ['init', initBody],
  ['retryInit', retryInitBody],
])('App.tsx TERM-05: %s restore filters stopped sessions', (_name, body) => {
  it("restores only sessions whose state is not 'stopped'", () => {
    const b = body()
    expect(b).toContain("const sessions = allSessions.filter((s) => s.state !== 'stopped')")
    // Tabs and hubSessions are built from the filtered list.
    expect(b).toContain('setHubSessions(sessions)')
    expect(b).toContain('sessions.map((s) => ({')
    expect(b).not.toContain('allSessions.map(')
  })

  it('passes each stopped session to KillSession with a warning catch', () => {
    const b = body()
    const stopped = b.indexOf("allSessions.filter((s) => s.state === 'stopped')")
    expect(stopped).toBeGreaterThan(-1)
    const after = b.slice(stopped, stopped + 250)
    expect(after).toContain('KillSession(s.id)')
    expect(after).toContain('console.warn')
  })
})
