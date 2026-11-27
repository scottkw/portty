// Phase 23-07 — the rest of the Locked experience (23-UI-SPEC Surface 3): the guest's Locked chip, the blocked-input
// notice (once per lock episode, then at most every 10 s) and the bar while locked. The client is mocked with emitters.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import type { DesktopEvents } from '../../lib/desktopClient'
import { DesktopView, type DesktopViewProps } from '../DesktopView'

type Listener = (v: never) => void
interface MockClient {
  sendInput: ReturnType<typeof vi.fn>
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

const OWN = 'Clicks and typing are off while this Mac is locked. Use Unlock This Mac to enter the password.'
const GUEST = 'Clicks and typing are off while this Mac is locked.'

let container: HTMLDivElement
let root: Root

const client = () => h.clients[h.clients.length - 1] as MockClient
const notice = () => container.querySelector('.desktop-chip--notice')?.textContent
const chips = () => Array.from(container.querySelectorAll('.desktop-chip:not(.desktop-chip--notice)')).map((c) => c.textContent)
const advance = (ms: number) => act(() => { vi.advanceTimersByTime(ms) })
const stageInput = () => container.querySelector('.desktop-stage__input') as HTMLTextAreaElement
const press = (key = 'a') =>
  act(() => { stageInput().dispatchEvent(new KeyboardEvent('keydown', { key, code: 'KeyA', bubbles: true, cancelable: true })) })
const lock = (locked: boolean) =>
  act(() => client().emit('state', { state: locked ? 'locked' : 'streaming', viewers: 1, locked }))

function render(props: DesktopViewProps = {}) {
  act(() => root.render(<DesktopView {...props} />))
}
async function lockedMac(rights = { control: true, clipboard: true, surface: true }, caps = { audio: false, unlock: true }) {
  act(() => client().emit('rights', rights))
  act(() => client().emit('caps', caps))
  act(() => client().emit('status', { kind: 'streaming' }))
  act(() => client().emit('track', {} as MediaStream))
  await act(async () => {
    container.querySelector('video')!.dispatchEvent(new Event('playing'))
  })
  lock(true)
}

beforeEach(() => {
  ;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
  vi.useFakeTimers()
  h.clients.length = 0
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
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

describe('a guest on a locked Mac', () => {
  it('the chip says Locked and who can unlock; no Unlock panel and no password field are in the DOM', async () => {
    render({ guest: true })
    await lockedMac({ control: false, clipboard: false, surface: false })
    expect(chips()).toEqual(['Locked. Ask the person at the Mac to unlock it.'])
    expect(container.querySelector('input')).toBeNull()
    expect(container.textContent).not.toContain('Unlock This Mac')
  })

  it('an own-account viewer keeps the plain Locked chip', async () => {
    render()
    await lockedMac()
    expect(chips()).toEqual(['Locked'])
  })

  it('a guest with Control on gets the short blocked-input notice', async () => {
    render({ guest: true })
    await lockedMac({ control: true, clipboard: false, surface: false })
    press()
    expect(notice()).toBe(GUEST)
  })
})

describe('input is blocked while locked', () => {
  it('a printable key sends nothing and shows the own-account notice with a lock icon', async () => {
    render()
    await lockedMac()
    const sent = client().sendInput.mock.calls.length
    press('a')
    expect(client().sendInput.mock.calls.length).toBe(sent)
    expect(notice()).toBe(OWN)
    expect(container.querySelector('.desktop-chip--notice svg')).not.toBeNull()
  })

  it('pressing the picture shows it too', async () => {
    render()
    await lockedMac()
    act(() => { container.querySelector('.desktop-stage')!.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true })) })
    expect(notice()).toBe(OWN)
  })

  it('a second press within 10 s shows nothing; after 10 s it may show again', async () => {
    render()
    await lockedMac()
    press()
    expect(notice()).toBe(OWN)
    advance(6000)
    expect(notice()).toBeUndefined()
    press()
    expect(notice()).toBeUndefined()
    advance(4100)
    press()
    expect(notice()).toBe(OWN)
  })

  it('a new lock episode shows it again at once', async () => {
    render()
    await lockedMac()
    press()
    advance(6000)
    lock(false)
    lock(true)
    press()
    expect(notice()).toBe(OWN)
  })

  it('a non-printable key or a Cmd chord does not count', async () => {
    render()
    await lockedMac()
    press('Shift')
    expect(notice()).toBeUndefined()
    act(() => { stageInput().dispatchEvent(new KeyboardEvent('keydown', { key: 'a', metaKey: true, bubbles: true, cancelable: true })) })
    expect(notice()).toBeUndefined()
  })

  it('a View Only viewer never sees it', async () => {
    render({ controlMode: 'view' })
    await lockedMac()
    press()
    act(() => { container.querySelector('.desktop-stage')!.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true })) })
    expect(notice()).toBeUndefined()
  })

  it('when the Mac is not locked a keypress shows no such notice', async () => {
    render()
    await lockedMac()
    lock(false)
    press()
    expect(notice()).toBeUndefined()
  })
})

describe('the bar while locked', () => {
  it('Sound stays enabled and every item that sends to the Mac stays greyed', async () => {
    render()
    await lockedMac(undefined, { audio: true, unlock: true })
    const bar = Array.from(container.querySelectorAll('[role="toolbar"] button'))
    const sound = container.querySelector('.desktop-sound') as HTMLButtonElement
    expect(sound).not.toBeNull()
    expect(sound.getAttribute('aria-disabled')).not.toBe('true')
    const greyed = bar.filter((b) => b.getAttribute('aria-disabled') === 'true').map((b) => b.getAttribute('aria-label') ?? b.textContent)
    expect(greyed.length).toBeGreaterThanOrEqual(2)
    expect(greyed.join('|')).toMatch(/Send Keys/)
    expect(greyed.join('|')).toMatch(/Clipboard/)
  })
})
