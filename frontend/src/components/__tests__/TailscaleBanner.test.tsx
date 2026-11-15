import { describe, it, expect, afterEach, vi } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { flushSync } from 'react-dom'

const openTailscale = vi.fn(() => Promise.resolve())
vi.mock('../../wailsjs/go/main/App', () => ({
  OpenTailscale: () => openTailscale(),
}))

import { TailscaleBanner } from '../TailscaleBanner'

describe('NET-01: TailscaleBanner', () => {
  let container: HTMLElement
  let root: ReturnType<typeof createRoot>

  afterEach(() => {
    root.unmount()
    container.remove()
    openTailscale.mockClear()
  })

  function render() {
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
    flushSync(() => root.render(React.createElement(TailscaleBanner)))
  }

  it('shows the exact NET-01 sentence', () => {
    render()
    expect(container.querySelector('.banner__message')!.textContent).toBe(
      "Tailscale isn't connected. Remote access and sharing are off until it is."
    )
  })

  it('is a warning-variant status region in the shared banner style (LOOK-01)', () => {
    render()
    const banner = container.querySelector('.banner')!
    expect(banner.getAttribute('role')).toBe('status')
    expect(banner.classList.contains('banner--warning')).toBe(true)
  })

  // Portty 02-08: the toolbar Share button's aria-describedby target.
  it('the sentence carries id="ts-off-reason"', () => {
    render()
    expect(container.querySelector('#ts-off-reason')?.textContent).toBe(
      "Tailscale isn't connected. Remote access and sharing are off until it is."
    )
  })

  it('uses an aria-hidden svg warning icon, not the ⚠ glyph', () => {
    render()
    expect(container.textContent).not.toContain('⚠')
    expect(container.querySelector('svg.banner__icon')?.getAttribute('aria-hidden')).toBe('true')
  })

  it('has no dismiss button; its only button is "Open Tailscale"', () => {
    render()
    expect(container.querySelector('.banner__dismiss')).toBeNull()
    expect(container.querySelector('[aria-label="Dismiss"]')).toBeNull()
    const buttons = container.querySelectorAll('button')
    expect(buttons).toHaveLength(1)
    expect(buttons[0].textContent).toBe('Open Tailscale')
    expect(buttons[0].classList.contains('banner__action')).toBe(true)
  })

  it('"Open Tailscale" calls the OpenTailscale binding (Portty 02-10)', () => {
    render()
    container.querySelector<HTMLButtonElement>('.banner__action')!.click()
    expect(openTailscale).toHaveBeenCalledTimes(1)
  })

  it('a failing OpenTailscale is caught and logged, not thrown', async () => {
    const err = vi.spyOn(console, 'error').mockImplementation(() => {})
    openTailscale.mockImplementationOnce(() => Promise.reject(new Error('boom')))
    render()
    container.querySelector<HTMLButtonElement>('.banner__action')!.click()
    await vi.waitFor(() => expect(err).toHaveBeenCalled())
    err.mockRestore()
  })
})
