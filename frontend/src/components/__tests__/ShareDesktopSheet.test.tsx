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
import { NO_SHARE, type ShareState } from '../../lib/desktopShare'
import * as copy from '../../lib/shareDesktopCopy'
import { type TailscaleHealth } from '../../lib/tailscale'
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

const okStart = (): App.DesktopShareResult => ({ url: LINK, firstShare: false, errorCode: '', detail: '' })
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
