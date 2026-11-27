/**
 * The Watching list (22-10, 22-UI-SPEC Surface 2 and Names). The bindings are mocked;
 * nothing here talks to a daemon or shares a screen.
 */
import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { act } from 'react'
import { ViewerList } from '../ViewerList'
import type { ShareViewer } from '../../lib/desktopShare'
import * as copy from '../../lib/shareDesktopCopy'
import * as App from '../../wailsjs/go/main/App'

vi.mock('../../wailsjs/go/main/App', () => ({
  SetDesktopViewerRights: vi.fn(),
  DisconnectDesktopViewer: vi.fn(),
}))

const setRights = vi.mocked(App.SetDesktopViewerRights)
const disconnect = vi.mocked(App.DisconnectDesktopViewer)

const ok = (): App.DesktopShareActionResult => ({ errorCode: '', detail: '' })

const guest = (over: Partial<ShareViewer> = {}): ShareViewer => ({
  id: 'g1', mode: 'guest', displayName: 'Pat Colleague', loginName: 'colleague@example.com', nodeName: 'pat-laptop',
  control: false, clipboard: false, since: '2026-11-27T10:00:00Z', ...over,
})
const own = (over: Partial<ShareViewer> = {}): ShareViewer => ({
  id: 'o1', mode: 'own', displayName: 'Ken', loginName: 'ken@example.com', nodeName: 'Office iPad',
  control: true, clipboard: true, since: '2026-11-27T09:00:00Z', ...over,
})

let container: HTMLElement
let root: ReturnType<typeof createRoot>

async function render(viewers: ShareViewer[]): Promise<void> {
  if (!container) {
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  }
  await act(async () => { root.render(React.createElement(ViewerList, { viewers })) })
}

async function click(el: Element): Promise<void> {
  await act(async () => { el.dispatchEvent(new MouseEvent('click', { bubbles: true })) })
}

const rows = (): HTMLElement[] => Array.from(container.querySelectorAll<HTMLElement>('[role="group"]'))
const row = (i = 0): HTMLElement => rows()[i]
const switches = (r: HTMLElement): HTMLButtonElement[] => Array.from(r.querySelectorAll<HTMLButtonElement>('[role="switch"]'))
const button = (r: HTMLElement, name: string): HTMLButtonElement | undefined =>
  Array.from(r.querySelectorAll<HTMLButtonElement>('button')).find((b) => b.textContent?.trim() === name)
const accessibleName = (el: Element): string =>
  (el.getAttribute('aria-labelledby') ?? '').split(' ').map((id) => document.getElementById(id)?.textContent ?? '').join(' ')

beforeEach(() => {
  setRights.mockResolvedValue(ok())
  disconnect.mockResolvedValue(ok())
})

afterEach(async () => {
  await act(async () => { root.unmount() })
  container.remove()
  container = undefined as unknown as HTMLElement
  vi.clearAllMocks()
})

