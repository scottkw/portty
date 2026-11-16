import { describe, it, expect, vi, afterEach } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { act } from 'react'
import { useColorScheme } from '../useColorScheme'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

// Minimal renderHook: no @testing-library in this repo.
type Override = 'system' | 'light' | 'dark'

function renderHook(override?: Override) {
  const result = { current: '' as 'light' | 'dark' | '' }
  function Probe({ o }: { o?: Override }) {
    result.current = useColorScheme(o)
    return null
  }
  const root = createRoot(document.createElement('div'))
  act(() => root.render(React.createElement(Probe, { o: override })))
  return {
    result,
    rerender: (o: Override) => act(() => root.render(React.createElement(Probe, { o }))),
    unmount: () => act(() => root.unmount()),
  }
}

type Listener = (e: MediaQueryListEvent) => void

function stubMatchMedia(initial: boolean) {
  const listeners = new Set<Listener>()
  const mql = {
    matches: initial,
    media: '(prefers-color-scheme: dark)',
    addEventListener: vi.fn((_: string, l: Listener) => listeners.add(l)),
    removeEventListener: vi.fn((_: string, l: Listener) => listeners.delete(l)),
  }
  vi.stubGlobal('matchMedia', vi.fn(() => mql))
  const fire = (matches: boolean) => {
    mql.matches = matches
    listeners.forEach((l) => l({ matches } as MediaQueryListEvent))
  }
  return { mql, listeners, fire }
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('useColorScheme (THEM-01)', () => {
  it('starts from the current matchMedia value', () => {
    stubMatchMedia(true)
    expect(renderHook().result.current).toBe('dark')
    stubMatchMedia(false)
    expect(renderHook().result.current).toBe('light')
  })

  it('flips light to dark on a change event', () => {
    const { fire } = stubMatchMedia(false)
    const { result } = renderHook()
    act(() => fire(true))
    expect(result.current).toBe('dark')
  })

  it('rapid changes leave the last value in effect', () => {
    const { fire } = stubMatchMedia(false)
    const { result } = renderHook()
    act(() => {
      fire(true)
      fire(false)
      fire(true)
      fire(false)
    })
    expect(result.current).toBe('light')
  })

  it('removes the same handler on unmount', () => {
    const { mql, listeners } = stubMatchMedia(false)
    const { unmount } = renderHook()
    const added = mql.addEventListener.mock.calls[0][1]
    unmount()
    expect(mql.removeEventListener).toHaveBeenCalledWith('change', added)
    expect(listeners.size).toBe(0)
  })

  // Portty 02-13 THEM-01: the Settings ▸ Appearance override.
  describe('override', () => {
    it("'dark' returns dark while the system is light", () => {
      stubMatchMedia(false)
      expect(renderHook('dark').result.current).toBe('dark')
    })

    it("'light' returns light while the system is dark", () => {
      stubMatchMedia(true)
      expect(renderHook('light').result.current).toBe('light')
    })

    it("'system' follows matchMedia changes", () => {
      const { fire } = stubMatchMedia(false)
      const { result } = renderHook('system')
      expect(result.current).toBe('light')
      act(() => fire(true))
      expect(result.current).toBe('dark')
    })

    it("switching from 'dark' back to 'system' returns to the media value", () => {
      const { fire } = stubMatchMedia(false)
      const { result, rerender } = renderHook('dark')
      act(() => fire(false)) // a media change under an override is still tracked
      expect(result.current).toBe('dark')
      rerender('system')
      expect(result.current).toBe('light')
    })
  })
})
