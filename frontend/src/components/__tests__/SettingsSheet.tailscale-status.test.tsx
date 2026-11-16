/**
 * NET-01 (Portty 01-12, 02-11) — Settings ▸ Sharing Tailscale status and the
 * web server row. Tailscale is "Connected as <host>" or "Not connected" (with
 * the NET-01 sentence); "Checking…" while the first check is pending. A
 * blocked Start Server keeps its name and focus, is aria-disabled, and points
 * aria-describedby at the on-screen reason (UI-REVIEW 01 #2/#3: never
 * tooltip-only).
 */
import { describe, it, expect, afterEach, vi, beforeEach } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { act } from 'react'
import { SettingsSheet } from '../SettingsSheet'
import { TAILSCALE_OFF_REASON, type TailscaleHealth } from '../../lib/tailscale'
import * as AppBindings from '../../wailsjs/go/main/App'

vi.mock('../../wailsjs/go/main/App', () => ({
  StartWebServer: vi.fn().mockResolvedValue(undefined),
  StopWebServer: vi.fn().mockResolvedValue(undefined),
  GetWebServerURL: vi.fn().mockResolvedValue('https://example.ts.net'),
  IsWebServerRunning: vi.fn().mockResolvedValue(false),
  HasCTDisclosure: vi.fn().mockResolvedValue(true),
  AcknowledgeCTDisclosure: vi.fn().mockResolvedValue(undefined),
  GetLocalNetworkPassword: vi.fn().mockResolvedValue(''),
  GetWebServerQRCode: vi.fn().mockResolvedValue(''),
  GetStartMinimized: vi.fn().mockResolvedValue(false),
  SetStartMinimized: vi.fn().mockResolvedValue(undefined),
  GetAppearance: vi.fn().mockResolvedValue('system'),
  SetAppearance: vi.fn().mockResolvedValue(undefined),
  GetTerminalFont: vi.fn().mockResolvedValue({ family: '', size: 13 }),
  SetTerminalFont: vi.fn().mockResolvedValue(undefined),
  RegenerateSigningKey: vi.fn().mockResolvedValue(undefined),
  GetShellWebShareWarningEnabled: vi.fn().mockResolvedValue(true),
  SetShellWebShareWarningEnabled: vi.fn().mockResolvedValue(undefined),
}))

vi.mock('../../wailsjs/wailsjs/runtime/runtime', () => ({
  BrowserOpenURL: vi.fn(),
  ClipboardSetText: vi.fn().mockResolvedValue(undefined),
}))

vi.mock('../PluginsSection', () => ({ PluginsSection: () => null }))

function makeHealth(overrides: Partial<TailscaleHealth> = {}): TailscaleHealth {
  return { connected: false, hasCerts: false, ip: '', domain: '', ...overrides }
}
const CONNECTED = makeHealth({ connected: true, hasCerts: true, ip: '100.64.0.1', domain: 'my-node.tail1234.ts.net' })
const CT_SENTENCE = 'Acknowledge Certificate Transparency in Security first.'
const REMOVED_STATES = ['Permission Limited', 'Daemon Stopped', 'Not Installed', 'Show diagnostics']

let container: HTMLElement
let root: ReturnType<typeof createRoot>

async function renderSheet(
  tailscaleHealth: TailscaleHealth | null,
  webServerMode: 'tailscale' | 'local' | null = null,
) {
  const onWebServerStateChange = vi.fn().mockResolvedValue(undefined)
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  await act(async () => {
    root.render(React.createElement(SettingsSheet, {
      isOpen: true,
      onClose: vi.fn(),
      tailscaleHealth,
      webServerMode,
      onWebServerStateChange,
    }))
  })
  await act(async () => { await new Promise<void>((r) => setTimeout(r, 0)) })
  return { onWebServerStateChange }
}

function sharing(): HTMLElement {
  return container.querySelector<HTMLElement>('#settings-pane-sharing')!
}

function tailscaleRow(): HTMLElement {
  return container.querySelector('.ts-status')!.closest('.settings-row') as HTMLElement
}

function serverButton(): HTMLButtonElement {
  const btn = Array.from(sharing().querySelectorAll('button')).find((b) => /^(Start|Stop) Server$/.test(b.textContent ?? ''))
  expect(btn).toBeDefined()
  return btn as HTMLButtonElement
}

function reasonText(btn: HTMLElement): string {
  const id = btn.getAttribute('aria-describedby')
  expect(id).toBeTruthy()
  return container.querySelector(`#${id}`)!.textContent ?? ''
}

beforeEach(() => {
  vi.mocked(AppBindings.HasCTDisclosure).mockResolvedValue(true)
  vi.mocked(AppBindings.IsWebServerRunning).mockResolvedValue(false)
  vi.mocked(AppBindings.StartWebServer).mockResolvedValue(undefined)
})

afterEach(async () => {
  await act(async () => root.unmount())
  container.remove()
  vi.clearAllMocks()
})