describe('ViewerList', () => {
  it('orders rows oldest connection first, then by id, and keeps them in place when someone joins', async () => {
    await render([
      guest({ id: 'b', displayName: 'Bea', since: '2026-11-27T10:05:00Z' }),
      guest({ id: 'a', displayName: 'Al', since: '2026-11-27T10:05:00Z' }),
      guest({ id: 'c', displayName: 'Cy', since: '2026-11-27T10:00:00Z' }),
    ])
    expect(rows().map((r) => r.querySelector('.viewer-row__name')!.textContent)).toEqual(['Cy', 'Al', 'Bea'])
    await render([
      guest({ id: 'b', displayName: 'Bea', since: '2026-11-27T10:05:00Z' }),
      guest({ id: 'd', displayName: 'Di', since: '2026-11-27T10:09:00Z' }),
      guest({ id: 'a', displayName: 'Al', since: '2026-11-27T10:05:00Z' }),
      guest({ id: 'c', displayName: 'Cy', since: '2026-11-27T10:00:00Z' }),
    ])
    expect(rows().map((r) => r.querySelector('.viewer-row__name')!.textContent)).toEqual(['Cy', 'Al', 'Bea', 'Di'])
  })

  it('a guest row has the bold name in bdi, login and device, View only, two switches, the caption and Disconnect', async () => {
    await render([guest()])
    const r = row()
    expect(r.querySelector('.viewer-row__name bdi')!.textContent).toBe('Pat Colleague')
    expect(r.querySelector('.viewer-row__login')!.textContent).toBe('colleague@example.com')
    const device = r.querySelector('.viewer-row__device')!
    expect(device.textContent).toBe('pat-laptop')
    expect(device.getAttribute('title')).toBe('pat-laptop')
    expect(r.querySelector('.viewer-badge')!.textContent).toBe(copy.BADGE_VIEW_ONLY)
    expect(r.querySelector('.viewer-badge svg')!.getAttribute('aria-hidden')).toBe('true')
    expect(switches(r)).toHaveLength(2)
    expect(switches(r).map((s) => s.getAttribute('aria-checked'))).toEqual(['false', 'false'])
    expect(r.textContent).toContain(copy.grantCaption('Pat Colleague'))
    expect(r.querySelector('.viewer-row__caption bdi')!.textContent).toBe('Pat Colleague')
    expect(button(r, copy.DISCONNECT)).toBeDefined()
  })

  it('an own-account row has the Own account badge, Your own device and Disconnect only', async () => {
    await render([own()])
    const r = row()
    expect(r.querySelector('.viewer-row__name')!.textContent).toBe('Office iPad')
    expect(r.querySelector('.viewer-badge')!.textContent).toBe(copy.BADGE_OWN_ACCOUNT)
    expect(r.textContent).toContain(copy.OWN_DEVICE_LINE)
    expect(r.textContent).not.toContain('ken@example.com')
    expect(switches(r)).toHaveLength(0)
    expect(r.querySelector('.viewer-row__caption')).toBeNull()
    expect(button(r, copy.DISCONNECT)).toBeDefined()
  })

  it('shows the login as the bold line and omits the login text when the display name is empty or equals it', async () => {
    await render([
      guest({ id: 'e', displayName: '', loginName: 'empty@example.com', since: '2026-11-27T10:00:00Z' }),
      guest({ id: 'q', displayName: 'same@example.com', loginName: 'same@example.com', since: '2026-11-27T10:01:00Z' }),
    ])
    for (const [i, login] of ['empty@example.com', 'same@example.com'].entries()) {
      const r = row(i)
      expect(r.querySelector('.viewer-row__name')!.textContent).toBe(login)
      expect(r.querySelector('.viewer-row__login')).toBeNull()
      expect(r.textContent!.split(login).length - 1).toBe(2) // the bold line and the grant caption, nothing else
      expect(r.querySelector('.viewer-row__device')!.textContent).toBe('pat-laptop')
    }
  })

  it('the badge follows the control grant, and clipboard alone stays View only', async () => {
    await render([
      guest({ id: 'a', since: '2026-11-27T10:00:00Z', control: true }),
      guest({ id: 'b', since: '2026-11-27T10:01:00Z', clipboard: true }),
    ])
    expect(row(0).querySelector('.viewer-badge')!.textContent).toBe(copy.BADGE_CONTROLLING)
    expect(switches(row(0)).map((s) => s.getAttribute('aria-checked'))).toEqual(['true', 'false'])
    expect(row(1).querySelector('.viewer-badge')!.textContent).toBe(copy.BADGE_VIEW_ONLY)
    expect(switches(row(1)).map((s) => s.getAttribute('aria-checked'))).toEqual(['false', 'true'])
  })

  it('Allow control calls SetDesktopViewerRights(id, true, clipboard) with the switch saving, then follows the next state', async () => {
    let release: (r: App.DesktopShareActionResult) => void = () => {}
    setRights.mockReturnValueOnce(new Promise((res) => { release = res }))
    await render([guest({ clipboard: true })])
    const [control, clip] = switches(row())
    await click(control)
    expect(setRights).toHaveBeenCalledWith('g1', true, true)
    expect(control.getAttribute('aria-disabled')).toBe('true') // saving: dimmed, focusable
    expect(clip.getAttribute('aria-disabled')).toBe('true')
    await click(clip) // inert while the call runs
    expect(setRights).toHaveBeenCalledTimes(1)
    await act(async () => { release(ok()) })
    expect(control.hasAttribute('aria-disabled')).toBe(false)
    expect(row().querySelector('[role="alert"]')).toBeNull()
    await render([guest({ clipboard: true, control: true })])
    expect(switches(row())[0].getAttribute('aria-checked')).toBe('true')
    expect(row().querySelector('.viewer-badge')!.textContent).toBe(copy.BADGE_CONTROLLING)
  })

  it('Allow clipboard is independent: it sends the current control value', async () => {
    await render([guest({ control: true })])
    await click(switches(row())[1])
    expect(setRights).toHaveBeenCalledWith('g1', true, true)
    await render([guest({ control: true, clipboard: true })])
    await click(switches(row())[0]) // control off keeps clipboard
    expect(setRights).toHaveBeenLastCalledWith('g1', false, true)
  })

  it('a grant error shows the inline alert, and the switch stays at the state the daemon reported', async () => {
    setRights.mockResolvedValueOnce({ errorCode: 'unavailable', detail: 'daemon not connected' })
    await render([guest()])
    const control = switches(row())[0]
    await click(control)
    expect(row().querySelector('[role="alert"]')!.textContent).toBe(copy.GRANT_CHANGE_ERROR)
    expect(control.getAttribute('aria-checked')).toBe('false')
    expect(control.hasAttribute('aria-disabled')).toBe(false)
    await click(control) // a retry clears the error
    expect(row().querySelector('[role="alert"]')).toBeNull()
  })

  it('Disconnect calls DisconnectDesktopViewer at once with no confirm; no_viewer means they already left and is quiet', async () => {
    disconnect.mockResolvedValueOnce({ errorCode: 'no_viewer', detail: '' })
    await render([guest()])
    await click(button(row(), copy.DISCONNECT)!)
    expect(disconnect).toHaveBeenCalledWith('g1')
    expect(document.querySelector('[role="alertdialog"]')).toBeNull()
    expect(row().querySelector('[role="alert"]')).toBeNull()
  })

  it('a Disconnect error shows the next step', async () => {
    disconnect.mockResolvedValueOnce({ errorCode: 'unavailable', detail: 'x' })
    await render([own()])
    await click(button(row(), copy.DISCONNECT)!)
    expect(row().querySelector('[role="alert"]')!.textContent).toBe(copy.DISCONNECT_ERROR)
  })

  it('renders a bidi display name as text inside bdi, with control and zero-width characters dropped', async () => {
    await render([guest({ displayName: 'Pat‮tab​\nname', loginName: 'colleague@example.com' })])
    const name = row().querySelector('.viewer-row__name bdi')!
    expect(name.textContent).toBe('Pattabname')
    expect(row().querySelector('.viewer-row__name')!.innerHTML).not.toContain('<script')
    expect(row().textContent).not.toMatch(/[‮​]/)
    expect(row().textContent).toContain('colleague@example.com')
  })

  it('names are never markup', async () => {
    await render([guest({ displayName: '<img src=x onerror=alert(1)>', loginName: '<b>x@example.com</b>' })])
    expect(row().querySelector('img')).toBeNull()
    expect(row().querySelector('.viewer-row__login b')).toBeNull()
    expect(row().querySelector('.viewer-row__name')!.textContent).toBe('<img src=x onerror=alert(1)>')
  })

  it('each row is a group named by the person; switches are labelled by label plus name and described by the caption; Disconnect names the person', async () => {
    await render([guest()])
    const r = row()
    expect(accessibleName(r)).toBe('Pat Colleague')
    const [control, clip] = switches(r)
    expect(accessibleName(control)).toBe(`${copy.ALLOW_CONTROL} Pat Colleague`)
    expect(accessibleName(clip)).toBe(`${copy.ALLOW_CLIPBOARD} Pat Colleague`)
    const caption = document.getElementById(control.getAttribute('aria-describedby')!)!
    expect(clip.getAttribute('aria-describedby')).toBe(caption.id)
    expect(caption.textContent).toBe(copy.grantCaption('Pat Colleague'))
    expect(button(r, copy.DISCONNECT)!.getAttribute('aria-label')).toBe(`${copy.DISCONNECT} Pat Colleague`)
  })

  it('shows the one-line empty state when nobody is watching', async () => {
    await render([])
    expect(rows()).toHaveLength(0)
    expect(container.textContent).toContain(copy.WATCHING_HEADER)
    expect(container.textContent).toContain(copy.WATCHING_EMPTY)
  })

  it('the list is a polite live region, and a hidden status line announces who joined and left', async () => {
    await render([guest({ id: 'a', displayName: 'Al' })])
    expect(container.querySelector('.settings-group')!.getAttribute('aria-live')).toBe('polite')
    const status = (): string => container.querySelector('.visually-hidden[role="status"]')!.textContent ?? ''
    expect(status()).toBe('') // the first render announces nothing
    await render([guest({ id: 'a', displayName: 'Al' }), guest({ id: 'b', displayName: 'Bea', since: '2026-11-27T11:00:00Z' })])
    expect(status()).toBe(copy.liveJoined('Bea'))
    await render([guest({ id: 'b', displayName: 'Bea', since: '2026-11-27T11:00:00Z' })])
    expect(status()).toBe(copy.liveLeft('Al'))
  })

  it('a guest who drops and returns shows up again with the grants the daemon reports', async () => {
    await render([guest({ control: true, clipboard: true })])
    await render([])
    expect(rows()).toHaveLength(0)
    await render([guest({ control: true, clipboard: true })])
    expect(switches(row()).map((s) => s.getAttribute('aria-checked'))).toEqual(['true', 'true'])
  })
})
