// DEBT-08: an updated GUI found the old daemon still running live sessions.
// Go kept it and asks here; "Not Now" is the default and keeps it for this
// launch, "Restart Now" restarts it, which ends the sessions.

import { describe, it, expect, vi, afterEach } from 'vitest'
import React, { act } from 'react'

import { mountApp, unmountApp, flush, wailsEvents } from './appHarness'

// jsdom has no xterm.
vi.mock('../TerminalPanel', () => ({
  TerminalPanel: () => React.createElement('div', { 'data-testid': 'terminal-panel' }),
}))

afterEach(async () => {
  await unmountApp()
})

const dialog = () => document.querySelector('[role="dialog"]')
const button = (label: string) =>
  [...document.querySelectorAll('[role="dialog"] button')].find((b) => b.textContent === label) as HTMLButtonElement

describe('App — daemon restart consent (DEBT-08)', () => {
  it('asks at launch with Not Now focused, and Not Now keeps the daemon', async () => {
    const answer = vi.fn(() => null)
    await mountApp({ GetDaemonRestartPrompt: () => 2, AnswerDaemonRestart: answer })
    expect(dialog()?.textContent).toContain("Update Portty's background service?")
    expect(dialog()?.textContent).toContain('2 sessions are running. Restarting ends them.')
    expect(document.activeElement).toBe(button('Not Now'))

    await act(async () => { button('Not Now').click() })
    await flush()
    expect(answer).toHaveBeenCalledWith(false)
    expect(dialog()).toBeNull()
  })

  it('Restart Now restarts and closes', async () => {
    const answer = vi.fn(() => null)
    await mountApp({ GetDaemonRestartPrompt: () => 3, AnswerDaemonRestart: answer })
    await act(async () => { button('Restart Now').click() })
    await flush()
    expect(answer).toHaveBeenCalledWith(true)
    expect(dialog()).toBeNull()
  })

  // 08-06-REVIEW IN-02: Go closes the question and raises daemon:error once the old daemon is gone.
  it('a failed Restart Now that stopped the old daemon closes and shows the error banner', async () => {
    const msg = "Restarting Portty's background service failed: daemon exited during startup"
    await mountApp({
      GetDaemonRestartPrompt: () => 2,
      AnswerDaemonRestart: () => {
        wailsEvents.get('daemon:restart-needed')?.(0)
        wailsEvents.get('daemon:error')?.(msg)
        return Promise.reject(new Error(msg))
      },
    })
    await act(async () => { button('Restart Now').click() })
    await flush()
    expect(dialog()).toBeNull()
    expect(document.querySelector('.banner[role="alert"]')?.textContent).toContain(msg)
  })

  it('shows no dialog without a prompt, and asks on daemon:restart-needed', async () => {
    await mountApp({})
    expect(dialog()).toBeNull()

    await act(async () => { wailsEvents.get('daemon:restart-needed')?.(1) })
    expect(dialog()?.textContent).toContain('1 session is running. Restarting ends it.')

    await act(async () => { wailsEvents.get('daemon:restart-needed')?.(-1) })
    expect(dialog()?.textContent).toContain('Sessions may be running. Restarting ends them.')
  })
})
