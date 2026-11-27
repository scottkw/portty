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
  sendInput: ReturnType<typeof vi.fn>
  pasteToMac: ReturnType<typeof vi.fn>
  emit: <K extends keyof DesktopEvents>(ev: K, v: DesktopEvents[K]) => void
}

const HOST = 'office-air.tail1234.ts.net'
const h = vi.hoisted(() => ({
  clients: [] as unknown[],
  deps: null as null | { onLocalNetwork(blocked: boolean): void },
  connect: vi.fn(),
  openSettings: vi.fn(),
  openBrowser: vi.fn(),
  readHome: vi.fn(),
}))

vi.mock('../../wailsjs/go/main/App', () => ({
  ConnectPeerDesktop: (host: string) => h.connect(host),
  OpenLocalNetworkSettings: () => h.openSettings(),
  OpenPeerDesktopInBrowser: (host: string) => h.openBrowser(host),
  ReadHomeClipboard: () => h.readHome(),
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

const last = () => onLabel.mock.calls[onLabel.mock.calls.length - 1][0] as
  | { line1: string; line2: string; controlling: boolean; tooltip: string }
  | null
/** Control is allowed: the viewer owns the desktop, rights arrive, and the picture is sized so a click lands on it. */
const ownAccount = () => {
  const video = container.querySelector('video')!
  for (const [a, b] of [['clientWidth', 1000], ['clientHeight', 500], ['videoWidth', 1600], ['videoHeight', 1000]] as const) {
    Object.defineProperty(video, a, { configurable: true, value: b })
  }
  emit('rights', { control: true, clipboard: true, surface: true })
}
async function picture() {
  emit('status', { kind: 'streaming' })
  emit('track', {} as MediaStream)
  await act(async () => { container.querySelector('video')!.dispatchEvent(new Event('playing')) })
}

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
  h.readHome.mockReset()
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

describe('the toolbar label feed (Task 3)', () => {
  it('says Not connected before a picture, then Click the picture to control once one shows and control is allowed', async () => {
    render()
    expect(last()).toMatchObject({ line1: 'On office-air', line2: 'Not connected', controlling: false })
    ownAccount()
    await picture()
    expect(last()).toMatchObject({ line1: 'On office-air', line2: 'Click the picture to control', controlling: false })
    expect(last()?.tooltip).toContain('While you control office-air, every key goes to that Mac')
  })

  it('says Controlling while the stage has the keyboard, and Locked when the Mac is locked and the keyboard is not captured', async () => {
    render()
    ownAccount()
    await picture()
    act(() => container.querySelector('.desktop-stage')!.dispatchEvent(new PointerEvent('pointerdown', { clientX: 500, clientY: 250, bubbles: true, button: 0, pointerId: 1 })))
    expect(last()).toMatchObject({ line1: 'Controlling office-air', controlling: true })
    act(() => (container.querySelector('textarea') as HTMLTextAreaElement).blur())
    emit('state', { state: 'locked', viewers: 1, locked: true })
    expect(last()).toMatchObject({ line2: 'Locked', controlling: false })
  })

  it('says View only without the control right, and reports null while the tab is not active', async () => {
    render()
    emit('rights', { control: false, clipboard: false, surface: false })
    await picture()
    expect(last()).toMatchObject({ line2: 'View only' })
    render({ active: false })
    expect(last()).toBeNull()
  })
})

describe('control mode (21-09): a click captures the keyboard and every way out lets go first', () => {
  const stage = () => container.querySelector('.desktop-stage') as HTMLElement
  const input = () => container.querySelector('textarea') as HTMLTextAreaElement
  const click = () => act(() => stage().dispatchEvent(new PointerEvent('pointerdown', { clientX: 500, clientY: 250, bubbles: true, button: 0, pointerId: 1 })))
  // The key and release messages the viewer sent, in order (moves, buttons and heartbeats are not under test).
  const sent = () =>
    (client().sendInput.mock.calls as [{ t: string; code?: string; d?: boolean }][])
      .map(([m]) => m)
      .filter((m) => m.t === 'key' || m.t === 'release')
      .map((m) => (m.t === 'key' ? `${m.code} ${m.d ? 'down' : 'up'}` : 'release'))
  const chord = (init: KeyboardEventInit) => {
    const ev = new KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...init })
    act(() => { input().dispatchEvent(ev) })
    return ev
  }
  async function controlling() {
    render()
    ownAccount()
    await picture()
    click()
    expect(document.activeElement).toBe(input())
  }

  it('tracer: Cmd+W goes to the Mac as a tap with the local default stopped, and leaving the tab releases everything and gives the keyboard back', async () => {
    await controlling()
    const ev = chord({ code: 'KeyW', key: 'w', metaKey: true })
    expect(ev.defaultPrevented).toBe(true)
    expect(sent()).toEqual(['KeyW down', 'KeyW up'])
    render({ active: false })
    expect(sent()[sent().length - 1]).toBe('release')
    expect(document.activeElement).not.toBe(input())
  })

  it('a click outside the picture (the keyboard leaves the stage) sends release', async () => {
    await controlling()
    act(() => input().blur())
    expect(sent()).toContain('release')
    expect(last()?.controlling).toBe(false)
  })

  it('Portty losing active-app status sends release and the stage input is blurred, so coming back is not control mode', async () => {
    await controlling()
    act(() => { window.dispatchEvent(new Event('blur')) })
    expect(sent()).toContain('release')
    expect(document.activeElement).not.toBe(input())
    act(() => { window.dispatchEvent(new Event('focus')) })
    expect(last()?.controlling).toBe(false)
  })

  it('a hidden window releases', async () => {
    await controlling()
    setVisibility('hidden')
    expect(sent()).toContain('release')
    expect(document.activeElement).not.toBe(input())
  })

  it('the stream leaving Streaming releases and blurs the stage input', async () => {
    await controlling()
    emit('status', { kind: 'reconnecting' })
    expect(sent()).toContain('release')
    expect(document.activeElement).not.toBe(input())
  })

  it('closing the tab sends release before the client is torn down', async () => {
    await controlling()
    const c = client()
    act(() => root.render(<div />))
    const calls = c.sendInput.mock.calls as [{ t: string }][]
    const lastRelease = calls.map(([m]) => m.t).lastIndexOf('release')
    expect(lastRelease).toBeGreaterThanOrEqual(0)
    expect(c.sendInput.mock.invocationCallOrder[lastRelease]).toBeLessThan(c.stop.mock.invocationCallOrder[0])
  })

  it('a Mac that locks keeps control mode on', async () => {
    await controlling()
    emit('state', { state: 'locked', viewers: 1, locked: true })
    expect(document.activeElement).toBe(input())
    expect(last()).toMatchObject({ line1: 'Controlling office-air', controlling: true })
  })

  it('the app\'s release chord Ctrl+Option+Cmd+Shift+Return is never forwarded, releases, and gives the keyboard back', async () => {
    await controlling()
    const ev = chord({ code: 'Enter', key: 'Enter', ctrlKey: true, altKey: true, metaKey: true, shiftKey: true })
    expect(ev.defaultPrevented).toBe(true)
    // Only releases went out (the tab's own exit repeats the chord's, which is harmless), never a key.
    expect(sent().length).toBeGreaterThan(0)
    expect(sent().every((m) => m === 'release')).toBe(true)
    expect(document.activeElement).not.toBe(input())
    expect(last()?.controlling).toBe(false)
  })

  it('the Control segment names the same chord in its tooltip', async () => {
    render()
    ownAccount()
    await picture()
    const seg = Array.from(container.querySelectorAll<HTMLButtonElement>('button')).find((b) => b.textContent === 'Control')
    expect(seg?.title).toContain('press Ctrl+Option+Cmd+Shift+Return.')
  })

  it('Cmd+C goes out as a tap and nothing reads the home clipboard', async () => {
    await controlling()
    const ev = chord({ code: 'KeyC', key: 'c', metaKey: true })
    expect(ev.defaultPrevented).toBe(true)
    expect(sent()).toEqual(['KeyC down', 'KeyC up'])
    expect(h.readHome).not.toHaveBeenCalled()
  })

  describe('Cmd+V pastes the home clipboard text, then the key', () => {
    const keys = () =>
      (client().sendInput.mock.calls as [{ t: string; code?: string; d?: boolean; m?: number }][])
        .map(([m]) => m)
        .filter((m) => m.t === 'key')
    const paste = async (init: KeyboardEventInit = {}) => {
      const ev = new KeyboardEvent('keydown', { bubbles: true, cancelable: true, code: 'KeyV', key: 'v', metaKey: true, ...init })
      await act(async () => { input().dispatchEvent(ev) })
      return ev
    }
    const home = (outcome: string, text = '') => h.readHome.mockResolvedValue({ outcome, text })

    it('different text: pasteToMac with it, then KeyV down and up with the keydown\'s modifier mask', async () => {
      await controlling()
      home('ok', 'hello')
      client().pasteToMac.mockResolvedValue({ outcome: 'ok' })
      const ev = await paste()
      expect(ev.defaultPrevented).toBe(true)
      expect(h.readHome).toHaveBeenCalledTimes(1)
      expect(client().pasteToMac).toHaveBeenCalledWith('hello')
      expect(keys().map((m) => [m.code, m.d, m.m])).toEqual([['KeyV', true, 8], ['KeyV', false, 8]])
      const firstKey = client().sendInput.mock.calls.findIndex(([m]) => (m as { t: string }).t === 'key')
      expect(client().pasteToMac.mock.invocationCallOrder[0]).toBeLessThan(client().sendInput.mock.invocationCallOrder[firstKey])
    })

    it('the same text again sends only the key, so copy then paste on the Mac keeps working', async () => {
      await controlling()
      home('ok', 'hello')
      client().pasteToMac.mockResolvedValue({ outcome: 'ok' })
      await paste()
      await paste()
      expect(client().pasteToMac).toHaveBeenCalledTimes(1)
      expect(keys()).toHaveLength(4)
      home('ok', 'other')
      await paste()
      expect(client().pasteToMac).toHaveBeenCalledTimes(2)
      expect(client().pasteToMac).toHaveBeenLastCalledWith('other')
    })

    it('no text sends only the key', async () => {
      await controlling()
      home('empty')
      await paste()
      expect(client().pasteToMac).not.toHaveBeenCalled()
      expect(sent()).toEqual(['KeyV down', 'KeyV up'])
    })

    it.each([
      ['concealed', 'Your clipboard holds a hidden item (password manager); not sent'],
      ['too_large', 'Your clipboard text is too large to send (limit 1 MB)'],
      ['failed', "Couldn't send to the Mac. Try again."],
    ])('%s sends nothing and no key, and says so', async (outcome, notice) => {
      await controlling()
      home(outcome, outcome === 'concealed' ? 'secret' : '')
      await paste()
      expect(client().pasteToMac).not.toHaveBeenCalled()
      expect(keys()).toHaveLength(0)
      expect(chip()).toBe(notice)
    })

    it('a refused or rejected read fails closed', async () => {
      await controlling()
      h.readHome.mockRejectedValue(new Error('boom'))
      await paste()
      expect(client().pasteToMac).not.toHaveBeenCalled()
      expect(keys()).toHaveLength(0)
      expect(chip()).toBe("Couldn't send to the Mac. Try again.")
    })

    it('a failed send sends no key, says so, and the text is not remembered as sent', async () => {
      await controlling()
      home('ok', 'hello')
      client().pasteToMac.mockResolvedValue({ outcome: 'failed' })
      await paste()
      expect(keys()).toHaveLength(0)
      expect(chip()).toBe("Couldn't send to the Mac. Try again.")
      client().pasteToMac.mockResolvedValue({ outcome: 'ok' })
      await paste()
      expect(client().pasteToMac).toHaveBeenCalledTimes(2)
      expect(keys()).toHaveLength(2)
    })

    it('a second Cmd+V while the first is in flight is ignored, and still never reaches the Mac as a plain key', async () => {
      await controlling()
      let resolve!: (v: { outcome: string; text: string }) => void
      h.readHome.mockReturnValue(new Promise((r) => { resolve = r }))
      client().pasteToMac.mockResolvedValue({ outcome: 'ok' })
      await paste()
      const second = await paste()
      expect(second.defaultPrevented).toBe(true)
      await act(async () => { resolve({ outcome: 'ok', text: 'hello' }) })
      expect(h.readHome).toHaveBeenCalledTimes(1)
      expect(client().pasteToMac).toHaveBeenCalledTimes(1)
      expect(keys()).toHaveLength(2)
    })

    it('leaving control mode while the paste is in flight sends no key', async () => {
      await controlling()
      let resolve!: (v: { outcome: string; text: string }) => void
      h.readHome.mockReturnValue(new Promise((r) => { resolve = r }))
      client().pasteToMac.mockResolvedValue({ outcome: 'ok' })
      await paste()
      act(() => { window.dispatchEvent(new Event('blur')) })
      await act(async () => { resolve({ outcome: 'ok', text: 'hello' }) })
      expect(keys()).toHaveLength(0)
    })

    it('Cmd+Shift+V is a plain key and reads nothing', async () => {
      await controlling()
      await paste({ shiftKey: true })
      expect(h.readHome).not.toHaveBeenCalled()
      expect(sent()).toEqual(['KeyV down', 'KeyV up'])
    })

    it('Cmd+V while not controlling is Portty\'s own paste: nothing is read or sent here', async () => {
      render()
      ownAccount()
      await picture()
      await paste()
      expect(h.readHome).not.toHaveBeenCalled()
      expect(client().pasteToMac).not.toHaveBeenCalled()
    })
  })
})
