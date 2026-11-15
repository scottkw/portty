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
const removeTabBody = () => slice('const removeTabState = useCallback(', 'removeTabStateRef.current = removeTabState')

describe('App.tsx TERM-05: session:exit closes the tab', () => {
  // Iteration-2 WR-03: the Go watcher already removed the session, so the exit
  // path is UI-only; a redundant KillSession would always fail and its warn
  // would hide real kill failures from user-initiated closes.
  it('calls the UI-only removeTabStateRef.current with data.sessionId (no kill)', () => {
    const h = exitHandler()
    expect(h).toContain('removeTabStateRef.current?.(data.sessionId)')
    expect(h).not.toContain('closeTab')
    expect(h).not.toContain('KillSession')
    expect(h).not.toContain('ToggleWebServing')
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
    expect(guard).toBeLessThan(body.indexOf('setSessions('))
  })

  // WR-04: the Hub reflects the exit at once (and its open modal closes).
  it('removes the exited session from hubSessions', () => {
    expect(exitHandler()).toContain('setSessions((prev) => prev.filter((s) => s.id !== data.sessionId))')
  })

  it('removeTabStateRef stays in sync with removeTabState', () => {
    expect(raw).toContain('removeTabStateRef.current = removeTabState')
  })

  it('removeTabState is UI-only and shifts focus to the neighbour tab', () => {
    const body = removeTabBody()
    expect(body).not.toContain('KillSession')
    expect(body).not.toContain('ToggleWebServing')
    expect(body).toContain('setTabs(')
    expect(body).toContain('setFontSizes(')
    expect(body).toContain('nextActiveTabId(prev, id, cur)')
  })
  // Portty 02-08 (TERM-03): a user close only detaches; App.test.tsx asserts
  // closeTab makes no session RPC. Only End Session… kills.
})

describe.each([
  ['init', initBody],
  ['retryInit', retryInitBody],
])('App.tsx TERM-05: %s restore filters stopped sessions', (_name, body) => {
  it("restores only sessions whose state is not 'stopped'", () => {
    const b = body()
    // Iteration-2 WR-02: a session that exited while Promise.all was pending
    // (its one session:exit already handled) must not come back as a tab.
    expect(b).toContain("const sessions = allSessions.filter((s) => s.state !== 'stopped' && !exitedIdsRef.current.has(s.id))")
    // Tabs and hubSessions are built from the filtered list.
    expect(b).toContain('setSessions(sessions)')
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
