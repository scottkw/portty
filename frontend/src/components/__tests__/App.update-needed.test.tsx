// UPGR-04: a daemon whose API is below what this GUI needs shows an "Update
// needed" banner instead of failing calls. The banner never offers an action
// that ends sessions.

import { describe, it, expect, vi, afterEach } from 'vitest'
import React, { act } from 'react'

import { mountApp, unmountApp, wailsEvents } from './appHarness'

// jsdom has no xterm.
vi.mock('../TerminalPanel', () => ({
  TerminalPanel: () => React.createElement('div', { 'data-testid': 'terminal-panel' }),
}))

afterEach(async () => {
  await unmountApp()
})

const body = "Portty's background service (version v1.1.0) is too old for this version of the app. Relaunch Portty to update it."
const banner = () => [...document.querySelectorAll('.banner')].find((b) => b.textContent?.includes('Update needed'))

describe('App — Update needed banner (UPGR-04)', () => {
  it('shows the banner when the daemon is too old at load', async () => {
    await mountApp({ GetDaemonUpdateNeeded: () => 'v1.1.0' })
    expect(banner()?.querySelector('strong')?.textContent).toBe('Update needed')
    expect(banner()?.textContent).toContain(body)
    expect(banner()?.querySelector('button')).toBeNull()
  })

  it('shows the banner on daemon:update-needed', async () => {
    await mountApp({})
    expect(banner()).toBeUndefined()
    await act(async () => { wailsEvents.get('daemon:update-needed')?.('v1.1.0') })
    expect(banner()?.textContent).toContain(body)
  })

  it('shows no banner for a compatible daemon', async () => {
    await mountApp({ GetDaemonUpdateNeeded: () => '' })
    expect(banner()).toBeUndefined()
  })
})
