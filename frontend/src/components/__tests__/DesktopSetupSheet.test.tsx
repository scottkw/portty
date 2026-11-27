/**
 * DesktopSetupSheet (19-11, DPRM-01): live permission rows, Open System
 * Settings, Test Capture states, and the accessible sheet shell
 * (19-UI-SPEC Surface 2). The App bindings are mocked; nothing here reads a
 * real permission or captures a real screen.
 */
import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { act } from 'react'
import { DesktopSetupSheet } from '../DesktopSetupSheet'
import * as App from '../../wailsjs/go/main/App'

vi.mock('../../wailsjs/go/main/App', () => ({
  GetDesktopPermissions: vi.fn(),
  RunDesktopTestCapture: vi.fn(),
  OpenDesktopPermissionPane: vi.fn().mockResolvedValue(undefined),
}))

const getPerms = vi.mocked(App.GetDesktopPermissions)
const runCapture = vi.mocked(App.RunDesktopTestCapture)
const openPane = vi.mocked(App.OpenDesktopPermissionPane)

const SR_HOWTO = 'Open System Settings, go to Privacy & Security, then Screen & System Audio Recording, and turn on Portty.'
const AX_HOWTO =
  'Open System Settings, go to Privacy & Security, then Device Control and Data Access (called Accessibility on earlier macOS), and turn on Portty. Portty uses it to let your other devices control this Mac, and to tell a still screen from an unapproved one.'
const NOTE_1 =
  'macOS 27.2 and later show a dialog the first time Portty captures the screen, asking whether it may bypass the system private window picker. Choose Allow. It does not come back.'
const NOTE_2 =
  'Portty also adds an entry under Login Items so the desktop stays reachable after a restart. macOS lists it under the name of the developer who signed Portty, not "Portty", and its status text can look out of date. Leave it on.'
const FAILURE =
  "Couldn't capture the screen. Check that Screen Recording is on above. If a macOS dialog is waiting on the screen, choose Allow, then try again."

let container: HTMLElement
let root: ReturnType<typeof createRoot>

function perms(sr: boolean, ax: boolean, errorCode = ''): App.DesktopPermissionsResult {
  return { sr, ax, errorCode }
}

async function mount(props: Partial<React.ComponentProps<typeof DesktopSetupSheet>> = {}) {
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  await render({ open: true, onClose: vi.fn(), ...props })
}

async function render(props: React.ComponentProps<typeof DesktopSetupSheet>) {
  await act(async () => {
    root.render(React.createElement(DesktopSetupSheet, props))
  })
}

async function tick(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms)
  })
}

function row(label: string): HTMLElement {
  const el = Array.from(container.querySelectorAll<HTMLElement>('.settings-row')).find(
    (r) => r.querySelector('.settings-row__label')?.textContent === label,
  )
  if (!el) throw new Error(`no row ${label}`)
  return el
}

function button(name: string): HTMLButtonElement {
  const el = Array.from(container.querySelectorAll('button')).find((b) => b.textContent === name)
  if (!el) throw new Error(`no button ${name}`)
  return el as HTMLButtonElement
}

async function click(el: HTMLElement) {
  await act(async () => {
    el.dispatchEvent(new MouseEvent('click', { bubbles: true }))
  })
}

beforeEach(() => {
  vi.useFakeTimers()
  getPerms.mockReset()
  getPerms.mockResolvedValue(perms(false, false))
  runCapture.mockReset()
  openPane.mockClear()
})

afterEach(() => {
  act(() => root?.unmount())
  container?.remove()
  vi.useRealTimers()
})

describe('DesktopSetupSheet polling', () => {
  it('reads on open, every 2 s, and at once when the window regains focus', async () => {
    await mount()
    expect(getPerms).toHaveBeenCalledTimes(1)
    await tick(1999)
    expect(getPerms).toHaveBeenCalledTimes(1)
    await tick(1)
    expect(getPerms).toHaveBeenCalledTimes(2)
    await tick(2000)
    expect(getPerms).toHaveBeenCalledTimes(3)
    await act(async () => {
      window.dispatchEvent(new Event('focus'))
    })
    expect(getPerms).toHaveBeenCalledTimes(4)
  })

  it('stops reading when closed, and never closes or stops by itself', async () => {
    const onClose = vi.fn()
    await mount({ onClose })
    await tick(10 * 60 * 1000) // ten minutes of waiting for the user: no timeout
    expect(onClose).not.toHaveBeenCalled()
    expect(container.querySelector('[role="dialog"]')).not.toBeNull()
    const callsOpen = getPerms.mock.calls.length
    expect(callsOpen).toBeGreaterThan(250)
    await render({ open: false, onClose })
    await tick(10_000)
    await act(async () => {
      window.dispatchEvent(new Event('focus'))
    })
    expect(getPerms.mock.calls.length).toBe(callsOpen)
  })

  it('keeps what the rows show when a read fails, and never reads a failure as not granted', async () => {
    getPerms.mockResolvedValue(perms(false, false, 'unavailable'))
    await mount()
    expect(row('Screen Recording').textContent).toContain('Checking…')
    expect(row('Screen Recording').textContent).not.toContain('Not granted')
    // Unknown is not blocked: a failing probe must not lock the user out of Test Capture.
    expect(button('Test Capture').getAttribute('aria-disabled')).toBe('false')

    getPerms.mockResolvedValue(perms(true, true))
    await tick(2000)
    expect(row('Screen Recording').textContent).toContain('Granted')
    getPerms.mockResolvedValue(perms(false, false, 'unavailable'))
    await tick(2000)
    expect(row('Screen Recording').textContent).toContain('Granted')
  })
})

