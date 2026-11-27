/**
 * Settings ▸ Sharing, Desktop group (19-13: DSHR-01, DPRM-01/02/03; 19-UI-SPEC
 * Surface 1). The App bindings and the Wails clipboard are mocked; nothing here
 * talks to a daemon, reads a permission or touches a clipboard.
 */
import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { act } from 'react'
import { SettingsSheet, desktopStatusText } from '../SettingsSheet'
import { TAILSCALE_OFF_REASON } from '../../lib/tailscale'
import * as App from '../../wailsjs/go/main/App'
import { ClipboardSetText } from '../../wailsjs/wailsjs/runtime/runtime'
import type { TailscaleHealth } from '../../lib/tailscale'

vi.mock('../../wailsjs/go/main/App', () => ({
  HasCTDisclosure: vi.fn(),
  AcknowledgeCTDisclosure: vi.fn().mockResolvedValue(undefined),
  GetStartMinimized: vi.fn().mockResolvedValue(false),
  SetStartMinimized: vi.fn().mockResolvedValue(undefined),
  GetAppearance: vi.fn().mockResolvedValue('system'),
  SetAppearance: vi.fn().mockResolvedValue(undefined),
  GetTerminalFont: vi.fn().mockResolvedValue({ family: '', size: 13 }),
  SetTerminalFont: vi.fn().mockResolvedValue(undefined),
  RegenerateSigningKey: vi.fn().mockResolvedValue(undefined),
  GetDesktopStatus: vi.fn(),
  SetDesktopAlwaysAvailable: vi.fn(),
  GetDesktopLink: vi.fn(),
  GetDesktopPermissions: vi.fn(),
  RunDesktopTestCapture: vi.fn(),
  OpenDesktopPermissionPane: vi.fn().mockResolvedValue(undefined),
  GetDaemonRepair: vi.fn(),
  OpenLoginItems: vi.fn(),
}))
vi.mock('../../wailsjs/wailsjs/runtime/runtime', () => ({
  BrowserOpenURL: vi.fn(),
  ClipboardSetText: vi.fn(),
}))
vi.mock('../PluginsSection', () => ({ PluginsSection: () => null }))

const getStatus = vi.mocked(App.GetDesktopStatus)
const setDesktop = vi.mocked(App.SetDesktopAlwaysAvailable)
const getLink = vi.mocked(App.GetDesktopLink)
const clipboard = vi.mocked(ClipboardSetText)
const getPerms = vi.mocked(App.GetDesktopPermissions)
const getRepair = vi.mocked(App.GetDaemonRepair)
const openLoginItems = vi.mocked(App.OpenLoginItems)

const LINK = 'https://example.invalid/desktop/?cap=SECRETTOKEN'
const CONNECTED: TailscaleHealth = { connected: true, hasCerts: true, ip: '100.64.0.1', domain: 'my-mac.tailnet.test' }
const DISCONNECTED: TailscaleHealth = { connected: false, hasCerts: false, ip: '', domain: '' }

function status(over: Partial<App.DesktopStatusResult> = {}): App.DesktopStatusResult {
  return {
    alwaysAvailable: true, state: 'idle', viewers: 0, locked: false, approachingLapse: false,
    tailscale: 'ok', launchd: true, reason: '', surfaceKind: '', surfaceName: '', control: '', errorCode: '', ...over,
  }
}

let container: HTMLElement
let root: ReturnType<typeof createRoot>

async function mount(health: TailscaleHealth | null = CONNECTED) {
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  await act(async () => {
    root.render(React.createElement(SettingsSheet, { isOpen: true, onClose: vi.fn(), tailscaleHealth: health }))
  })
  await tick(0)
  await click(container.querySelector<HTMLElement>('#settings-tab-sharing')!)
  await tick(0)
}

async function tick(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms)
  })
}

async function click(el: HTMLElement) {
  await act(async () => {
    el.dispatchEvent(new MouseEvent('click', { bubbles: true }))
  })
}

