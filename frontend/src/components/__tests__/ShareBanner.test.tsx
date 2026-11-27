/**
 * The Secure Event Input and Share stopped banners (22-11, 22-UI-SPEC Surface 5), and the
 * App wiring that decides when each shows (the real useDesktopShare store fed by a fake
 * Wails event bus; nothing here shares a screen).
 */
import { describe, it, expect, afterEach, vi } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { act } from 'react'
import { ShareBanner } from '../ShareBanner'
import { useDesktopShare, NO_SHARE, type ShareState, type ShareSubscribe } from '../../lib/desktopShare'
import * as copy from '../../lib/shareDesktopCopy'
import appRaw from '../../App.tsx?raw'

let container: HTMLElement
let root: ReturnType<typeof createRoot>

function render(node: React.ReactElement): void {
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  act(() => root.render(node))
}

afterEach(() => {
  act(() => root?.unmount())
  container?.remove()
})

describe('ShareBanner', () => {
  it('secure-input is a warning status with the spec sentence, an icon, no action and no dismiss', () => {
    render(<ShareBanner kind="secure-input" />)
    const banner = container.querySelector('.banner')!
    expect(banner.getAttribute('role')).toBe('status')
    expect(banner.classList.contains('banner--warning')).toBe(true)
    expect(container.querySelector('.banner__message')!.textContent).toBe(copy.BANNER_SECURE_INPUT)
    expect(container.querySelector('.banner__icon')!.getAttribute('aria-hidden')).toBe('true')
    expect(container.querySelector('button')).toBeNull()
  })

  it('stopped is an info status whose action opens the sheet and whose dismiss hides it', () => {
    const onShare = vi.fn()
    const onDismiss = vi.fn()
    render(<ShareBanner kind="stopped" onShare={onShare} onDismiss={onDismiss} />)
    const banner = container.querySelector('.banner')!
    expect(banner.getAttribute('role')).toBe('status')
    expect(banner.classList.contains('banner--info')).toBe(true)
    expect(container.querySelector('.banner__message')!.textContent).toBe(copy.BANNER_SHARE_STOPPED)
    const action = container.querySelector<HTMLButtonElement>('.banner__action')!
    expect(action.textContent).toBe('Share Desktop…')
    act(() => action.click())
    expect(onShare).toHaveBeenCalledTimes(1)
    act(() => container.querySelector<HTMLButtonElement>('.banner__dismiss')!.click())
    expect(onDismiss).toHaveBeenCalledTimes(1)
  })
})

// The same conditions App evaluates, driven through the real store.
describe('when the banners show (useDesktopShare)', () => {
  const handlers = new Map<string, (p: unknown) => void>()
  const subscribe: ShareSubscribe = (name, cb) => {
    handlers.set(name, cb as (p: unknown) => void)
    return () => { handlers.delete(name) }
  }
  const send = (name: string, payload?: unknown): void => act(() => handlers.get(name)!(payload))
  const push = (over: Partial<ShareState>): void => send('desktopshare:state', { ...NO_SHARE, ...over })

  function Probe(): React.ReactElement {
    const share = useDesktopShare(subscribe)
    const secure = share.state.secureInput && share.state.anyController
    return (
      <div>
        {secure && <ShareBanner kind="secure-input" />}
        {share.stoppedNotice && <ShareBanner kind="stopped" onShare={share.openSheet} onDismiss={share.closeSheet} />}
        <span id="sheet">{share.sheetOpen ? 'open' : 'closed'}</span>
      </div>
    )
  }
  const banners = (): string[] => Array.from(container.querySelectorAll('.banner__message')).map((m) => m.textContent ?? '')

  it('secure input shows only while it is on and someone can control; clears when either stops', () => {
    render(<Probe />)
    push({ active: true, secureInput: true, anyController: false })
    expect(banners()).toEqual([])
    push({ active: true, secureInput: true, anyController: true })
    expect(banners()).toEqual([copy.BANNER_SECURE_INPUT])
    push({ active: true, secureInput: false, anyController: true })
    expect(banners()).toEqual([])
    push({ active: true, secureInput: true, anyController: true })
    push({ active: true, secureInput: true, anyController: false })
    expect(banners()).toEqual([])
  })

  it('stopped shows after desktopshare:stopped; its action opens the sheet and clears it', () => {
    render(<Probe />)
    send('desktopshare:stopped')
    expect(banners()).toEqual([copy.BANNER_SHARE_STOPPED])
    act(() => container.querySelector<HTMLButtonElement>('.banner__action')!.click())
    expect(banners()).toEqual([])
    expect(container.querySelector('#sheet')!.textContent).toBe('open')
  })

  it('stopped is dismissable and a new active share clears it', () => {
    render(<Probe />)
    send('desktopshare:stopped')
    act(() => container.querySelector<HTMLButtonElement>('.banner__dismiss')!.click())
    expect(banners()).toEqual([])
    send('desktopshare:stopped')
    expect(banners()).toHaveLength(1)
    push({ active: true })
    expect(banners()).toEqual([])
    // A later plain inactive line must not bring the old notice back.
    push({ active: false })
    expect(banners()).toEqual([])
  })
})

describe('App wiring', () => {
  it('mounts both banners in the banner stack and counts them in its gating expression', () => {
    expect(appRaw).toContain('<ShareBanner kind="secure-input" />')
    expect(appRaw).toContain('<ShareBanner kind="stopped"')
    const gate = appRaw.slice(appRaw.indexOf('INVARIANT: this gating expression'), appRaw.indexOf('<div className="banner-stack">'))
    expect(gate).toContain('secureInputBanner')
    expect(gate).toContain('share.stoppedNotice')
  })

  it('passes the active share to the quit confirm and renders the Admit sheet last', () => {
    expect(appRaw).toContain('sharing={share.state.active}')
    expect(appRaw.lastIndexOf('<AdmitSheet')).toBeGreaterThan(appRaw.lastIndexOf('<RegenerateKeyModal'))
    expect(appRaw.lastIndexOf('<AdmitSheet')).toBeGreaterThan(appRaw.lastIndexOf('<QuitConfirmModal'))
  })
})
