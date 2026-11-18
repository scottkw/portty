/**
 * WebShareSessionView.test.tsx — Phase 155-03 (PARITY-01)
 *
 * Tests:
 *   1. Component renders without throwing given minimal props.
 *   2. wsURL construction: wss://{host}/sessions/{id}/ws?cap={encoded} is
 *      forwarded to TerminalPanel (Pitfall 6 guard).
 *   3. Root element carries hub-modal__body--interactive CSS class (layout).
 *   4. No chat control renders on either path (Portty 02-07, CHAT-01).
 *
 * Architecture: TerminalPanel is mocked so xterm / WebSocket side
 * effects do not run in jsdom. Props are captured via module-level variables so
 * assertions run after render without needing react-testing-library.
 */
import { describe, it, expect, vi, afterEach } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { act } from 'react'

// ── Mock TerminalPanel ────────────────────────────────────────────────────────
// Captures all props forwarded by WebShareSessionView so we can assert on
// sessionId and wsURL without importing xterm (absent in jsdom).

interface CapturedTerminalProps {
  sessionId?: string
  relayPort?: number
  isActive?: boolean
  wsURL?: string
  remote?: boolean
  [key: string]: unknown
}

let capturedTerminalProps: CapturedTerminalProps | null = null

vi.mock('../TerminalPanel', () => ({
  TerminalPanel: (props: CapturedTerminalProps) => {
    capturedTerminalProps = props
    return React.createElement('div', { 'data-testid': 'mock-terminal-panel' })
  },
}))

// ── Import under test (after vi.mock — vitest hoists vi.mock automatically) ──

import { WebShareSessionView } from '../WebShareSessionView'

// ── Helpers ───────────────────────────────────────────────────────────────────

function mountView(
  overrides: Partial<React.ComponentProps<typeof WebShareSessionView>> = {},
) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  act(() => {
    root.render(
      <WebShareSessionView
        sessionId="sess-123"
        capToken="tok-abc"
        relayPort={0}
        // Phase 168 FIX-01: explicitly pass pluginConfig=null (not omitted) so
        // this suite's wsURL/rendering assertions don't trigger the web-guest
        // self-fetch/EventSource effect (isWebGuest is keyed on `undefined`,
        // not `null`) — that behavior has its own dedicated suite in
        // __tests__/WebShareSessionView.plugin-config.test.tsx.
        pluginConfig={null}
        {...overrides}
      />,
    )
  })
  return {
    container,
    root,
    unmount: () => {
      act(() => { root.unmount() })
      container.remove()
    },
  }
}

// ── Tests ────────────────────────────────────────────────────────────────────

afterEach(() => {
  capturedTerminalProps = null
  vi.clearAllMocks()
})

describe('WebShareSessionView — renders without error', () => {
  it('mounts without throwing given minimal props', () => {
    const { unmount } = mountView()
    // If we get here without throwing, the render succeeded.
    unmount()
  })

  it('renders the hub-modal__body--interactive root class (parity gate)', () => {
    const { container, unmount } = mountView()
    const root = container.querySelector('.hub-modal__body--interactive')
    expect(root).not.toBeNull()
    unmount()
  })

  it('renders no chat control: only the terminal (CHAT-01)', () => {
    const { container, unmount } = mountView()
    expect(container.querySelector('[data-testid="mock-terminal-panel"]')).not.toBeNull()
    expect(container.querySelector('button')).toBeNull()
    expect(container.textContent ?? '').not.toMatch(/chat/i)
    unmount()
  })
})

