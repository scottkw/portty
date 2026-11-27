// Portty 21-08 (DVWR-01) — the Desktop tab: the Local Network and refused cards, Reconnect, the wake bridge,
// the hidden-window pause, and the feed to the sidebar. The Phase 19 client is replaced by an emitter mock
// (as DesktopView.test does); createPeerDesktopOptions is the real one, wrapped only to capture its deps.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import type { DesktopEvents } from '../../lib/desktopClient'
import type { Tab } from '../TabBar'
import { DesktopTab } from '../DesktopTab'
import { HIDDEN_PAUSE_MS, LOCAL_NETWORK_CARD } from '../../lib/peerDesktop'

type Listener = (v: never) => void
interface MockClient {
  start: ReturnType<typeof vi.fn>
  stop: ReturnType<typeof vi.fn>
  wake: ReturnType<typeof vi.fn>
  emit: <K extends keyof DesktopEvents>(ev: K, v: DesktopEvents[K]) => void
}

const HOST = 'office-air.tail1234.ts.net'
const h = vi.hoisted(() => ({
  clients: [] as unknown[],
  deps: null as null | { onLocalNetwork(blocked: boolean): void },
  connect: vi.fn(),
  openSettings: vi.fn(),
  openBrowser: vi.fn(),
}))

vi.mock('../../wailsjs/go/main/App', () => ({
  ConnectPeerDesktop: (host: string) => h.connect(host),
  OpenLocalNetworkSettings: () => h.openSettings(),
  OpenPeerDesktopInBrowser: (host: string) => h.openBrowser(host),
}))

vi.mock('../../lib/peerDesktop', async (importOriginal) => {
  const real = await importOriginal<typeof import('../../lib/peerDesktop')>()
  return {
    ...real,
    createPeerDesktopOptions: (deps: Parameters<typeof real.createPeerDesktopOptions>[0]) => {
      h.deps = deps
      return real.createPeerDesktopOptions(deps)
    },
  }
})

vi.mock('../../lib/desktopClient', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../lib/desktopClient')>()),
  createDesktopClient: vi.fn(() => {
    const ls = new Map<string, Set<Listener>>()
    const client = {
      start: vi.fn(),
      stop: vi.fn(),
      wake: vi.fn(),
      send: vi.fn(),
      sendInput: vi.fn(() => true),
      currentEpoch: vi.fn(() => 1),
      requestSurfaces: vi.fn(),
      selectSurface: vi.fn(),
      copyFromMac: vi.fn(),
      pasteToMac: vi.fn(),
      on: (ev: string, cb: Listener) => {
        if (!ls.has(ev)) ls.set(ev, new Set())
        ls.get(ev)!.add(cb)
        return () => ls.get(ev)!.delete(cb)
      },
      emit: (ev: string, v: unknown) => {
        for (const cb of [...(ls.get(ev) ?? [])]) (cb as (x: unknown) => void)(v)
      },
    }
    h.clients.push(client)
    return client
  }),
}))

const tab: Tab = {
  id: `desktop:${HOST}`,
  sessionId: `desktop:${HOST}`,
  name: 'office-air › Desktop',
  type: 'desktop',
  peerHost: 'office-air',
  connectHost: HOST,
}

let container: HTMLDivElement
let root: Root
let visibility: DocumentVisibilityState = 'visible'

const client = () => h.clients[h.clients.length - 1] as MockClient
const card = () => container.querySelector('.desktop-card')
const heading = () => card()?.querySelector('h1')?.textContent
const chip = () => container.querySelector('.desktop-chip')?.textContent
const buttons = () => Array.from(container.querySelectorAll<HTMLButtonElement>('.desktop-card button'))
const button = (label: string) => buttons().find((b) => b.textContent === label)!

