/**
 * Phase 150 SET-01 / Portty 02-11 — Settings ▸ Sharing "Ask before sharing a
 * session" switch.
 *
 * Covers:
 *   - The switch renders (role=switch, <label for>) after GetShellWebShareWarningEnabled resolves
 *   - Turning it off opens the "Stop asking before sharing?" confirm (no setter call yet)
 *   - "Stop Asking" calls SetShellWebShareWarningEnabled(false) (T-02-35)
 *   - "Keep Asking" calls no setter and closes the confirm
 *   - Turning it on calls SetShellWebShareWarningEnabled(true) with no confirm
 */
import { describe, it, expect, afterEach, vi, beforeEach } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { act } from 'react'
import { SettingsSheet } from '../SettingsSheet'
import raw from '../SettingsSheet.tsx?raw'
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
  RegenerateSigningKey: vi.fn().mockResolvedValue(undefined),
  GetShellWebShareWarningEnabled: vi.fn().mockResolvedValue(true),
  SetShellWebShareWarningEnabled: vi.fn().mockResolvedValue(undefined),
}))

vi.mock('../PluginsSection', () => ({ PluginsSection: () => null }))
vi.mock('../../wailsjs/wailsjs/runtime/runtime', () => ({
  BrowserOpenURL: vi.fn(),
  ClipboardSetText: vi.fn().mockResolvedValue(undefined),
}))

let container: HTMLElement | undefined
let root: ReturnType<typeof createRoot>

async function renderSheet(onShellWarnEnabledChange = vi.fn()) {
  const el = document.createElement('div')
  container = el
  document.body.appendChild(el)
  root = createRoot(el)
  await act(async () => {
    root.render(React.createElement(SettingsSheet, {
      isOpen: true,
      onClose: vi.fn(),
      tailscaleHealth: null,
      webServerMode: null,
      onWebServerStateChange: vi.fn().mockResolvedValue(undefined),
      onShellWarnEnabledChange,
    }))
  })
  await flushEffects()
}

async function flushEffects() {
  await act(async () => { await new Promise<void>((r) => setTimeout(r, 0)) })
}

const theSwitch = () => container!.querySelector<HTMLButtonElement>('#shellWebShareWarningEnabled')
const confirmDialog = () =>
  Array.from(container!.querySelectorAll('[role="dialog"]')).find((d) => d.getAttribute('aria-labelledby') === 'disable-shell-warn-title')
const button = (root: Element, text: string) =>
  Array.from(root.querySelectorAll('button')).find((b) => b.textContent === text) as HTMLButtonElement

afterEach(async () => {
  // The source-contract tests mount nothing.
  if (container) {
    await act(async () => root.unmount())
    container.remove()
  }
  vi.clearAllMocks()
})

describe('SET-01: source contract', () => {
  it('keeps the loaded / saving / error state and the D-03 re-arm prop', () => {
    expect(raw).toContain('shellWarnLoaded')
    expect(raw).toContain('setShellWarnSaving')
    expect(raw).toContain('setShellWarnError')
    expect(raw).toContain('onShellWarnEnabledChange?:')
  })

  it('the disable confirm passes the UI-SPEC copy, not the signing-key defaults', () => {
    const modalCall = raw.slice(raw.indexOf('isOpen={showDisableWarnConfirm}'), raw.indexOf('isOpen={showRegenModal}'))
    expect(modalCall).toContain('title="Stop asking before sharing?"')
    expect(modalCall).toContain('confirmLabel="Stop Asking"')
    expect(modalCall).toContain('cancelLabel="Keep Asking"')
    expect(modalCall).not.toContain('Invalidate All Links')
  })
})

describe('SET-01: switch DOM (initial value true)', () => {
  beforeEach(() => {
    vi.mocked(AppBindings.GetShellWebShareWarningEnabled).mockResolvedValue(true)
  })

  it('renders a labelled role=switch, checked, with its description', async () => {
    await renderSheet()
    const sw = theSwitch()!
    expect(sw.getAttribute('role')).toBe('switch')
    expect(sw.getAttribute('aria-checked')).toBe('true')
    expect(container!.querySelector('label[for="shellWebShareWarningEnabled"]')!.textContent).toBe('Ask before sharing a session')
    expect(container!.textContent).toContain('Shows a one-time security reminder the first time you share a session.')
  })

  it('turning it off opens the confirm with the UI-SPEC copy and saves nothing yet', async () => {
    await renderSheet()
    await act(async () => theSwitch()!.click())
    const dialog = confirmDialog()!
    expect(dialog).toBeDefined()
    expect(dialog.textContent).toContain('Stop asking before sharing?')
    expect(dialog.textContent).toContain(
      "You won't see the one-time security reminder before sharing a session. You can turn it back on in Settings ▸ Sharing.",
    )
    expect(button(dialog, 'Keep Asking')).toBeDefined()
    expect(button(dialog, 'Stop Asking')).toBeDefined()
    expect(AppBindings.SetShellWebShareWarningEnabled).not.toHaveBeenCalled()
  })

  it('"Stop Asking" calls SetShellWebShareWarningEnabled(false) and reports it', async () => {
    const onChange = vi.fn()
    await renderSheet(onChange)
    await act(async () => theSwitch()!.click())
    await act(async () => button(confirmDialog()!, 'Stop Asking').click())
    await flushEffects()
    expect(AppBindings.SetShellWebShareWarningEnabled).toHaveBeenCalledWith(false)
    expect(onChange).toHaveBeenCalledWith(false)
    expect(theSwitch()!.getAttribute('aria-checked')).toBe('false')
    expect(confirmDialog()).toBeUndefined()
  })

  it('"Keep Asking" saves nothing and closes the confirm', async () => {
    await renderSheet()
    await act(async () => theSwitch()!.click())
    await act(async () => button(confirmDialog()!, 'Keep Asking').click())
    await flushEffects()
    expect(AppBindings.SetShellWebShareWarningEnabled).not.toHaveBeenCalled()
    expect(confirmDialog()).toBeUndefined()
    expect(theSwitch()!.getAttribute('aria-checked')).toBe('true')
  })
})

describe('SET-01: switch DOM (initial value false)', () => {
  beforeEach(() => {
    vi.mocked(AppBindings.GetShellWebShareWarningEnabled).mockResolvedValue(false)
  })

  it('turning it on saves true at once with no confirm', async () => {
    await renderSheet()
    await act(async () => theSwitch()!.click())
    await flushEffects()
    expect(AppBindings.SetShellWebShareWarningEnabled).toHaveBeenCalledWith(true)
    expect(confirmDialog()).toBeUndefined()
  })
})
