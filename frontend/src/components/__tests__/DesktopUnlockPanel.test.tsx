// Phase 23-07 — the Unlock This Mac panel (UNLK-01, 23-UI-SPEC Surface 2). The tracer: type, press Unlock, and the
// bytes leave on the data channel with the field already empty. The sentinel below is a made-up string, never a real password.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import type { DesktopClient, DesktopEvents, DesktopUnlockMsg } from '../../lib/desktopClient'
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

interface PanelOpts {
  unlock?: DesktopUnlockMsg | null
  reconnecting?: boolean
  needsAccessibility?: boolean
}
function renderPanel(client: DesktopClient, o: PanelOpts = {}) {
  act(() =>
    root.render(
      <DesktopUnlockPanel
        client={client}
        unlock={o.unlock ?? null}
        reconnecting={o.reconnecting ?? false}
        needsAccessibility={o.needsAccessibility ?? false}
        phone={false}
      />,
    ),
  )
}

/** The daemon's golden unlock lines (internal/desktop/testdata/signal-v1.jsonl), as the viewer's typed event. */
function golden(name: string): DesktopUnlockMsg {
  const lines = readFileSync(resolve(__dirname, '../../../../internal/desktop/testdata/signal-v1.jsonl'), 'utf8')
    .trim().split('\n').map((l) => JSON.parse(l) as { name: string; wire: Record<string, unknown> })
  const w = lines.find((l) => l.name === name)!.wire
  return {
    state: w.state as DesktopUnlockMsg['state'],
    failures: w.failures as number,
    waitMs: (w.wait_ms as number | undefined) ?? 0,
    reason: (w.reason as DesktopUnlockMsg['reason'] | undefined) ?? '',
    byPortty: w.by_portty === true,
  }
}
const statusText = () => container.querySelector('.desktop-unlock__status')?.textContent ?? ''
const lineTexts = () => Array.from(container.querySelectorAll('.desktop-unlock__line')).map((l) => l.textContent)
const inert = () => field()!.getAttribute('aria-disabled') === 'true' && field()!.readOnly
const buttonDimmed = () => unlockButton().getAttribute('aria-disabled') === 'true'
const okClient = () => fakeClient(() => true)

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