type Props = Partial<Parameters<typeof DesktopTab>[0]>
const onDesktopState = vi.fn()
const onLabel = vi.fn()
function render(over: Props = {}) {
  act(() => root.render(<DesktopTab tab={tab} active onDesktopState={onDesktopState} onLabel={onLabel} {...over} />))
}
const emit = <K extends keyof DesktopEvents>(ev: K, v: DesktopEvents[K]) => act(() => client().emit(ev, v))
const setVisibility = (v: DocumentVisibilityState) => {
  visibility = v
  act(() => { document.dispatchEvent(new Event('visibilitychange')) })
}
const advance = async (ms: number) => { await act(async () => { await vi.advanceTimersByTimeAsync(ms) }) }
const press = async (b: HTMLButtonElement) => { await act(async () => { b.click() }) }

beforeEach(() => {
  ;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
  vi.useFakeTimers()
  h.clients.length = 0
  h.deps = null
  h.connect.mockReset()
  h.connect.mockResolvedValue({ host: 'office-air', url: 'ws://127.0.0.1:7001/api/relay/remote-desktop/x/ws', errorCode: '' })
  h.openSettings.mockReset()
  h.openSettings.mockResolvedValue(undefined)
  h.openBrowser.mockReset()
  h.openBrowser.mockResolvedValue('')
  onDesktopState.mockReset()
  onLabel.mockReset()
  visibility = 'visible'
  Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => visibility })
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
})

