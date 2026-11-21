// Phase 11 (PREV-01, PREV-02, PREV-04): preview tabs in the real App.
//
// The real App mounts with the Wails bridge faked (appHarness). TerminalPanel and
// WebShareSessionView are stubbed (no xterm in jsdom); each stub records the onPreview
// prop App gave it, so a test can play the events a session's socket would produce.

import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import React, { act } from 'react'

import { mountApp, unmountApp, flush, wailsEvents } from './appHarness'
import type { PreviewEvent } from '../../lib/previewAssembler'

const stub = vi.hoisted(() => ({
  panels: new Map<string, { onPreview?: (e: unknown) => void; isActive?: boolean }>(),
  views: new Map<string, { onPreview?: (e: unknown) => void; isActive?: boolean }>(),
  viewMounts: 0,
  viewUnmounts: 0,
}))

vi.mock('../TerminalPanel', () => ({
  TerminalPanel: (p: { sessionId: string; onPreview?: (e: unknown) => void; isActive: boolean }) => {
    stub.panels.set(p.sessionId, p)
    return React.createElement('div', { 'data-testid': `terminal-${p.sessionId}` },
      React.createElement('textarea', { className: 'xterm-helper-textarea', 'aria-label': `input ${p.sessionId}` }))
  },
}))

vi.mock('../WebShareSessionView', () => ({
  WebShareSessionView: (p: { sessionId: string; onPreview?: (e: unknown) => void; isActive?: boolean }) => {
    stub.views.set(p.sessionId, p)
    React.useEffect(() => {
      stub.viewMounts++
      return () => { stub.viewUnmounts++ }
    }, [])
    return React.createElement('div', { 'data-testid': 'web-session-view', 'data-active': String(p.isActive) })
  },
}))

const session = (id: string, name: string) => ({
  id, name, state: 'running', status: 'idle', createdAt: '', hostname: '', webEnabled: false, viewerCount: 0, workDir: '/',
})

let urlSeq = 0
const created: string[] = []
const revoked: string[] = []

beforeEach(() => {
  urlSeq = 0
  created.length = 0
  revoked.length = 0
  stub.panels.clear()
  stub.views.clear()
  stub.viewMounts = 0
  stub.viewUnmounts = 0
  // jsdom has no object URLs (RESEARCH Pitfall 10).
  URL.createObjectURL = vi.fn(() => { const u = `blob:test/${++urlSeq}`; created.push(u); return u })
  URL.revokeObjectURL = vi.fn((u: string) => { revoked.push(u) })
})

afterEach(async () => {
  await unmountApp()
})

function mountLocal(ids = ['s1', 's2']) {
  return mountApp({
    GetRelayPort: () => 4321,
    ListSessions: () => ids.map((id) => session(id, `${id} shell`)),
    GetTailscaleStatus: () => ({ connected: true, hasCerts: true, ip: '100.64.0.1', domain: 'home-air.tail1234.ts.net' }),
  })
}

const KEY_A = 'a'.repeat(32)

async function play(onPreview: ((e: unknown) => void) | undefined, events: PreviewEvent[]) {
  expect(onPreview, 'the view got an onPreview prop').toBeTypeOf('function')
  for (const e of events) await act(async () => { onPreview!(e) })
  await flush()
}

function png(id: number, name: string, key: string, focus: boolean): PreviewEvent[] {
  return [
    { type: 'start', id, name, kind: 'image', mime: 'image/png', size: 4, key, focus },
    { type: 'complete', id, blob: new Blob([new Uint8Array([1, 2, 3, 4])], { type: 'image/png' }) },
  ]
}

function tabNames(c: HTMLElement): string[] {
  return Array.from(c.querySelectorAll('[role="tab"]')).map((t) => t.textContent ?? '')
}

function selected(c: HTMLElement): string | null {
  return c.querySelector('[role="tab"][aria-selected="true"]')?.textContent ?? null
}