const sharing = () => container.querySelector<HTMLElement>('#settings-pane-sharing')!
const desktopSwitch = () => container.querySelector<HTMLButtonElement>('#desktopAlways')!

function row(label: string): HTMLElement | null {
  return (
    Array.from(sharing().querySelectorAll<HTMLElement>('.settings-row')).find(
      (r) => r.querySelector('.settings-row__label')?.textContent === label,
    ) ?? null
  )
}

function statusText(): string | undefined {
  return row('Status')?.querySelector('[aria-live="polite"]')?.textContent ?? undefined
}

beforeEach(() => {
  vi.useFakeTimers()
  for (const m of [getStatus, setDesktop, getLink, clipboard, getPerms, getRepair, openLoginItems, vi.mocked(App.HasCTDisclosure)]) m.mockReset()
  getPerms.mockResolvedValue({ sr: true, ax: true, errorCode: '' })
  getRepair.mockResolvedValue('')
  openLoginItems.mockResolvedValue(undefined)
  getStatus.mockResolvedValue(status({ alwaysAvailable: false, state: 'off' }))
  setDesktop.mockResolvedValue({ alwaysAvailable: true, firstShare: false, errorCode: '', errorDetail: '' })
  getLink.mockResolvedValue({ url: LINK, errorCode: '' })
  clipboard.mockResolvedValue(true)
  vi.mocked(App.HasCTDisclosure).mockResolvedValue(true)
})

afterEach(() => {
  act(() => root?.unmount())
  container?.remove()
  vi.useRealTimers()
})

describe('DSHR-01 tracer: turn on, watch the state, copy the link', () => {
  it('turns on, shows Getting a link…, then the live state, then copies the link', async () => {
    await mount()
    expect(sharing().textContent).toContain('Always-available desktop')
    expect(desktopSwitch().getAttribute('aria-checked')).toBe('false')
    expect(row('Status')).toBeNull()

    // The daemon takes a moment: the switch is on and dimmed, row 2 reads Getting a link….
    let finish!: () => void
    setDesktop.mockImplementation(() => new Promise((res) => {
      finish = () => res({ alwaysAvailable: true, firstShare: false, errorCode: '', errorDetail: '' })
    }))
    getStatus.mockResolvedValue(status())
    await click(desktopSwitch())
    expect(setDesktop).toHaveBeenCalledWith(true)
    expect(desktopSwitch().getAttribute('aria-checked')).toBe('true')
    expect(desktopSwitch().getAttribute('aria-disabled')).toBe('true')
    expect(statusText()).toBe('Getting a link…')
    expect(row('Status')!.querySelector('.ts-status__dot')).toBeNull()
    expect(row('Link')!.querySelector('button')!.getAttribute('aria-disabled')).toBe('true')
    await act(async () => { finish() })
    await tick(0)

    expect(statusText()).toBe('Ready. Nobody is watching.')
    expect(row('Status')!.querySelector('.ts-status__dot--ok')).not.toBeNull()
    expect(row('Link')!.querySelector('button')!.getAttribute('aria-disabled')).toBe('false')

    // The next poll says two people are watching.
    getStatus.mockResolvedValue(status({ state: 'streaming', viewers: 2 }))
    await tick(2000)
    expect(statusText()).toBe('Streaming to 2 viewers')

    // Copy Link reads the link only now, writes it to the clipboard, and says Copied! for 2 s.
    expect(getLink).not.toHaveBeenCalled()
    const copy = row('Link')!.querySelector('button')!
    await click(copy)
    expect(getLink).toHaveBeenCalledTimes(1)
    expect(clipboard).toHaveBeenCalledWith(LINK)
    expect(copy.textContent).toBe('Copied!')
    await tick(1999)
    expect(copy.textContent).toBe('Copied!')
    await tick(1)
    expect(copy.textContent).toBe('Copy Link')

    // T-19-44: the link text never reaches the DOM.
    expect(container.innerHTML).not.toContain('SECRETTOKEN')
    expect(container.innerHTML).not.toContain('example.invalid')
  })
})

// ---- Task 2: every state, refusal and error -------------------------------

