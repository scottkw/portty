// Phase 23-06 — the viewer's Sound (DCTL-07, 23-UI-SPEC Surface 1): starts muted, one click unmutes,
// the hint, the failures and the sound-only bar. The client is mocked with emitters; the view is rendered into jsdom.
// The offer's stereo=1 and the hello's caps audio 1 are pinned at the client seam (lib/__tests__/desktopClient.test.ts).

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import type { ClientStatus, DesktopEvents } from '../../lib/desktopClient'
import { DesktopView, type DesktopViewProps } from '../DesktopView'
import { SOUND_HINT, SOUND_HINT_MS, SOUND_HINT_PHONE } from '../DesktopSoundButton'

type Listener = (v: never) => void
interface MockClient {
  emit: <K extends keyof DesktopEvents>(ev: K, v: DesktopEvents[K]) => void
}

const h = vi.hoisted(() => ({ clients: [] as unknown[] }))

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
      requestSurfaces: vi.fn(() => Promise.resolve({ displays: [], windows: [] })),
      selectSurface: vi.fn(),
      copyFromMac: vi.fn(() => Promise.resolve({ outcome: 'failed' })),
      pasteToMac: vi.fn(() => Promise.resolve({ outcome: 'ok' })),
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

let container: HTMLDivElement
let root: Root
let play: ReturnType<typeof vi.spyOn>

const client = () => h.clients[h.clients.length - 1] as MockClient
const video = () => container.querySelector('video') as HTMLVideoElement
const soundBtn = () => container.querySelector('.desktop-sound') as HTMLButtonElement | null
const toolbarButtons = () => Array.from(container.querySelectorAll('[role="toolbar"] button'))
const notice = () => container.querySelector('.desktop-chip--notice')
const status = (s: ClientStatus) => act(() => client().emit('status', s))
const advance = (ms: number) => act(() => { vi.advanceTimersByTime(ms) })
const announced = () => container.querySelector('[role="toolbar"] [role="status"]')?.textContent
const click = async (el: Element) => {
  await act(async () => {
    el.dispatchEvent(new MouseEvent('click', { bubbles: true }))
  })
}

function render(props: DesktopViewProps = {}) {
  act(() => root.render(<DesktopView {...props} />))
}
async function firstFrame() {
  status({ kind: 'streaming' })
  act(() => client().emit('track', {} as MediaStream))
  await act(async () => {
    video().dispatchEvent(new Event('playing'))
  })
}
const withAudio = () => act(() => client().emit('caps', { audio: true, unlock: false }))
const phoneQuery = () =>
  vi.spyOn(window, 'matchMedia').mockImplementation(
    (q: string) => ({ matches: true, media: q, addEventListener() {}, removeEventListener() {} }) as unknown as MediaQueryList,
  )

beforeEach(() => {
  ;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
  vi.useFakeTimers()
  h.clients.length = 0
  play = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
})