describe('App preview tabs: a local session (PREV-01, PREV-02)', () => {
  it('a focused PNG opens a tab right of its session tab, shows the image, and × returns to the terminal and revokes the URL', async () => {
    const c = await mountLocal()
    expect(selected(c)).toBe('s1 shell')
    await play(stub.panels.get('s1')?.onPreview, png(1, 'diagram.png', KEY_A, true))

    expect(tabNames(c)).toEqual(['s1 shell', 'diagram.png', 's2 shell'])
    expect(selected(c)).toBe('diagram.png')
    const img = c.querySelector<HTMLImageElement>('img.preview-image__img')
    expect(img?.getAttribute('src')).toBe('blob:test/1')
    expect(img?.getAttribute('alt')).toBe('diagram.png')
    expect(stub.panels.get('s1')?.isActive).toBe(false)

    await act(async () => { c.querySelector<HTMLButtonElement>('button[aria-label="Close diagram.png"]')!.click() })
    await flush()
    expect(tabNames(c)).toEqual(['s1 shell', 's2 shell'])
    expect(selected(c)).toBe('s1 shell')
    expect(revoked).toEqual(['blob:test/1'])
    expect(c.querySelector('img.preview-image__img')).toBeNull()
  })
})

const keyN = (n: number) => n.toString(16).padStart(32, '0')

async function click(el: Element | null | undefined) {
  expect(el, 'element to click').toBeTruthy()
  await act(async () => { (el as HTMLElement).click() })
  await flush()
}

function tabButton(c: HTMLElement, name: string): HTMLButtonElement | undefined {
  return Array.from(c.querySelectorAll<HTMLButtonElement>('[role="tab"]')).find((t) => t.textContent === name)
}

describe('App preview tabs: position (PREV-01)', () => {
  it("a session's previews sit after its tab in arrival order, never between another session and its previews", async () => {
    const c = await mountLocal()
    await play(stub.panels.get('s2')?.onPreview, png(1, 'b.png', keyN(1), false))
    await play(stub.panels.get('s1')?.onPreview, png(1, 'x.png', keyN(2), false))
    await play(stub.panels.get('s1')?.onPreview, png(2, 'y.png', keyN(3), false))
    expect(tabNames(c)).toEqual(['s1 shell', 'x.png', 'y.png', 's2 shell', 'b.png'])
    await play(stub.panels.get('s2')?.onPreview, png(2, 'c.png', keyN(4), false))
    expect(tabNames(c)).toEqual(['s1 shell', 'x.png', 'y.png', 's2 shell', 'b.png', 'c.png'])
  })
})

describe('App preview tabs: focus rule (locked: only the viewer you typed in)', () => {
  it('a focused arrival moves keyboard focus into the preview pane', async () => {
    const c = await mountLocal()
    await play(stub.panels.get('s1')?.onPreview, png(1, 'diagram.png', KEY_A, true))
    const pane = c.querySelector('.preview-panel')
    expect(pane?.getAttribute('role')).toBe('tabpanel')
    expect(pane?.contains(document.activeElement)).toBe(true)
  })

  it('focus true while another session is active is a background tab', async () => {
    const c = await mountLocal()
    await play(stub.panels.get('s2')?.onPreview, png(1, 'b.png', KEY_A, true))
    expect(tabNames(c)).toContain('b.png')
    expect(selected(c)).toBe('s1 shell')
    expect(c.querySelector('.preview-panel')).toBeNull()
  })

  it('focus false never switches, even from the active session', async () => {
    const c = await mountLocal()
    await play(stub.panels.get('s1')?.onPreview, png(1, 'a.png', KEY_A, false))
    expect(tabNames(c)).toEqual(['s1 shell', 'a.png', 's2 shell'])
    expect(selected(c)).toBe('s1 shell')
  })

  it('a focused arrival from the session while one of its previews is active switches to the new one', async () => {
    const c = await mountLocal()
    await play(stub.panels.get('s1')?.onPreview, png(1, 'a.png', keyN(1), true))
    await play(stub.panels.get('s1')?.onPreview, png(2, 'b.png', keyN(2), true))
    expect(selected(c)).toBe('b.png')
  })

  it('a background arrival leaves keyboard focus and an open dialog alone', async () => {
    const c = await mountLocal()
    const input = c.querySelector<HTMLTextAreaElement>('textarea[aria-label="input s1"]')!
    input.focus()
    expect(document.activeElement).toBe(input)
    await play(stub.panels.get('s2')?.onPreview, png(1, 'b.png', KEY_A, true))
    expect(document.activeElement).toBe(input)

    await act(async () => { wailsEvents.get('menu:new-session')?.() })
    await flush()
    const dialog = document.querySelector('[role="dialog"]')
    expect(dialog).not.toBeNull()
    const before = document.activeElement
    await play(stub.panels.get('s2')?.onPreview, png(2, 'c.png', keyN(2), false))
    expect(document.querySelector('[role="dialog"]')).toBe(dialog)
    expect(document.activeElement).toBe(before)
    expect(selected(c)).toBe('s1 shell')
  })
})