describe('the eleven states follow the daemon (golden lines)', () => {
  it('1 Waiting: the privacy line, the field enabled and empty, Unlock dimmed; typing makes Unlock live and keeps the line', () => {
    renderPanel(okClient())
    expect(statusText()).toBe("Portty never saves it. Wrong tries count toward the Mac's own limit.")
    expect(inert()).toBe(false)
    expect(buttonDimmed()).toBe(true)
    type(SENTINEL)
    expect(buttonDimmed()).toBe(false)
    expect(statusText()).toContain('Portty never saves it')
  })

  it('3 and 4 Unlocking and Trying once more: an icon and the words, a status role, the field and Unlock inert', () => {
    renderPanel(okClient(), { unlock: golden('unlock-unlocking') })
    expect(lineTexts()).toEqual(['Unlocking…'])
    expect(container.querySelector('.desktop-unlock__line svg')).not.toBeNull()
    expect(container.querySelector('.desktop-unlock__status')?.getAttribute('role')).toBe('status')
    expect(container.querySelector('[role="alert"]')).toBeNull()
    expect(inert()).toBe(true)
    expect(buttonDimmed()).toBe(true)
    renderPanel(okClient(), { unlock: golden('unlock-retrying') })
    expect(lineTexts()).toEqual(['Trying once more…'])
    expect(inert()).toBe(true)
  })

  it('6 Wrong password: an alert with the words, the field enabled and focused; the pause warning only from the second failure', () => {
    renderPanel(okClient(), { unlock: golden('unlock-failed') })
    expect(container.querySelector('[role="alert"]')?.textContent).toBe("Unlock didn't work. Check the password and try again.")
    expect(lineTexts()).toHaveLength(1)
    expect(inert()).toBe(false)
    expect(document.activeElement).toBe(field())
    renderPanel(okClient(), { unlock: { ...golden('unlock-failed'), failures: 2 } })
    expect(lineTexts()).toEqual([
      "Unlock didn't work. Check the password and try again.",
      'If the next try fails, Unlock pauses for 15 minutes.',
    ])
  })

  it('7 Rate limited: Try again in N s counts down once a second, says so once, and goes live at zero', () => {
    renderPanel(okClient(), { unlock: golden('unlock-rate-limited') })
    expect(lineTexts()).toEqual(["Unlock didn't work. Check the password and try again.", 'Try again in 3 s'])
    expect(container.querySelectorAll('.visually-hidden')).toHaveLength(1)
    expect(container.querySelector('.visually-hidden')?.textContent).toBe('You can try again in a few seconds')
    expect(container.querySelector('.desktop-unlock__line--plain span')?.getAttribute('aria-live')).toBe('off')
    expect(inert()).toBe(false)
    type(SENTINEL)
    expect(buttonDimmed()).toBe(true)
    act(() => { vi.advanceTimersByTime(1000) })
    expect(lineTexts()[1]).toBe('Try again in 2 s')
    act(() => { vi.advanceTimersByTime(1000) })
    expect(lineTexts()[1]).toBe('Try again in 1 s')
    act(() => { vi.advanceTimersByTime(1000) })
    expect(statusText()).toContain('Portty never saves it')
    expect(container.querySelector('.visually-hidden')).toBeNull()
    expect(buttonDimmed()).toBe(false)
  })

  it('7 Rate limited: with no failure counted the countdown is the only line, and a submit sends nothing', () => {
    const sendUnlock = vi.fn(() => true)
    renderPanel(fakeClient(sendUnlock), { unlock: { ...golden('unlock-rate-limited'), failures: 0 } })
    expect(lineTexts()).toEqual(['Try again in 3 s'])
    type(SENTINEL)
    click(unlockButton())
    expect(sendUnlock).not.toHaveBeenCalled()
  })

  it('8 Paused: the minutes count down as in 15 min, in 1 min, in under 1 min; an alert on entry; field and Unlock inert; nothing is sent', () => {
    const sendUnlock = vi.fn(() => true)
    renderPanel(fakeClient(sendUnlock), { unlock: golden('unlock-paused') })
    const paused = (when: string) => `Unlock is paused after 3 failed tries. Try again ${when}, or unlock it at the Mac.`
    expect(lineTexts()).toEqual([paused('in 15 min')])
    expect(container.querySelector('[role="alert"]')).not.toBeNull()
    expect(inert()).toBe(true)
    expect(buttonDimmed()).toBe(true)
    act(() => { vi.advanceTimersByTime(14 * 60000) })
    expect(lineTexts()).toEqual([paused('in 1 min')])
    act(() => { vi.advanceTimersByTime(1000) })
    expect(lineTexts()).toEqual([paused('in under 1 min')])
    click(unlockButton())
    expect(sendUnlock).not.toHaveBeenCalled()
    act(() => { vi.advanceTimersByTime(60000) })
    expect(statusText()).toContain('Portty never saves it')
  })

  it('9 Refused: Caps Lock and an unmappable character each get their alert, the field enabled', () => {
    renderPanel(okClient(), { unlock: golden('unlock-refused-caps-lock') })
    expect(container.querySelector('[role="alert"]')?.textContent).toBe('Caps Lock is on at the Mac. Turn it off there, then try again.')
    expect(inert()).toBe(false)
    renderPanel(okClient(), { unlock: golden('unlock-refused-unmappable') })
    expect(container.querySelector('[role="alert"]')?.textContent).toBe(
      "This password has a character Portty can't type. Unlock it at the Mac instead.",
    )
  })

  it('10 Inert: Reconnecting… with no icon, and the Accessibility line from the daemon or from the control state', () => {
    renderPanel(okClient(), { reconnecting: true, unlock: golden('unlock-failed') })
    expect(lineTexts()).toEqual(['Reconnecting…'])
    expect(inert()).toBe(true)
    type('x')
    click(unlockButton())
    expect(buttonDimmed()).toBe(true)
    const access = 'Unlock needs Accessibility. On the Mac, allow Portty in Privacy & Security.'
    renderPanel(okClient(), { unlock: golden('unlock-unavailable-accessibility') })
    expect(lineTexts()).toEqual([access])
    expect(inert()).toBe(true)
    renderPanel(okClient(), { needsAccessibility: true })
    expect(lineTexts()).toEqual([access])
    expect(inert()).toBe(true)
  })

  it('11 Could not run: the daemon error, or a channel that was not open, both read as an alert with a next step', () => {
    renderPanel(okClient(), { unlock: golden('unlock-error') })
    expect(container.querySelector('[role="alert"]')?.textContent).toBe("Couldn't unlock the Mac right now. Try again.")
    expect(inert()).toBe(false)
    renderPanel(fakeClient(() => false))
    type(SENTINEL)
    click(unlockButton())
    expect(container.querySelector('[role="alert"]')?.textContent).toBe("Couldn't unlock the Mac right now. Try again.")
    expect(field()!.value).toBe('')
  })

  it('after a submit the line reads Unlocking… until the daemon answers; with no answer in 8 s it is Could not run', () => {
    renderPanel(okClient())
    type(SENTINEL)
    click(unlockButton())
    expect(lineTexts()).toEqual(['Unlocking…'])
    act(() => { vi.advanceTimersByTime(8000) })
    expect(container.querySelector('[role="alert"]')?.textContent).toBe("Couldn't unlock the Mac right now. Try again.")
  })

  it("a second viewer's panel follows the daemon with no local submit: Unlocking…, then Wrong password", () => {
    renderPanel(okClient(), { unlock: golden('unlock-unlocking') })
    expect(lineTexts()).toEqual(['Unlocking…'])
    renderPanel(okClient(), { unlock: golden('unlock-failed') })
    expect(container.querySelector('[role="alert"]')).not.toBeNull()
    expect(document.activeElement).toBe(field())
  })
})

