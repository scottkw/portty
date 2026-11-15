import { describe, it, expect } from 'vitest'
import { nextActiveTabId } from '../tabs'

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