describe('App preview tabs: close rules (UI-SPEC "Close behavior")', () => {
  it('closing the active preview activates its session tab and focuses that terminal', async () => {
    const c = await mountLocal()
    await click(tabButton(c, 's2 shell'))
    await play(stub.panels.get('s2')?.onPreview, png(1, 'b.png', KEY_A, true))
    expect(selected(c)).toBe('b.png')
    await click(c.querySelector('button[aria-label="Close b.png"]'))
    expect(selected(c)).toBe('s2 shell')
    expect(document.activeElement).toBe(c.querySelector('textarea[aria-label="input s2"]'))
  })

  it("closing the session's tab leaves its previews open", async () => {
    const c = await mountLocal()
    await play(stub.panels.get('s1')?.onPreview, png(1, 'a.png', KEY_A, false))
    await click(c.querySelector('button[aria-label="Close s1 shell"]'))
    expect(tabNames(c)).toEqual(['a.png', 's2 shell'])
    expect(revoked).toEqual([])
    // With the session's tab gone, closing the active preview falls back to nextActiveTabId.
    await click(tabButton(c, 'a.png'))
    expect(c.querySelector('img.preview-image__img')?.getAttribute('src')).toBe('blob:test/1')
    await click(c.querySelector('button[aria-label="Close a.png"]'))
    expect(selected(c)).toBe('s2 shell')
  })

  it('the session ending (session:exit) leaves its previews open', async () => {
    const c = await mountLocal()
    await play(stub.panels.get('s1')?.onPreview, png(1, 'a.png', KEY_A, true))
    await act(async () => { wailsEvents.get('session:exit')?.({ sessionId: 's1' }) })
    await flush()
    expect(tabNames(c)).toEqual(['a.png', 's2 shell'])
    expect(selected(c)).toBe('a.png')
    expect(c.querySelector('img.preview-image__img')).not.toBeNull()
  })

  it('⌘W (menu:close-tab) closes the active preview and returns to its session', async () => {
    const c = await mountLocal()
    await play(stub.panels.get('s1')?.onPreview, png(1, 'a.png', KEY_A, true))
    await act(async () => { wailsEvents.get('menu:close-tab')?.() })
    await flush()
    expect(tabNames(c)).toEqual(['s1 shell', 's2 shell'])
    expect(selected(c)).toBe('s1 shell')
    expect(revoked).toEqual(['blob:test/1'])
  })
})

