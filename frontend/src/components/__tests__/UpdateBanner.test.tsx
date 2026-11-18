import { describe, it, expect, vi, afterEach } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { flushSync } from 'react-dom'
import { UpdateBanner } from '../UpdateBanner'
import type { UpdateStatus } from '../UpdateBanner'

const available: UpdateStatus = { state: 'available', current: '1.0.0', latest: '2.0.0', done: 0, total: 0, error: '' }

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

  afterEach(() => {
    root?.unmount()
    container?.remove()
    vi.clearAllMocks()
  })

  it('renders Dismiss button with correct aria-label', () => {
    ;({ container, root } = renderUpdateBanner(available, vi.fn()))
    const dismissBtn = container.querySelector('.banner__dismiss')
    expect(dismissBtn).not.toBeNull()
    expect(dismissBtn?.getAttribute('aria-label')).toBe('Dismiss')
  })

  it('calls onDismiss when Dismiss button clicked', () => {
    const onDismiss = vi.fn()
    ;({ container, root } = renderUpdateBanner(available, onDismiss))
    const dismissBtn = container.querySelector('.banner__dismiss') as HTMLButtonElement
    flushSync(() => { dismissBtn.click() })
    expect(onDismiss).toHaveBeenCalledOnce()
  })

  it('has role="alert" for accessibility', () => {
    ;({ container, root } = renderUpdateBanner(available, vi.fn()))
    const alertEl = container.querySelector('[role="alert"]')
    expect(alertEl).not.toBeNull()
  })

  it('has aria-live="polite"', () => {
    ;({ container, root } = renderUpdateBanner(available, vi.fn()))
    const alertEl = container.querySelector('[aria-live="polite"]')
    expect(alertEl).not.toBeNull()
  })

  it('applies className when provided (for banner-exit animation)', () => {
    ;({ container, root } = renderUpdateBanner(available, vi.fn(), 'banner-exit'))
    const banner = container.querySelector('.banner')
    expect(banner?.classList.contains('banner-exit')).toBe(true)
  })

  it('is the info variant of the shared banner with an InformationCircleIcon (Portty 02-10)', () => {
    ;({ container, root } = renderUpdateBanner(available, vi.fn()))
    const banner = container.querySelector('.banner')!
    expect(banner.classList.contains('banner--info')).toBe(true)
    expect(banner.querySelector('svg.banner__icon')?.getAttribute('aria-hidden')).toBe('true')
  })

  it('dismisses with an XMarkIcon button, not a text label or ×', () => {
    ;({ container, root } = renderUpdateBanner(available, vi.fn()))
    const dismiss = container.querySelector('.banner__dismiss')!
    expect(dismiss.querySelector('svg')).not.toBeNull()
    expect(dismiss.textContent).toBe('')
  })
})
