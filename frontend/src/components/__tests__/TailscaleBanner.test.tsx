import { describe, it, expect, afterEach } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { flushSync } from 'react-dom'
import { TailscaleBanner } from '../TailscaleBanner'

describe('NET-01: TailscaleBanner', () => {
  let container: HTMLElement
  let root: ReturnType<typeof createRoot>

  afterEach(() => {
    root.unmount()
    container.remove()
  })

  function render() {
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
    flushSync(() => root.render(React.createElement(TailscaleBanner)))
  }

  it('shows the exact NET-01 sentence', () => {
    render()
    expect(container.querySelector('.tailscale-banner__message')!.textContent).toBe(
      "Tailscale isn't connected. Remote access and sharing are off until it is."
    )
  })

  it('is a status region', () => {
    render()
    expect(container.querySelector('.tailscale-banner')!.getAttribute('role')).toBe('status')
  })

  it('has no dismiss (or any other) button', () => {
    render()
    expect(container.querySelector('button')).toBeNull()
  })
})
