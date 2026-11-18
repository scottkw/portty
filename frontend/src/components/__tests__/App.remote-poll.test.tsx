// Portty 05-13 (folded-in todo 2026-11-18-faster-remote-session-discovery):
// newly shared sessions on other Macs show up within about 5 s while the window
// is visible. The peer poll runs every 5 s when visible and every 30 s when
// hidden, refreshes at once on focus, on becoming visible and on a system wake,
// and never overlaps two fan-outs. The Go system:wake event reaches the page as
// a window 'portty:wake' event.

import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import { act } from 'react'

import type { RemotePeerSessions } from '../../lib/remoteSession'
import { mountApp, unmountApp, flush, wailsEvents } from './appHarness'

const peer: RemotePeerSessions = { hostname: 'office-air.tail1234.ts.net', reachable: true, sessions: [] }

let visibility = 'visible'
let poll: ReturnType<typeof vi.fn<(...args: unknown[]) => unknown>>

function mount() {
  return mountApp({
    GetDaemonError: () => '',
    GetRelayPort: () => 0,
    ListSessions: () => [],
    GetTailscaleStatus: () => ({ connected: true, hasCerts: true, ip: '100.64.0.1', domain: 'home-air.tail1234.ts.net' }),
    GetRemoteSessionsWithMeta: poll,
  })
}

const advance = async (ms: number) => { await act(async () => { await vi.advanceTimersByTimeAsync(ms) }); await flush() }
const fire = async (target: EventTarget, e: Event) => { await act(async () => { target.dispatchEvent(e) }); await flush() }

describe('App — remote peer poll cadence (05-13)', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    visibility = 'visible'
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => visibility })
    poll = vi.fn<(...args: unknown[]) => unknown>(() => [peer])
  })

  afterEach(async () => {
    await unmountApp()
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    delete (document as any).visibilityState
    vi.useRealTimers()
  })

  it('polls every 5 s while the window is visible', async () => {
    await mount()
    expect(poll).toHaveBeenCalledTimes(1)
    await advance(4_999)
    expect(poll).toHaveBeenCalledTimes(1)
    await advance(1)
    expect(poll).toHaveBeenCalledTimes(2)
    await advance(5_000)
    expect(poll).toHaveBeenCalledTimes(3)
  })

  it('polls every 30 s while hidden', async () => {
    visibility = 'hidden'
    await mount()
    expect(poll).toHaveBeenCalledTimes(1)
    await advance(25_000)
    expect(poll).toHaveBeenCalledTimes(1)
    await advance(5_000)
    expect(poll).toHaveBeenCalledTimes(2)
  })

  it('refreshes at once on focus, on becoming visible and on portty:wake', async () => {
    visibility = 'hidden'
    await mount()
    await advance(1_000)
    await fire(window, new Event('focus'))
    expect(poll).toHaveBeenCalledTimes(2)
    visibility = 'visible'
    await fire(document, new Event('visibilitychange'))
    expect(poll).toHaveBeenCalledTimes(3)
    await fire(window, new CustomEvent('portty:wake', { detail: 'wake' }))
    expect(poll).toHaveBeenCalledTimes(4)
  })

  it('never starts a second fan-out while one is in flight', async () => {
    let release!: (v: RemotePeerSessions[]) => void
    poll = vi.fn<(...args: unknown[]) => unknown>()
      .mockImplementationOnce(() => new Promise<RemotePeerSessions[]>((r) => { release = r }))
      .mockImplementation(() => [peer])
    await mount()
    await advance(15_000)
    await fire(window, new Event('focus'))
    await fire(window, new CustomEvent('portty:wake', { detail: 'network' }))
    expect(poll).toHaveBeenCalledTimes(1)
    await act(async () => { release([peer]) })
    await flush()
    await advance(5_000)
    expect(poll).toHaveBeenCalledTimes(2)
  })

  it('the Go system:wake event is re-dispatched on window as portty:wake with its reason', async () => {
    await mount()
    const seen: unknown[] = []
    const onWake = (e: Event) => seen.push((e as CustomEvent).detail)
    window.addEventListener('portty:wake', onWake)
    try {
      await act(async () => { wailsEvents.get('system:wake')!('network') })
      expect(seen).toEqual(['network'])
    } finally {
      window.removeEventListener('portty:wake', onWake)
    }
  })
})