const OFF_DESCRIPTION =
  "View and control this Mac's screen from your other devices any time, even when Portty is quit. Only devices signed in to your Tailscale account can open it."
const USERSPACE_SENTENCE =
  "Tailscale is running in userspace-networking mode, so Portty can't open a desktop connection. Switch Tailscale to its standard mode, then try again."
const FOOTNOTE =
  'While it is plugged in, Portty keeps this Mac from sleeping on its own, though the display may still sleep. Closing the lid or choosing Sleep still puts it to sleep. On battery, including a low battery, it sleeps as usual. Nothing can wake it from here. macOS asks you to approve Screen Recording again about once a month, at the Mac. The first time Chrome opens the link, it may ask to find devices nearby. Choose Allow.'

const description = (label: string) => row(label)?.querySelector('.settings-row__description')?.textContent
const errorSlot = () => sharing().querySelector('[role="alert"]')

async function mountOn(over: Partial<App.DesktopStatusResult> = {}, health: TailscaleHealth | null = CONNECTED) {
  getStatus.mockResolvedValue(status(over))
  await mount(health)
}

describe('the wording table (desktopStatusText)', () => {
  const cases: Array<[Partial<App.DesktopStatusResult>, string, 'on' | 'warn' | 'off', string]> = [
    [{ state: 'needs_permission' }, 'Needs permission', 'warn', 'Desktop: Needs permission'],
    [{ state: 'awaiting_approval' }, 'Waiting for approval at the Mac', 'warn', 'Desktop: Waiting for approval at the Mac'],
    [{ state: 'display_asleep' }, 'Display asleep', 'off', 'Desktop: Display asleep'],
    [{ state: 'locked' }, 'Locked', 'off', 'Desktop: Locked'],
    [{ state: 'locked', viewers: 1 }, 'Locked, streaming to 1 viewer', 'off', 'Desktop: Locked, streaming to 1 viewer'],
    [{ state: 'locked', viewers: 3 }, 'Locked, streaming to 3 viewers', 'off', 'Desktop: Locked, streaming to 3 viewers'],
    [{ state: 'idle' }, 'Ready. Nobody is watching.', 'on', 'Desktop: Ready'],
    [{ state: 'streaming', viewers: 1 }, 'Streaming to 1 viewer', 'on', 'Desktop: Streaming to 1 viewer'],
    [{ state: 'streaming', viewers: 2 }, 'Streaming to 2 viewers', 'on', 'Desktop: Streaming to 2 viewers'],
    [{ state: 'capture_paused' }, 'Paused after repeated errors. Run Test Capture to resume.', 'warn', 'Desktop: Paused after repeated errors'],
  ]
  it.each(cases)('%j reads %s', (over, text, dot, tray) => {
    expect(desktopStatusText(status(over), false)).toEqual({ text, dot, tray })
  })

  it('Waiting for Tailscale wins over every state', () => {
    for (const state of ['needs_permission', 'awaiting_approval', 'display_asleep', 'locked', 'idle', 'streaming', 'capture_paused']) {
      expect(desktopStatusText(status({ state, viewers: 2 }), true)).toEqual({
        text: 'Waiting for Tailscale', dot: 'warn', tray: 'Desktop: Waiting for Tailscale',
      })
    }
  })

  it('no reply from the daemon is Checking… with no dot and no tray line', () => {
    expect(desktopStatusText(status({ errorCode: 'unavailable' }), false)).toEqual({ text: 'Checking…', dot: null, tray: null })
  })
})

describe('descriptions under the Status row (DPRM-03)', () => {
  it('awaiting_approval, needs_permission and approachingLapse add their lines; others add none', async () => {
    await mountOn({ state: 'awaiting_approval' })
    expect(description('Status')).toBe(
      "macOS is asking for approval on this Mac's screen. Someone at the Mac needs to click Allow, or approve Portty in System Settings.",
    )
    getStatus.mockResolvedValue(status({ state: 'needs_permission' }))
    await tick(2000)
    expect(description('Status')).toBe('Choose Set Up Permissions below to turn on Screen Recording.')
    getStatus.mockResolvedValue(status({ state: 'idle', approachingLapse: true }))
    await tick(2000)
    expect(description('Status')).toBe('macOS will ask for approval again soon. Someone needs to approve it at the Mac.')
    getStatus.mockResolvedValue(status({ state: 'streaming', viewers: 2 }))
    await tick(2000)
    expect(description('Status')).toBeUndefined()
  })
})