describe('DesktopSetupSheet permission rows', () => {
  it('flips each row between Not granted and Granted on the poll that sees the change, how-to included', async () => {
    await mount()
    const sr = () => row('Screen Recording')
    const ax = () => row('Accessibility')
    for (const r of [sr(), ax()]) {
      expect(r.textContent).toContain('Not granted')
      expect(r.querySelector('.ts-status__dot--warn')).not.toBeNull()
      expect(r.querySelector('.ts-status')?.getAttribute('aria-live')).toBe('polite')
      expect(r.querySelector('.ts-status__dot')?.getAttribute('aria-hidden')).toBe('true')
    }
    expect(sr().textContent).toContain(SR_HOWTO)
    expect(ax().textContent).toContain(AX_HOWTO)

    getPerms.mockResolvedValue(perms(true, false))
    await tick(2000)
    expect(sr().textContent).toContain('Granted')
    expect(sr().textContent).not.toContain('Not granted')
    expect(sr().querySelector('.ts-status__dot--ok')).not.toBeNull()
    expect(sr().querySelector('.ts-status__dot--warn')).toBeNull()
    expect(sr().textContent).not.toContain(SR_HOWTO)
    expect(ax().textContent).toContain('Not granted')
    expect(ax().textContent).toContain(AX_HOWTO)

    getPerms.mockResolvedValue(perms(true, true))
    await tick(2000)
    expect(ax().textContent).toContain('Granted')
    expect(ax().textContent).not.toContain(AX_HOWTO)

    // Revoked again in System Settings: the row follows.
    getPerms.mockResolvedValue(perms(false, true))
    await tick(2000)
    expect(sr().textContent).toContain('Not granted')
    expect(sr().textContent).toContain(SR_HOWTO)
  })

  it('opens the matching System Settings pane from each row, and the button stays after a grant', async () => {
    getPerms.mockResolvedValue(perms(true, true))
    await mount()
    const buttons = container.querySelectorAll('button')
    const open = Array.from(buttons).filter((b) => b.textContent === 'Open System Settings')
    expect(open).toHaveLength(2)
    await click(row('Screen Recording').querySelector('button')!)
    expect(openPane).toHaveBeenLastCalledWith('screen')
    await click(row('Accessibility').querySelector('button')!)
    expect(openPane).toHaveBeenLastCalledWith('accessibility')
  })

  it('does not let a refused pane open reject into the page', async () => {
    openPane.mockRejectedValueOnce(new Error('refused'))
    await mount()
    await click(row('Screen Recording').querySelector('button')!)
    expect(openPane).toHaveBeenCalledTimes(1)
  })
})

