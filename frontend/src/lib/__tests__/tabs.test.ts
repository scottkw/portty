import { describe, it, expect } from 'vitest'
import { hostlessWebSessionId, nextActiveTabId, peerTabsRemoved, renameInList, tabLabel, webSessionTabId } from '../tabs'

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

  // Portty 05-09: duplicate ids made it return the closing id, which blanked the window.
  it('never returns the closing id, even when the list holds duplicates', () => {
    expect(nextActiveTabId([{ id: 'a' }, { id: 'x' }, { id: 'x' }, { id: 'b' }], 'x', 'x')).toBe('b')
    expect(nextActiveTabId([{ id: 'a' }, { id: 'x' }, { id: 'x' }], 'x', 'x')).toBe('a')
    expect(nextActiveTabId([{ id: 'x' }, { id: 'x' }], 'x', 'x')).toBeNull()
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

// INPT-04: two Macs' daemons can hand out equal session ids; their tabs must differ.
describe('hostlessWebSessionId (IN-05)', () => {
  it('returns the session id of a hostless web-session tab id', () => {
    expect(hostlessWebSessionId(webSessionTabId('s1'))).toBe('s1')
  })
  it('returns null for a peer tab id (host/id), so it is never dialed as a session id', () => {
    expect(hostlessWebSessionId(webSessionTabId('s1', 'office.tail.ts.net'))).toBeNull()
  })
})

describe('webSessionTabId (INPT-04 peer tab identity)', () => {
  it('a peer tab id carries the host and the session id', () => {
    expect(webSessionTabId('s1', 'office.tail.ts.net')).toBe('__websession__office.tail.ts.net/s1')
  })

  it('the hostless web bootstrap tab keeps the session-only id', () => {
    expect(webSessionTabId('s1')).toBe('__websession__s1')
  })

  it('two hosts with one session id give two ids', () => {
    expect(webSessionTabId('s1', 'a.tail.ts.net')).not.toBe(webSessionTabId('s1', 'B'))
  })
})

// INPT-04: Retry replaces the tab list; the peer tabs it drops lose their snapshots.
describe('peerTabsRemoved (INPT-04 Retry cleanup)', () => {
  const local = { id: 'l1', sessionId: 'l1' }
  const bootstrap = { id: '__websession__w1', sessionId: 'w1', type: 'web-session' }
  const peerA = { id: '__websession__a.ts.net/s1', sessionId: 's1', type: 'web-session', peerHost: 'A', connectHost: 'a.ts.net' }
  const peerB = { id: '__websession__B/s1', sessionId: 's1', type: 'web-session', peerHost: 'B' }

  it('returns the peer tabs the next list lacks', () => {
    expect(peerTabsRemoved([local, peerA, peerB], [local, peerB])).toEqual([peerA])
  })

  it('returns nothing for local tabs or the hostless web bootstrap tab', () => {
    expect(peerTabsRemoved([local, bootstrap], [])).toEqual([])
  })

  it('returns nothing when every peer tab stays', () => {
    expect(peerTabsRemoved([peerA, peerB], [peerB, peerA])).toEqual([])
  })
})
