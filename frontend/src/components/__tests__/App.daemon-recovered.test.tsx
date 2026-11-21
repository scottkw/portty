// Review IN-01 (Phase 5): after a failed startup the app's poller can bring the
// daemon back in the background. Go then emits daemon:recovered, and the app
// clears the "Unable to connect" banner and reloads as Retry Connection does.

import { describe, it, expect, vi, afterEach } from 'vitest'
import React, { act } from 'react'

import { mountApp, unmountApp, flush, wailsEvents } from './appHarness'
import { peerSnapshotKey, putPeerSnapshot, takePeerSnapshot } from '../../lib/peerSnapshots'

// jsdom has no xterm.
vi.mock('../TerminalPanel', () => ({
  TerminalPanel: () => React.createElement('div', { 'data-testid': 'terminal-panel' }),
}))
// jsdom has no EventSource either (the peer view's plugin-config stream).
vi.mock('../WebShareSessionView', () => ({
  WebShareSessionView: (p: { fontSize?: number; onFontSizeChange?: (delta: number) => void }) =>
    React.createElement('div', { 'data-testid': 'web-session-view', 'data-font-size': p.fontSize },
      React.createElement('button', { type: 'button', 'data-testid': 'fire-font-up', onClick: () => p.onFontSizeChange?.(1) })),
}))

const session = (id: string, name: string) => ({
  id, name, state: 'running', status: 'idle', createdAt: '', hostname: '', webEnabled: false, viewerCount: 0, workDir: '/',
})

afterEach(async () => {
  await unmountApp()
})