describe('DesktopSetupSheet Test Capture', () => {
  it('is aria-disabled with its reason while Screen Recording is off, and a click does nothing', async () => {
    await mount()
    const btn = button('Test Capture')
    expect(btn.getAttribute('aria-disabled')).toBe('true')
    const reasonId = btn.getAttribute('aria-describedby')
    expect(reasonId).toBeTruthy()
    expect(document.getElementById(reasonId!)?.textContent).toBe('Turn on Screen Recording first.')
    await click(btn)
    expect(runCapture).not.toHaveBeenCalled()
    expect(container.querySelector('[data-testid="capture-placeholder"]')).toBeNull()
  })

  it('needs only Screen Recording: Accessibility off never blocks it', async () => {
    getPerms.mockResolvedValue(perms(true, false))
    await mount()
    const btn = button('Test Capture')
    expect(btn.getAttribute('aria-disabled')).toBe('false')
    expect(btn.getAttribute('aria-describedby')).toBeNull()
    expect(container.textContent).not.toContain('Turn on Screen Recording first.')
  })

  it('shows Capturing… with the placeholder, then the thumbnail, the success line and Test Again', async () => {
    getPerms.mockResolvedValue(perms(true, true))
    let finish!: (r: App.TestCaptureResult) => void
    runCapture.mockReturnValue(new Promise((r) => { finish = r }))
    await mount()
    await click(button('Test Capture'))
    expect(button('Capturing…').getAttribute('aria-disabled')).toBe('true')
    expect(container.querySelector('[data-testid="capture-placeholder"]')).not.toBeNull()
    await click(button('Capturing…')) // a second click while capturing is ignored
    expect(runCapture).toHaveBeenCalledTimes(1)

    await act(async () => finish({ imageDataURL: 'data:image/jpeg;base64,AAAA', errorCode: '' }))
    expect(container.querySelector('[data-testid="capture-placeholder"]')).toBeNull()
    const img = container.querySelector('img')!
    expect(img.getAttribute('alt')).toBe("Preview of this Mac's main display")
    expect(img.getAttribute('src')).toBe('data:image/jpeg;base64,AAAA')
    expect(container.querySelector('[role="status"]')?.textContent).toBe('Capture works. This is what viewers will see.')
    expect(container.querySelector('[role="alert"]')).toBeNull()
    expect(button('Test Again')).toBeTruthy()
  })

  it('shows the failure line in role=alert, no placeholder and no thumbnail, and offers Test Again', async () => {
    getPerms.mockResolvedValue(perms(true, true))
    runCapture.mockResolvedValue({ imageDataURL: '', errorCode: 'timeout' })
    await mount()
    await click(button('Test Capture'))
    expect(container.querySelector('[role="alert"]')?.textContent).toBe(FAILURE)
    expect(container.querySelector('[data-testid="capture-placeholder"]')).toBeNull()
    expect(container.querySelector('img')).toBeNull()
    expect(container.querySelector('[role="status"]')).toBeNull()
    expect(button('Test Again').getAttribute('aria-disabled')).toBe('false')

    runCapture.mockResolvedValue({ imageDataURL: 'data:image/jpeg;base64,BBBB', errorCode: '' })
    await click(button('Test Again'))
    expect(container.querySelector('[role="alert"]')).toBeNull()
    expect(container.querySelector('img')).not.toBeNull()
  })
})

describe('DesktopSetupSheet dialog', () => {
  it('is a modal dialog labelled by its title, with the exact intro and both notes', async () => {
    await mount()
    const dialog = container.querySelector('[role="dialog"]')!
    expect(dialog.getAttribute('aria-modal')).toBe('true')
    const title = document.getElementById(dialog.getAttribute('aria-labelledby')!)
    expect(title?.textContent).toBe('Set Up Desktop Sharing')
    const text = dialog.textContent ?? ''
    expect(text).toContain("Portty needs two permissions to show and control this Mac's screen. Grant them here, once, while you are at the Mac.")
    expect(text).toContain("Checks that Portty can see this Mac's main display.")
    // 20-VM-CHECKS PASTE_ROW: none, so no paste row ships (the VM raised no paste alert).
    expect(text).not.toContain('Paste from Other Apps')
    expect(text).toContain(NOTE_1)
    expect(text).toContain(NOTE_2)
  })

  it('closes with Esc and with Done, and Done is enabled with nothing granted', async () => {
    const onClose = vi.fn()
    await mount({ onClose })
    const done = button('Done')
    expect(done.disabled).toBe(false)
    expect(done.getAttribute('aria-disabled')).toBeNull()
    expect(document.activeElement).toBe(done) // initial focus is the primary button
    await act(async () => {
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    })
    expect(onClose).toHaveBeenCalledTimes(1)
    await click(done)
    expect(onClose).toHaveBeenCalledTimes(2)
  })

  it('traps Tab inside the sheet and returns focus to the opener on close', async () => {
    const opener = document.createElement('button')
    document.body.appendChild(opener)
    opener.focus()
    const onClose = vi.fn()
    await mount({ onClose, returnFocusTo: opener })

    // Tab order: the two Open System Settings buttons, Test Capture, Done; it wraps.
    const order = Array.from(container.querySelectorAll('button')).map((b) => b.textContent)
    expect(order).toEqual(['Open System Settings', 'Open System Settings', 'Test Capture', 'Done'])
    const done = button('Done')
    expect(document.activeElement).toBe(done)
    const tab = new KeyboardEvent('keydown', { key: 'Tab', cancelable: true })
    await act(async () => {
      window.dispatchEvent(tab)
    })
    expect(tab.defaultPrevented).toBe(true)
    expect(document.activeElement).toBe(container.querySelector('button'))

    await render({ open: false, onClose, returnFocusTo: opener })
    expect(document.activeElement).toBe(opener)
    opener.remove()
  })

  it('renders nothing while closed', async () => {
    await mount({ open: false })
    expect(container.querySelector('[role="dialog"]')).toBeNull()
    expect(getPerms).not.toHaveBeenCalled()
  })
})