afterEach(() => {
  act(() => root.unmount())
  container.remove()
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('Sound item (tracer)', () => {
  it('a viewer whose hello offers audio sees Unmute with the crossed speaker, and the video starts muted', async () => {
    render()
    withAudio()
    await firstFrame()
    expect(video().muted).toBe(true)
    expect(soundBtn()?.getAttribute('aria-label')).toBe('Unmute')
    expect(soundBtn()?.textContent).toContain('Unmute')
    expect(soundBtn()?.title).toBe("Unmute: hear this Mac's sound")
  })

  it('one click clears muted and calls play() in the same handler, then the item reads Mute', async () => {
    render()
    withAudio()
    await firstFrame()
    play.mockClear()
    await click(soundBtn()!)
    expect(video().muted).toBe(false)
    expect(play).toHaveBeenCalledTimes(1)
    expect(soundBtn()?.getAttribute('aria-label')).toBe('Mute')
    expect(soundBtn()?.title).toBe('Mute')
  })

  it('a rejected play() mutes again, returns to Unmute and shows the alert', async () => {
    render()
    withAudio()
    await firstFrame()
    play.mockRejectedValueOnce(new DOMException('no gesture', 'NotAllowedError'))
    await click(soundBtn()!)
    expect(video().muted).toBe(true)
    expect(soundBtn()?.getAttribute('aria-label')).toBe('Unmute')
    expect(notice()?.getAttribute('role')).toBe('alert')
    expect(notice()?.textContent).toBe("Couldn't turn sound on. Choose Unmute to try again.")
  })

  it('a hello with no audio shows no Sound item and no bar', async () => {
    render()
    act(() => client().emit('caps', { audio: false, unlock: false }))
    await firstFrame()
    expect(soundBtn()).toBeNull()
    expect(container.querySelector('[role="toolbar"]')).toBeNull()
    advance(10000)
    expect(notice()).toBeNull()
  })
})

describe('the one-time hint', () => {
  it('shows from the first frame for 5 s with the pointer words, then goes', async () => {
    render()
    withAudio()
    await firstFrame()
    expect(notice()?.textContent).toBe(SOUND_HINT)
    expect(SOUND_HINT).toBe('Sound is off. Click Unmute in the bar to hear this Mac.')
    advance(SOUND_HINT_MS - 1)
    expect(notice()).not.toBeNull()
    advance(1 + 150)
    expect(notice()).toBeNull()
  })

  it('says Tap on a phone', async () => {
    phoneQuery()
    render()
    withAudio()
    await firstFrame()
    expect(notice()?.textContent).toBe(SOUND_HINT_PHONE)
    expect(SOUND_HINT_PHONE).toBe('Sound is off. Tap Unmute to hear this Mac.')
  })

  it('never shows under a card, then shows once the picture runs', async () => {
    play.mockRejectedValue(new DOMException('no gesture', 'NotAllowedError'))
    render()
    withAudio()
    status({ kind: 'streaming' })
    await act(async () => {
      client().emit('track', {} as MediaStream)
    })
    expect(container.querySelector('.desktop-card h1')?.textContent).toBe('Tap to start')
    expect(notice()).toBeNull()
    play.mockResolvedValue(undefined)
    await act(async () => {
      video().dispatchEvent(new Event('playing'))
    })
    expect(notice()?.textContent).toBe(SOUND_HINT)
  })

  it('is not shown when audio is not negotiated, and not again after a reconnect', async () => {
    render()
    withAudio()
    await firstFrame()
    advance(SOUND_HINT_MS + 150)
    expect(notice()).toBeNull()
    // A card covers the picture (so the picture is gone), then it returns: the hint stays used up.
    status({ kind: 'unreachable' } as ClientStatus)
    expect(container.querySelector('.desktop-card h1')?.textContent).toBe("Can't reach this Mac")
    withAudio()
    await firstFrame()
    expect(container.querySelector('.desktop-card')).toBeNull()
    expect(notice()).toBeNull()
  })

  it('unmuting removes it', async () => {
    render()
    withAudio()
    await firstFrame()
    expect(notice()?.textContent).toBe(SOUND_HINT)
    await click(soundBtn()!)
    advance(0)
    expect(notice()).toBeNull()
  })
})

describe('audio failed', () => {
  async function failed() {
    render()
    withAudio()
    await firstFrame()
    advance(SOUND_HINT_MS + 150)
    act(() => client().emit('state', { state: 'streaming', viewers: 1, locked: false, audio: 'failed' }))
  }

  it('greys the item with its reason, leaves the picture and the bar alone, and a click shows the alert', async () => {
    await failed()
    const b = soundBtn()!
    expect(b.getAttribute('aria-disabled')).toBe('true')
    expect(b.title).toBe("Sound isn't available from this Mac right now.")
    expect(container.querySelector('.desktop-card')).toBeNull()
    expect(container.querySelector('[role="toolbar"]')).not.toBeNull()
    expect(notice()).toBeNull()
    play.mockClear()
    await click(b)
    expect(play).not.toHaveBeenCalled()
    expect(notice()?.getAttribute('role')).toBe('alert')
    expect(notice()?.textContent).toBe("Sound isn't available from this Mac right now. The picture is not affected.")
  })

  it('losing sound that was on says so once; a viewer that was muted is told nothing', async () => {
    render()
    withAudio()
    await firstFrame()
    await click(soundBtn()!)
    advance(SOUND_HINT_MS + 150)
    expect(notice()).toBeNull()
    act(() => client().emit('state', { state: 'streaming', viewers: 1, locked: false, audio: 'failed' }))
    expect(notice()?.textContent).toBe("Sound isn't available from this Mac right now. The picture is not affected.")
    // The same failure again changes nothing.
    advance(5150)
    act(() => client().emit('state', { state: 'streaming', viewers: 2, locked: false, audio: 'failed' }))
    expect(notice()).toBeNull()
  })

  it('audio returning quietly makes the item live again', async () => {
    await failed()
    act(() => client().emit('state', { state: 'streaming', viewers: 1, locked: false }))
    expect(soundBtn()?.getAttribute('aria-disabled')).toBeNull()
    expect(notice()).toBeNull()
  })
})

describe('who gets the item', () => {
  it('a view-only guest with audio gets a bar holding only Sound', async () => {
    render({ guest: true })
    act(() => client().emit('admission', { state: 'admitted' }))
    act(() => client().emit('rights', { control: false, clipboard: false, surface: false }))
    withAudio()
    await firstFrame()
    const items = toolbarButtons()
    expect(items).toHaveLength(1)
    expect(items[0].getAttribute('aria-label')).toBe('Unmute')
    expect(container.querySelector('.desktop-view--bar')).not.toBeNull()
  })

  it('an own-account viewer gets Sound before Full Screen in the last group', async () => {
    Object.defineProperty(document, 'fullscreenEnabled', { value: true, configurable: true })
    render()
    act(() => client().emit('rights', { control: true, clipboard: true, surface: true }))
    withAudio()
    await firstFrame()
    const names = toolbarButtons().map((b) => b.getAttribute('aria-label'))
    expect(names.indexOf('Unmute')).toBeGreaterThan(-1)
    expect(names.indexOf('Unmute')).toBe(names.indexOf('Full Screen') - 1)
  })

  it('stays enabled while Locked, Reconnecting and in View Only', async () => {
    render()
    act(() => client().emit('rights', { control: true, clipboard: true, surface: true }))
    withAudio()
    await firstFrame()
    act(() => client().emit('state', { state: 'streaming', viewers: 1, locked: true }))
    expect(soundBtn()?.getAttribute('aria-disabled')).toBeNull()
    status({ kind: 'reconnecting', attempt: 1 } as ClientStatus)
    expect(soundBtn()?.getAttribute('aria-disabled')).toBeNull()
    const viewOnly = Array.from(container.querySelectorAll('[role="radio"]')).find((r) => r.textContent === 'View Only')!
    await click(viewOnly)
    expect(soundBtn()?.getAttribute('aria-disabled')).toBeNull()
  })

  it('a phone has no Sound item in the page bar; its Sound item is in the phone bottom bar (23-08)', async () => {
    phoneQuery()
    render()
    withAudio()
    await firstFrame()
    expect(container.querySelector('.desktop-overlay .desktop-sound')).toBeNull()
    expect(container.querySelector('.desktop-phone__slot .desktop-sound')).not.toBeNull()
  })
})

describe('not remembered, and said aloud', () => {
  it('a remount starts muted and no storage or cookie was touched by toggling', async () => {
    const before = [JSON.stringify({ ...window.localStorage }), JSON.stringify({ ...window.sessionStorage }), document.cookie]
    render()
    withAudio()
    await firstFrame()
    await click(soundBtn()!)
    expect(video().muted).toBe(false)
    await click(soundBtn()!)
    await click(soundBtn()!)
    expect(video().muted).toBe(false)
    expect([JSON.stringify({ ...window.localStorage }), JSON.stringify({ ...window.sessionStorage }), document.cookie]).toEqual(before)

    act(() => root.unmount())
    root = createRoot(container)
    h.clients.length = 0
    render()
    withAudio()
    await firstFrame()
    expect(video().muted).toBe(true)
    expect(soundBtn()?.getAttribute('aria-label')).toBe('Unmute')
  })

  it('a reconnect inside the page keeps what the user chose', async () => {
    render()
    withAudio()
    await firstFrame()
    await click(soundBtn()!)
    status({ kind: 'reconnecting', attempt: 1 } as ClientStatus)
    await firstFrame()
    expect(video().muted).toBe(false)
    expect(soundBtn()?.getAttribute('aria-label')).toBe('Mute')
  })

  it('a reconnect that the browser will not play with sound drops the sound and keeps the picture', async () => {
    render()
    withAudio()
    await firstFrame()
    await click(soundBtn()!)
    play.mockReset()
    play.mockRejectedValueOnce(new DOMException('no gesture', 'NotAllowedError')).mockResolvedValue(undefined)
    await act(async () => {
      client().emit('track', {} as MediaStream)
    })
    expect(video().muted).toBe(true)
    expect(soundBtn()?.getAttribute('aria-label')).toBe('Unmute')
    expect(container.querySelector('.desktop-card h1')?.textContent).not.toBe('Tap to start')
    expect(notice()?.textContent).toBe("Couldn't turn sound on. Choose Unmute to try again.")
  })

  it('announces Sound on and Sound off after each change', async () => {
    render()
    withAudio()
    await firstFrame()
    expect(announced()).toBe('')
    await click(soundBtn()!)
    expect(announced()).toBe('Sound on')
    await click(soundBtn()!)
    expect(announced()).toBe('Sound off')
  })
})

// Phase 23-08: the optional onSound prop feeds the app tab's playing indicator.
describe('onSound (the app tab indicator)', () => {
  it('is true once Unmute plays, false on Mute', async () => {
    const onSound = vi.fn()
    render({ onSound })
    withAudio()
    await firstFrame()
    expect(onSound).toHaveBeenLastCalledWith(false)
    await click(soundBtn()!)
    expect(onSound).toHaveBeenLastCalledWith(true)
    await click(soundBtn()!)
    expect(onSound).toHaveBeenLastCalledWith(false)
  })

  it('is false on disconnect and true again when the picture returns with sound still on', async () => {
    const onSound = vi.fn()
    render({ onSound })
    withAudio()
    await firstFrame()
    await click(soundBtn()!)
    status({ kind: 'reconnecting' } as ClientStatus)
    expect(onSound).toHaveBeenLastCalledWith(false)
    status({ kind: 'streaming' })
    expect(onSound).toHaveBeenLastCalledWith(true)
  })

  it('is false while the host has the client paused (a hidden window)', async () => {
    const onSound = vi.fn()
    render({ onSound })
    withAudio()
    await firstFrame()
    await click(soundBtn()!)
    render({ onSound, paused: true })
    expect(onSound).toHaveBeenLastCalledWith(false)
  })

  it('is false on unmount', async () => {
    const onSound = vi.fn()
    render({ onSound })
    withAudio()
    await firstFrame()
    await click(soundBtn()!)
    act(() => root.render(<div />))
    expect(onSound).toHaveBeenLastCalledWith(false)
  })

  it('is optional: no prop, no error', async () => {
    render()
    withAudio()
    await firstFrame()
    await click(soundBtn()!)
    expect(soundBtn()?.getAttribute('aria-label')).toBe('Mute')
  })
})