describe('NET-01: Sharing pane Tailscale status', () => {
  it('shows "Connected as <host>" with the on dot', async () => {
    await renderSheet(CONNECTED)
    expect(container.querySelector('.ts-status__text')!.textContent).toBe('Connected as my-node')
    expect(container.querySelector('.ts-status__dot')!.className).toContain('ts-status__dot--ok')
    expect(tailscaleRow().textContent).not.toContain(TAILSCALE_OFF_REASON)
    for (const label of REMOVED_STATES) expect(container.textContent).not.toContain(label)
  })

  it('falls back to the IP when there is no domain', async () => {
    await renderSheet(makeHealth({ connected: true, ip: '100.64.0.1' }))
    expect(container.querySelector('.ts-status__text')!.textContent).toBe('Connected as 100.64.0.1')
  })

  it('shows Not connected with the off dot and the NET-01 sentence', async () => {
    await renderSheet(makeHealth())
    expect(container.querySelector('.ts-status__text')!.textContent).toBe('Not connected')
    expect(container.querySelector('.ts-status__dot')!.className).toContain('ts-status__dot--error')
    expect(tailscaleRow().textContent).toContain(TAILSCALE_OFF_REASON)
    for (const label of REMOVED_STATES) expect(container.textContent).not.toContain(label)
  })

  it('shows Checking… with no dot while health is null', async () => {
    await renderSheet(null)
    expect(container.querySelector('.ts-status__text')!.textContent).toBe('Checking…')
    expect(container.querySelector('.ts-status__dot')).toBeNull()
  })
})

describe('NET-01 / Amendment 3 / T-02-34: a blocked Start Server explains itself on screen', () => {
  it('Tailscale off: aria-disabled, described by the NET-01 sentence, keeps its name, never starts', async () => {
    await renderSheet(makeHealth())
    const btn = serverButton()
    expect(btn.textContent).toBe('Start Server')
    expect(btn.getAttribute('aria-disabled')).toBe('true')
    expect(btn.disabled).toBe(false)
    expect(btn.hasAttribute('title')).toBe(false)
    expect(reasonText(btn)).toBe(TAILSCALE_OFF_REASON)
    await act(async () => btn.click())
    expect(AppBindings.StartWebServer).not.toHaveBeenCalled()
  })

  it('CT not acknowledged: described by the CT sentence and never starts', async () => {
    vi.mocked(AppBindings.HasCTDisclosure).mockResolvedValue(false)
    await renderSheet(CONNECTED)
    const btn = serverButton()
    expect(btn.getAttribute('aria-disabled')).toBe('true')
    expect(reasonText(btn)).toBe(CT_SENTENCE)
    await act(async () => btn.click())
    expect(AppBindings.StartWebServer).not.toHaveBeenCalled()
  })

  it('unblocked: no aria-disabled or describedby, the description reads Stopped, and a click starts', async () => {
    const { onWebServerStateChange } = await renderSheet(CONNECTED)
    const btn = serverButton()
    expect(btn.hasAttribute('aria-disabled')).toBe(false)
    expect(btn.hasAttribute('aria-describedby')).toBe(false)
    expect(container.querySelector('#web-server-reason')!.textContent).toBe('Stopped')
    await act(async () => btn.click())
    await act(async () => { await new Promise<void>((r) => setTimeout(r, 0)) })
    expect(AppBindings.StartWebServer).toHaveBeenCalledWith(7444)
    expect(onWebServerStateChange).toHaveBeenCalled()
    expect(container.querySelector('#web-server-reason')!.textContent).toBe('Running on https://example.ts.net')
    expect(serverButton().textContent).toBe('Stop Server')
  })

  it('a start failure shows the UI-SPEC copy in the error style and the sheet stays open', async () => {
    vi.mocked(AppBindings.StartWebServer).mockRejectedValue(new Error('listen tcp :7444: bind: address already in use'))
    await renderSheet(CONNECTED)
    await act(async () => serverButton().click())
    await act(async () => { await new Promise<void>((r) => setTimeout(r, 0)) })
    const reason = container.querySelector('#web-server-reason')!
    expect(reason.textContent).toBe(
      "Couldn't start the web server: listen tcp :7444: bind: address already in use. Check that port 7444 isn't in use, then try again.",
    )
    expect(reason.querySelector('.settings-row__error')).not.toBeNull()
    expect(container.querySelector('[role="dialog"]')).not.toBeNull()
  })

  it('no element carries a title tooltip, and the footnote points at the toolbar Share button', async () => {
    await renderSheet(makeHealth())
    expect(container.querySelector('[title]')).toBeNull()
    expect(sharing().textContent).toContain("Share a session with the Share button in the main window's toolbar.")
  })
})

describe('NET-01 (quick 261115-rc8): the running Web server says when it is on the local network', () => {
  const LAN_URL = 'https://192.168.64.108:7444'

  it('LAN mode with Tailscale off names the local network and says Tailscale sharing is off', async () => {
    vi.mocked(AppBindings.IsWebServerRunning).mockResolvedValueOnce(true)
    vi.mocked(AppBindings.GetWebServerURL).mockResolvedValueOnce(LAN_URL)
    await renderSheet(makeHealth(), 'local')
    expect(container.querySelector('#web-server-reason')!.textContent).toBe(
      `Running on your local network at ${LAN_URL}. Tailscale sharing is off.`,
    )
  })

  it('LAN mode with Tailscale connected names the local network only', async () => {
    vi.mocked(AppBindings.IsWebServerRunning).mockResolvedValueOnce(true)
    vi.mocked(AppBindings.GetWebServerURL).mockResolvedValueOnce(LAN_URL)
    await renderSheet(CONNECTED, 'local')
    expect(container.querySelector('#web-server-reason')!.textContent).toBe(
      `Running on your local network at ${LAN_URL}.`,
    )
  })
})
