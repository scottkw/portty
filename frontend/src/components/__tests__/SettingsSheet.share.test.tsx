/**
 * Settings ▸ Sharing Row 6 (Share Desktop) and the Security pane's Session keys
 * sentence (22-10, 22-UI-SPEC Surface 1 and Surface 2). The bindings are mocked;
 * nothing here starts a share.
 */
import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { act } from 'react'
import { SettingsSheet } from '../SettingsSheet'
import * as copy from '../../lib/shareDesktopCopy'
import { TAILSCALE_OFF_REASON, type TailscaleHealth } from '../../lib/tailscale'
import * as App from '../../wailsjs/go/main/App'

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

const CONNECTED: TailscaleHealth = { connected: true, hasCerts: true, ip: '100.64.0.1', domain: 'my-mac.tailnet.ts.net' }
const DISCONNECTED: TailscaleHealth = { connected: false, hasCerts: false, ip: '', domain: '' }

let container: HTMLElement
let root: ReturnType<typeof createRoot>
const onOpen = vi.fn()

interface ShareProp { active: boolean; viewers: number; onOpen: () => void }

async function mount(opts: { health?: TailscaleHealth | null; share?: ShareProp; omitShare?: boolean; pane?: 'sharing' | 'security' } = {}): Promise<void> {
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  const share = opts.omitShare ? undefined : (opts.share ?? { active: false, viewers: 0, onOpen })
  await act(async () => {
    root.render(React.createElement(SettingsSheet, {
      isOpen: true, onClose: vi.fn(), tailscaleHealth: opts.health === undefined ? CONNECTED : opts.health, share,
    }))
  })
  await tick(0)
  await click(container.querySelector<HTMLElement>(`#settings-tab-${opts.pane ?? 'sharing'}`)!)
  await tick(0)
}

async function tick(ms: number): Promise<void> {
  await act(async () => { await vi.advanceTimersByTimeAsync(ms) })
}
async function click(el: Element): Promise<void> {
  await act(async () => { el.dispatchEvent(new MouseEvent('click', { bubbles: true })) })
}

const group = (): HTMLElement => container.querySelector<HTMLElement>('#settings-pane-sharing .settings-group:last-of-type')!
function row6(): HTMLElement | null {
  return Array.from(container.querySelectorAll<HTMLElement>('#settings-pane-sharing .settings-row')).find(
    (r) => r.querySelector('.settings-row__label')?.textContent === copy.SETTINGS_ROW_TITLE,
  ) ?? null
}
const buttons = (r: HTMLElement): string[] => Array.from(r.querySelectorAll('button')).map((b) => b.textContent ?? '')

beforeEach(() => {
  vi.useFakeTimers()
  vi.mocked(App.HasCTDisclosure).mockResolvedValue(true)
  vi.mocked(App.GetDesktopStatus).mockResolvedValue({
    alwaysAvailable: false, state: 'off', viewers: 0, locked: false, approachingLapse: false,
    tailscale: 'ok', launchd: true, reason: '', surfaceKind: '', surfaceName: '', control: '', errorCode: '',
  })
  vi.mocked(App.GetDesktopPermissions).mockResolvedValue({ sr: true, ax: true, errorCode: '' })
  vi.mocked(App.GetDaemonRepair).mockResolvedValue('')
})

afterEach(() => {
  act(() => root?.unmount())
  container?.remove()
  vi.clearAllMocks()
  vi.useRealTimers()
})

describe('Settings Row 6: Share Desktop', () => {
  it('is always visible, the last row of the Desktop group, even with always-available off', async () => {
    await mount()
    const r = row6()!
    expect(r).not.toBeNull()
    expect(group().lastElementChild).toBe(r)
    expect(r.querySelector('.settings-row__description')!.textContent).toContain(copy.SETTINGS_ROW_DESCRIPTION)
  })

  it('Share Desktop… opens the sheet', async () => {
    await mount()
    expect(buttons(row6()!)).toEqual([copy.SHARE_DESKTOP_ACTION])
    await click(row6()!.querySelector('button')!)
    expect(onOpen).toHaveBeenCalledTimes(1)
  })

  it('shows the Certificate Transparency line until it is acknowledged', async () => {
    vi.mocked(App.HasCTDisclosure).mockResolvedValue(false)
    await mount()
    expect(row6()!.textContent).toContain(copy.SHARE_CT_LINE('my-mac.tailnet'))
  })

  it('has no CT line once acknowledged', async () => {
    await mount()
    expect(row6()!.textContent).not.toContain('Certificate Transparency')
  })

  it('with Tailscale off the button is aria-disabled and described by the reason, and does not open', async () => {
    await mount({ health: DISCONNECTED })
    const r = row6()!
    const b = r.querySelector('button')!
    expect(b.getAttribute('aria-disabled')).toBe('true')
    expect(document.getElementById(b.getAttribute('aria-describedby')!)!.textContent).toBe(TAILSCALE_OFF_REASON)
    expect(r.textContent).not.toContain(copy.SETTINGS_ROW_DESCRIPTION)
    await click(b)
    expect(onOpen).not.toHaveBeenCalled()
  })

  it.each([
    [0, 'Shared with nobody yet'],
    [1, 'Shared with 1 person'],
    [3, 'Shared with 3 people'],
  ])('while sharing with %i viewers it shows the status %s and Manage Sharing…', async (viewers, words) => {
    await mount({ share: { active: true, viewers, onOpen } })
    const r = row6()!
    expect(r.querySelector('.ts-status__text')!.textContent).toBe(words)
    expect(r.querySelector('.ts-status__dot--ok')).not.toBeNull()
    expect(buttons(r)).toEqual([copy.MANAGE_SHARING_ACTION])
    expect(r.querySelector('.settings-row__description')).toBeNull()
    await click(r.querySelector('button')!)
    expect(onOpen).toHaveBeenCalledTimes(1)
  })

  it('Manage Sharing… still opens with Tailscale off: opening is never blocked while sharing', async () => {
    await mount({ health: DISCONNECTED, share: { active: true, viewers: 2, onOpen } })
    const b = row6()!.querySelector('button')!
    expect(b.getAttribute('aria-disabled')).not.toBe('true')
    await click(b)
    expect(onOpen).toHaveBeenCalledTimes(1)
  })

  it('without a share prop the row still renders as Not sharing', async () => {
    await mount({ omitShare: true })
    expect(buttons(row6()!)).toEqual([copy.SHARE_DESKTOP_ACTION])
  })
})

describe('Security pane', () => {
  it('shows the Session keys sentence that ends any desktop sharing', async () => {
    await mount({ pane: 'security' })
    const security = container.querySelector('#settings-pane-security')!.textContent!
    expect(security).toContain(copy.SECURITY_KEYS_DESCRIPTION)
    expect(security).toContain('stops every saved desktop link, and ends any desktop sharing.')
  })
})