describe('refusals (Interaction Contract 2)', () => {
  it('Tailscale off blocks the switch while it is off, pointing at the reason, and never blocks turning off', async () => {
    getStatus.mockResolvedValue(status({ alwaysAvailable: false, state: 'off' }))
    await mount(DISCONNECTED)
    const sw = desktopSwitch()
    expect(sw.getAttribute('aria-disabled')).toBe('true')
    const reason = document.getElementById(sw.getAttribute('aria-describedby')!)!
    expect(reason.textContent).toBe(TAILSCALE_OFF_REASON)
    expect(row('Always-available desktop')!.contains(reason)).toBe(true)
    await click(sw)
    expect(setDesktop).not.toHaveBeenCalled()
  })

  it('Tailscale off with the desktop on keeps the switch enabled, and Status reads Waiting for Tailscale', async () => {
    await mountOn({ state: 'streaming', viewers: 2 }, DISCONNECTED)
    expect(desktopSwitch().getAttribute('aria-disabled')).toBeNull()
    expect(statusText()).toBe('Waiting for Tailscale')
    expect(row('Status')!.querySelector('.ts-status__dot--warn')).not.toBeNull()
    expect(row('Link')!.querySelector('button')!.getAttribute('aria-disabled')).toBe('true')
    setDesktop.mockResolvedValue({ alwaysAvailable: false, firstShare: false, errorCode: '', errorDetail: '' })
    getStatus.mockResolvedValue(status({ alwaysAvailable: false, state: 'off' }))
    await click(desktopSwitch())
    expect(setDesktop).toHaveBeenCalledWith(false)
  })

  it('userspace networking blocks the switch with the userspace sentence', async () => {
    await mountOn({ alwaysAvailable: false, state: 'off', tailscale: 'userspace' })
    expect(desktopSwitch().getAttribute('aria-disabled')).toBe('true')
    expect(description('Always-available desktop')).toBe(USERSPACE_SENTENCE)
    expect(document.getElementById(desktopSwitch().getAttribute('aria-describedby')!)!.textContent).toBe(USERSPACE_SENTENCE)
    await click(desktopSwitch())
    expect(setDesktop).not.toHaveBeenCalled()
  })

  it('a userspace refusal from the daemon blocks the switch at once', async () => {
    await mountOn({ alwaysAvailable: false, state: 'off' })
    setDesktop.mockResolvedValue({ alwaysAvailable: false, firstShare: false, errorCode: 'tailscale_userspace', errorDetail: '' })
    // The daemon records the verdict, so its next status says userspace.
    getStatus.mockResolvedValue(status({ alwaysAvailable: false, state: 'off', tailscale: 'userspace' }))
    await click(desktopSwitch())
    expect(desktopSwitch().getAttribute('aria-checked')).toBe('false')
    expect(description('Always-available desktop')).toBe(USERSPACE_SENTENCE)
  })
})

