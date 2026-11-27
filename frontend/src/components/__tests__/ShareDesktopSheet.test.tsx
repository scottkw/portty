/**
 * The Share Desktop sheet (22-10, 22-UI-SPEC Surface 2). The App bindings and the
 * Wails clipboard are mocked; nothing here starts a share, reads a permission or
 * touches a clipboard. The link in these tests is a placeholder.
 */
import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { act } from 'react'
import { ShareDesktopSheet } from '../ShareDesktopSheet'
import { NO_SHARE, type ShareState, type ShareViewer } from '../../lib/desktopShare'
import * as copy from '../../lib/shareDesktopCopy'
import { TAILSCALE_OFF_REASON, USERSPACE_SENTENCE, type TailscaleHealth } from '../../lib/tailscale'
import * as App from '../../wailsjs/go/main/App'
import { ClipboardSetText } from '../../wailsjs/wailsjs/runtime/runtime'

vi.mock('../../wailsjs/go/main/App', () => ({
  HasCTDisclosure: vi.fn(),
  AcknowledgeCTDisclosure: vi.fn(),
  StartDesktopShare: vi.fn(),
  GetDesktopShare: vi.fn(),
  StopDesktopShare: vi.fn(),
  SetDesktopViewerRights: vi.fn(),
  DisconnectDesktopViewer: vi.fn(),
  GetDesktopStatus: vi.fn(),
  GetDesktopPermissions: vi.fn(),
  RunDesktopTestCapture: vi.fn(),
  OpenDesktopPermissionPane: vi.fn().mockResolvedValue(undefined),
}))
vi.mock('../../wailsjs/wailsjs/runtime/runtime', () => ({
  BrowserOpenURL: vi.fn(),
  ClipboardSetText: vi.fn(),
}))

const hasCT = vi.mocked(App.HasCTDisclosure)
const ackCT = vi.mocked(App.AcknowledgeCTDisclosure)
const start = vi.mocked(App.StartDesktopShare)
const getShare = vi.mocked(App.GetDesktopShare)
const stop = vi.mocked(App.StopDesktopShare)
const getStatus = vi.mocked(App.GetDesktopStatus)
const getPerms = vi.mocked(App.GetDesktopPermissions)
const clipboard = vi.mocked(ClipboardSetText)

const LINK = 'https://placeholder.invalid/desktop/guest/?cap=PLACEHOLDER'
const CONNECTED: TailscaleHealth = { connected: true, hasCerts: true, ip: '100.64.0.1', domain: 'my-mac.tailnet.ts.net' }

const DISCONNECTED: TailscaleHealth = { connected: false, hasCerts: false, ip: '', domain: '' }

const okStart = (): App.DesktopShareResult => ({ url: LINK, firstShare: false, errorCode: '', detail: '' })
const fail = (errorCode: string, detail = ''): App.DesktopShareResult => ({ url: '', firstShare: false, errorCode, detail })
const status = (over: Partial<App.DesktopStatusResult> = {}): App.DesktopStatusResult => ({
  alwaysAvailable: false, state: 'idle', viewers: 0, locked: false, approachingLapse: false,
  tailscale: 'ok', launchd: true, reason: '', surfaceKind: '', surfaceName: '', control: '', errorCode: '', ...over,
})

let container: HTMLElement
let root: ReturnType<typeof createRoot>
const onClose = vi.fn()

interface MountOpts { health?: TailscaleHealth | null; state?: ShareState; stoppedNotice?: boolean }

function view(opts: MountOpts = {}): React.ReactElement {
  return React.createElement(ShareDesktopSheet, {
    open: true,
    onClose,
    tailscaleHealth: opts.health === undefined ? CONNECTED : opts.health,
    state: opts.state ?? NO_SHARE,
    stoppedNotice: opts.stoppedNotice ?? false,
  })
}

async function mount(opts: MountOpts = {}): Promise<void> {
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  await act(async () => { root.render(view(opts)) })
  await tick(0)
}

async function rerender(opts: MountOpts): Promise<void> {
  await act(async () => { root.render(view(opts)) })
  await tick(0)
}

async function tick(ms: number): Promise<void> {
  await act(async () => { await vi.advanceTimersByTimeAsync(ms) })
}

async function click(el: Element): Promise<void> {
  await act(async () => { el.dispatchEvent(new MouseEvent('click', { bubbles: true })) })
  await tick(0)
}

const button = (name: string): HTMLButtonElement | undefined =>
  Array.from(container.querySelectorAll<HTMLButtonElement>('button')).find((b) => b.textContent?.trim() === name)
