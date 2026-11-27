// Phase 21-10 (DVWR-03) — the iPhone touch layer, driven through DesktopView's phone branch.
// The client is mocked with emitters; touches are plain events with a touches array (jsdom's TouchEvent is incomplete).

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import type { DesktopEvents } from '../../lib/desktopClient'
import { normalizedDelta } from '../../lib/desktopCoords'
import { accelGain } from '../../lib/touchTrackpad'
import { DesktopView, videoContentRect } from '../DesktopView'

type Listener = (v: never) => void
interface MockClient {
  sendInput: ReturnType<typeof vi.fn>
  currentEpoch: ReturnType<typeof vi.fn>
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
const stage = () => container.querySelector('.desktop-stage') as HTMLElement
const video = () => container.querySelector('video') as HTMLVideoElement

const media = (matches: boolean) =>
  vi.spyOn(window, 'matchMedia').mockImplementation(
    (q: string) => ({ matches, media: q, addEventListener() {}, removeEventListener() {} }) as unknown as MediaQueryList,
  )

function sizeVideo() {
  for (const [a, b] of [['clientWidth', 1000], ['clientHeight', 500], ['videoWidth', 1600], ['videoHeight', 1000]] as const) {
    Object.defineProperty(video(), a, { configurable: true, value: b })
  }
}

function mount(rights: DesktopEvents['rights'] = { control: true, clipboard: true, surface: true }) {
  act(() => root.render(<DesktopView />))
  sizeVideo()
  act(() => client().emit('rights', rights))
}

let touchId = 0
const pt = (x: number, y: number) => ({ identifier: ++touchId, clientX: x, clientY: y })
function touch(type: 'touchstart' | 'touchmove' | 'touchend', touches: ReturnType<typeof pt>[], changed = touches) {
  const ev = new Event(type, { bubbles: true, cancelable: true })
  Object.assign(ev, { touches, changedTouches: changed })
  act(() => {
    stage().dispatchEvent(ev)
  })
  return ev
}
const sent = () => client().sendInput.mock.calls.map((c) => c[0] as { t: string; dx?: number; dy?: number; e?: number })
const advance = (ms: number) => act(() => { vi.advanceTimersByTime(ms) })

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

describe('tracer: a one-finger drag on a phone sends a relative move', () => {
  it('a 40 px drag records rmove messages whose dx sums to normalizedDelta for the picture rect', () => {
    media(true)
    mount()
    const t = pt(0, 0)
    touch('touchstart', [t])
    for (let i = 1; i <= 10; i++) {
      advance(10)
      touch('touchmove', [{ ...t, clientX: i * 4, clientY: 0 }])
    }
    touch('touchend', [], [{ ...t, clientX: 40, clientY: 0 }])
    const moves = sent().filter((m) => m.t === 'rmove')
    expect(moves.length).toBeGreaterThan(0)
    expect(moves.every((m) => m.e === 1)).toBe(true)
    expect(client().sendInput.mock.calls.every((c) => c[1] === true)).toBe(true) // droppable
    const r = videoContentRect(1000, 500, 1600, 1000)
    const want = normalizedDelta(40 * accelGain(400), 0, 1, r.width, r.height).ndx
    expect(moves.reduce((a, m) => a + m.dx!, 0)).toBeCloseTo(want, 9)
    expect(moves.reduce((a, m) => a + m.dy!, 0)).toBe(0)
  })

  it('sends nothing while the epoch is unknown', () => {
    media(true)
    mount()
    client().currentEpoch.mockReturnValue(null)
    const t = pt(0, 0)
    touch('touchstart', [t])
    advance(10)
    touch('touchmove', [{ ...t, clientX: 30, clientY: 0 }])
    expect(sent()).toEqual([])
  })

  it('a mouse device gets no layer and sends no message, and the stage keeps no touch handlers', () => {
    media(false)
    mount()
    expect(container.querySelector('.desktop-view--phone')).toBeNull()
    const ev = touch('touchstart', [pt(0, 0)])
    expect(ev.defaultPrevented).toBe(false)
    touch('touchmove', [pt(30, 0)])
    expect(sent()).toEqual([])
  })

  it('a viewer without the control right sends nothing', () => {
    media(true)
    mount({ control: false, clipboard: false, surface: false })
    const t = pt(0, 0)
    touch('touchstart', [t])
    advance(10)
    touch('touchmove', [{ ...t, clientX: 30, clientY: 0 }])
    expect(sent()).toEqual([])
  })
})
