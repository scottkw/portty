import { describe, it, expect, vi, beforeEach } from 'vitest'
import { PreviewStore, PREVIEW_MAX_TABS, PREVIEW_MAX_RETAINED } from '../previewStore'
import type { PreviewEvent } from '../previewAssembler'

// jsdom has no URL.createObjectURL (RESEARCH Pitfall 10): the store takes injected URL functions.
let clock: number
let urlSeq: number
let createURL: ReturnType<typeof vi.fn<(b: Blob) => string>>
let revokeURL: ReturnType<typeof vi.fn<(u: string) => void>>
let store: PreviewStore

beforeEach(() => {
  clock = 1000
  urlSeq = 0
  createURL = vi.fn((_b: Blob) => `blob:test/${++urlSeq}`)
  revokeURL = vi.fn((_u: string) => {})
  store = new PreviewStore({ createURL, revokeURL, now: () => clock })
})

const hex = (n: number) => n.toString(16).padStart(32, '0')
let nextId = 1
function start(owner: string, key: string, size = 4, name = 'a.png') {
  const id = nextId++
  const ev: PreviewEvent = { type: 'start', id, name, kind: 'image', mime: 'image/png', size, key, focus: false }
  return { id, res: store.handle(owner, ev) }
}
const complete = (owner: string, id: number, size = 4) =>
  store.handle(owner, { type: 'complete', id, blob: new Blob([new Uint8Array(size)], { type: 'image/png' }) })
function open(owner: string, key: string, size = 4) {
  const { id, res } = start(owner, key, size)
  complete(owner, id, size)
  clock += 10
  return res!.tabId
}
const entry = (tabId: string) => store.getSnapshot().find((e) => e.tabId === tabId)

