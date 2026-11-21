// Phase 11 (PREV-01, PREV-02, PREV-04): preview tabs in the real App.
//
// The real App mounts with the Wails bridge faked (appHarness). TerminalPanel and
// WebShareSessionView are stubbed (no xterm in jsdom); each stub records the onPreview
// prop App gave it, so a test can play the events a session's socket would produce.

import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import React, { act } from 'react'

import { mountApp, unmountApp, flush } from './appHarness'
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