const text = (): string => container.textContent ?? ''

const guest = (over: Partial<ShareViewer> = {}): ShareViewer => ({
  id: 'v1', mode: 'guest', displayName: 'Pat Colleague', loginName: 'colleague@example.com', nodeName: 'pat-laptop',
  control: false, clipboard: false, since: '2026-11-27T10:00:00Z', ...over,
})

beforeEach(() => {
  vi.useFakeTimers()
  hasCT.mockResolvedValue(true)
  ackCT.mockResolvedValue(undefined)
  start.mockResolvedValue(okStart())
  getShare.mockResolvedValue({ active: false, url: '', errorCode: '', detail: '' })
  stop.mockResolvedValue({ errorCode: '', detail: '' })
  getStatus.mockResolvedValue(status())
  getPerms.mockResolvedValue({ sr: true, ax: true, errorCode: '' })
  clipboard.mockResolvedValue(true)
})

afterEach(async () => {
  await act(async () => { root.unmount() })
  container.remove()
  vi.clearAllMocks()
  vi.useRealTimers()
})

describe('ShareDesktopSheet: start and copy (tracer)', () => {
  it('open starts the share at once, shows the link row, and Copy Link copies then reads Copied! for 2 s', async () => {
    let release: (r: App.DesktopShareResult) => void = () => {}
    start.mockReturnValueOnce(new Promise((res) => { release = res }))
    await mount()
    // Starting: the description is replaced and the buttons are inert.
    expect(text()).toContain(copy.SHARE_STARTING)
    expect(button(copy.START_SHARING)!.getAttribute('aria-disabled')).toBe('true')
    expect(button(copy.NOT_NOW)!.getAttribute('aria-disabled')).toBe('true')
    expect(start).toHaveBeenCalledTimes(1)
    expect(ackCT).not.toHaveBeenCalled()

    await act(async () => { release(okStart()) })
    await tick(0)
    expect(text()).not.toContain(copy.SHARE_STARTING)
    expect(text()).toContain(copy.SHARE_LINK_LABEL)
    expect(text()).toContain(copy.SHARE_LINK_DESCRIPTION)
    expect(text()).toContain(copy.WATCHING_HEADER)
    expect(text()).toContain(copy.WATCHING_EMPTY)
    expect(text()).toContain(copy.SHARE_FOOTNOTE)
    // Everyone watching hears the Mac, even before they unmute; there is no audio switch (DCTL-07).
    expect(text()).toContain("hears this Mac's sound, even before they unmute")
    expect(container.querySelector('[role="switch"]')).toBeNull()
    // The link is never shown.
    expect(text()).not.toContain('PLACEHOLDER')
    expect(container.innerHTML).not.toContain('PLACEHOLDER')
    expect(container.querySelector('input, textarea, canvas, svg[aria-label*="QR" i]')).toBeNull()

    await click(button(copy.COPY_LINK)!)
    expect(clipboard).toHaveBeenCalledWith(LINK)
    expect(button(copy.COPIED)).toBeDefined()
    await tick(1999)
    expect(button(copy.COPIED)).toBeDefined()
    await tick(2)
    expect(button(copy.COPY_LINK)).toBeDefined()
    expect(button(copy.COPIED)).toBeUndefined()
  })

  it('is a labelled modal dialog whose primary button is Done, and Esc closes without stopping', async () => {
    await mount()
    const dlg = container.querySelector('[role="dialog"]')!
    expect(dlg.getAttribute('aria-modal')).toBe('true')
    expect(dlg.className).toContain('dialog--share')
    const title = document.getElementById(dlg.getAttribute('aria-labelledby')!)!
    expect(title.textContent).toBe(copy.SHARE_SHEET_TITLE)
    expect(button(copy.DONE)!.hasAttribute('data-dialog-primary')).toBe(true)
    await act(async () => { document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })) })
    expect(onClose).toHaveBeenCalledTimes(1)
    expect(stop).not.toHaveBeenCalled()
    await click(button(copy.DONE)!)
    expect(onClose).toHaveBeenCalledTimes(2)
    expect(stop).not.toHaveBeenCalled()
  })

  it('a failed copy shows the inline error and no Copied!', async () => {
    clipboard.mockResolvedValue(false)
    await mount()
    await click(button(copy.COPY_LINK)!)
    expect(container.querySelector('[role="alert"]')!.textContent).toBe(copy.COPY_LINK_FAILED)
    expect(button(copy.COPIED)).toBeUndefined()
  })

  it('the Watching list shows who is connected, from the latest state, in the Sharing view only', async () => {
    await mount()
    expect(container.querySelectorAll('[role="group"]')).toHaveLength(0)
    await rerender({ state: { ...NO_SHARE, active: true, viewers: [guest()] } })
    expect(container.querySelectorAll('[role="group"]')).toHaveLength(1)
    expect(text()).toContain('colleague@example.com')
    expect(text()).not.toContain(copy.WATCHING_EMPTY)
  })

  it('reopening while a share runs shows Sharing with the same link and starts nothing', async () => {
    getShare.mockResolvedValue({ active: true, url: LINK, errorCode: '', detail: '' })
    await mount()
    expect(start).not.toHaveBeenCalled()
    expect(ackCT).not.toHaveBeenCalled()
    expect(text()).toContain(copy.SHARE_LINK_LABEL)
    await click(button(copy.COPY_LINK)!)
    expect(clipboard).toHaveBeenCalledWith(LINK)
  })

  it('first run waits for Start Sharing, shows the CT line, and Start Sharing records the acknowledgement first', async () => {
    hasCT.mockResolvedValue(false)
    await mount()
    expect(start).not.toHaveBeenCalled()
    expect(text()).toContain(copy.SHARE_NOT_SHARING_DESCRIPTION)
    expect(text()).toContain('Show your screen and sound to someone')
    expect(text()).toContain(copy.SHARE_CT_LINE('my-mac.tailnet'))
    expect(button(copy.NOT_NOW)).toBeDefined()

    const order: string[] = []
    ackCT.mockImplementation(async () => { order.push('ack') })
    start.mockImplementation(async () => { order.push('start'); return okStart() })
    await click(button(copy.START_SHARING)!)
    expect(order).toEqual(['ack', 'start'])
    expect(text()).toContain(copy.SHARE_LINK_LABEL)
  })

  it('Not Now closes and shares nothing', async () => {
    hasCT.mockResolvedValue(false)
    await mount()
    await click(button(copy.NOT_NOW)!)
    expect(onClose).toHaveBeenCalledTimes(1)
    expect(start).not.toHaveBeenCalled()
  })

  it('Stop Sharing stops at once with no confirm, drops to Not sharing and does not restart', async () => {
    await mount()
    expect(start).toHaveBeenCalledTimes(1)
    await click(button(copy.STOP_SHARING)!)
    expect(stop).toHaveBeenCalledTimes(1)
    expect(container.querySelector('[role="alertdialog"]')).toBeNull()
    expect(text()).toContain(copy.SHARE_NOT_SHARING_DESCRIPTION)
    expect(button(copy.START_SHARING)).toBeDefined()
    await tick(5000)
    expect(start).toHaveBeenCalledTimes(1)
    expect(onClose).not.toHaveBeenCalled()
  })

  it('a failed stop shows the error with the local detail and stays in Sharing', async () => {
    stop.mockResolvedValue({ errorCode: 'stop_failed', detail: 'daemon not connected.' })
    await mount()
    await click(button(copy.STOP_SHARING)!)
    expect(container.querySelector('[role="alert"]')!.textContent).toBe(copy.shareDesktopStopError('daemon not connected'))
    expect(button(copy.STOP_SHARING)).toBeDefined()
  })
})

