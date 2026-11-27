// Phase 22-07 — the guest page: DesktopView in guest mode (22-UI-SPEC Surface 6). The client is
// mocked with emitters; the view is rendered into jsdom.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import type { ClientStatus, DesktopEvents } from '../../lib/desktopClient'
import { createDesktopClient } from '../../lib/desktopClient'
import * as copy from '../../lib/shareDesktopCopy'
import { DesktopView, type DesktopViewProps } from '../DesktopView'

type Listener = (v: never) => void
interface MockClient {
  start: ReturnType<typeof vi.fn>
  stop: ReturnType<typeof vi.fn>
  sendInput: ReturnType<typeof vi.fn>
  on: (ev: string, cb: Listener) => () => void
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

const client = () => h.clients[h.clients.length - 1] as MockClient
const video = () => container.querySelector('video') as HTMLVideoElement
const chips = () => Array.from(container.querySelectorAll('.desktop-chip:not(.desktop-chip--notice)')).map((c) => c.textContent)
const card = () => container.querySelector('.desktop-card')
const heading = () => card()?.querySelector('h1')?.textContent
const body = () => card()?.querySelector('p')?.textContent
const buttons = () => Array.from(container.querySelectorAll('.desktop-card button')).map((b) => b.textContent)

function render(props: DesktopViewProps = {}) {
  act(() => root.render(<DesktopView guest {...props} />))
}
const status = (s: ClientStatus) => act(() => client().emit('status', s))
const admission = (state: 'waiting' | 'admitted') => act(() => client().emit('admission', { state }))
async function firstFrame() {
  status({ kind: 'streaming' })
  act(() => client().emit('track', {} as MediaStream))
  await act(async () => {
    video().dispatchEvent(new Event('playing'))
  })
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

describe('guest page at Admit (tracer)', () => {
  it('waits at Admit, then Connecting…, then the picture with the View only chip and no Streaming chip', async () => {
    render()
    // The host label is the first DNS label of the page's own host (jsdom serves localhost).
    admission('waiting')
    expect(heading()).toBe(copy.guestWaitingHeading('localhost'))
    expect(body()).toBe(copy.GUEST_CARD_WAITING_BODY)
    expect(buttons()).toEqual([])
    admission('admitted')
    expect(heading()).toBe('Connecting…')
    expect(body()).toBe(copy.guestConnectingBody('localhost'))
    await firstFrame()
    expect(card()).toBeNull()
    expect(chips()).toEqual([copy.GUEST_CHIP_VIEW_ONLY])
    act(() => client().emit('state', { state: 'streaming', viewers: 3, locked: false }))
    act(() => { vi.advanceTimersByTime(200) })
    expect(chips()).toEqual([copy.GUEST_CHIP_VIEW_ONLY])
    expect(container.textContent).not.toContain('Streaming to')
  })

  it('passes guest on to the client when the prop is given, and leaves the options alone when it is not', () => {
    render({ guest: true, clientOptions: { check: async () => 'ok' } })
    expect(vi.mocked(createDesktopClient)).toHaveBeenLastCalledWith(expect.objectContaining({ guest: true }))
    act(() => root.unmount())
    root = createRoot(container)
    const options = { check: async () => 'ok' as const }
    act(() => root.render(<DesktopView clientOptions={options} />))
    expect(vi.mocked(createDesktopClient)).toHaveBeenLastCalledWith(options)
  })
})

describe('guest cards, one at a time', () => {
  it('Not admitted (denied): the denied body, no button, final', () => {
    render()
    admission('waiting')
    status({ kind: 'guest_final', reason: 'denied' })
    expect(heading()).toBe(copy.GUEST_CARD_NOT_ADMITTED)
    expect(body()).toBe(copy.GUEST_CARD_DENIED_BODY)
    expect(buttons()).toEqual([])
    expect(container.querySelectorAll('.desktop-card')).toHaveLength(1)
  })

  it('Not admitted (no answer in 5 minutes): the timeout body and Reload Page, which reloads', () => {
    const onReload = vi.fn()
    render({ onReload })
    admission('waiting')
    status({ kind: 'guest_final', reason: 'admit_timeout' })
    expect(heading()).toBe(copy.GUEST_CARD_NOT_ADMITTED)
    expect(body()).toBe(copy.GUEST_CARD_TIMEOUT_BODY)
    expect(buttons()).toEqual(['Reload Page'])
    act(() => (container.querySelector('.desktop-card button') as HTMLButtonElement).click())
    expect(onReload).toHaveBeenCalledTimes(1)
  })

  it('You were disconnected: final, no button', async () => {
    render()
    await firstFrame()
    status({ kind: 'guest_final', reason: 'revoked' })
    expect(heading()).toBe(copy.GUEST_CARD_REVOKED)
    expect(body()).toBe(copy.GUEST_CARD_REVOKED_BODY)
    expect(buttons()).toEqual([])
  })

  it('Sharing has ended: final, the scrim stays over the frozen last frame', async () => {
    render()
    await firstFrame()
    expect(container.querySelector('.desktop-scrim')).toBeNull()
    status({ kind: 'guest_final', reason: 'share_ended' })
    expect(heading()).toBe(copy.GUEST_CARD_ENDED)
    expect(body()).toBe(copy.GUEST_CARD_ENDED_BODY)
    expect(buttons()).toEqual([])
    expect(container.querySelector('.desktop-scrim')).not.toBeNull()
    expect(video()).not.toBeNull()
    expect(chips()).toEqual([])
  })

  it('This link no longer works: final, no button, and not the own-account card words', () => {
    render()
    status({ kind: 'guest_final', reason: 'token_invalid' })
    expect(heading()).toBe(copy.GUEST_CARD_LINK)
    expect(body()).toBe(copy.GUEST_CARD_LINK_BODY)
    expect(buttons()).toEqual([])
    expect(container.textContent).not.toContain('Settings')
  })

  it("Can't reach this Mac: the access-rules hint and Reload Page", () => {
    render()
    status({ kind: 'unreachable' })
    expect(heading()).toBe(copy.GUEST_CARD_UNREACHABLE)
    expect(body()).toContain("Your tailnet's access rules may also be blocking the connection.")
    expect(buttons()).toEqual(['Reload Page'])
  })

  it('a card wraps in the live region as plain text, and the host label is set as text', () => {
    render()
    admission('waiting')
    expect(container.querySelector('.desktop-card [role="status"]')?.querySelector('h1')?.textContent).toBe(copy.guestWaitingHeading('localhost'))
    expect(container.querySelector('.desktop-card h1')?.children).toHaveLength(0)
  })

  it('a dropped connection replaces the Waiting card with Reconnecting…, and the wait resumes when the share server says so', () => {
    render()
    admission('waiting')
    status({ kind: 'reconnecting' })
    expect(card()).toBeNull()
    expect(chips()).toEqual(['Reconnecting…'])
    status({ kind: 'connecting' })
    admission('waiting')
    expect(heading()).toBe(copy.guestWaitingHeading('localhost'))
  })
})

describe('guest chips, grants and the lease', () => {
  const stage = () => container.querySelector('.desktop-stage') as HTMLElement
  const ring = () => container.querySelector('.desktop-ring')
  const notice = () => container.querySelector('.desktop-chip--notice')?.textContent
  const rights = (r: Partial<DesktopEvents['rights']>) =>
    act(() => client().emit('rights', { control: false, clipboard: false, surface: false, ...r }))
  const lease = (state: DesktopEvents['lease']['state'], name = '') => act(() => client().emit('lease', { state, name }))

  async function admitted(r: Partial<DesktopEvents['rights']> = {}) {
    render()
    for (const [a, b] of [['clientWidth', 1000], ['clientHeight', 500], ['videoWidth', 1600], ['videoHeight', 1000]] as const) {
      Object.defineProperty(video(), a, { configurable: true, value: b })
    }
    admission('admitted')
    await firstFrame()
    rights(r)
  }

  it('View only: forwards no pointer, key or clipboard input, shows no bar, and the stage is out of the tab order', async () => {
    await admitted()
    expect(chips()).toEqual([copy.GUEST_CHIP_VIEW_ONLY])
    expect(stage().getAttribute('tabindex')).toBe('-1')
    act(() => {
      stage().dispatchEvent(new PointerEvent('pointerdown', { clientX: 500, clientY: 250, bubbles: true, button: 0, pointerId: 1 }))
      stage().dispatchEvent(new PointerEvent('pointermove', { clientX: 510, clientY: 250, bubbles: true, pointerId: 1 }))
    })
    expect(client().sendInput).not.toHaveBeenCalled()
    expect(container.querySelector('.desktop-bar')).toBeNull()
    expect(container.querySelector('.desktop-dot')).toBeNull()
    expect(ring()).toBeNull()
  })

  it('control on: the You have control chip; control off: held input is released, the ring goes, View only returns', async () => {
    await admitted({ control: true })
    expect(chips()).toEqual([copy.GUEST_CHIP_CONTROL])
    expect(stage().getAttribute('tabindex')).toBe('0')
    act(() => {
      stage().dispatchEvent(new PointerEvent('pointerdown', { clientX: 500, clientY: 250, bubbles: true, button: 0, pointerId: 1 }))
    })
    expect(ring()).not.toBeNull()
    client().sendInput.mockClear()
    rights({ control: false })
    expect(client().sendInput.mock.calls.map((c) => (c[0] as { t: string }).t)).toContain('release')
    expect(ring()).toBeNull()
    expect(container.querySelector('.desktop-dot')).toBeNull()
    expect(chips()).toEqual([copy.GUEST_CHIP_VIEW_ONLY])
    expect(stage().getAttribute('tabindex')).toBe('-1')
    rights({ control: true })
    expect(chips()).toEqual([copy.GUEST_CHIP_CONTROL])
    expect(stage().getAttribute('tabindex')).toBe('0')
  })

  it('a clipboard grant change shows its notice for 3 s and no chip of its own', async () => {
    await admitted()
    rights({ clipboard: true })
    expect(notice()).toBe(copy.GUEST_NOTICE_CLIPBOARD_ON)
    expect(chips()).toEqual([copy.GUEST_CHIP_VIEW_ONLY])
    act(() => { vi.advanceTimersByTime(3200) })
    expect(notice()).toBeUndefined()
    rights({ clipboard: true, control: true })
    expect(notice()).toBeUndefined()
    rights({ clipboard: false, control: true })
    expect(notice()).toBe(copy.GUEST_NOTICE_CLIPBOARD_OFF)
    act(() => { vi.advanceTimersByTime(3200) })
    expect(notice()).toBeUndefined()
  })

  it('another guest holding the lease shows "<name> is controlling" as text; free and you show You have control', async () => {
    await admitted({ control: true })
    lease('other', '<b>Ada</b>')
    expect(chips()).toEqual([copy.controllingChip('<b>Ada</b>')])
    expect(container.querySelector('.desktop-chip b')).toBeNull()
    lease('you')
    expect(chips()).toEqual([copy.GUEST_CHIP_CONTROL])
    lease('other', 'Ada')
    lease('free')
    expect(chips()).toEqual([copy.GUEST_CHIP_CONTROL])
  })

  it('a view-only guest never shows a lease chip', async () => {
    await admitted()
    lease('other', 'Ada')
    expect(chips()).toEqual([copy.GUEST_CHIP_VIEW_ONLY])
    lease('physical')
    expect(chips()).toEqual([copy.GUEST_CHIP_VIEW_ONLY])
  })

  it('Someone is using this Mac stays at least 2 s even when the lease frees at once', async () => {
    await admitted({ control: true })
    lease('physical')
    expect(chips()).toEqual([copy.GUEST_CHIP_SOMEONE])
    act(() => { vi.advanceTimersByTime(500) })
    lease('free')
    expect(chips()).toEqual([copy.GUEST_CHIP_SOMEONE])
    act(() => { vi.advanceTimersByTime(1400) })
    expect(chips()).toEqual([copy.GUEST_CHIP_SOMEONE])
    act(() => { vi.advanceTimersByTime(200) })
    expect(chips()).toEqual([copy.GUEST_CHIP_CONTROL])
  })

  it('an owner lease shows Someone is using this Mac too, so the sharer is never named, and it clears at once after 2 s', async () => {
    await admitted({ control: true })
    lease('owner')
    expect(chips()).toEqual([copy.GUEST_CHIP_SOMEONE])
    act(() => { vi.advanceTimersByTime(2500) })
    expect(chips()).toEqual([copy.GUEST_CHIP_SOMEONE])
    lease('free')
    expect(chips()).toEqual([copy.GUEST_CHIP_CONTROL])
  })

  it('precedence: Reconnecting over Locked over Someone over name over You have control over View only', async () => {
    await admitted({ control: true })
    lease('other', 'Ada')
    expect(chips()).toEqual([copy.controllingChip('Ada')])
    lease('physical')
    expect(chips()).toEqual([copy.GUEST_CHIP_SOMEONE])
    act(() => client().emit('state', { state: 'locked', viewers: 1, locked: true }))
    expect(chips()).toEqual([copy.GUEST_CHIP_LOCKED])
    status({ kind: 'reconnecting' })
    expect(chips()).toEqual(['Reconnecting…'])
  })

  it('shows the Phase 19 Streaming to N viewers chip never, for any count', async () => {
    await admitted({ control: true })
    for (const viewers of [1, 2, 5]) {
      act(() => client().emit('state', { state: 'streaming', viewers, locked: false }))
      act(() => { vi.advanceTimersByTime(100) })
      expect(container.textContent).not.toContain('Streaming to')
    }
  })

  it('renders no identity of its own or the sharer: only the lease holder display name appears', async () => {
    await admitted({ control: true, clipboard: true })
    lease('other', 'Ada')
    const text = container.textContent ?? ''
    expect(text).toContain('Ada is controlling')
    expect(text.replace('Ada is controlling', '')).not.toMatch(/Ada|@|login/i)
  })
})

describe('guest bar inside the page', () => {
  const rights = (r: Partial<DesktopEvents['rights']>) =>
    act(() => client().emit('rights', { control: false, clipboard: false, surface: false, ...r }))
  const bar = () => Array.from(container.querySelectorAll('.desktop-bar button')).map((b) => b.getAttribute('aria-label'))

  async function admitted() {
    render()
    admission('admitted')
    await firstFrame()
  }

  it('clipboard only: the bar holds the Clipboard menu alone and the chip still says View only', async () => {
    await admitted()
    expect(container.querySelector('.desktop-bar')).toBeNull()
    rights({ clipboard: true })
    expect(bar()).toEqual(['Clipboard'])
    expect(chips()).toEqual([copy.GUEST_CHIP_VIEW_ONLY])
    rights({ clipboard: false })
    expect(container.querySelector('.desktop-bar')).toBeNull()
  })

  it('control and clipboard: no picker even when the wire says surface, and no Lock Screen… in Send Keys', async () => {
    await admitted()
    rights({ control: true, clipboard: true, surface: true })
    expect(bar()).toEqual(expect.arrayContaining(['Send Keys', 'Clipboard', 'Swap Ctrl and Cmd']))
    expect(container.querySelector('.desktop-bar__picker')).toBeNull()
    await act(async () => {
      ;(container.querySelector('button[aria-label="Send Keys"]') as HTMLButtonElement).click()
    })
    const items = Array.from(document.querySelectorAll('[role="menuitem"]')).map((e) => e.textContent)
    expect(items.join('|')).toContain('Cmd+Tab')
    expect(items.join('|')).not.toContain('Lock Screen')
  })
})
