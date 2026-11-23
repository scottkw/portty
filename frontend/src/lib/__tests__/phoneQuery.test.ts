// Phase 14 (MOB-03): the one module that decides "phone" and "Home Screen app".
import { describe, it, expect, afterEach, vi } from 'vitest'
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { PHONE_QUERY, isPhone, usePhone, isStandalone } from '../phoneQuery'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

type Listener = (e: { matches: boolean }) => void

// Stubs matchMedia: queries in `matching` match; 'change' listeners are recorded per query.
function stubMedia(matching: string[]) {
  const listeners = new Map<string, Set<Listener>>()
  const spy = vi.fn((q: string) => ({
    matches: matching.includes(q),
    media: q,
    addEventListener: (_: string, l: Listener) => {
      if (!listeners.has(q)) listeners.set(q, new Set())
      listeners.get(q)!.add(l)
    },
    removeEventListener: (_: string, l: Listener) => { listeners.get(q)?.delete(l) },
  }))
  vi.stubGlobal('matchMedia', spy)
  window.matchMedia = spy as unknown as typeof window.matchMedia
  return listeners
}

describe('phoneQuery', () => {
  const original = window.matchMedia
  afterEach(() => {
    window.matchMedia = original
    vi.unstubAllGlobals()
    Object.defineProperty(navigator, 'standalone', { value: undefined, configurable: true })
  })

  it('PHONE_QUERY is a coarse pointer with no hover', () => {
    expect(PHONE_QUERY).toBe('(pointer: coarse) and (hover: none)')
  })

  it('isPhone follows matchMedia for PHONE_QUERY', () => {
    stubMedia([PHONE_QUERY])
    expect(isPhone()).toBe(true)
    stubMedia([])
    expect(isPhone()).toBe(false)
  })

  it('usePhone re-renders on a change event and removes its listener on unmount', () => {
    const listeners = stubMedia([])
    const seen: boolean[] = []
    function Probe() {
      const p = usePhone()
      seen.push(p)
      return null
    }
    const container = document.createElement('div')
    const root: Root = createRoot(container)
    act(() => { root.render(React.createElement(Probe)) })
    expect(seen[seen.length - 1]).toBe(false)
    expect(listeners.get(PHONE_QUERY)!.size).toBe(1)
    act(() => { for (const l of listeners.get(PHONE_QUERY)!) l({ matches: true }) })
    expect(seen[seen.length - 1]).toBe(true)
    act(() => root.unmount())
    expect(listeners.get(PHONE_QUERY)!.size).toBe(0)
  })

  it('isStandalone is true for display-mode: standalone or navigator.standalone, false otherwise', () => {
    stubMedia([])
    expect(isStandalone()).toBe(false)
    stubMedia(['(display-mode: standalone)'])
    expect(isStandalone()).toBe(true)
    stubMedia([])
    Object.defineProperty(navigator, 'standalone', { value: true, configurable: true })
    expect(isStandalone()).toBe(true)
    Object.defineProperty(navigator, 'standalone', { value: false, configurable: true })
    expect(isStandalone()).toBe(false)
  })
})