describe('WebShareSessionView — wsURL construction (Pitfall 6 guard)', () => {
  it('constructs {wsScheme}://{host}/sessions/{id}/ws?cap={encoded} and forwards to TerminalPanel', () => {
    const { unmount } = mountView({ sessionId: 'sess-123', capToken: 'tok-abc' })
    // Derive expected URL from window.location so the test is hermetic regardless
    // of jsdom's configured origin (vitest configures http://localhost:3000 by
    // default). Phase 168 FIX-01: wsURL's scheme now derives from the resolved
    // origin's http/https scheme (ws/wss) instead of a hardcoded 'wss'.
    const wsScheme = window.location.protocol === 'https:' ? 'wss' : 'ws'
    const expectedURL = `${wsScheme}://${window.location.host}/sessions/${encodeURIComponent('sess-123')}/ws?cap=${encodeURIComponent('tok-abc')}`
    expect(capturedTerminalProps?.wsURL).toBe(expectedURL)
    unmount()
  })

  it('percent-encodes special characters in sessionId and capToken', () => {
    const { unmount } = mountView({ sessionId: 'sess/123', capToken: 'tok=abc&x=1' })
    const wsScheme = window.location.protocol === 'https:' ? 'wss' : 'ws'
    const expectedURL = `${wsScheme}://${window.location.host}/sessions/${encodeURIComponent('sess/123')}/ws?cap=${encodeURIComponent('tok=abc&x=1')}`
    expect(capturedTerminalProps?.wsURL).toBe(expectedURL)
    unmount()
  })

  it('wsURL shape matches {ws|wss}://{host}/sessions/{id}/ws?cap= pattern', () => {
    const { unmount } = mountView({ sessionId: 'sess-abc', capToken: 'cap-xyz' })
    const wsURL = capturedTerminalProps?.wsURL as string
    expect(wsURL).toMatch(/^wss?:\/\//)
    expect(wsURL).toContain('/sessions/')
    expect(wsURL).toContain('/ws?cap=')
    unmount()
  })
})

// Phase 168-09 (FIX-03 RC-A): remote in-app tabs MUST route the terminal through
// the daemon proxy (remote:true, NO wsURL) rather than the direct cross-origin peer
// wss the peer's byte-exact Origin allowlist 403s. The native web-guest path is unchanged.
describe('WebShareSessionView — remote-tab daemon-proxy transport (FIX-03 RC-A)', () => {
  it('remote={true}: TerminalPanel gets remote:true AND no wsURL (daemon proxy, not direct peer wss)', () => {
    const { unmount } = mountView({ remote: true, relayPort: 42, baseURL: 'https://peer.example.ts.net' })
    // Daemon-proxy transport: RelayClient will build
    // ws://127.0.0.1:{relayPort}/api/relay/remote/{id}/ws (relayClient.ts:247-250).
    expect(capturedTerminalProps?.remote).toBe(true)
    expect(capturedTerminalProps?.wsURL).toBeUndefined()
    unmount()
  })

  it('default path (no remote): TerminalPanel still gets the direct wsURL, remote falsy', () => {
    const { unmount } = mountView({ sessionId: 'sess-123', capToken: 'tok-abc' })
    const wsScheme = window.location.protocol === 'https:' ? 'wss' : 'ws'
    const expectedURL = `${wsScheme}://${window.location.host}/sessions/${encodeURIComponent('sess-123')}/ws?cap=${encodeURIComponent('tok-abc')}`
    expect(capturedTerminalProps?.wsURL).toBe(expectedURL)
    expect(capturedTerminalProps?.remote).toBeFalsy()
    unmount()
  })

  it('remote={true}: renders the terminal and no chat control (CHAT-01)', () => {
    const { container, unmount } = mountView({ remote: true, relayPort: 42, baseURL: 'https://peer.example.ts.net' })
    expect(container.querySelector('[data-testid="mock-terminal-panel"]')).not.toBeNull()
    expect(container.querySelector('button')).toBeNull()
    unmount()
  })
})

// Portty 04-06 (SHAR-04): the stopped-sharing banner is rendered by TerminalPanel
// (TerminalPanel.guestClose.test.tsx); this view hands it the host label and callback.
describe('WebShareSessionView — stopped-sharing props (SHAR-04)', () => {
  it('passes hostLabel and onShareStopped through to TerminalPanel', () => {
    const onShareStopped = vi.fn()
    const { unmount } = mountView({ hostLabel: 'office-air', onShareStopped })
    expect(capturedTerminalProps?.hostLabel).toBe('office-air')
    ;(capturedTerminalProps?.onShareStopped as () => void)()
    expect(onShareStopped).toHaveBeenCalledTimes(1)
    unmount()
  })

  it('an empty host label reaches TerminalPanel as empty (the banner then shows the fallback)', () => {
    const { unmount } = mountView({ hostLabel: '' })
    expect(capturedTerminalProps?.hostLabel).toBe('')
    unmount()
  })
})

// Portty 05-05 (SHAR-07): before each redial the browser view asks the share
// server whether its cap still works; remote tabs pass App's hook through.
describe('WebShareSessionView — pre-redial check (SHAR-07)', () => {
  afterEach(() => { vi.unstubAllGlobals() })

  type Hook = () => Promise<unknown>

  it('browser path: probes /info with the encoded id and cap and maps the status', async () => {
    const fetchMock = vi.fn(async () => new Response(null, { status: 403 }))
    vi.stubGlobal('fetch', fetchMock)
    const { unmount } = mountView({ sessionId: 'sess/1', capToken: 'tok=a&b', baseURL: 'https://office-air.ts.net:7444' })
    await expect((capturedTerminalProps?.beforeReconnect as Hook)()).resolves.toEqual({ kind: 'stop', reason: 'share-stopped' })
    expect(fetchMock).toHaveBeenCalledWith(
      `https://office-air.ts.net:7444/api/sessions/${encodeURIComponent('sess/1')}/info?cap=${encodeURIComponent('tok=a&b')}`,
      expect.objectContaining({ cache: 'no-store' }),
    )
    unmount()
  })

  it.each([
    [200, { kind: 'ok' }],
    [404, { kind: 'stop', reason: 'ended' }],
    [500, { kind: 'retry' }],
  ])('browser path: status %i resolves %o', async (status, verdict) => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(null, { status })))
    const { unmount } = mountView()
    await expect((capturedTerminalProps?.beforeReconnect as Hook)()).resolves.toEqual(verdict)
    unmount()
  })

  it('browser path: a probe that hangs times out after 10 s and retries (WR-06)', async () => {
    const ctrl = new AbortController()
    const timeout = vi.spyOn(AbortSignal, 'timeout').mockReturnValue(ctrl.signal)
    vi.stubGlobal('fetch', vi.fn((_url: string, init: RequestInit) => new Promise((_resolve, reject) => {
      init.signal?.addEventListener('abort', () => reject(new DOMException('timed out', 'TimeoutError')))
    })))
    const { unmount } = mountView()
    const verdict = (capturedTerminalProps?.beforeReconnect as Hook)()
    expect(timeout).toHaveBeenCalledWith(10_000)
    ctrl.abort()
    await expect(verdict).resolves.toEqual({ kind: 'retry' })
    timeout.mockRestore()
    unmount()
  })

  it('browser path: a rejected fetch retries', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('network') }))
    const { unmount } = mountView()
    await expect((capturedTerminalProps?.beforeReconnect as Hook)()).resolves.toEqual({ kind: 'retry' })
    unmount()
  })

  it('remote path: passes the given hook unchanged and never fetches', () => {
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    const hook = vi.fn(async () => ({ kind: 'ok' as const, port: 1 }))
    const { unmount } = mountView({ remote: true, relayPort: 42, baseURL: 'https://peer.example.ts.net', beforeReconnect: hook })
    expect(capturedTerminalProps?.beforeReconnect).toBe(hook)
    expect(fetchMock).not.toHaveBeenCalled()
    unmount()
  })
})