describe('ShareDesktopSheet: start failures and reasons (Surface 2 table)', () => {
  const startBlocked = (): boolean => button(copy.START_SHARING)!.getAttribute('aria-disabled') === 'true'

  it('tailscale_off: the Phase 4 reason replaces the description and Start Sharing is blocked and described by it', async () => {
    start.mockResolvedValue(fail('tailscale_off', "Tailscale isn't connected"))
    await mount()
    expect(text()).toContain(TAILSCALE_OFF_REASON)
    expect(text()).not.toContain(copy.SHARE_NOT_SHARING_DESCRIPTION)
    expect(startBlocked()).toBe(true)
    const reason = document.getElementById(button(copy.START_SHARING)!.getAttribute('aria-describedby')!)!
    expect(reason.textContent).toBe(TAILSCALE_OFF_REASON)
  })

  it('Tailscale disconnected at open: nothing is started, the reason shows and Start Sharing is blocked', async () => {
    await mount({ health: DISCONNECTED })
    expect(start).not.toHaveBeenCalled()
    expect(text()).toContain(TAILSCALE_OFF_REASON)
    expect(startBlocked()).toBe(true)
    await click(button(copy.START_SHARING)!)
    expect(start).not.toHaveBeenCalled()
  })

  it('tailscale_userspace: the userspace sentence replaces the description and Start Sharing is blocked', async () => {
    start.mockResolvedValue(fail('tailscale_userspace', 'userspace'))
    await mount()
    expect(text()).toContain(USERSPACE_SENTENCE)
    expect(startBlocked()).toBe(true)
  })

  it('https_off shows the HTTPS sentence as an inline error and Start Sharing stays usable', async () => {
    start.mockResolvedValue(fail('https_off', 'HTTPS certificates are off for your tailnet'))
    await mount()
    expect(container.querySelector('[role="alert"]')!.textContent).toBe(copy.SHARE_HTTPS_OFF_ERROR)
    expect(startBlocked()).toBe(false)
  })

  it('any other code shows the start error with the cleaned detail and Start Sharing retries', async () => {
    start.mockResolvedValueOnce(fail('start_failed', 'Error: no listener.'))
    await mount()
    expect(container.querySelector('[role="alert"]')!.textContent).toBe(copy.shareDesktopStartError('no listener'))
    expect(startBlocked()).toBe(false)
    await click(button(copy.START_SHARING)!)
    expect(start).toHaveBeenCalledTimes(2)
    expect(container.querySelector('[role="alert"]')).toBeNull()
    expect(text()).toContain(copy.SHARE_LINK_LABEL)
  })

  it('a share that ends while the sheet is open drops to Not sharing with the status notice and never restarts', async () => {
    await mount()
    expect(text()).toContain(copy.SHARE_LINK_LABEL)
    await rerender({ stoppedNotice: true })
    expect(text()).toContain(copy.SHARE_NOT_SHARING_DESCRIPTION)
    expect(container.querySelector('[role="status"]')!.textContent).toContain(copy.SHARE_STOPPED_NOTICE)
    await tick(5000)
    expect(start).toHaveBeenCalledTimes(1)
  })

  it('a share stopped elsewhere (the tray) drops the open sheet to Not sharing without a notice', async () => {
    await mount()
    await rerender({ state: { ...NO_SHARE, active: true } })
    await rerender({ state: NO_SHARE })
    expect(text()).toContain(copy.SHARE_NOT_SHARING_DESCRIPTION)
    expect(text()).not.toContain(copy.SHARE_STOPPED_NOTICE)
    expect(start).toHaveBeenCalledTimes(1)
  })

  it('a stale inactive state line right after Start does not drop the new share', async () => {
    await mount()
    await rerender({ state: NO_SHARE })
    expect(text()).toContain(copy.SHARE_LINK_LABEL)
  })
})

