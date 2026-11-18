// Review IN-01 (Phase 5): after a failed startup the app's poller can bring the
// daemon back in the background. Go then emits daemon:recovered, and the app
// clears the "Unable to connect" banner and reloads as Retry Connection does.

import { describe, it, expect, vi, afterEach } from 'vitest'
import { act } from 'react'

import { mountApp, unmountApp, flush, wailsEvents } from './appHarness'

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
})