describe('PreviewStore (PREV-01, T-11-17)', () => {
  it('a start then complete gives one rendered entry with one URL; the id never looks like a web-session tab', () => {
    const { id, res } = start('s1', hex(1))
    expect(res).toEqual({ tabId: `__preview__s1/${hex(1)}`, isNew: true })
    expect(entry(res!.tabId)).toMatchObject({ state: 'receiving', received: 0, size: 4, refreshing: false, ownerTabId: 's1', key: hex(1), name: 'a.png', kind: 'image', mime: 'image/png', arrivedAt: 1000 })
    store.handle('s1', { type: 'progress', id, received: 2 })
    expect(entry(res!.tabId)!.received).toBe(2)
    complete('s1', id)
    expect(store.getSnapshot()).toHaveLength(1)
    expect(entry(res!.tabId)).toMatchObject({ state: 'rendered', url: 'blob:test/1', received: 4 })
    expect(entry(res!.tabId)!.blob!.size).toBe(4)
    expect(createURL).toHaveBeenCalledTimes(1)
    expect(res!.tabId.startsWith('__websession__')).toBe(false)
  })

  it('the same (owner, key) refreshes in place: old URL kept while refreshing, swapped on complete, revoked once', () => {
    const tab = open('s1', hex(1))
    const { id, res } = start('s1', hex(1), 8, 'b.png')
    expect(res).toEqual({ tabId: tab, isNew: false })
    expect(store.getSnapshot()).toHaveLength(1)
    expect(entry(tab)).toMatchObject({ state: 'rendered', url: 'blob:test/1', refreshing: true, received: 0, size: 8, name: 'a.png' })
    store.handle('s1', { type: 'progress', id, received: 5 })
    expect(entry(tab)).toMatchObject({ url: 'blob:test/1', received: 5 })
    expect(revokeURL).not.toHaveBeenCalled()
    complete('s1', id, 8)
    expect(entry(tab)).toMatchObject({ state: 'rendered', url: 'blob:test/2', refreshing: false, size: 8, received: 8, name: 'b.png' })
    expect(revokeURL).toHaveBeenCalledTimes(1)
    expect(revokeURL).toHaveBeenCalledWith('blob:test/1')
  })

  for (const reason of ['incomplete', 'tooLarge'] as const) {
    it(`a refresh that fails (${reason}) keeps the old content`, () => {
      const tab = open('s1', hex(1))
      const { id } = start('s1', hex(1), 8)
      store.handle('s1', { type: 'failed', id, reason })
      expect(entry(tab)).toMatchObject({ state: 'rendered', url: 'blob:test/1', refreshing: false, size: 4, received: 4 })
      expect(revokeURL).not.toHaveBeenCalled()
    })
  }

  it('a failed first copy shows its state; a start for an entry that never rendered restarts it', () => {
    const { id, res } = start('s1', hex(1))
    store.handle('s1', { type: 'failed', id, reason: 'tooLarge' })
    expect(entry(res!.tabId)!.state).toBe('tooLarge')
    const again = start('s1', hex(1))
    expect(again.res).toEqual({ tabId: res!.tabId, isNew: false })
    expect(entry(res!.tabId)).toMatchObject({ state: 'receiving', refreshing: false })
    store.handle('s1', { type: 'failed', id: again.id, reason: 'incomplete' })
    expect(entry(res!.tabId)!.state).toBe('failed')
  })

  it('the same key under another owner is a separate entry', () => {
    const a = open('s1', hex(1))
    const b = open('__websession__peer/s2', hex(1))
    expect(a).not.toBe(b)
    expect(store.getSnapshot()).toHaveLength(2)
  })

  it('events for an unknown id or owner are ignored', () => {
    const { id } = start('s1', hex(1))
    expect(store.handle('s2', { type: 'progress', id, received: 1 })).toBeNull()
    expect(store.handle('s1', { type: 'complete', id: 999, blob: new Blob([]) })).toBeNull()
    expect(createURL).not.toHaveBeenCalled()
  })

  it(`an entry past ${PREVIEW_MAX_TABS} evicts the oldest non-active one and revokes its URL; the active one survives`, () => {
    expect(PREVIEW_MAX_TABS).toBe(10)
    const tabs = Array.from({ length: 10 }, (_, i) => open('s1', hex(i)))
    store.setActive(tabs[0])
    open('s1', hex(10))
    const ids = store.getSnapshot().map((e) => e.tabId)
    expect(ids).toHaveLength(10)
    expect(ids).toContain(tabs[0])
    expect(ids).not.toContain(tabs[1])
    expect(revokeURL).toHaveBeenCalledTimes(1)
    expect(revokeURL).toHaveBeenCalledWith('blob:test/2')
  })

  it('completing entries past 100 MiB evicts the oldest non-active entries until under', () => {
    expect(PREVIEW_MAX_RETAINED).toBe(104_857_600)
    const size = 20_000_000
    const tabs = Array.from({ length: 5 }, (_, i) => open('s1', hex(i), size)) // 100,000,000: under
    expect(store.getSnapshot()).toHaveLength(5)
    store.setActive(tabs[0])
    open('s1', hex(5), size) // 120,000,000: drop the oldest non-active (tabs[1])
    const ids = store.getSnapshot().map((e) => e.tabId)
    expect(ids).toHaveLength(5)
    expect(ids).toContain(tabs[0])
    expect(ids).not.toContain(tabs[1])
    expect(store.getSnapshot().reduce((n, e) => n + (e.blob?.size ?? 0), 0)).toBeLessThanOrEqual(PREVIEW_MAX_RETAINED)
    expect(revokeURL).toHaveBeenCalledTimes(1)
  })

  it('an evicted in-flight entry ignores its later events', () => {
    store.setActive(null)
    const { id } = start('s9', hex(99))
    for (let i = 0; i < 10; i++) open('s1', hex(i))
    expect(store.getSnapshot().some((e) => e.ownerTabId === 's9')).toBe(false)
    expect(complete('s9', id)).toBeNull()
  })

  it('close revokes the URL and drops the entry; closeAll revokes every URL', () => {
    const a = open('s1', hex(1))
    open('s1', hex(2))
    open('s2', hex(3))
    store.close(a)
    expect(revokeURL).toHaveBeenCalledWith('blob:test/1')
    expect(entry(a)).toBeUndefined()
    store.closeAll()
    expect(store.getSnapshot()).toEqual([])
    expect(revokeURL.mock.calls.map((c) => c[0]).sort()).toEqual(['blob:test/1', 'blob:test/2', 'blob:test/3'])
  })

  it('getSnapshot returns the same array when nothing changed and notifies subscribers on change', () => {
    const listener = vi.fn()
    const unsub = store.subscribe(listener)
    const s0 = store.getSnapshot()
    expect(store.getSnapshot()).toBe(s0)
    store.setActive('x')
    store.close('nope')
    expect(store.getSnapshot()).toBe(s0)
    expect(listener).not.toHaveBeenCalled()
    open('s1', hex(1))
    expect(store.getSnapshot()).not.toBe(s0)
    expect(listener).toHaveBeenCalled()
    const s1 = store.getSnapshot()
    expect(store.getSnapshot()).toBe(s1)
    unsub()
    listener.mockClear()
    open('s1', hex(2))
    expect(listener).not.toHaveBeenCalled()
  })
})