describe('App — daemon:recovered (review IN-01)', () => {
  it('clears the startup error banner and reloads like Retry', async () => {
    const retry = vi.fn(() => null)
    const container = await mountApp({
      GetDaemonError: () => 'daemon did not start',
      RetryDaemon: retry,
      GetRelayPort: () => 4321,
      ListSessions: () => [],
      GetTailscaleStatus: () => ({ connected: true, hasCerts: true, ip: '100.64.0.1', domain: 'home-air.tail1234.ts.net' }),
    })
    expect(container.textContent).toContain('Unable to connect to session daemon')
    expect(retry).not.toHaveBeenCalled()

    await act(async () => { wailsEvents.get('daemon:recovered')?.() })
    await flush()

    expect(retry).toHaveBeenCalledTimes(1)
    expect(container.textContent).not.toContain('Unable to connect to session daemon')
  })

  // Review it3 WR-02: recovery with tabs open must not replace them with the daemon's sessions.
  it('keeps the open tabs and the active tab when tabs are open: no reload, only the relay port', async () => {
    const retry = vi.fn(() => null)
    let sessions = [session('s1', 'first shell'), session('s2', 'second shell')]
    const relayPort = vi.fn(() => 4321)
    const container = await mountApp({
      RetryDaemon: retry,
      GetRelayPort: relayPort,
      ListSessions: () => sessions,
      GetTailscaleStatus: () => ({ connected: true, hasCerts: true, ip: '100.64.0.1', domain: 'home-air.tail1234.ts.net' }),
    })
    const tabs = () => [...container.querySelectorAll('[role="tab"]')].map((t) => t.textContent)
    const selected = () => container.querySelector('[role="tab"][aria-selected="true"]')?.textContent
    expect(tabs().join('|')).toMatch(/first shell.*\|.*second shell/)
    await act(async () => { (container.querySelectorAll('[role="tab"]')[1] as HTMLElement).click() })
    await flush()
    expect(selected()).toContain('second shell')
    await act(async () => { wailsEvents.get('daemon:error')?.('session daemon keeps stopping') })
    const portCalls = relayPort.mock.calls.length

    sessions = [session('s3', 'respawned shell')] // the daemon restarted: only a new session in it
    await act(async () => { wailsEvents.get('daemon:recovered')?.() })
    await flush()

    expect(retry).not.toHaveBeenCalled()
    expect(tabs().join('|')).toMatch(/first shell.*\|.*second shell/)
    expect(tabs().join('|')).not.toContain('respawned shell')
    expect(selected()).toContain('second shell')
    expect(relayPort.mock.calls.length).toBe(portCalls + 1)
  })

  // INPT-04: both Retry entry points need zero tabs, so a peer tab exists only if it
  // opened while retryInit awaited. Retry's restored tabs replace it; its snapshot goes too.
  it('Retry that replaces a peer tab opened while it awaited drops that tab\'s snapshot (takePeerSnapshot is undefined)', async () => {
    let resolveRetry!: (v: unknown) => void
    const retry = vi.fn(() => new Promise((r) => { resolveRetry = r }))
    let sessions: ReturnType<typeof session>[] = []
    const peer = { hostname: 'office-air', dnsName: 'office-air.tail1234.ts.net', reachable: true, sessions: [{ id: 'p1', name: 'Remote Shell', status: 'idle', url: '' }] }
    const container = await mountApp({
      RetryDaemon: retry,
      GetRelayPort: () => 4321,
      ListSessions: () => sessions,
      GetTailscaleStatus: () => ({ connected: true, hasCerts: true, ip: '100.64.0.1', domain: 'home-air.tail1234.ts.net' }),
      GetRemoteSessionsWithMeta: () => [peer],
      ConnectPeerSession: (host: unknown, id: unknown) => ({ sessionId: id, name: 'Remote Shell', host: 'office-air', url: `https://${host}:7444/sessions/${id}?cap=C`, errorCode: '' }),
    })
    const tabs = () => [...container.querySelectorAll('[role="tab"]')].map((t) => t.textContent ?? '')
    expect(tabs()).toHaveLength(0)

    await act(async () => { wailsEvents.get('daemon:recovered')?.() })
    await flush()
    expect(retry).toHaveBeenCalledTimes(1) // retryInit is now waiting on RetryDaemon

    const row = [...container.querySelectorAll<HTMLButtonElement>('button.sidebar__row')].find((b) => b.textContent === 'Remote Shell')!
    await act(async () => { row.click() })
    await flush()
    expect(tabs().some((t) => t.includes('Remote Shell'))).toBe(true)
    const key = peerSnapshotKey('office-air.tail1234.ts.net', 'p1')
    putPeerSnapshot(key, { data: 'x', cols: 80, rows: 24, offset: 1 })

    sessions = [session('s1', 'local shell')]
    await act(async () => { resolveRetry(null) })
    await flush()

    expect(takePeerSnapshot(key)).toBeUndefined()
    expect(tabs().some((t) => t.includes('Remote Shell'))).toBe(false)
    expect(tabs().some((t) => t.includes('local shell'))).toBe(true)
  })

  // IN-06: the replaced peer tab's font size goes too, so reopening it starts from the default.
  it('Retry that replaces a peer tab drops its font size: the reopened tab starts from the default', async () => {
    let resolveRetry!: (v: unknown) => void
    const retry = vi.fn(() => new Promise((r) => { resolveRetry = r }))
    let sessions: ReturnType<typeof session>[] = []
    const peer = { hostname: 'office-air', dnsName: 'office-air.tail1234.ts.net', reachable: true, sessions: [{ id: 'p1', name: 'Remote Shell', status: 'idle', url: '' }] }
    const container = await mountApp({
      RetryDaemon: retry,
      GetRelayPort: () => 4321,
      ListSessions: () => sessions,
      GetTailscaleStatus: () => ({ connected: true, hasCerts: true, ip: '100.64.0.1', domain: 'home-air.tail1234.ts.net' }),
      GetRemoteSessionsWithMeta: () => [peer],
      ConnectPeerSession: (host: unknown, id: unknown) => ({ sessionId: id, name: 'Remote Shell', host: 'office-air', url: `https://${host}:7444/sessions/${id}?cap=C`, errorCode: '' }),
    })
    const openPeer = async () => {
      const row = [...container.querySelectorAll<HTMLButtonElement>('button.sidebar__row')].find((b) => b.textContent === 'Remote Shell')!
      await act(async () => { row.click() })
      await flush()
    }
    const fontSize = () => Number(container.querySelector('[data-testid="web-session-view"]')!.getAttribute('data-font-size'))

    await act(async () => { wailsEvents.get('daemon:recovered')?.() })
    await flush()
    await openPeer()
    const base = fontSize()
    await act(async () => { container.querySelector<HTMLButtonElement>('[data-testid="fire-font-up"]')!.click() })
    await flush()
    expect(fontSize()).toBe(base + 1)

    sessions = [session('s1', 'local shell')]
    await act(async () => { resolveRetry(null) })
    await flush()
    await openPeer()
    expect(fontSize()).toBe(base)
  })
})
