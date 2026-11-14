import { describe, it, expect, vi, afterEach } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { act } from 'react'
import { HubFilterBar } from './HubFilterBar'

// Render helper
function renderFilterBar(
  overrides: Partial<React.ComponentProps<typeof HubFilterBar>> = {},
) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  const defaultProps: React.ComponentProps<typeof HubFilterBar> = {
    searchText: '',
    searchRef: React.createRef<HTMLInputElement>(),
    onSearchChange: vi.fn(),
    onNewSession: vi.fn(),
  }
  const props = { ...defaultProps, ...overrides }
  act(() => {
    root.render(<HubFilterBar {...props} />)
  })
  return { container, root, ...props }
}

describe('HubFilterBar — no status pills (STRP-01)', () => {
  let container: HTMLElement
  let root: ReturnType<typeof createRoot>

  afterEach(() => {
    root.unmount()
    container.remove()
  })

  it('renders only the search input and the New session button', () => {
    ;({ container, root } = renderFilterBar())
    expect(container.querySelectorAll('.hub-filter__pill')).toHaveLength(0)
    expect(container.querySelector('[role="group"]')).toBeNull()
    expect(container.querySelectorAll('input')).toHaveLength(1)
    const buttons = Array.from(container.querySelectorAll('button')).map((b) => b.textContent?.trim())
    expect(buttons).toEqual(['New session'])
  })
})

describe('HubFilterBar — search input', () => {
  let container: HTMLElement
  let root: ReturnType<typeof createRoot>

  afterEach(() => {
    root.unmount()
    container.remove()
  })

  it('renders a search input with correct placeholder', () => {
    ;({ container, root } = renderFilterBar())
    const input = container.querySelector('.hub-filter__search') as HTMLInputElement
    expect(input).not.toBeNull()
    expect(input.placeholder).toBe('Search sessions…')
  })

  it('renders the search input with correct aria-label', () => {
    ;({ container, root } = renderFilterBar())
    const input = container.querySelector('.hub-filter__search') as HTMLInputElement
    expect(input.getAttribute('aria-label')).toBe('Search sessions by name or host')
  })

  it('typing in the search input fires onSearchChange', () => {
    const onSearchChange = vi.fn()
    ;({ container, root } = renderFilterBar({ onSearchChange }))
    const input = container.querySelector('.hub-filter__search') as HTMLInputElement
    act(() => {
      // Simulate change event
      Object.defineProperty(input, 'value', { writable: true, value: 'my-session' })
      input.dispatchEvent(new Event('input', { bubbles: true }))
    })
    // React uses synthetic onChange — fire a React change event
    act(() => {
      const nativeInputValueSetter = Object.getOwnPropertyDescriptor(
        window.HTMLInputElement.prototype,
        'value',
      )?.set
      nativeInputValueSetter?.call(input, 'my-session')
      input.dispatchEvent(new Event('change', { bubbles: true }))
    })
    // The value is bound — check that firing the event triggers the callback
    // We test via the ref-controlled pattern by directly verifying the handler
    expect(onSearchChange).toHaveBeenCalledWith('my-session')
  })

  it('pressing Escape in the search input fires onSearchChange("") and blurs', () => {
    const onSearchChange = vi.fn()
    ;({ container, root } = renderFilterBar({ searchText: 'hello', onSearchChange }))
    const input = container.querySelector('.hub-filter__search') as HTMLInputElement
    const blurSpy = vi.spyOn(input, 'blur')
    act(() => {
      input.dispatchEvent(
        new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }),
      )
    })
    expect(onSearchChange).toHaveBeenCalledWith('')
    expect(blurSpy).toHaveBeenCalled()
  })

  it('search input reflects the searchText prop (controlled)', () => {
    ;({ container, root } = renderFilterBar({ searchText: 'my search' }))
    const input = container.querySelector('.hub-filter__search') as HTMLInputElement
    expect(input.value).toBe('my search')
  })
})

describe('HubFilterBar — New session button', () => {
  let container: HTMLElement
  let root: ReturnType<typeof createRoot>

  afterEach(() => {
    root.unmount()
    container.remove()
  })

  it('renders a "New session" button', () => {
    ;({ container, root } = renderFilterBar())
    const btn = Array.from(container.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'New session',
    )
    expect(btn).toBeDefined()
  })

  it('clicking "New session" fires onNewSession', () => {
    const onNewSession = vi.fn()
    ;({ container, root } = renderFilterBar({ onNewSession }))
    const btn = Array.from(container.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'New session',
    ) as HTMLButtonElement
    act(() => {
      btn.click()
    })
    expect(onNewSession).toHaveBeenCalledTimes(1)
  })
})

describe('HubFilterBar — searchRef forwarding', () => {
  let container: HTMLElement
  let root: ReturnType<typeof createRoot>

  afterEach(() => {
    root.unmount()
    container.remove()
  })

  it('forwards searchRef to the search input element', () => {
    const searchRef = React.createRef<HTMLInputElement>()
    ;({ container, root } = renderFilterBar({ searchRef }))
    expect(searchRef.current).not.toBeNull()
    expect(searchRef.current?.tagName).toBe('INPUT')
    expect(searchRef.current?.classList.contains('hub-filter__search')).toBe(true)
  })
})