describe('inline errors', () => {
  it('https_off returns the switch to off with the exact HTTPS error', async () => {
    await mountOn({ alwaysAvailable: false, state: 'off' })
    setDesktop.mockResolvedValue({ alwaysAvailable: false, firstShare: false, errorCode: 'https_off', errorDetail: 'x' })
    await click(desktopSwitch())
    expect(desktopSwitch().getAttribute('aria-checked')).toBe('false')
    expect(row('Status')).toBeNull()
    expect(errorSlot()!.textContent).toBe(
      "Couldn't turn on desktop sharing: HTTPS certificates are off for your tailnet. Turn on HTTPS on the DNS page of the Tailscale admin console, then try again.",
    )
  })

  it('a start failure shows its detail and returns to off', async () => {
    await mountOn({ alwaysAvailable: false, state: 'off' })
    setDesktop.mockResolvedValue({ alwaysAvailable: false, firstShare: false, errorCode: 'start_failed', errorDetail: 'Error: port 4000 is busy.' })
    await click(desktopSwitch())
    expect(desktopSwitch().getAttribute('aria-checked')).toBe('false')
    expect(errorSlot()!.textContent).toBe(
      "Couldn't turn on desktop sharing: port 4000 is busy. Check that Tailscale is connected, then try again.",
    )
  })

  it('a failed turn-off returns the switch to on with its error; the next try clears it', async () => {
    await mountOn({ state: 'idle' })
    setDesktop.mockResolvedValue({ alwaysAvailable: true, firstShare: false, errorCode: 'unavailable', errorDetail: 'daemon not connected' })
    await click(desktopSwitch())
    expect(setDesktop).toHaveBeenCalledWith(false)
    expect(desktopSwitch().getAttribute('aria-checked')).toBe('true')
    expect(row('Status')).not.toBeNull()
    expect(errorSlot()!.textContent).toBe("Couldn't turn off desktop sharing: daemon not connected. Try again.")
    setDesktop.mockResolvedValue({ alwaysAvailable: false, firstShare: false, errorCode: '', errorDetail: '' })
    getStatus.mockResolvedValue(status({ alwaysAvailable: false, state: 'off' }))
    await click(desktopSwitch())
    expect(errorSlot()).toBeNull()
  })

  it('a clipboard failure says so', async () => {
    await mountOn({ state: 'idle' })
    clipboard.mockRejectedValue(new Error('denied'))
    await click(row('Link')!.querySelector('button')!)
    expect(errorSlot()!.textContent).toBe("Couldn't copy the link. Try again.")
    expect(row('Link')!.querySelector('button')!.textContent).toBe('Copy Link')
  })

  it('a link the daemon cannot give says so and writes nothing to the clipboard', async () => {
    await mountOn({ state: 'idle' })
    getLink.mockResolvedValue({ url: '', errorCode: 'unavailable' })
    await click(row('Link')!.querySelector('button')!)
    expect(clipboard).not.toHaveBeenCalled()
    expect(errorSlot()!.textContent).toBe("Couldn't copy the link. Try again.")
  })
})

describe('Certificate Transparency line', () => {
  const CT = 'The first time, this gets an HTTPS certificate for my-mac.tailnet.test. That name is published in public Certificate Transparency logs.'

  it('shows under the off description until acknowledged, and is read again after a successful turn-on', async () => {
    vi.mocked(App.HasCTDisclosure).mockResolvedValue(false)
    await mountOn({ alwaysAvailable: false, state: 'off' })
    expect(description('Always-available desktop')).toBe(OFF_DESCRIPTION + CT)
    const reads = vi.mocked(App.HasCTDisclosure).mock.calls.length
    vi.mocked(App.HasCTDisclosure).mockResolvedValue(true)
    setDesktop.mockResolvedValue({ alwaysAvailable: true, firstShare: true, errorCode: '', errorDetail: '' })
    getStatus.mockResolvedValue(status())
    await click(desktopSwitch())
    expect(vi.mocked(App.HasCTDisclosure).mock.calls.length).toBeGreaterThan(reads)
    expect(App.AcknowledgeCTDisclosure).not.toHaveBeenCalled() // the daemon records it on the turn-on
    setDesktop.mockResolvedValue({ alwaysAvailable: false, firstShare: false, errorCode: '', errorDetail: '' })
    getStatus.mockResolvedValue(status({ alwaysAvailable: false, state: 'off' }))
    await click(desktopSwitch()) // off again
    expect(description('Always-available desktop')).toBe(OFF_DESCRIPTION)
  })

  it('is absent once acknowledged', async () => {
    await mountOn({ alwaysAvailable: false, state: 'off' })
    expect(description('Always-available desktop')).toBe(OFF_DESCRIPTION)
  })
})

