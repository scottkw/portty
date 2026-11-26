/**
 * NET-01 (Portty 01-12, 02-11) — Settings ▸ Sharing Tailscale status.
 * Tailscale is "Connected as <host>" or "Not connected" (with the NET-01
 * sentence); "Checking…" while the first check is pending. Portty 04-05: the
 * pane is that status plus one explanation; the server row and its controls are gone.
 */
import { describe, it, expect, afterEach, vi, beforeEach } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { act } from 'react'
import { SettingsSheet } from '../SettingsSheet'
import { TAILSCALE_OFF_REASON, type TailscaleHealth } from '../../lib/tailscale'
import * as AppBindings from '../../wailsjs/go/main/App'

vi.mock('../../wailsjs/go/main/App', () => ({
  HasCTDisclosure: vi.fn().mockResolvedValue(true),
  AcknowledgeCTDisclosure: vi.fn().mockResolvedValue(undefined),
  GetStartMinimized: vi.fn().mockResolvedValue(false),
  SetStartMinimized: vi.fn().mockResolvedValue(undefined),
  GetAppearance: vi.fn().mockResolvedValue('system'),
  SetAppearance: vi.fn().mockResolvedValue(undefined),
  GetTerminalFont: vi.fn().mockResolvedValue({ family: '', size: 13 }),
  SetTerminalFont: vi.fn().mockResolvedValue(undefined),
  RegenerateSigningKey: vi.fn().mockResolvedValue(undefined),
}))

vi.mock('../PluginsSection', () => ({ PluginsSection: () => null }))

function makeHealth(overrides: Partial<TailscaleHealth> = {}): TailscaleHealth {
  return { connected: false, hasCerts: false, ip: '', domain: '', ...overrides }
}
const CONNECTED = makeHealth({ connected: true, hasCerts: true, ip: '100.64.0.1', domain: 'my-node.tail1234.ts.net' })
const REMOVED_STATES = ['Permission Limited', 'Daemon Stopped', 'Not Installed', 'Show diagnostics']

let container: HTMLElement
let root: ReturnType<typeof createRoot>

async function renderSheet(tailscaleHealth: TailscaleHealth | null) {
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  await act(async () => {
    root.render(React.createElement(SettingsSheet, {
      isOpen: true,
      onClose: vi.fn(),
      tailscaleHealth,
    }))
  })
  await act(async () => { await new Promise<void>((r) => setTimeout(r, 0)) })
}

function sharing(): HTMLElement {
  return container.querySelector<HTMLElement>('#settings-pane-sharing')!
}

function tailscaleRow(): HTMLElement {
  return container.querySelector('.ts-status')!.closest('.settings-row') as HTMLElement
}

beforeEach(() => {
  vi.mocked(AppBindings.HasCTDisclosure).mockResolvedValue(true)
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

describe('Portty 04-05: Sharing pane is the status plus one explanation', () => {
  const EXPLANATION =
    'Share a session from the toolbar, or right-click it in the sidebar. Macs signed in to your Tailscale account see shared sessions in their sidebar, and each shared session gets a link that opens in a browser on any device signed in to your account. Turning Share off disconnects everyone.'

  it('shows the explanation under the Tailscale group', async () => {
    await renderSheet(CONNECTED)
    expect(sharing().querySelector('.settings-sheet__footnote')!.textContent!.trim()).toBe(EXPLANATION)
  })

  it('has only the Tailscale row: no server, port, URL, credential or reminder controls', async () => {
    await renderSheet(makeHealth())
    // Phase 19 (19-13) adds the Desktop group below the footnote on purpose; the
    // terminal-sharing group above it is still the Tailscale row alone.
    const tailscaleGroup = sharing().querySelector('.settings-group')!
    expect(tailscaleGroup.querySelectorAll('.settings-row')).toHaveLength(1)
    expect(tailscaleGroup.querySelector('button, input, [role="switch"], img')).toBeNull()
    for (const gone of ['Web server', 'Start Server', 'Port', 'Password', 'Username', 'Ask before']) {
      expect(tailscaleGroup.textContent).not.toContain(gone)
    }
  })

  it('no element carries a title tooltip', async () => {
    await renderSheet(makeHealth())
    expect(container.querySelector('[title]')).toBeNull()
  })
})
