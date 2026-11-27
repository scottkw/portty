// Phase 23-08 — the iPhone bar's Sound item, Show Keyboard greyed while Locked, and the empty slot while the
// Unlock field has focus (23-UI-SPEC Surface 1 iPhone row, Surface 2 Phone and keyboard rules).
// The first group renders DesktopPhoneBars alone; the second drives DesktopView's phone branch with a mocked client.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import type { DesktopClient, DesktopEvents } from '../../lib/desktopClient'
import { DesktopPhoneBars, type DesktopPhoneBarsProps, type PhoneModifier } from '../DesktopPhoneBars'
import { PHONE_FIELDS } from '../DesktopPhoneLayer'
import { DesktopView } from '../DesktopView'

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
      sendUnlock: vi.fn(() => true),
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

function props(over: Partial<DesktopPhoneBarsProps> = {}): DesktopPhoneBarsProps {
  return {
    keyboardUp: false,
    zoom: 1,
    onFit: vi.fn(),
    onShowKeyboard: vi.fn(),
    onKey: vi.fn(),
    armed: new Set<PhoneModifier>(),
    onToggleArm: vi.fn(),
    sendChord: vi.fn(),
    onSelectSurface: vi.fn(),
    surfaceName: 'Main Display',
    surface: { kind: 'display', id: 1 },
    canControl: true,
    canPickSurface: true,
    disabledReason: null,
    client: { requestSurfaces: vi.fn(), selectSurface: vi.fn() } as unknown as DesktopClient,
    ...over,
  }
}
const sound = (video: HTMLVideoElement | null, over: Record<string, unknown> = {}) => ({
  video, failed: false, onNotice: vi.fn(), onChange: vi.fn(), ...over,
})

const renderBars = (p: DesktopPhoneBarsProps) => act(() => root.render(<DesktopPhoneBars {...p} />))
const toolbar = () => container.querySelector('[role="toolbar"]') as HTMLElement | null
const labels = () => Array.from(toolbar()!.querySelectorAll('button')).map((b) => b.getAttribute('aria-label') ?? b.textContent)
const btn = (label: string) =>
  Array.from(container.querySelectorAll<HTMLButtonElement>('button')).find(
    (b) => b.getAttribute('aria-label') === label || b.textContent === label,
  )!
const click = async (el: Element) => {
  await act(async () => {
    el.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, detail: 1 }))
  })
}
const pointerdown = (el: Element) => act(() => { el.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true, cancelable: true })) })

beforeEach(() => {
  ;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
  h.clients.length = 0
  play = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
})
afterEach(() => {
  act(() => root.unmount())
  container.remove()
  document.body.innerHTML = ''
  vi.restoreAllMocks()
})

describe('Sound in the bottom bar (tracer)', () => {
  it('is the last item, a 44 px icon-only button named Unmute, and the key bar has none', () => {
    const video = document.createElement('video')
    video.muted = true
    renderBars(props({ sound: sound(video), zoom: 2 }))
    expect(labels()).toEqual(['Main Display', 'Send Keys', 'Show Keyboard', 'Fit to screen, zoom 2x', 'Unmute'])
    const item = btn('Unmute')
    expect(item.classList.contains('desktop-sound--phone')).toBe(true)
    expect(item.textContent).toBe('')
    renderBars(props({ sound: sound(video), keyboardUp: true }))
    expect(container.querySelector('.desktop-sound')).toBeNull()
  })

  it('fires on click, not on pointerdown, and turns the item into Mute', async () => {
    const video = document.createElement('video')
    video.muted = true
    const s = sound(video)
    renderBars(props({ sound: s }))
    pointerdown(btn('Unmute'))
    expect(video.muted).toBe(true)
    expect(play).not.toHaveBeenCalled()
    await click(btn('Unmute'))
    expect(video.muted).toBe(false)
    expect(play).toHaveBeenCalledTimes(1)
    expect(btn('Mute')).toBeTruthy()
    expect(s.onChange).toHaveBeenLastCalledWith(true)
  })

  it('a phone with no other bar items shows a bottom bar with Sound alone', () => {
    const video = document.createElement('video')
    video.muted = true
    renderBars(props({ canControl: false, canPickSurface: false, sound: sound(video) }))
    expect(labels()).toEqual(['Unmute'])
  })

  it('without a sound prop (the hello offered no audio) there is no Sound item', () => {
    renderBars(props())
    expect(labels()).toEqual(['Main Display', 'Send Keys', 'Show Keyboard'])
  })

  it('stays live while Locked and Reconnecting', async () => {
    const video = document.createElement('video')
    video.muted = true
    for (const reason of ['Locked', 'Reconnecting'] as const) {
      video.muted = true
      renderBars(props({ disabledReason: reason, sound: sound(video) }))
      expect(btn('Unmute').hasAttribute('aria-disabled')).toBe(false)
      await click(btn('Unmute'))
      expect(video.muted).toBe(false)
    }
  })
})

