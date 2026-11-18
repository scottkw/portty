// Portty 05-09 (SHAR-07): a crashing pane must not take the window with it.
// The fallback is fixed copy: an error message can carry a ?cap= URL (T-05-30).
import { describe, it, expect, afterEach, vi } from 'vitest'
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { ErrorBoundary } from '../ErrorBoundary'

const SECRET = 'wss://office-air:7444/ws?cap=SECRETCAP'

function Bomb({ when = 'render' }: { when?: 'render' | 'effect' }) {
  React.useEffect(() => { if (when === 'effect') throw new SyntaxError(SECRET) }, [when])
  if (when === 'render') throw new SyntaxError(SECRET)
  return React.createElement('div', null, 'unreachable')
}

let root: Root | undefined
let container: HTMLDivElement | undefined

function render(el: React.ReactElement) {
  if (!container) {
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  }
  act(() => { root!.render(el) })
  return container
}

function tree(child: React.ReactNode, resetKey?: unknown) {
  return React.createElement('div', null,
    React.createElement('nav', { className: 'sidebar' }, 'sidebar'),
    React.createElement(ErrorBoundary, { label: 'test', resetKey }, child))
}

describe('ErrorBoundary', () => {
  afterEach(() => {
    if (root) act(() => root!.unmount())
    container?.remove()
    root = undefined
    container = undefined
    vi.restoreAllMocks()
  })

  it('a render throw shows the fixed-copy fallback, never the message, and siblings stay', () => {
    const log = vi.spyOn(console, 'error').mockImplementation(() => {})
    const c = render(tree(React.createElement(Bomb)))
    const fb = c.querySelector('[role="status"]')
    expect(fb).not.toBeNull()
    expect(fb!.className).toContain('banner--warning')
    expect(fb!.textContent).toBe('This tab hit an error. Close it and open the session again.')
    expect(c.textContent).not.toContain('SECRETCAP')
    expect(c.querySelector('.sidebar')).not.toBeNull()
    // Its own log names only the label and error name.
    const own = log.mock.calls.filter((a) => a[0] === '[ErrorBoundary]')
    expect(own).toEqual([['[ErrorBoundary]', 'test', 'SyntaxError']])
  })

  it('an effect throw is caught too', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const c = render(tree(React.createElement(Bomb, { when: 'effect' })))
    expect(c.querySelector('[role="status"]')).not.toBeNull()
    expect(c.querySelector('.sidebar')).not.toBeNull()
  })

  it('a new resetKey renders the children again', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const c = render(tree(React.createElement(Bomb), 'a'))
    expect(c.querySelector('[role="status"]')).not.toBeNull()
    render(tree(React.createElement('p', null, 'healthy'), 'b'))
    expect(c.querySelector('[role="status"]')).toBeNull()
    expect(c.textContent).toContain('healthy')
  })

  it('a custom fallback replaces the default copy', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const c = render(React.createElement(ErrorBoundary, { label: 'root', fallback: React.createElement('p', null, 'custom') }, React.createElement(Bomb)))
    expect(c.textContent).toBe('custom')
  })
})
