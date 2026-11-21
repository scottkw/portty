import type { PreviewEvent, PreviewKind, PreviewMime } from './previewAssembler'

/** UI-SPEC: at most 10 preview tabs per viewer; the 11th closes the oldest one not being viewed. */
export const PREVIEW_MAX_TABS = 10
/** CONTEXT: at most 100 MiB of finished previews retained per viewer. */
export const PREVIEW_MAX_RETAINED = 100 * 1024 * 1024

/**
 * One preview as App shows it. While `refreshing`, `size` and `received` describe the copy
 * arriving; name, kind, mime, blob and url describe the copy shown until the swap.
 */
export interface PreviewEntry {
  tabId: string
  ownerTabId: string
  key: string
  name: string
  kind: PreviewKind
  mime: PreviewMime
  size: number
  state: 'receiving' | 'rendered' | 'tooLarge' | 'failed'
  received: number
  blob?: Blob
  url?: string
  arrivedAt: number
  refreshing: boolean
}

type Start = Extract<PreviewEvent, { type: 'start' }>

/**
 * Finished previews, held outside any terminal view so they outlive it (UI-SPEC "Close
 * behavior": a session ending or its tab closing never touches them). One entry per
 * (owner tab, key); subscribe/getSnapshot fit useSyncExternalStore.
 */
export class PreviewStore {
  private entries: PreviewEntry[] = []
  // The preview in flight per owner tab (one assembler per panel, one preview at a time).
  private inflight = new Map<string, { id: number; tabId: string; start: Start }>()
  private listeners = new Set<() => void>()
  private active: string | null = null
  private readonly createURL: (b: Blob) => string
  private readonly revokeURL: (u: string) => void
  private readonly now: () => number

  constructor(opts: { createURL?: (b: Blob) => string; revokeURL?: (u: string) => void; now?: () => number } = {}) {
    this.createURL = opts.createURL ?? ((b) => URL.createObjectURL(b))
    this.revokeURL = opts.revokeURL ?? ((u) => URL.revokeObjectURL(u))
    this.now = opts.now ?? Date.now
  }

  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener)
    return () => { this.listeners.delete(listener) }
  }

  getSnapshot = (): PreviewEntry[] => this.entries

  /** Eviction never picks the active tab. */
  setActive(tabId: string | null): void {
    this.active = tabId
  }

  /** Apply one of an owner tab's preview events; null when it refers to nothing held. */
  handle(ownerTabId: string, ev: PreviewEvent): { tabId: string; isNew: boolean } | null {
    if (ev.type === 'start') return this.start(ownerTabId, ev)
    const fl = this.inflight.get(ownerTabId)
    const cur = fl && fl.id === ev.id ? this.entries.find((e) => e.tabId === fl.tabId) : undefined
    if (!fl || !cur) return null
    if (ev.type === 'progress') {
      this.replace(cur, { received: ev.received })
      return { tabId: cur.tabId, isNew: false }
    }
    this.inflight.delete(ownerTabId)
    if (ev.type === 'complete') {
      const { name, kind, mime } = fl.start
      const size = ev.blob.size
      const url = this.createURL(ev.blob)
      if (cur.url) this.revokeURL(cur.url)
      this.replace(cur, { name, kind, mime, size, received: size, blob: ev.blob, url, state: 'rendered', refreshing: false })
      this.evictOverBytes()
    } else if (cur.blob) {
      // A failed refresh keeps the content already shown.
      this.replace(cur, { size: cur.blob.size, received: cur.blob.size, refreshing: false })
    } else {
      this.replace(cur, { state: ev.reason === 'tooLarge' ? 'tooLarge' : 'failed', refreshing: false })
    }
    return { tabId: cur.tabId, isNew: false }
  }

  close(tabId: string): void {
    const e = this.entries.find((x) => x.tabId === tabId)
    if (!e) return
    this.drop(e)
    this.emit()
  }

  closeAll(): void {
    if (this.entries.length === 0) return
    for (const e of this.entries) if (e.url) this.revokeURL(e.url)
    this.entries = []
    this.inflight.clear()
    this.emit()
  }

  private start(ownerTabId: string, ev: Start): { tabId: string; isNew: boolean } {
    const tabId = `__preview__${ownerTabId}/${ev.key}`
    this.inflight.set(ownerTabId, { id: ev.id, tabId, start: ev })
    const cur = this.entries.find((e) => e.tabId === tabId)
    const arrivedAt = this.now()
    if (cur?.blob) {
      this.replace(cur, { size: ev.size, received: 0, refreshing: true, arrivedAt })
      return { tabId, isNew: false }
    }
    const fresh: PreviewEntry = {
      tabId, ownerTabId, key: ev.key, name: ev.name, kind: ev.kind, mime: ev.mime, size: ev.size,
      state: 'receiving', received: 0, arrivedAt, refreshing: false,
    }
    if (cur) {
      this.replace(cur, fresh) // never rendered: restart it
      return { tabId, isNew: false }
    }
    if (this.entries.length >= PREVIEW_MAX_TABS) {
      const old = this.oldestEvictable()
      if (old) this.drop(old)
    }
    this.entries = [...this.entries, fresh]
    this.emit()
    return { tabId, isNew: true }
  }

  private evictOverBytes(): void {
    for (;;) {
      const retained = this.entries.reduce((n, e) => n + (e.blob?.size ?? 0), 0)
      const old = retained > PREVIEW_MAX_RETAINED ? this.oldestEvictable() : undefined
      if (!old) return
      this.drop(old)
      this.emit()
    }
  }

  private oldestEvictable(): PreviewEntry | undefined {
    let old: PreviewEntry | undefined
    for (const e of this.entries) if (e.tabId !== this.active && (!old || e.arrivedAt < old.arrivedAt)) old = e
    return old
  }

  // Removes without notifying; callers emit.
  private drop(e: PreviewEntry): void {
    if (e.url) this.revokeURL(e.url)
    this.entries = this.entries.filter((x) => x !== e)
    const fl = this.inflight.get(e.ownerTabId)
    if (fl?.tabId === e.tabId) this.inflight.delete(e.ownerTabId)
  }

  private replace(cur: PreviewEntry, patch: Partial<PreviewEntry>): void {
    this.entries = this.entries.map((e) => (e === cur ? { ...cur, ...patch } : e))
    this.emit()
  }

  private emit(): void {
    for (const l of this.listeners) l()
  }
}
