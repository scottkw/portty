import { describe, it, expect, vi, afterEach } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { flushSync } from 'react-dom'
import { UpdateBanner } from '../UpdateBanner'
import type { UpdateStatus } from '../UpdateBanner'

// Any URL opener in the runtime would show up here; the banner must never call one (T-06-14).
vi.mock('../../wailsjs/wailsjs/runtime/runtime', () => ({ BrowserOpenURL: vi.fn() }))
import { BrowserOpenURL } from '../../wailsjs/wailsjs/runtime/runtime'

const available: UpdateStatus = { state: 'available', current: '1.0.0', latest: '2.0.0', done: 0, total: 0, error: '' }
const signatureError = "The download didn't pass the signature check, so Portty deleted it."
const revealedError = "Portty couldn't install the update, so the download is shown in Finder."

const states: UpdateStatus[] = [
  available,
  { ...available, state: 'downloading', done: 10, total: 40 },
  { ...available, state: 'verifying' },
  { ...available, state: 'revealed' },
  { ...available, error: signatureError },
  { ...available, state: 'installing' },
  { ...available, state: 'relaunching' },
  { ...available, state: 'revealed', error: revealedError },
]

function renderUpdateBanner(
  status: UpdateStatus,
  onDismiss: () => void,
  className?: string,
  onDownload: () => void = vi.fn(),
) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  flushSync(() => {
    root.render(React.createElement(UpdateBanner, { status, onDownload, onDismiss, className }))
  })
  return { container, root }
}

describe('UpdateBanner', () => {
  let container: HTMLElement
  let root: ReturnType<typeof createRoot>

  const button = (label: string) =>
    [...container.querySelectorAll('button')].find((b) => b.textContent === label) as HTMLButtonElement | undefined

  afterEach(() => {
    root?.unmount()
    container?.remove()
    vi.clearAllMocks()
  })

  it('calls onDismiss when Dismiss button clicked', () => {
    const onDismiss = vi.fn()
    ;({ container, root } = renderUpdateBanner(available, onDismiss))
    const dismissBtn = container.querySelector('.banner__dismiss') as HTMLButtonElement
    flushSync(() => { dismissBtn.click() })
    expect(onDismiss).toHaveBeenCalledOnce()
  })

  it('is the info variant of the shared banner with an InformationCircleIcon (Portty 02-10)', () => {
    ;({ container, root } = renderUpdateBanner(available, vi.fn()))
    const banner = container.querySelector('.banner')!
    expect(banner.classList.contains('banner--info')).toBe(true)
    expect(banner.querySelector('svg.banner__icon')?.getAttribute('aria-hidden')).toBe('true')
  })

  it('available shows "Portty X is available." and Update calls onDownload', () => {
    const onDownload = vi.fn()
    ;({ container, root } = renderUpdateBanner(available, vi.fn(), undefined, onDownload))
    expect(container.textContent).toContain('Portty 2.0.0 is available.')
    expect(button('Download')).toBeUndefined()
    flushSync(() => { button('Update')!.click() })
    expect(onDownload).toHaveBeenCalledOnce()
  })

  it('downloading shows the percent and a progress bar', () => {
    ;({ container, root } = renderUpdateBanner(states[1], vi.fn()))
    expect(container.textContent).toContain('Downloading Portty 2.0.0… 25%')
    const p = container.querySelector('progress.banner__progress') as HTMLProgressElement
    expect(p.getAttribute('aria-label')).toBe('Download progress')
    expect(p.value).toBe(10)
    expect(p.max).toBe(40)
    expect(button('Update')).toBeUndefined()
  })

  it('downloading with total 0 shows 0% and no NaN', () => {
    ;({ container, root } = renderUpdateBanner({ ...available, state: 'downloading' }, vi.fn()))
    expect(container.textContent).toContain('Downloading Portty 2.0.0… 0%')
    expect(container.textContent).not.toContain('NaN')
  })

  it('verifying shows its text with no action button', () => {
    ;({ container, root } = renderUpdateBanner(states[2], vi.fn()))
    expect(container.textContent).toContain('Verifying Portty 2.0.0…')
    expect(container.querySelector('.banner__action')).toBeNull()
  })

  it('revealed shows "verified and shown in Finder" with no action button', () => {
    ;({ container, root } = renderUpdateBanner(states[3], vi.fn()))
    expect(container.textContent).toContain('Portty 2.0.0 is verified and shown in Finder.')
    expect(container.querySelector('.banner__action')).toBeNull()
  })

  it('installing shows its text on the info variant with no action button, dismiss kept', () => {
    ;({ container, root } = renderUpdateBanner(states[5], vi.fn()))
    expect(container.textContent).toContain('Installing Portty 2.0.0…')
    expect(container.querySelector('.banner')!.classList.contains('banner--info')).toBe(true)
    expect(container.querySelector('.banner__action')).toBeNull()
    expect(container.querySelector('.banner__dismiss')).not.toBeNull()
  })

  it('relaunching shows its text on the info variant with no action button', () => {
    ;({ container, root } = renderUpdateBanner(states[6], vi.fn()))
    expect(container.textContent).toContain('Restarting Portty 2.0.0…')
    expect(container.querySelector('.banner')!.classList.contains('banner--info')).toBe(true)
    expect(container.querySelector('.banner__action')).toBeNull()
  })

  it("revealed with an error shows Go's text on the warning variant with no action button", () => {
    ;({ container, root } = renderUpdateBanner(states[7], vi.fn()))
    expect(container.textContent).toContain(revealedError)
    expect(container.textContent).not.toContain('verified and shown in Finder')
    const banner = container.querySelector('.banner')!
    expect(banner.classList.contains('banner--warning')).toBe(true)
    expect(banner.classList.contains('banner--info')).toBe(false)
    expect(container.querySelector('.banner__action')).toBeNull()
  })

  it("an error shows Go's text on the warning variant with Try Again calling onDownload", () => {
    const onDownload = vi.fn()
    ;({ container, root } = renderUpdateBanner(states[4], vi.fn(), undefined, onDownload))
    expect(container.textContent).toContain(signatureError)
    expect(container.textContent).not.toContain('is available')
    const banner = container.querySelector('.banner')!
    expect(banner.classList.contains('banner--warning')).toBe(true)
    expect(banner.classList.contains('banner--info')).toBe(false)
    expect(button('Update')).toBeUndefined()
    flushSync(() => { button('Try Again')!.click() })
    expect(onDownload).toHaveBeenCalledOnce()
  })

  it.each(states.map((s) => [s.error ? `${s.state} error` : s.state, s] as const))(
    '%s keeps the XMarkIcon Dismiss, role alert, aria-live polite and className, and opens no URL',
    (_name, status) => {
      ;({ container, root } = renderUpdateBanner(status, vi.fn(), 'banner-exit'))
      const banner = container.querySelector('.banner')!
      expect(banner.getAttribute('role')).toBe('alert')
      expect(banner.getAttribute('aria-live')).toBe('polite')
      expect(banner.classList.contains('banner-exit')).toBe(true)
      expect(banner.querySelector('svg.banner__icon')).not.toBeNull()
      const dismiss = container.querySelector('.banner__dismiss') as HTMLButtonElement
      expect(dismiss.getAttribute('aria-label')).toBe('Dismiss')
      expect(dismiss.textContent).toBe('')
      expect(dismiss.querySelector('svg')).not.toBeNull()
      expect(container.querySelector('a')).toBeNull()
      for (const b of container.querySelectorAll('button')) flushSync(() => { (b as HTMLButtonElement).click() })
      expect(BrowserOpenURL).not.toHaveBeenCalled()
    },
  )
})
