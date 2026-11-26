/**
 * Settings ▸ Sharing, Desktop group (19-13: DSHR-01, DPRM-01/02/03; 19-UI-SPEC
 * Surface 1). The App bindings and the Wails clipboard are mocked; nothing here
 * talks to a daemon, reads a permission or touches a clipboard.
 */
import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { act } from 'react'
import { SettingsSheet } from '../SettingsSheet'
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

const LINK = 'https://example.invalid/desktop/?cap=SECRETTOKEN'
const CONNECTED: TailscaleHealth = { connected: true, hasCerts: true, ip: '100.64.0.1', domain: 'my-mac.example.ts.net' }

function status(over: Partial<App.DesktopStatusResult> = {}): App.DesktopStatusResult {
  return {
    alwaysAvailable: true, state: 'idle', viewers: 0, locked: false, approachingLapse: false,
    tailscale: 'ok', launchd: true, reason: '', errorCode: '', ...over,
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
  for (const m of [getStatus, setDesktop, getLink, clipboard, vi.mocked(App.HasCTDisclosure)]) m.mockReset()
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
