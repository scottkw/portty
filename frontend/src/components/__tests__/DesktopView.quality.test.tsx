// Phase 23-08 — the Slow connection chip (DESK-07, 23-UI-SPEC Surface 4): the daemon marks this viewer
// below its floor, the chip says so in words with an icon, and nothing else announces it. The client is
// mocked with emitters; the quality events are built from the golden lines in signal-v1.jsonl.

import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import type { DesktopEvents } from '../../lib/desktopClient'
import * as copy from '../../lib/shareDesktopCopy'
import { DesktopView, type DesktopViewProps } from '../DesktopView'

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

const SLOW = 'Slow connection. The picture may freeze.'
const golden = (name: string) => {
  const lines = readFileSync(resolve(__dirname, '../../../../internal/desktop/testdata/signal-v1.jsonl'), 'utf8')
    .trim().split('\n').map((l) => JSON.parse(l) as { name: string; wire: { type: string; slow: boolean } })
  return lines.find((l) => l.name === name)!.wire
}

let container: HTMLDivElement
let root: Root

const client = () => h.clients[h.clients.length - 1] as MockClient
const chips = () => Array.from(container.querySelectorAll('.desktop-chip:not(.desktop-chip--notice)')).map((c) => c.textContent)
const notice = () => container.querySelector('.desktop-chip--notice')
const quality = (name: string) => act(() => client().emit('quality', { slow: golden(name).slow }))

function render(props: DesktopViewProps = {}) {
  act(() => root.render(<DesktopView {...props} />))
}
async function streaming(rights = { control: true, clipboard: false, surface: false }) {
  act(() => client().emit('rights', rights))
  act(() => client().emit('status', { kind: 'streaming' }))
  act(() => client().emit('track', {} as MediaStream))
  await act(async () => {
    container.querySelector('video')!.dispatchEvent(new Event('playing'))
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

describe('Slow connection chip (tracer)', () => {
  it('the golden quality-slow-true line shows the chip with a signal icon, and quality-slow-false clears it', async () => {
    render()
    await streaming()
    expect(chips()).not.toContain(SLOW)
    quality('quality-slow-true')
    expect(chips()).toEqual([SLOW])
    expect(container.querySelector('.desktop-chip .desktop-chip__icon')?.getAttribute('aria-hidden')).toBe('true')
    quality('quality-slow-false')
    expect(chips()).toEqual([])
  })

  it('sits in the polite live region, so a change is announced', async () => {
    render()
    await streaming()
    quality('quality-slow-true')
    const chip = Array.from(container.querySelectorAll('.desktop-chip')).find((c) => c.textContent === SLOW)!
    expect(chip.closest('[role="status"]')?.getAttribute('aria-live')).toBe('polite')
  })

  it('shows no notice of any kind when the flag changes either way', async () => {
    render()
    await streaming()
    quality('quality-slow-true')
    expect(notice()).toBeNull()
    quality('quality-slow-false')
    expect(notice()).toBeNull()
    act(() => { vi.advanceTimersByTime(10000) })
    expect(notice()).toBeNull()
  })

  it('has no setting and no sharpening indicator', async () => {
    render()
    await streaming()
    quality('quality-slow-true')
    expect(container.textContent).not.toMatch(/sharp|quality|bitrate|resolution/i)
  })
})

describe('chip precedence', () => {
  it('Locked outranks Slow connection', async () => {
    render()
    await streaming()
    quality('quality-slow-true')
    act(() => client().emit('state', { state: 'locked', viewers: 1, locked: true }))
    expect(chips()).toEqual(['Locked'])
    act(() => client().emit('state', { state: 'streaming', viewers: 1, locked: false }))
    expect(chips()).toEqual([SLOW])
  })

  it('Reconnecting outranks Slow connection, and a lost connection clears the flag for the next one', async () => {
    render()
    await streaming()
    quality('quality-slow-true')
    act(() => client().emit('status', { kind: 'reconnecting' } as never))
    expect(chips()).toEqual(['Reconnecting…'])
    act(() => client().emit('status', { kind: 'streaming' }))
    expect(chips()).not.toContain(SLOW)
  })

  it('Slow connection outranks the guest View only chip and Streaming N viewers', async () => {
    render({ guest: true })
    await streaming({ control: false, clipboard: false, surface: false })
    act(() => client().emit('state', { state: 'streaming', viewers: 2, locked: false }))
    expect(chips()).toEqual([copy.GUEST_CHIP_VIEW_ONLY])
    quality('quality-slow-true')
    expect(chips()).toEqual([SLOW])
    quality('quality-slow-false')
    expect(chips()).toEqual([copy.GUEST_CHIP_VIEW_ONLY])
  })

  it('Slow connection outranks Streaming N viewers on the own-account page', async () => {
    render()
    await streaming()
    act(() => client().emit('state', { state: 'streaming', viewers: 1, locked: false }))
    expect(chips()).toEqual(['Streaming to 1 viewer'])
    quality('quality-slow-true')
    expect(chips()).toEqual([SLOW])
  })
})
