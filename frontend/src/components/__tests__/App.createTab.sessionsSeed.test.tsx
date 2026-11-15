/**
 * createTab seeds the sessions list and always activates the new tab.
 *
 * Originally Phase 168 code review CR-01 (168-REVIEW.md): the footer
 * "Share Session" button resolves the active session via
 * `sessions.find(s => s.id === activeId)`, so createTab must append a
 * SessionInfo-shaped entry synchronously, or Share silently no-ops for a
 * just-created session until the next poll.
 *
 * Portty 02-06 (NAV-03): the Hub and its "stay on Hub after creating a
 * session" preference are gone, so createTab always activates the new tab
 * and reads no preference.
 *
 * App.tsx's createTab is not exported and App is not fully mounted in this
 * codebase's test suite (established convention: source-inspection via
 * App.tsx?raw for App-level logic — see App.open-remote.test.tsx,
 * App.wiring.test.tsx).
 */
import { describe, it, expect } from 'vitest'
import raw from '../../App.tsx?raw'

describe('createTab seeds sessions and activates the new tab — source contract', () => {
  it('createTab calls setSessions in its success path (not just setTabs)', () => {
    const idx = raw.indexOf('const createTab = useCallback')
    expect(idx).toBeGreaterThan(-1)
    const slice = raw.slice(idx, idx + 3200)
    expect(slice).toContain('setTabs((prev) => [...prev, tab])')
    expect(slice).toContain('setSessions((prev) => [')
  })

  it('the appended sessions entry is keyed by the same sessionId as the new tab', () => {
    const idx = raw.indexOf('const createTab = useCallback')
    const slice = raw.slice(idx, idx + 3200)
    const seedIdx = slice.indexOf('setSessions((prev) => [')
    expect(seedIdx).toBeGreaterThan(-1)
    const seedSlice = slice.slice(seedIdx, seedIdx + 500)
    expect(seedSlice).toContain('id: sessionId,')
  })

  it('setSessions runs inside the same try block as CreateSession — after the id is known, before the catch', () => {
    const idx = raw.indexOf('const createTab = useCallback')
    const slice = raw.slice(idx, idx + 3200)
    const sessionIdIdx = slice.indexOf('const sessionId = await CreateSession(')
    const seedIdx = slice.indexOf('setSessions((prev) => [')
    const catchIdx = slice.indexOf('} catch (err) {')
    expect(sessionIdIdx).toBeGreaterThan(-1)
    expect(seedIdx).toBeGreaterThan(sessionIdIdx)
    expect(catchIdx).toBeGreaterThan(seedIdx)
  })

  it('openShareModalForActiveSession still resolves from sessions + activeId (unchanged lookup contract)', () => {
    const idx = raw.indexOf('const openShareModalForActiveSession')
    expect(idx).toBeGreaterThan(-1)
    const block = raw.slice(idx, idx + 400)
    expect(block).toContain('sessions.find(')
    expect(block).toContain('activeId')
    expect(block).toContain('setShareModalSession(')
  })

  it('createTab always activates the new tab and reads no stay-on-Hub preference (NAV-03)', () => {
    const idx = raw.indexOf('const createTab = useCallback')
    const slice = raw.slice(idx, idx + 3200)
    const tabsIdx = slice.indexOf('setTabs((prev) => [...prev, tab])')
    const activeIdx = slice.indexOf('setActiveId(sessionId)')
    expect(tabsIdx).toBeGreaterThan(-1)
    expect(activeIdx).toBeGreaterThan(tabsIdx)
    // Unconditional: no if-guard sits between adding the tab and activating it.
    const before = slice.slice(tabsIdx, activeIdx)
    expect(before).not.toMatch(/\bif\s*\(/)
    expect(raw).not.toMatch(/stayOnHub/i)
  })
})
