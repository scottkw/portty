/**
 * NET-01 (Portty 01-12) — SettingsTab two-state Tailscale status.
 *
 * Tailscale is either Connected (via its domain or IP) or Not connected (the
 * NET-01 sentence); "Checking…" while the first check is pending. The old
 * multi-state labels and the diagnostics checklist never render.
 */
import { describe, it, expect, afterEach, vi, beforeEach } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { flushSync } from 'react-dom'
import { act } from 'react-dom/test-utils'
import { SettingsTab } from '../SettingsTab'
import { TAILSCALE_OFF_REASON, type TailscaleHealth } from '../../lib/tailscale'

// --- Wails mock -----------------------------------------------------------
// Every binding SettingsTab currently imports, mirroring
// SettingsTab.notify-permission-hint.test.tsx.

vi.mock('../../wailsjs/go/main/App', () => ({
  UpdateCLIPath: vi.fn().mockResolvedValue(undefined),
  GetCLIPaths: vi.fn().mockResolvedValue({}),
  OpenFileDialog: vi.fn().mockResolvedValue(''),
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
  RegenerateSigningKey: vi.fn().mockResolvedValue(undefined),
  GetPluginSettings: vi.fn().mockResolvedValue({ searchConfig: {}, webLinksConfig: {}, imageConfig: {} }),
  SetPluginSettings: vi.fn().mockResolvedValue(undefined),
  SetSearchConfig: vi.fn().mockResolvedValue(undefined),
  SetWebLinksConfig: vi.fn().mockResolvedValue(undefined),
  SetImageConfig: vi.fn().mockResolvedValue(undefined),
  GetTailscaleStatus: vi.fn().mockResolvedValue(null),
  GetShellPath: vi.fn().mockResolvedValue('/bin/zsh'),
  SetShellPath: vi.fn().mockResolvedValue(undefined),
  GetShellWebShareWarningEnabled: vi.fn().mockResolvedValue(true),
  SetShellWebShareWarningEnabled: vi.fn().mockResolvedValue(undefined),
  GetNotifyOnWaiting: vi.fn().mockResolvedValue(false),
  SetNotifyOnWaiting: vi.fn().mockResolvedValue(undefined),
  GetStayOnHubAfterCreate: vi.fn().mockResolvedValue(false),
  SetStayOnHubAfterCreate: vi.fn().mockResolvedValue(undefined),
}))

vi.mock('../../wailsjs/wailsjs/runtime/runtime', () => ({
  BrowserOpenURL: vi.fn(),
  ClipboardSetText: vi.fn().mockResolvedValue(undefined),
  EventsOn: vi.fn(() => vi.fn()),
}))

vi.mock('../RegenerateKeyModal', () => ({
  RegenerateKeyModal: ({ isOpen }: { isOpen: boolean }) => {
    if (!isOpen) return null
    return React.createElement('div', { 'data-testid': 'regen-modal' })
  }
}))

vi.mock('../PluginsSection', () => ({
  PluginsSection: () => null,
}))
vi.mock('../SettingsJumpBar', () => ({
  SettingsJumpBar: () => null,
}))
vi.mock('../SettingsSearch', () => ({
  SettingsSearch: () => null,
}))

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

function makeHealth(overrides: Partial<TailscaleHealth> = {}): TailscaleHealth {
  return { connected: false, hasCerts: false, ip: '', domain: '', ...overrides }
}

const REMOVED_STATES = ['Permission Limited', 'Daemon Stopped', 'Not Installed', 'Show diagnostics']

function makeProps(overrides: Partial<Parameters<typeof SettingsTab>[0]> = {}) {
  return {
    tailscaleHealth: null as TailscaleHealth | null,
    webServerMode: null as null,
    onWebServerStateChange: vi.fn().mockResolvedValue(undefined),
    selectedTheme: 'Tomorrow_Night',
    onThemeChange: vi.fn(),
    uiTheme: 'dark' as const,
    onUiThemeChange: vi.fn(),
    onPluginToggleSideEffect: undefined,
    ...overrides,
  }
}

function renderSettingsTab(props: ReturnType<typeof makeProps>) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  flushSync(() => {
    root.render(React.createElement(SettingsTab, props))
  })
  return { container, root }
}

async function flushEffects() {
  await act(async () => {
    await new Promise<void>((resolve) => setTimeout(resolve, 0))
  })
}

describe('NET-01: SettingsTab two-state Tailscale status', () => {
  let container: HTMLElement
  let root: ReturnType<typeof createRoot>

  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    root.unmount()
    container.remove()
    vi.clearAllMocks()
  })

  function fieldGroupText(): string {
    const group = container.querySelector('.ts-status')!.closest('.settings-panel__field-group') as HTMLElement
    return group.textContent ?? ''
  }

  function expectNoRemovedStates() {
    for (const label of REMOVED_STATES) {
      expect(container.textContent).not.toContain(label)
    }
    expect(container.querySelector('.ts-status__dot--warn')).toBeNull()
  }

  it('shows Connected via the domain with the ok dot', async () => {
    ;({ container, root } = renderSettingsTab(makeProps({
      tailscaleHealth: makeHealth({ connected: true, hasCerts: true, ip: '100.64.0.1', domain: 'my-node.ts.net' }),
    })))
    await flushEffects()

    expect(container.querySelector('.ts-status__text')!.textContent).toBe('Connected')
    expect(container.querySelector('.ts-status__dot')!.className).toContain('ts-status__dot--ok')
    expect(fieldGroupText()).toContain('Connected via my-node.ts.net')
    expectNoRemovedStates()
  })

  it('falls back to the IP when there is no domain', async () => {
    ;({ container, root } = renderSettingsTab(makeProps({
      tailscaleHealth: makeHealth({ connected: true, ip: '100.64.0.1' }),
    })))
    await flushEffects()

    expect(fieldGroupText()).toContain('Connected via 100.64.0.1')
  })

  it('shows Not connected with the error dot and the NET-01 sentence', async () => {
    ;({ container, root } = renderSettingsTab(makeProps({ tailscaleHealth: makeHealth() })))
    await flushEffects()

    expect(container.querySelector('.ts-status__text')!.textContent).toBe('Not connected')
    const dot = container.querySelector('.ts-status__dot')!
    expect(dot.className).toContain('ts-status__dot--error')
    expect(dot.className).not.toContain('ts-status__dot--ok')
    expect(fieldGroupText()).toContain(TAILSCALE_OFF_REASON)
    expect(container.querySelector('details')).toBeNull()
    expectNoRemovedStates()
  })

  it('shows Checking… with no dot while health is null', async () => {
    ;({ container, root } = renderSettingsTab(makeProps({ tailscaleHealth: null })))
    await flushEffects()

    expect(container.querySelector('.ts-status__text')!.textContent).toBe('Checking\u2026')
    expect(container.querySelector('.ts-status__dot')).toBeNull()
    expectNoRemovedStates()
  })
})
