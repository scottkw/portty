/**
 * Phase 175-06 (BUG-02 / #125) — SessionEndedBanner render tests.
 *
 * Fixed copy: "Session ended — the owner stopped this session."
 * Accessibility: role="status" + aria-live="polite" (mirrors WebGLRecoveryBanner).
 * Security (T-175-06-02): the raw CloseEvent.reason must NEVER be rendered
 * into the DOM, even when it is a hostile string (script tags, HTML, etc.).
 */
import { describe, it, expect, vi, afterEach } from 'vitest'
import React from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { flushSync } from 'react-dom'
import { SessionEndedBanner } from '../SessionEndedBanner'

function render(onDismiss: () => void, reason?: string) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  flushSync(() => {
    root.render(React.createElement(SessionEndedBanner, { onDismiss, reason }))
  })
  return { container, root }
}

describe('SessionEndedBanner', () => {
  let container: HTMLElement | undefined
  let root: Root | undefined

  afterEach(() => {
    if (root) {
      flushSync(() => root!.unmount())
      root = undefined
    }
    if (container) {
      container.remove()
      container = undefined
    }
    vi.clearAllMocks()
  })

  it('renders the fixed generic copy', () => {
    ;({ container, root } = render(vi.fn()))
    expect(container.textContent).toContain('Session ended — the owner stopped this session.')
  })

  it('has role="status" and aria-live="polite" (accessibility contract)', () => {
    ;({ container, root } = render(vi.fn()))
    const statusEl = container.querySelector('[role="status"]')
    expect(statusEl).not.toBeNull()
    expect(statusEl?.getAttribute('aria-live')).toBe('polite')
  })

  it('dismiss button has aria-label="Dismiss" and fires onDismiss when clicked', () => {
    const onDismiss = vi.fn()
    ;({ container, root } = render(onDismiss))
    const btn = container.querySelector('[aria-label="Dismiss"]') as HTMLButtonElement | null
    expect(btn).not.toBeNull()
    flushSync(() => { btn!.click() })
    expect(onDismiss).toHaveBeenCalledTimes(1)
  })

  it('renders no raw reason text even when a benign reason is passed', () => {
    ;({ container, root } = render(vi.fn(), 'session ended'))
    // The passed reason string must never surface verbatim in the rendered
    // text alongside the fixed copy — only the fixed message is visible.
    expect(container.textContent).toBe('Session ended — the owner stopped this session.')
  })

  it('renders no raw reason text and injects no HTML when a hostile reason string is passed (T-175-06-02)', () => {
    const hostile = '<img src=x onerror=alert(1)><script>alert(document.cookie)</script>'
    ;({ container, root } = render(vi.fn(), hostile))

    // The hostile string must not appear anywhere in the rendered text.
    expect(container.textContent).not.toContain(hostile)
    expect(container.textContent).not.toContain('<script>')
    expect(container.textContent).not.toContain('onerror')
    // No injected <script> or <img> elements — dangerouslySetInnerHTML was
    // never used to render the reason.
    expect(container.querySelector('script')).toBeNull()
    expect(container.querySelector('img')).toBeNull()
    // The fixed copy is still the only visible text.
    expect(container.textContent).toBe('Session ended — the owner stopped this session.')
  })

  it('does not throw when reason is omitted', () => {
    expect(() => {
      ;({ container, root } = render(vi.fn()))
    }).not.toThrow()
  })

  it('is the info variant of the shared banner with an svg icon and an XMarkIcon Dismiss (Portty 02-10)', () => {
    ;({ container, root } = render(vi.fn()))
    const banner = container.querySelector('.banner')!
    expect(banner.classList.contains('banner--info')).toBe(true)
    expect(banner.querySelector('svg.banner__icon')?.getAttribute('aria-hidden')).toBe('true')
    const dismiss = banner.querySelector('.banner__dismiss')!
    expect(dismiss.getAttribute('aria-label')).toBe('Dismiss')
    expect(dismiss.querySelector('svg')).not.toBeNull()
    expect(dismiss.textContent).toBe('')
    expect(container.textContent).not.toContain('×')
  })
})