describe('Show Keyboard while the Mac is Locked', () => {
  it('is aria-disabled and does nothing when Locked; Fit to Screen stays live', async () => {
    const p = props({ disabledReason: 'Locked', zoom: 2 })
    renderBars(p)
    expect(btn('Show Keyboard').getAttribute('aria-disabled')).toBe('true')
    await click(btn('Show Keyboard'))
    expect(p.onShowKeyboard).not.toHaveBeenCalled()
    pointerdown(btn('Fit to screen, zoom 2x'))
    expect(p.onFit).toHaveBeenCalledTimes(1)
  })

  it('stays live while Reconnecting', async () => {
    const p = props({ disabledReason: 'Reconnecting' })
    renderBars(p)
    expect(btn('Show Keyboard').hasAttribute('aria-disabled')).toBe(false)
    await click(btn('Show Keyboard'))
    expect(p.onShowKeyboard).toHaveBeenCalledTimes(1)
  })
})

describe('the slot while the Unlock field has focus', () => {
  it('hidden empties the slot: neither the bottom bar nor the key bar', () => {
    renderBars(props({ hidden: true }))
    expect(toolbar()).toBeNull()
    renderBars(props({ hidden: true, keyboardUp: true }))
    expect(toolbar()).toBeNull()
    renderBars(props({ hidden: false }))
    expect(toolbar()?.getAttribute('aria-label')).toBe('Desktop controls')
  })

  it('the visual-viewport field selector matches the typing field and the Unlock field', () => {
    const unlock = document.createElement('input')
    unlock.className = 'desktop-unlock__input'
    const typing = document.createElement('input')
    typing.className = 'desktop-phone__input'
    const other = document.createElement('input')
    expect(unlock.matches(PHONE_FIELDS)).toBe(true)
    expect(typing.matches(PHONE_FIELDS)).toBe(true)
    expect(other.matches(PHONE_FIELDS)).toBe(false)
  })
})

describe('DesktopView on a phone', () => {
  const client = () => h.clients[h.clients.length - 1] as MockClient
  const media = () =>
    vi.spyOn(window, 'matchMedia').mockImplementation(
      (q: string) => ({ matches: true, media: q, addEventListener() {}, removeEventListener() {} }) as unknown as MediaQueryList,
    )
  async function lockedPhone(caps = { audio: true, unlock: true }) {
    media()
    act(() => root.render(<DesktopView />))
    act(() => client().emit('rights', { control: true, clipboard: false, surface: false }))
    act(() => client().emit('caps', caps))
    act(() => client().emit('status', { kind: 'streaming' }))
    act(() => client().emit('track', {} as MediaStream))
    await act(async () => {
      container.querySelector('video')!.dispatchEvent(new Event('playing'))
    })
  }
  const lock = (locked: boolean) =>
    act(() => client().emit('state', { state: locked ? 'locked' : 'streaming', viewers: 1, locked }))
  const unlockInput = () => container.querySelector('.desktop-unlock__input') as HTMLInputElement

  it('shows Sound in the bottom bar when the hello offered audio, and Unmute plays the picture\'s sound', async () => {
    await lockedPhone()
    const item = container.querySelector('.desktop-phone__slot .desktop-sound') as HTMLButtonElement
    expect(item.getAttribute('aria-label')).toBe('Unmute')
    await click(item)
    expect(container.querySelector('video')!.muted).toBe(false)
  })

  it('no Sound item on a phone whose hello offered no audio', async () => {
    await lockedPhone({ audio: false, unlock: true })
    expect(container.querySelector('.desktop-sound')).toBeNull()
  })

  it('Show Keyboard is greyed while Locked and live again after the unlock', async () => {
    await lockedPhone()
    lock(false)
    expect(btn('Show Keyboard').hasAttribute('aria-disabled')).toBe(false)
    lock(true)
    expect(btn('Show Keyboard').getAttribute('aria-disabled')).toBe('true')
    lock(false)
    expect(btn('Show Keyboard').hasAttribute('aria-disabled')).toBe(false)
  })

  it('focusing the Unlock input empties the slot, and blurring it brings the bottom bar back', async () => {
    await lockedPhone()
    lock(true)
    expect(toolbar()?.getAttribute('aria-label')).toBe('Desktop controls')
    act(() => unlockInput().focus())
    expect(toolbar()).toBeNull()
    expect(container.querySelector('.desktop-phone__slot')).not.toBeNull()
    act(() => unlockInput().blur())
    expect(toolbar()?.getAttribute('aria-label')).toBe('Desktop controls')
  })

  it('the slot also comes back when the Mac unlocks while the field still has focus', async () => {
    await lockedPhone()
    lock(true)
    act(() => unlockInput().focus())
    expect(toolbar()).toBeNull()
    lock(false)
    expect(toolbar()?.getAttribute('aria-label')).toBe('Desktop controls')
  })
})