afterEach(() => {
  act(() => root.unmount())
  container.remove()
  delete (document as unknown as { visibilityState?: unknown }).visibilityState
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('Desktop tab states in the app\'s words', () => {
  it('shows the Connecting card with the Allow hint before the first frame', () => {
    render()
    expect(heading()).toBe('Connecting…')
    expect(card()?.querySelector('p')?.textContent).toContain('choose Allow')
    expect(card()?.querySelector('p')?.textContent).not.toMatch(/browser|link|page/i)
  })

  it('asks ConnectPeerDesktop for the connect host once at mount and never shows a token', () => {
    render()
    expect(h.connect).toHaveBeenCalledTimes(1)
    expect(h.connect).toHaveBeenCalledWith(HOST)
    expect(container.innerHTML).not.toMatch(/[?&]cap=|token_/i)
  })

  it('a blocked Local Network shows its card with both actions; Open System Settings and Open in Browser call the bindings; the card clears when ICE progresses', async () => {
    render()
    act(() => h.deps!.onLocalNetwork(true))
    expect(heading()).toBe(LOCAL_NETWORK_CARD.heading)
    expect(buttons().map((b) => b.textContent)).toEqual(['Open System Settings', 'Open in Browser'])
    await press(button('Open System Settings'))
    expect(h.openSettings).toHaveBeenCalledTimes(1)
    await press(button('Open in Browser'))
    expect(h.openBrowser).toHaveBeenCalledWith(HOST)
    act(() => h.deps!.onLocalNetwork(false))
    expect(heading()).toBe('Connecting…')
  })

  it('the Local Network card replaces a client card, and the client keeps running behind it', () => {
    render()
    emit('status', { kind: 'unreachable' })
    expect(heading()).toBe("Can't reach this Mac")
    act(() => h.deps!.onLocalNetwork(true))
    expect(heading()).toBe(LOCAL_NETWORK_CARD.heading)
    expect(client().stop).not.toHaveBeenCalled()
  })

  it('a refusal and the other-account case show the Refused card with Reconnect, never the browser page', () => {
    render()
    emit('status', { kind: 'token_rejected' })
    expect(heading()).toBe("Can't open this desktop")
    expect(buttons().map((b) => b.textContent)).toEqual(['Reconnect'])
    emit('status', { kind: 'other_account' })
    expect(heading()).toBe("Can't open this desktop")
    expect(container.querySelector('.link-dead')).toBeNull()
  })

  it('desktop turned off shows the Desktop sharing is off card and no retry', () => {
    render()
    emit('status', { kind: 'desktop_off' })
    expect(heading()).toBe('Desktop sharing is off on this Mac')
    expect(buttons()).toHaveLength(0)
  })
})

describe('Reconnect and wake', () => {
  it('Reconnect on the unreachable card re-runs ConnectPeerDesktop, then wakes the running client at once', async () => {
    render()
    emit('status', { kind: 'unreachable' })
    expect(heading()).toBe("Can't reach this Mac")
    await press(button('Reconnect'))
    expect(h.connect).toHaveBeenCalledTimes(2)
    expect(client().wake).toHaveBeenCalledWith('external')
    expect(client().start).toHaveBeenCalledTimes(1)
  })

  it('Reconnect after a final status re-runs ConnectPeerDesktop and starts the client again', async () => {
    render()
    emit('status', { kind: 'token_rejected' })
    await press(button('Reconnect'))
    expect(h.connect).toHaveBeenCalledTimes(2)
    expect(client().start).toHaveBeenCalledTimes(2)
    expect(client().wake).not.toHaveBeenCalled()
  })

  it('portty:wake reaches the client as wake(external), once', () => {
    render()
    act(() => { window.dispatchEvent(new CustomEvent('portty:wake', { detail: 'system:wake' })) })
    expect(client().wake).toHaveBeenCalledTimes(1)
    expect(client().wake).toHaveBeenCalledWith('external')
  })
})

describe('hidden window pause (T-21-26)', () => {
  it('hidden 119 s changes nothing; hidden 120 s stops the client and shows Reconnecting', async () => {
    render()
    emit('status', { kind: 'streaming' })
    setVisibility('hidden')
    await advance(HIDDEN_PAUSE_MS - 1000)
    expect(client().stop).not.toHaveBeenCalled()
    await advance(1000)
    expect(client().stop).toHaveBeenCalledTimes(1)
    expect(chip()).toBe('Reconnecting…')
  })

  it('showing the window reconnects with a fresh ConnectPeerDesktop; the next picture clears the chip', async () => {
    render()
    emit('status', { kind: 'streaming' })
    setVisibility('hidden')
    await advance(HIDDEN_PAUSE_MS)
    expect(h.connect).toHaveBeenCalledTimes(1)
    setVisibility('visible')
    await advance(0)
    expect(h.connect).toHaveBeenCalledTimes(2)
    expect(client().start).toHaveBeenCalledTimes(2)
    expect(chip()).toBe('Reconnecting…')
    emit('track', {} as MediaStream)
    expect(chip() ?? '').not.toBe('Reconnecting…')
  })

  it('a window the system reports occluded counts as hidden even though the document says visible', async () => {
    render()
    act(() => { window.dispatchEvent(new CustomEvent('portty:window-hidden', { detail: true })) })
    await advance(HIDDEN_PAUSE_MS)
    expect(client().stop).toHaveBeenCalledTimes(1)
    act(() => { window.dispatchEvent(new CustomEvent('portty:window-hidden', { detail: false })) })
    await advance(0)
    expect(client().start).toHaveBeenCalledTimes(2)
  })
})

describe('the feed to the sidebar row', () => {
  const connectHost = HOST
  it('reports the five muted states and clears the row on streaming or when the connection goes', () => {
    render()
    emit('state', { state: 'needs_permission', viewers: 0, locked: false })
    emit('state', { state: 'awaiting_approval', viewers: 0, locked: false })
    emit('state', { state: 'display_asleep', viewers: 0, locked: false })
    emit('state', { state: 'locked', viewers: 1, locked: true })
    emit('state', { state: 'capture_paused', viewers: 0, locked: false })
    emit('state', { state: 'streaming', viewers: 1, locked: false })
    expect(onDesktopState.mock.calls).toEqual([
      [connectHost, 'needs_permission'],
      [connectHost, 'awaiting_approval'],
      [connectHost, 'display_asleep'],
      [connectHost, 'locked'],
      [connectHost, 'capture_paused'],
      [connectHost, null],
    ])
    emit('state', { state: 'display_asleep', viewers: 0, locked: false })
    emit('status', { kind: 'reconnecting' })
    expect(onDesktopState).toHaveBeenLastCalledWith(connectHost, null)
  })
})

describe('closing the tab', () => {
  it('unmounting disconnects at once', () => {
    render()
    const c = client()
    act(() => root.render(<div />))
    expect(c.stop).toHaveBeenCalledTimes(1)
  })
})
