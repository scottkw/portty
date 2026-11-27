// Phase 23-07 — the Unlock This Mac panel (UNLK-01, 23-UI-SPEC Surface 2). The tracer: type, press Unlock, and the
// bytes leave on the data channel with the field already empty. The sentinel below is a made-up string, never a real password.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import type { DesktopClient, DesktopEvents } from '../../lib/desktopClient'
import { DesktopUnlockPanel } from '../DesktopUnlockPanel'
import { DesktopView, type DesktopViewProps } from '../DesktopView'

type Listener = (v: never) => void
interface MockClient {
  sendUnlock: ReturnType<typeof vi.fn>
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

const SENTINEL = 'sentinel-pw-ünï-0451'

let container: HTMLDivElement
let root: Root

const view = () => h.clients[h.clients.length - 1] as MockClient
const field = () => container.querySelector('input[type="password"]') as HTMLInputElement | null
const unlockButton = () => Array.from(container.querySelectorAll('button')).find((b) => b.textContent === 'Unlock') as HTMLButtonElement
const type = (text: string) =>
  act(() => {
    field()!.value = text
    field()!.dispatchEvent(new Event('input', { bubbles: true }))
  })
const click = (el: Element) => act(() => el.dispatchEvent(new MouseEvent('click', { bubbles: true })))

function fakeClient(sendUnlock: (b: Uint8Array) => boolean): DesktopClient {
  return { sendUnlock } as unknown as DesktopClient
}

function renderPanel(client: DesktopClient) {
  act(() =>
    root.render(<DesktopUnlockPanel client={client} unlock={null} reconnecting={false} needsAccessibility={false} phone={false} />),
  )
}

function renderView(props: DesktopViewProps = {}) {
  act(() => root.render(<DesktopView {...props} />))
}
async function lockedMac(caps = { audio: false, unlock: true }) {
  act(() => view().emit('caps', caps))
  act(() => view().emit('status', { kind: 'streaming' }))
  act(() => view().emit('track', {} as MediaStream))
  await act(async () => {
    container.querySelector('video')!.dispatchEvent(new Event('playing'))
  })
  act(() => view().emit('state', { state: 'locked', viewers: 1, locked: true }))
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

describe('Unlock This Mac (tracer)', () => {
  it('Unlock sends one binary message, type byte 1 then the UTF-8 text, with the field already empty, then zeroes the array', () => {
    let valueAtSend: string | undefined
    let sent: Uint8Array | undefined
    let copyAtSend: number[] = []
    const sendUnlock = vi.fn((b: Uint8Array) => {
      valueAtSend = field()!.value
      sent = b
      copyAtSend = Array.from(b)
      return true
    })
    renderPanel(fakeClient(sendUnlock))
    type(SENTINEL)
    click(unlockButton())
    expect(sendUnlock).toHaveBeenCalledTimes(1)
    expect(valueAtSend).toBe('')
    expect(copyAtSend[0]).toBe(1)
    expect(new TextDecoder().decode(Uint8Array.from(copyAtSend.slice(1)))).toBe(SENTINEL)
    expect(sent!.every((b) => b === 0)).toBe(true)
  })

  it('Return in a non-empty field submits; Return in an empty field sends nothing', () => {
    const sendUnlock = vi.fn(() => true)
    renderPanel(fakeClient(sendUnlock))
    const enter = () => act(() => field()!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })))
    enter()
    expect(sendUnlock).not.toHaveBeenCalled()
    type(SENTINEL)
    enter()
    expect(sendUnlock).toHaveBeenCalledTimes(1)
    expect(field()!.value).toBe('')
  })

  it('the field is an uncontrolled password input outside any form, with the password-manager opt-outs and no show control', () => {
    renderPanel(fakeClient(() => true))
    const f = field()!
    expect(f.closest('form')).toBeNull()
    expect(f.getAttribute('name')).toBeNull()
    expect(f.getAttribute('aria-label')).toBe('Mac password')
    expect(f.placeholder).toBe('Mac password')
    expect(f.getAttribute('autocomplete')).toBe('off')
    expect(f.getAttribute('maxlength')).toBe('256')
    expect(f.hasAttribute('data-1p-ignore')).toBe(true)
    expect(f.getAttribute('data-lpignore')).toBe('true')
    expect(f.hasAttribute('data-bwignore')).toBe(true)
    expect(container.querySelectorAll('input').length).toBe(1)
    expect(container.querySelector('[role="group"]')?.getAttribute('aria-labelledby')).toBe(container.querySelector('h2')?.id)
  })

  it('a sentinel typed and submitted appears in no console output and not in the page text', () => {
    const spies = (['log', 'info', 'warn', 'error', 'debug'] as const).map((m) => vi.spyOn(console, m).mockImplementation(() => {}))
    renderPanel(fakeClient(() => true))
    type(SENTINEL)
    click(unlockButton())
    for (const s of spies) expect(JSON.stringify(s.mock.calls)).not.toContain(SENTINEL)
    expect(document.body.textContent).not.toContain(SENTINEL)
    expect(window.localStorage.length + window.sessionStorage.length).toBe(0)
  })
})

describe('Unlock panel render gate', () => {
  it('an own-account viewer on a locked Mac whose hello carried caps unlock sees the panel and it is not focused', async () => {
    renderView()
    await lockedMac()
    expect(field()).not.toBeNull()
    expect(container.querySelector('h2')?.textContent).toBe('Unlock This Mac')
    expect(document.activeElement).not.toBe(field())
    // A sibling of the stage, never inside the element that owns the key listener.
    expect(container.querySelector('.desktop-stage')!.contains(field())).toBe(false)
  })

  it('submitting from the view calls the client once and keeps the page free of the sentinel', async () => {
    renderView()
    await lockedMac()
    type(SENTINEL)
    click(unlockButton())
    expect(view().sendUnlock).toHaveBeenCalledTimes(1)
    expect(document.body.textContent).not.toContain(SENTINEL)
  })

  it('no panel without caps unlock, even on a locked Mac', async () => {
    renderView()
    await lockedMac({ audio: false, unlock: false })
    expect(field()).toBeNull()
    expect(container.textContent).not.toContain('Unlock This Mac')
  })

  it('no panel on a guest page, whatever the hello said', async () => {
    renderView({ guest: true })
    await lockedMac()
    expect(field()).toBeNull()
    expect(container.querySelector('input')).toBeNull()
  })

  it('no panel when the Mac is not locked', async () => {
    renderView()
    await lockedMac()
    act(() => view().emit('state', { state: 'streaming', viewers: 1, locked: false }))
    expect(field()).toBeNull()
  })
})
