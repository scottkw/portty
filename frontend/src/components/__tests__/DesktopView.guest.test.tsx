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
