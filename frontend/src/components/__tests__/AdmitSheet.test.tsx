/**
 * The Admit sheet (22-11, 22-UI-SPEC Surface 3). The App bindings are mocked; nothing
 * here shares a screen or answers a real guest. Fake timers drive the 600 ms click guard.
 */
import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { act } from 'react'
import { AdmitSheet, ADMIT_GUARD_MS } from '../AdmitSheet'
import type { SharePending } from '../../lib/desktopShare'
import * as copy from '../../lib/shareDesktopCopy'
import * as App from '../../wailsjs/go/main/App'

vi.mock('../../wailsjs/go/main/App', () => ({
  AdmitDesktopGuest: vi.fn(),
  DenyDesktopGuest: vi.fn(),
}))

const admit = vi.mocked(App.AdmitDesktopGuest)
const deny = vi.mocked(App.DenyDesktopGuest)
const ok = (): App.DesktopShareActionResult => ({ errorCode: '', detail: '' })

const req = (over: Partial<SharePending> = {}): SharePending => ({
  id: 'r1', mode: 'guest', displayName: 'Pat Colleague', loginName: 'colleague@example.com', nodeName: 'pat-laptop',
  control: false, clipboard: false, since: '2026-11-27T10:00:00Z', ...over,
})

let container: HTMLElement
let root: ReturnType<typeof createRoot>

async function render(pending: SharePending[]): Promise<void> {
  if (!container) {
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  }
  await act(async () => { root.render(React.createElement(AdmitSheet, { pending })) })
}

async function click(el: Element): Promise<void> {
  await act(async () => { el.dispatchEvent(new MouseEvent('click', { bubbles: true })) })
}

async function pass(ms: number): Promise<void> {
  await act(async () => { await vi.advanceTimersByTimeAsync(ms) })
}

const buttons = (): HTMLButtonElement[] => Array.from(container.querySelectorAll<HTMLButtonElement>('.dialog__buttons button'))
const btn = (name: string): HTMLButtonElement => buttons().find((b) => b.textContent === name)!
const dialog = (): HTMLElement | null => container.querySelector<HTMLElement>('[role="dialog"]')
const text = (): string => container.textContent ?? ''

beforeEach(() => {
  vi.useFakeTimers()
  admit.mockResolvedValue(ok())
  deny.mockResolvedValue(ok())
})

afterEach(() => {
  act(() => root?.unmount())
  container?.remove()
  container = undefined as unknown as HTMLElement
  vi.useRealTimers()
  vi.clearAllMocks()
})

describe('AdmitSheet', () => {
  it('knock then deliberate admit: names the person, ignores a click inside the guard, admits after it', async () => {
    await render([req()])
    expect(container.querySelector('.dialog__title')?.textContent).toBe(copy.ADMIT_TITLE)
    expect(container.querySelector('.admit__name')?.textContent).toBe('Pat Colleague')
    expect(document.activeElement).toBe(dialog())
    expect(text()).toContain('colleague@example.com')
    expect(text()).toContain('pat-laptop')

    await pass(ADMIT_GUARD_MS - 100)
    await click(btn('Admit'))
    expect(admit).not.toHaveBeenCalled()

    await pass(100)
    await click(btn('Admit'))
    expect(admit).toHaveBeenCalledWith('r1')
    expect(deny).not.toHaveBeenCalled()
  })
})
