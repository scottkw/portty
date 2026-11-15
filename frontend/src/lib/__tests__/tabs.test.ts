import { describe, it, expect } from 'vitest'
import { nextActiveTabId, renameInList, tabLabel } from '../tabs'

const tabs = [{ id: 'a' }, { id: 'b' }, { id: 'c' }]

describe('nextActiveTabId (TERM-03 detach neighbour rule)', () => {
  it('closing the active first tab activates the tab to its right', () => {
    expect(nextActiveTabId(tabs, 'a', 'a')).toBe('b')
  })

  it('closing the active middle tab activates the tab to its right', () => {
    expect(nextActiveTabId(tabs, 'b', 'b')).toBe('c')
  })

  it('closing the active last tab activates the tab to its left', () => {
    expect(nextActiveTabId(tabs, 'c', 'c')).toBe('b')
  })

  it('closing the only tab leaves nothing active (empty state)', () => {
    expect(nextActiveTabId([{ id: 'a' }], 'a', 'a')).toBeNull()
  })

  it('closing a background tab keeps the active tab', () => {
    expect(nextActiveTabId(tabs, 'a', 'c')).toBe('c')
    expect(nextActiveTabId(tabs, 'c', null)).toBeNull()
  })
})

describe('tabLabel and renameInList (one source of truth for names)', () => {
  const sessions = [{ id: 's1', name: 'shell 1' }, { id: 's2', name: 'logs' }]
  const local = { id: 's1', name: 'stale', sessionId: 's1' }
  const peer = { id: '__websession__s1', name: 'home-air › shell 1', sessionId: 's1', type: 'web-session' }

  it('a local tab shows its session name, falling back to tab.name', () => {
    expect(tabLabel(local, sessions)).toBe('shell 1')
    expect(tabLabel({ ...local, sessionId: 'gone' }, sessions)).toBe('stale')
    expect(tabLabel({ ...local, type: 'terminal' }, sessions)).toBe('shell 1')
  })

  it('a peer tab keeps its "<host> › <session>" name', () => {
    expect(tabLabel(peer, sessions)).toBe('home-air › shell 1')
  })

  it('renameInList returns a new list with only that session renamed', () => {
    const next = renameInList(sessions, 's1', 'build')
    expect(next).not.toBe(sessions)
    expect(next).toEqual([{ id: 's1', name: 'build' }, { id: 's2', name: 'logs' }])
    expect(sessions[0].name).toBe('shell 1')
  })

  it('a renamed list relabels the matching local tab and leaves a peer tab alone', () => {
    const next = renameInList(sessions, 's1', 'build')
    expect(tabLabel(local, next)).toBe('build')
    expect(tabLabel(peer, next)).toBe('home-air › shell 1')
  })
})
