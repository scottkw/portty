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

  it('Deny and Admit are plain secondary buttons in that order; neither is primary or the default', async () => {
    await render([req()])
    expect(buttons().map((b) => b.textContent)).toEqual([copy.DENY, copy.ADMIT])
    for (const b of buttons()) {
      expect(b.classList.contains('button--secondary')).toBe(true)
      expect(b.classList.contains('button--primary')).toBe(false)
      expect(b.hasAttribute('data-dialog-primary')).toBe(false)
    }
    expect(dialog()!.getAttribute('aria-modal')).toBe('true')
    expect(dialog()!.getAttribute('tabindex')).toBe('-1')
    expect(document.getElementById(dialog()!.getAttribute('aria-labelledby')!)?.textContent).toBe(copy.ADMIT_TITLE)
    const described = dialog()!.getAttribute('aria-describedby')!.split(' ').map((id) => document.getElementById(id)?.textContent)
    expect(described).toEqual([copy.ADMIT_PARAGRAPH_1, copy.ADMIT_PARAGRAPH_2])
    // The consent moment says sound reaches the guest too (DCTL-07).
    expect(described[0]).toContain("hear this Mac's sound")
  })

  it('stray Enter, Space and Escape answer nobody and the sheet stays, even after the guard', async () => {
    await render([req()])
    await pass(ADMIT_GUARD_MS)
    for (const k of ['Enter', ' ', 'Escape']) {
      await act(async () => { document.activeElement!.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true })) })
    }
    expect(admit).not.toHaveBeenCalled()
    expect(deny).not.toHaveBeenCalled()
    expect(dialog()).not.toBeNull()
    expect(document.activeElement).toBe(dialog())
  })

  it('Tab from the dialog reaches Deny, and Shift+Tab wraps to Admit', async () => {
    await render([req()])
    await act(async () => { document.activeElement!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true })) })
    expect(document.activeElement).toBe(btn(copy.DENY))
    dialog()!.focus()
    await act(async () => { document.activeElement!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, bubbles: true })) })
    expect(document.activeElement).toBe(btn(copy.ADMIT))
  })

  it('both buttons are aria-disabled inside the guard and enabled after it', async () => {
    await render([req()])
    expect(buttons().map((b) => b.getAttribute('aria-disabled'))).toEqual(['true', 'true'])
    await click(btn('Deny'))
    expect(deny).not.toHaveBeenCalled()
    await pass(ADMIT_GUARD_MS)
    expect(buttons().map((b) => b.getAttribute('aria-disabled'))).toEqual(['false', 'false'])
  })

  it('Deny calls DenyDesktopGuest with the request id', async () => {
    await render([req({ id: 'r9' })])
    await pass(ADMIT_GUARD_MS)
    await click(btn('Deny'))
    expect(deny).toHaveBeenCalledWith('r9')
    expect(admit).not.toHaveBeenCalled()
  })

  it('while an answer is in flight both buttons are aria-disabled and a second click does nothing', async () => {
    let finish!: (r: App.DesktopShareActionResult) => void
    admit.mockReturnValue(new Promise((res) => { finish = res }))
    await render([req()])
    await pass(ADMIT_GUARD_MS)
    await click(btn('Admit'))
    expect(buttons().map((b) => b.getAttribute('aria-disabled'))).toEqual(['true', 'true'])
    await click(btn('Admit'))
    await click(btn('Deny'))
    expect(admit).toHaveBeenCalledTimes(1)
    expect(deny).not.toHaveBeenCalled()
    await act(async () => finish(ok()))
    // Answered: this person's buttons stay off until the state line moves on.
    expect(btn('Admit').getAttribute('aria-disabled')).toBe('true')
  })

  it('after an answer the next request replaces it in the same sheet and the guard restarts', async () => {
    const second = req({ id: 'b', loginName: 'second@example.com', displayName: '', since: '2026-11-27T10:01:00Z' })
    await render([req({ id: 'a' }), second])
    await pass(ADMIT_GUARD_MS)
    await click(btn('Admit'))
    expect(admit).toHaveBeenCalledWith('a')

    await render([second])
    expect(container.querySelector('.admit__name')?.textContent).toBe('second@example.com')
    expect(btn('Admit').getAttribute('aria-disabled')).toBe('true')
    await click(btn('Admit'))
    expect(admit).toHaveBeenCalledTimes(1)
    await pass(ADMIT_GUARD_MS)
    await click(btn('Admit'))
    expect(admit).toHaveBeenLastCalledWith('b')
  })

  it('a re-knock from the same person and device (a new request id) does not restart the guard (WR-02)', async () => {
    await render([req({ id: 'r1' })])
    await pass(ADMIT_GUARD_MS)
    await render([req({ id: 'r2' })])
    expect(btn('Admit').getAttribute('aria-disabled')).toBe('false')
    await click(btn('Deny'))
    expect(deny).toHaveBeenCalledWith('r2')
  })

  it('the same login on another device restarts the guard', async () => {
    await render([req({ id: 'r1' })])
    await pass(ADMIT_GUARD_MS)
    await render([req({ id: 'r2', nodeName: 'pat-phone' })])
    expect(btn('Admit').getAttribute('aria-disabled')).toBe('true')
  })

  it('a failed answer shows one alert and the buttons return after the guard', async () => {
    admit.mockResolvedValueOnce({ errorCode: 'unavailable', detail: 'x' })
    await render([req()])
    await pass(ADMIT_GUARD_MS)
    await click(btn('Admit'))
    const alerts = container.querySelectorAll('[role="alert"]')
    expect(alerts).toHaveLength(1)
    expect(alerts[0].textContent).toBe(copy.ADMIT_ANSWER_ERROR)
    expect(btn('Admit').getAttribute('aria-disabled')).toBe('true')
    await pass(ADMIT_GUARD_MS)
    expect(btn('Admit').getAttribute('aria-disabled')).toBe('false')
    await click(btn('Admit'))
    expect(admit).toHaveBeenCalledTimes(2)
    expect(container.querySelector('[role="alert"]')).toBeNull()
  })

  it('a binding that throws counts as a failed answer', async () => {
    deny.mockRejectedValueOnce(new Error('boom'))
    await render([req()])
    await pass(ADMIT_GUARD_MS)
    await click(btn('Deny'))
    expect(container.querySelector('[role="alert"]')?.textContent).toBe(copy.ADMIT_ANSWER_ERROR)
  })

  it('the queue line is absent for one, "1 more person" for two, "3 more people" for four', async () => {
    const many = (n: number) => Array.from({ length: n }, (_, i) => req({ id: `q${i}`, since: `2026-11-27T10:0${i}:00Z` }))
    await render(many(1))
    expect(text()).not.toMatch(/waiting/)
    await render(many(2))
    expect(text()).toContain('1 more person is waiting.')
    await render(many(4))
    expect(text()).toContain('3 more people are waiting.')
  })

  it("shows the first request in the daemon's (arrival) order, even when two knocked in the same second (WR-06)", async () => {
    // The daemon sends pending in Seq order; `since` has whole seconds and ids are random, so neither may reorder it.
    const t = '2026-11-27T10:00:00Z'
    await render([req({ id: 'zz', displayName: 'First', nodeName: 'd1', since: t }), req({ id: 'aa', displayName: 'Second', nodeName: 'd2', since: t })])
    expect(container.querySelector('.admit__name')?.textContent).toBe('First')
    await pass(ADMIT_GUARD_MS)
    await click(btn('Admit'))
    expect(admit).toHaveBeenCalledWith('zz')
  })

  it('focus goes back to the dialog when the next person replaces the one answered from the keyboard (WR-06)', async () => {
    const second = req({ id: 'b', loginName: 'second@example.com', displayName: 'Second', nodeName: 'other-laptop' })
    await render([req({ id: 'a' }), second])
    await pass(ADMIT_GUARD_MS)
    btn('Admit').focus()
    await click(btn('Admit'))
    expect(admit).toHaveBeenCalledWith('a')
    await render([second])
    expect(container.querySelector('.admit__name')?.textContent).toBe('Second')
    expect(document.activeElement).toBe(dialog())
  })

  it('an empty display name, or one equal to the login, makes the login the bold line with no Login name row', async () => {
    await render([req({ displayName: '' })])
    expect(container.querySelector('.admit__name')?.textContent).toBe('colleague@example.com')
    expect(text()).not.toContain(copy.ADMIT_LOGIN_LABEL)
    await render([req({ displayName: 'colleague@example.com' })])
    expect(container.querySelector('.admit__name')?.textContent).toBe('colleague@example.com')
    expect(text()).not.toContain(copy.ADMIT_LOGIN_LABEL)
    // A display name that is only invisible characters is empty after sanitizing.
    await render([req({ displayName: '​‮' })])
    expect(container.querySelector('.admit__name')?.textContent).toBe('colleague@example.com')
  })

  it('a different display name shows the Login name row with the login', async () => {
    await render([req()])
    const rows = Array.from(container.querySelectorAll('.admit__fact')).map((r) => r.textContent)
    expect(rows).toEqual([`${copy.ADMIT_LOGIN_LABEL}colleague@example.com`, `${copy.ADMIT_DEVICE_LABEL}pat-laptop`])
  })

  it('drops bidi and zero-width control characters and sets every name in bdi', async () => {
    await render([req({ displayName: 'Pat‮evil​', loginName: 'a⁦b@example.com', nodeName: 'lap‮top' })])
    expect(container.querySelector('.admit__name bdi')?.textContent).toBe('Patevil')
    const facts = Array.from(container.querySelectorAll('.admit__fact-value bdi')).map((b) => b.textContent)
    expect(facts).toEqual(['ab@example.com', 'laptop'])
    expect(text()).not.toMatch(/[‮​⁦]/)
  })

  it('a 200-character login is shown in full and wraps rather than truncating', async () => {
    const long = 'x'.repeat(196) + '@a.b'
    await render([req({ loginName: long })])
    expect(container.querySelector('.admit__fact-value')?.textContent).toBe(long)
    expect(container.innerHTML).not.toMatch(/ellipsis|truncate/)
  })

  it('a request that ends moves on or closes with no message', async () => {
    const second = req({ id: 'b', since: '2026-11-27T10:01:00Z', displayName: 'Second' })
    await render([req({ id: 'a' }), second])
    await render([second])
    expect(container.querySelector('.admit__name')?.textContent).toBe('Second')
    expect(container.querySelector('[role="alert"]')).toBeNull()
    await render([])
    expect(dialog()).toBeNull()
    expect(container.innerHTML).toBe('')
  })

  it('no_request is treated as already gone, not as a failure', async () => {
    admit.mockResolvedValueOnce({ errorCode: 'no_request', detail: '' })
    await render([req()])
    await pass(ADMIT_GUARD_MS)
    await click(btn('Admit'))
    expect(container.querySelector('[role="alert"]')).toBeNull()
  })
})