describe('App preview tabs: refresh, limit, live region, unmount', () => {
  it('the same file again keeps one tab and swaps its content', async () => {
    const c = await mountLocal()
    await play(stub.panels.get('s1')?.onPreview, png(1, 'a.png', KEY_A, true))
    await play(stub.panels.get('s1')?.onPreview, png(2, 'a.png', KEY_A, true))
    expect(tabNames(c)).toEqual(['s1 shell', 'a.png', 's2 shell'])
    expect(c.querySelector('img.preview-image__img')?.getAttribute('src')).toBe('blob:test/2')
    expect(revoked).toEqual(['blob:test/1'])
  })

  it('the tab underline shows received/size while the file arrives and drops to 0 when it is in', async () => {
    const c = await mountLocal()
    const [start, complete] = png(1, 'a.png', KEY_A, false)
    await play(stub.panels.get('s1')?.onPreview, [start, { type: 'progress', id: 1, received: 2 }])
    const underline = () => (c.querySelector(`[data-testid="tab-progress-__preview__s1/${KEY_A}"]`) as HTMLElement).style.transform
    expect(underline()).toBe('scaleX(0.5)')
    await play(stub.panels.get('s1')?.onPreview, [complete])
    expect(underline()).toBe('scaleX(0)')
  })

  it('an 11th preview closes the oldest one that is not active', async () => {
    const c = await mountLocal(['s1'])
    await play(stub.panels.get('s1')?.onPreview, png(1, 'p1.png', keyN(1), true))
    for (let i = 2; i <= 11; i++) await play(stub.panels.get('s1')?.onPreview, png(i, `p${i}.png`, keyN(i), false))
    const names = tabNames(c)
    expect(names.filter((n) => n.endsWith('.png'))).toHaveLength(10)
    expect(names).toContain('p1.png')
    expect(names).not.toContain('p2.png')
    expect(names).toContain('p11.png')
    expect(selected(c)).toBe('p1.png')
    expect(revoked).toEqual(['blob:test/2'])
  })

  it('every arrival, focused or background, is announced in one polite live region', async () => {
    const c = await mountLocal()
    const regions = document.querySelectorAll('[aria-live="polite"]')
    expect(regions).toHaveLength(1)
    await play(stub.panels.get('s1')?.onPreview, png(1, 'a.png', keyN(1), true))
    expect(regions[0].textContent).toBe('Preview opened: a.png')
    await play(stub.panels.get('s2')?.onPreview, png(1, 'b.png', keyN(2), false))
    expect(regions[0].textContent).toBe('Preview opened: b.png')
    expect(regions[0].classList.contains('visually-hidden')).toBe(true)
    expect(c.contains(regions[0])).toBe(true)
  })

  it('unmounting the app revokes every URL it created', async () => {
    await mountLocal()
    await play(stub.panels.get('s1')?.onPreview, png(1, 'a.png', keyN(1), false))
    await play(stub.panels.get('s2')?.onPreview, png(1, 'b.png', keyN(2), false))
    expect(created).toHaveLength(2)
    await unmountApp()
    expect([...revoked].sort()).toEqual([...created].sort())
  })
})

const remotePeer = {
  hostname: 'office-air.tail1234.ts.net',
  reachable: true,
  sessions: [{ id: 'sess-remote-1', name: 'Remote Session', status: 'idle', url: 'https://office-air.tail1234.ts.net:7444/sessions/sess-remote-1' }],
}

