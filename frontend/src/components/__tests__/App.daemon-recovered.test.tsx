// Review IN-01 (Phase 5): after a failed startup the app's poller can bring the
// daemon back in the background. Go then emits daemon:recovered, and the app
// clears the "Unable to connect" banner and reloads as Retry Connection does.

import { describe, it, expect, vi, afterEach } from 'vitest'
import React, { act } from 'react'

import { mountApp, unmountApp, flush, wailsEvents } from './appHarness'

// jsdom has no xterm.
vi.mock('../TerminalPanel', () => ({
  TerminalPanel: () => React.createElement('div', { 'data-testid': 'terminal-panel' }),
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
})