describe('ShareDesktopSheet: status row', () => {
  it.each([
    ['needs_permission', copy.STATUS_NEEDS_PERMISSION],
    ['awaiting_approval', copy.STATUS_AWAITING_APPROVAL],
    ['display_asleep', copy.STATUS_DISPLAY_ASLEEP],
    ['capture_paused', copy.STATUS_CAPTURE_PAUSED],
  ])('shows %s with the Phase 19 words', async (state, words) => {
    getStatus.mockResolvedValue(status({ state }))
    await mount()
    const row = container.querySelector('.share-desktop__status')!
    expect(row.textContent).toContain(words)
    expect(row.querySelector('[aria-live="polite"]')).not.toBeNull()
    expect(row.querySelector('.ts-status__dot')!.getAttribute('aria-hidden')).toBe('true')
  })

  it('Needs permission offers Set Up Permissions…, which opens the setup sheet after this one', async () => {
    getStatus.mockResolvedValue(status({ state: 'needs_permission' }))
    await mount()
    await click(button(copy.SET_UP_PERMISSIONS)!)
    const modals = container.ownerDocument.querySelectorAll('[aria-modal="true"]')
    expect(modals.length).toBe(2)
    expect(modals[1].textContent).toContain('Set Up Desktop Sharing')
  })

  it.each(['idle', 'streaming', 'locked'])('is absent while the desktop is %s', async (state) => {
    getStatus.mockResolvedValue(status({ state }))
    await mount()
    expect(container.querySelector('.share-desktop__status')).toBeNull()
  })

  it('shows Waiting for Tailscale when Tailscale drops during a share, and has no button', async () => {
    await mount()
    await rerender({ health: DISCONNECTED, state: { ...NO_SHARE, active: true } })
    const row = container.querySelector('.share-desktop__status')!
    expect(row.textContent).toContain(copy.STATUS_WAITING_FOR_TAILSCALE)
    expect(row.querySelector('button')).toBeNull()
    expect(button(copy.COPY_LINK)!.getAttribute('aria-disabled')).toBe('true')
  })
})