describe('Permissions row (DPRM-01)', () => {
  it('reads Granted with Review Permissions… when both are granted', async () => {
    await mountOn()
    const r = row('Permissions')!
    expect(r.querySelector('.settings-row__description')!.textContent).toBe('Screen Recording and Accessibility.')
    expect(r.querySelector('.ts-status__text')!.textContent).toBe('Granted')
    expect(r.querySelector('.ts-status__text--granted')).not.toBeNull()
    expect(r.querySelector('.ts-status__dot--ok')).not.toBeNull()
    expect(r.querySelector('button')!.textContent).toBe('Review Permissions…')
  })

  it('reads Needs permission with Set Up Permissions… when either is missing, also with the desktop off', async () => {
    getPerms.mockResolvedValue({ sr: true, ax: false, errorCode: '' })
    await mountOn({ alwaysAvailable: false, state: 'off' })
    const r = row('Permissions')!
    expect(r.querySelector('.ts-status__text')!.textContent).toBe('Needs permission')
    expect(r.querySelector('.ts-status__dot--warn')).not.toBeNull()
    expect(r.querySelector('button')!.textContent).toBe('Set Up Permissions…')
  })

  it('an unknown read stays Checking… with no dot', async () => {
    getPerms.mockResolvedValue({ sr: false, ax: false, errorCode: 'unavailable' })
    await mountOn()
    const r = row('Permissions')!
    expect(r.querySelector('.ts-status__text')!.textContent).toBe('Checking…')
    expect(r.querySelector('.ts-status__dot')).toBeNull()
    expect(r.querySelector('button')!.textContent).toBe('Set Up Permissions…')
  })

  it('the button opens the setup sheet, and closing it reads the grants again', async () => {
    await mountOn()
    const reads = getPerms.mock.calls.length
    const button = row('Permissions')!.querySelector('button')!
    await click(button)
    const dialogs = document.querySelectorAll('[role="dialog"]')
    expect(dialogs).toHaveLength(2)
    expect(dialogs[1].textContent).toContain('Set Up Desktop Sharing')
    // The setup sheet is after the settings sheet in the DOM, so it is the topmost modal.
    expect(dialogs[0].compareDocumentPosition(dialogs[1]) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    getPerms.mockResolvedValue({ sr: false, ax: true, errorCode: '' })
    const done = Array.from(dialogs[1].querySelectorAll('button')).find((b) => b.textContent === 'Done')!
    await click(done)
    await tick(0)
    expect(document.querySelectorAll('[role="dialog"]')).toHaveLength(1)
    expect(getPerms.mock.calls.length).toBeGreaterThan(reads + 1)
    expect(row('Permissions')!.querySelector('.ts-status__text')!.textContent).toBe('Needs permission')
  })
})

// 20-UI-SPEC Surface E: the host sees what is shared, whether control works, and
// consent copy that says the link can control the Mac (T-20-47).
describe('Showing row (20-10)', () => {
  const showing = () => row('Showing')?.querySelector('[aria-live="polite"]')

  it.each([
    [{ surfaceKind: 'display', surfaceName: 'Main Display' }, 'Main Display'],
    [{ surfaceKind: 'display', surfaceName: 'Display 2' }, 'Display 2'],
    [{ surfaceKind: 'window', surfaceName: 'Terminal — zsh' }, 'Window: Terminal — zsh'],
  ])('%j reads %s directly under Status, in an aria-live region', async (over, text) => {
    await mountOn({ state: 'streaming', viewers: 1, control: 'ok', ...over })
    expect(showing()!.textContent).toBe(text)
    expect(showing()!.className).toContain('settings-row__description')
    const labels = Array.from(sharing().querySelectorAll('.settings-row__label')).map((l) => l.textContent)
    expect(labels.indexOf('Showing')).toBe(labels.indexOf('Status') + 1)
  })

  it('is absent while nobody is watching', async () => {
    await mountOn({ state: 'idle', viewers: 0 })
    expect(row('Showing')).toBeNull()
  })

  it('appears with the first viewer and goes with the last', async () => {
    await mountOn({ state: 'idle' })
    getStatus.mockResolvedValue(status({ state: 'streaming', viewers: 1, surfaceKind: 'window', surfaceName: 'Notes' }))
    await tick(2000)
    expect(showing()!.textContent).toBe('Window: Notes')
    getStatus.mockResolvedValue(status({ state: 'idle' }))
    await tick(2000)
    expect(row('Showing')).toBeNull()
  })
})

describe('Permissions row and control (20-10)', () => {
  it('control needs_accessibility turns the row warn with the Set Up Permissions… copy', async () => {
    await mountOn({ state: 'streaming', viewers: 1, control: 'needs_accessibility' })
    const r = row('Permissions')!
    expect(r.querySelector('.settings-row__description')!.textContent).toBe('Control needs Accessibility. Choose Set Up Permissions…')
    expect(r.querySelector('.ts-status__text')!.textContent).toBe('Needs permission')
    expect(r.querySelector('.ts-status__dot--warn')).not.toBeNull()
    expect(r.querySelector('button')!.textContent).toBe('Set Up Permissions…')
  })

  it('returns to Granted and Review Permissions… when control works again', async () => {
    await mountOn({ state: 'streaming', viewers: 1, control: 'needs_accessibility' })
    getStatus.mockResolvedValue(status({ state: 'streaming', viewers: 1, control: 'ok' }))
    await tick(2000)
    const r = row('Permissions')!
    expect(r.querySelector('.settings-row__description')!.textContent).toBe('Screen Recording and Accessibility.')
    expect(r.querySelector('.ts-status__text')!.textContent).toBe('Granted')
    expect(r.querySelector('button')!.textContent).toBe('Review Permissions…')
  })
})

describe('consent copy says the link can control the Mac (T-20-47)', () => {
  it('Row 3 reads the new sentence', async () => {
    await mountOn({ state: 'idle' })
    expect(description('Link')).toBe(
      'Open it in Chrome or Safari on a device signed in to your Tailscale account. It can control this Mac, so treat it like a password.',
    )
  })
})

describe('turn-on with a permission missing', () => {
  it('keeps the switch on, opens the setup sheet at once, and gives focus back to the switch', async () => {
    getPerms.mockResolvedValue({ sr: false, ax: true, errorCode: '' })
    await mountOn({ alwaysAvailable: false, state: 'off' })
    setDesktop.mockResolvedValue({ alwaysAvailable: true, firstShare: false, errorCode: '', errorDetail: '' })
    getStatus.mockResolvedValue(status({ state: 'needs_permission' }))
    await click(desktopSwitch())
    await tick(0)
    const dialogs = document.querySelectorAll('[role="dialog"]')
    expect(dialogs).toHaveLength(2)
    expect(dialogs[1].textContent).toContain('Set Up Desktop Sharing')
    const done = Array.from(dialogs[1].querySelectorAll('button')).find((b) => b.textContent === 'Done')!
    await click(done)
    await tick(0)
    expect(document.querySelectorAll('[role="dialog"]')).toHaveLength(1)
    expect(document.activeElement).toBe(desktopSwitch())
    expect(desktopSwitch().getAttribute('aria-checked')).toBe('true')
    expect(statusText()).toBe('Needs permission')
  })

  it('does not open the sheet when both are granted', async () => {
    await mountOn({ alwaysAvailable: false, state: 'off' })
    getStatus.mockResolvedValue(status())
    await click(desktopSwitch())
    await tick(0)
    expect(document.querySelectorAll('[role="dialog"]')).toHaveLength(1)
  })
})

describe('no confirm dialog', () => {
  it('opens none on either change, and turning off removes rows 2 and 3 at once', async () => {
    await mountOn({ alwaysAvailable: false, state: 'off' })
    getStatus.mockResolvedValue(status())
    await click(desktopSwitch())
    expect(document.querySelectorAll('[role="dialog"], [role="alertdialog"]')).toHaveLength(1)
    expect(row('Status')).not.toBeNull()
    expect(row('Link')).not.toBeNull()
    expect(description('Always-available desktop')).toBe('Turning this off stops every saved desktop link.')

    let finish!: () => void
    setDesktop.mockImplementation(() => new Promise((res) => {
      finish = () => res({ alwaysAvailable: false, firstShare: false, errorCode: '', errorDetail: '' })
    }))
    await click(desktopSwitch())
    expect(document.querySelectorAll('[role="dialog"], [role="alertdialog"]')).toHaveLength(1)
    expect(desktopSwitch().getAttribute('aria-checked')).toBe('false')
    expect(desktopSwitch().getAttribute('aria-disabled')).toBe('true')
    expect(row('Status')).toBeNull()
    expect(row('Link')).toBeNull()
    getStatus.mockResolvedValue(status({ alwaysAvailable: false, state: 'off' }))
    await act(async () => { finish() })
    await tick(0)
    expect(desktopSwitch().getAttribute('aria-disabled')).toBeNull()
  })
})

describe('Background service row (USER DECISION 2026-11-25)', () => {
  const LOGIN_ITEMS =
    "Portty's background service is turned off in Login Items, so this Mac's desktop is only reachable while Portty is open."
  const kinds: Array<[string, string]> = [
    ['move_to_applications', "Portty is running from a disk image, so this Mac's desktop is only reachable while Portty is open. Move Portty to Applications, then open it again."],
    ['open_app', "Portty's background service isn't installed, so this Mac's desktop is only reachable while Portty is open. Open Portty.app from Applications once to install it."],
    ['rehome_pending', "Portty is still moving its background service under macOS's control. It keeps trying."],
    ['', "Portty's background service isn't running under macOS's control. Quit and reopen Portty to try again."],
  ]

  it('login_items shows Needs repair, its description and an Open Login Items button', async () => {
    getRepair.mockResolvedValue('login_items')
    await mountOn({ launchd: false })
    const r = row('Background service')!
    expect(r.querySelector('.ts-status__text')!.textContent).toBe('Needs repair')
    expect(r.querySelector('.ts-status__dot--warn')).not.toBeNull()
    expect(r.querySelector('.settings-row__description')!.textContent).toBe(LOGIN_ITEMS)
    expect(r.querySelector('button')!.textContent).toBe('Open Login Items')
    await click(r.querySelector('button')!)
    expect(openLoginItems).toHaveBeenCalledTimes(1)
  })

  it.each(kinds)('%j shows its description and no button', async (kind, text) => {
    getRepair.mockResolvedValue(kind)
    await mountOn({ launchd: false })
    const r = row('Background service')!
    expect(r.querySelector('.settings-row__description')!.textContent).toBe(text)
    expect(r.querySelector('button')).toBeNull()
  })

  it('shows no row while launchd runs the daemon or the desktop is off', async () => {
    getRepair.mockResolvedValue('login_items')
    await mountOn({ launchd: true })
    expect(row('Background service')).toBeNull()
    getStatus.mockResolvedValue(status({ alwaysAvailable: false, state: 'off', launchd: false }))
    await tick(2000)
    expect(row('Background service')).toBeNull()
    getStatus.mockResolvedValue(status({ launchd: false }))
    await tick(2000)
    expect(row('Background service')).not.toBeNull()
    getStatus.mockResolvedValue(status({ launchd: true }))
    await tick(2000)
    expect(row('Background service')).toBeNull()
  })
})

describe('footnote', () => {
  it('renders the 19-UI-SPEC footnote under the Desktop group', async () => {
    await mountOn()
    const notes = sharing().querySelectorAll('.settings-sheet__footnote')
    expect(notes[notes.length - 1].textContent!.trim()).toBe(FOOTNOTE)
    expect(notes[notes.length - 1].previousElementSibling!.classList.contains('settings-group')).toBe(true)
  })
})

describe('DSHR-01 prohibition: the link is never displayed', () => {
  it('shows no field, picture or canvas in the Desktop group (Copy Link only)', async () => {
    await mountOn()
    const group = row('Link')!.closest('.settings-group')!
    expect(group.querySelector('input, textarea, img, canvas, svg, [contenteditable]')).toBeNull()
  })
})