describe('App preview tabs: the session view stays mounted (UI-SPEC "Viewer Connection Requirement")', () => {
  it('a desktop peer tab keeps its view mounted, connected and inactive while its preview is shown', async () => {
    const c = await mountApp({
      GetRelayPort: () => 0,
      ListSessions: () => [],
      GetTailscaleStatus: () => ({ connected: true, hasCerts: true, ip: '100.64.0.1', domain: 'home-air.tail1234.ts.net' }),
      GetRemoteSessionsWithMeta: () => [remotePeer],
      ConnectPeerSession: () => ({ sessionId: 'sess-remote-1', name: 'Remote Session', host: 'office-air', url: 'https://office-air.tail1234.ts.net:7444/sessions/sess-remote-1?cap=CAP123', errorCode: '' }),
    })
    const row = Array.from(c.querySelectorAll<HTMLButtonElement>('button.sidebar__row')).find((b) => b.textContent === 'Remote Session')
    await click(row)
    expect(stub.viewMounts).toBe(1)
    expect(stub.views.get('sess-remote-1')?.isActive).toBe(true)

    await play(stub.views.get('sess-remote-1')?.onPreview, png(1, 'shot.png', KEY_A, true))
    expect(selected(c)).toBe('shot.png')
    expect(stub.viewMounts).toBe(1)
    expect(stub.viewUnmounts).toBe(0) // never unmounted, so its socket stays open
    expect(stub.views.get('sess-remote-1')?.isActive).toBe(false)
    const view = c.querySelector('[data-testid="web-session-view"]')!
    expect((view.closest('[role="tabpanel"]') as HTMLElement).style.display).toBe('none')
    // UI-SPEC: the sidebar keeps the session's row selected; the toolbar shows no chip or Share.
    expect(row!.getAttribute('aria-current')).toBe('page')
    expect(c.querySelector('.toolbar-chip')).toBeNull()
    expect(c.querySelector('.toolbar-share')).toBeNull()
    // Tooltip, accessible name and the peer dot name the Mac.
    const tab = tabButton(c, 'shot.png')!
    expect(tab.getAttribute('aria-label')).toBe('shot.png, preview from Remote Session on office-air')
    expect(tab.closest('.tab')?.getAttribute('title')).toBe('shot.png — Remote Session · office-air')
    expect(tab.querySelector('.peer-dot')).not.toBeNull()

    await click(tabButton(c, 'Remote Session · office-air'))
    expect(stub.viewMounts).toBe(1)
    expect(stub.viewUnmounts).toBe(0)
    expect(stub.views.get('sess-remote-1')?.isActive).toBe(true)
    expect(c.querySelector('.preview-panel')).toBeNull()
  })

  // 11-REVIEW WR-04: Retry's restored tabs replace a peer tab opened while it awaited, and its preview tab with it.
  it('Retry that replaces the tabs releases every preview: URLs revoked, no slot held against the 10-tab limit', async () => {
    let resolveRetry!: (v: unknown) => void
    let sessions: ReturnType<typeof session>[] = []
    const c = await mountApp({
      RetryDaemon: () => new Promise((r) => { resolveRetry = r }),
      GetRelayPort: () => 4321,
      ListSessions: () => sessions,
      GetTailscaleStatus: () => ({ connected: true, hasCerts: true, ip: '100.64.0.1', domain: 'home-air.tail1234.ts.net' }),
      GetRemoteSessionsWithMeta: () => [remotePeer],
      ConnectPeerSession: () => ({ sessionId: 'sess-remote-1', name: 'Remote Session', host: 'office-air', url: 'https://office-air.tail1234.ts.net:7444/sessions/sess-remote-1?cap=CAP123', errorCode: '' }),
    })
    await act(async () => { wailsEvents.get('daemon:recovered')?.() })
    await flush()
    await click(Array.from(c.querySelectorAll<HTMLButtonElement>('button.sidebar__row')).find((b) => b.textContent === 'Remote Session'))
    await play(stub.views.get('sess-remote-1')?.onPreview, png(1, 'shot.png', KEY_A, true))
    expect(tabNames(c)).toContain('shot.png')

    sessions = [session('s1', 's1 shell')]
    await act(async () => { resolveRetry(null) })
    await flush()
    expect(tabNames(c)).toEqual(['s1 shell'])
    expect(revoked).toEqual(['blob:test/1'])

    // The store is empty: ten new previews all get tabs, none evicted.
    for (let i = 1; i <= 10; i++) await play(stub.panels.get('s1')?.onPreview, png(i, `p${i}.png`, keyN(i), false))
    expect(tabNames(c).filter((n) => n.endsWith('.png'))).toHaveLength(10)
    expect(revoked).toEqual(['blob:test/1'])
  })

  it('the browser viewer keeps its view mounted behind a preview, and the preview closes with ×', async () => {
    window.history.pushState({}, '', '/app/?session=sess-web-1&cap=CAPW')
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({ hostname: 'office-air' }), { status: 200 })))
    try {
      const c = await mountApp({})
      expect(tabNames(c)).toEqual(['Session'])
      expect(stub.viewMounts).toBe(1)
      await play(stub.views.get('sess-web-1')?.onPreview, png(1, 'shot.png', KEY_A, true))
      expect(tabNames(c)).toEqual(['Session', 'shot.png'])
      expect(selected(c)).toBe('shot.png')
      expect(stub.viewUnmounts).toBe(0)
      expect(stub.views.get('sess-web-1')?.isActive).toBe(false)
      // The session pill keeps no close; the preview has one.
      expect(c.querySelectorAll('.tab__close')).toHaveLength(1)
      await click(c.querySelector('button[aria-label="Close shot.png"]'))
      expect(tabNames(c)).toEqual(['Session'])
      expect(selected(c)).toBe('Session')
      expect(stub.viewMounts).toBe(1)
      expect(stub.viewUnmounts).toBe(0)
      expect(stub.views.get('sess-web-1')?.isActive).toBe(true)
      expect(revoked).toEqual(['blob:test/1'])
    } finally {
      await unmountApp()
      vi.unstubAllGlobals()
      window.history.pushState({}, '', '/')
    }
  })
})