describe('clearing and focus rules', () => {
  it('Esc empties the field and keeps focus; a plain blur keeps the text', () => {
    renderPanel(okClient())
    field()!.focus()
    type(SENTINEL)
    act(() => field()!.blur())
    expect(field()!.value).toBe(SENTINEL)
    field()!.focus()
    act(() => field()!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })))
    expect(field()!.value).toBe('')
    expect(document.activeElement).toBe(field())
    expect(buttonDimmed()).toBe(true)
  })

  it('the field is emptied when the page becomes hidden and on pagehide', () => {
    renderPanel(okClient())
    type(SENTINEL)
    const state = vi.spyOn(document, 'visibilityState', 'get')
    state.mockReturnValue('visible')
    act(() => { document.dispatchEvent(new Event('visibilitychange')) })
    expect(field()!.value).toBe(SENTINEL)
    state.mockReturnValue('hidden')
    act(() => { document.dispatchEvent(new Event('visibilitychange')) })
    expect(field()!.value).toBe('')
    type(SENTINEL)
    act(() => { window.dispatchEvent(new Event('pagehide')) })
    expect(field()!.value).toBe('')
  })

  it('the field is emptied when the panel unmounts', () => {
    renderPanel(okClient())
    const el = field()!
    type(SENTINEL)
    act(() => root.render(<div />))
    expect(el.value).toBe('')
  })

  it('Return during an input-method composition does not submit', () => {
    const sendUnlock = vi.fn(() => true)
    renderPanel(fakeClient(sendUnlock))
    type(SENTINEL)
    act(() => field()!.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true })))
    act(() => field()!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })))
    expect(sendUnlock).not.toHaveBeenCalled()
    act(() => field()!.dispatchEvent(new CompositionEvent('compositionend', { bubbles: true })))
    act(() => field()!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })))
    expect(sendUnlock).toHaveBeenCalledTimes(1)
  })
})

describe('in the view: success, reconnecting and cards', () => {
  const controlView = async () => {
    renderView()
    act(() => view().emit('rights', { control: true, clipboard: true, surface: true }))
    await lockedMac()
  }
  const noticeText = () => container.querySelector('.desktop-chip--notice')?.textContent
  const unlocked = (by: boolean) =>
    act(() => view().emit('unlock', { state: 'unlocked', failures: 0, waitMs: 0, reason: '', byPortty: by }))
  const flip = () => act(() => view().emit('state', { state: 'streaming', viewers: 1, locked: false }))

  it('5 Success after a Portty unlock: the panel is gone, the stage has the keyboard, the notice says it will lock again, with no capture notice', async () => {
    await controlView()
    unlocked(true)
    flip()
    expect(field()).toBeNull()
    expect(container.querySelector('.desktop-stage')!.contains(document.activeElement)).toBe(true)
    expect(noticeText()).toBe('Mac unlocked. It will lock again after you disconnect.')
    act(() => { vi.advanceTimersByTime(4999) })
    expect(noticeText()).toBe('Mac unlocked. It will lock again after you disconnect.')
    act(() => { vi.advanceTimersByTime(300) })
    expect(noticeText()).toBeUndefined()
  })

  it('the by_portty message arriving just after the flip still gives the long notice, once', async () => {
    await controlView()
    flip()
    expect(noticeText()).toBeUndefined()
    unlocked(true)
    expect(noticeText()).toBe('Mac unlocked. It will lock again after you disconnect.')
    act(() => { vi.advanceTimersByTime(2000) })
    expect(noticeText()).toBe('Mac unlocked. It will lock again after you disconnect.')
  })

  it('a Mac unlocked at its own keyboard shows the plain Mac unlocked for 3 s, after about 1.5 s with no by_portty message', async () => {
    await controlView()
    flip()
    act(() => { vi.advanceTimersByTime(1400) })
    expect(noticeText()).toBeUndefined()
    act(() => { vi.advanceTimersByTime(200) })
    expect(noticeText()).toBe('Mac unlocked')
    act(() => { vi.advanceTimersByTime(2900) })
    expect(noticeText()).toBe('Mac unlocked')
    act(() => { vi.advanceTimersByTime(300) })
    expect(noticeText()).toBeUndefined()
  })

  it('a lost connection is not an unlock: no notice, and the panel stays inert with Reconnecting…', async () => {
    await controlView()
    act(() => view().emit('status', { kind: 'reconnecting' }))
    expect(lineTexts()).toEqual(['Reconnecting…'])
    expect(inert()).toBe(true)
    act(() => { vi.advanceTimersByTime(5000) })
    expect(noticeText()).toBeUndefined()
  })

  it('Control needing Accessibility shows the Accessibility line in the panel', async () => {
    await controlView()
    act(() => view().emit('control', { input: 'needs_accessibility', window: '' }))
    expect(lineTexts()).toEqual(['Unlock needs Accessibility. On the Mac, allow Portty in Privacy & Security.'])
  })

  it('a message from the last lock episode is not shown when the Mac locks again', async () => {
    await controlView()
    act(() => view().emit('unlock', golden('unlock-failed')))
    expect(container.querySelector('[role="alert"]')).not.toBeNull()
    flip()
    act(() => view().emit('state', { state: 'locked', viewers: 1, locked: true }))
    expect(field()).not.toBeNull()
    expect(statusText()).toContain('Portty never saves it')
  })

  it('under a Phase 19 card the panel is not shown', async () => {
    await controlView()
    act(() => view().emit('state', { state: 'display_asleep', viewers: 1, locked: true }))
    expect(field()).toBeNull()
  })
})
