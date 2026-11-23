import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { useVisualViewportInset } from '../visualViewportInset'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

// A stub visualViewport (an EventTarget with height and offsetTop) and a manual rAF queue.
class StubVV extends EventTarget {
  height = 800
  offsetTop = 0
  listeners = { resize: 0, scroll: 0 }
  addEventListener(t: string, l: EventListenerOrEventListenerObject | null, o?: boolean | AddEventListenerOptions) {
    if (t === 'resize' || t === 'scroll') this.listeners[t]++
    super.addEventListener(t, l, o)
  }
  removeEventListener(t: string, l: EventListenerOrEventListenerObject | null, o?: boolean | EventListenerOptions) {
    if (t === 'resize' || t === 'scroll') this.listeners[t]--
    super.removeEventListener(t, l, o)
  }
}

let vv: StubVV
let frames: Array<() => void>
let appEl: HTMLElement
let host: HTMLElement
let root: Root
let seen: boolean[]
let displays: string[]

function Probe({ enabled }: { enabled: boolean }) {
  const { keyboardUp } = useVisualViewportInset(enabled)
  seen.push(keyboardUp)
  return null
}
const mount = (enabled: boolean) => act(() => root.render(React.createElement(Probe, { enabled })))
const flushFrames = () => act(() => { const f = frames.splice(0); f.forEach((cb) => cb()) })
const setInner = (h: number) => Object.defineProperty(window, 'innerHeight', { value: h, configurable: true })
const kbUp = () => seen[seen.length - 1]

beforeEach(() => {
  vi.useFakeTimers()
  vv = new StubVV()
  frames = []
  seen = []
  displays = []
  setInner(800)
  Object.defineProperty(window, 'visualViewport', { value: vv, configurable: true })
  vi.stubGlobal('requestAnimationFrame', (cb: () => void) => { frames.push(cb); return frames.length })
  vi.stubGlobal('cancelAnimationFrame', () => {})
  appEl = document.createElement('div')
  appEl.className = 'app'
  // Record every display write, to see the toggle (jsdom reflows nothing, so a bare getter would miss it).
  const style = appEl.style
  let d = ''
  Object.defineProperty(style, 'display', { get: () => d, set: (v: string) => { d = v; displays.push(v) }, configurable: true })
  document.body.appendChild(appEl)
  host = document.createElement('div')
  document.body.appendChild(host)
  root = createRoot(host)
})

afterEach(() => {
  act(() => root.unmount())
  host.remove(); appEl.remove()
  vi.useRealTimers()
  vi.unstubAllGlobals()
  Reflect.deleteProperty(window, 'visualViewport')
})

describe('useVisualViewportInset', () => {
  it('sizes the root to the visual viewport and flags the keyboard', () => {
    mount(true)
    flushFrames()
    expect(appEl.style.getPropertyValue('--vv-height')).toBe('800px')
    expect(kbUp()).toBe(false)

    vv.height = 400
    act(() => { vv.dispatchEvent(new Event('resize')) })
    flushFrames()
    expect(appEl.style.getPropertyValue('--vv-height')).toBe('400px')
    expect(kbUp()).toBe(true)

    vv.offsetTop = 50
    act(() => { vv.dispatchEvent(new Event('scroll')) })
    flushFrames()
    expect(appEl.style.getPropertyValue('--vv-top')).toBe('50px')

    vv.height = 800
    vv.offsetTop = 0
    act(() => { vv.dispatchEvent(new Event('resize')) })
    flushFrames()
    expect(kbUp()).toBe(false)
    expect(appEl.style.getPropertyValue('--vv-height')).toBe('800px')
  })

  it('reads in requestAnimationFrame, once per frame however many events arrive', () => {
    mount(true)
    flushFrames()
    vv.height = 500
    act(() => { vv.dispatchEvent(new Event('resize')); vv.dispatchEvent(new Event('scroll')) })
    expect(appEl.style.getPropertyValue('--vv-height')).toBe('800px') // not yet
    expect(frames).toHaveLength(1)
  })

  it('a shift under 100 px (browser chrome) is not a keyboard', () => {
    mount(true)
    flushFrames()
    vv.height = 740
    act(() => { vv.dispatchEvent(new Event('resize')) })
    flushFrames()
    expect(kbUp()).toBe(false)
  })

  it('disabled: no listener added and no style written', () => {
    mount(false)
    flushFrames()
    expect(vv.listeners).toEqual({ resize: 0, scroll: 0 })
    expect(appEl.style.getPropertyValue('--vv-height')).toBe('')
    expect(frames).toHaveLength(0)
  })

  it('unmount removes the listeners and the root variables', () => {
    mount(true)
    flushFrames()
    expect(vv.listeners).toEqual({ resize: 1, scroll: 1 })
    act(() => root.render(null))
    expect(vv.listeners).toEqual({ resize: 0, scroll: 0 })
    expect(appEl.style.getPropertyValue('--vv-height')).toBe('')
  })

  describe('stuck-viewport heal', () => {
    let ta: HTMLTextAreaElement
    beforeEach(() => {
      ta = document.createElement('textarea')
      ta.className = 'xterm-helper-textarea'
      document.body.appendChild(ta)
    })
    afterEach(() => ta.remove())
    const blur = () => act(() => { ta.dispatchEvent(new FocusEvent('focusout', { bubbles: true })) })

    it('toggles the root display once when the viewport stays 300 px short after the blur', () => {
      mount(true)
      flushFrames()
      vv.height = 500 // stuck at keyboard height, the maximum seen is 800
      blur()
      vi.advanceTimersByTime(139)
      expect(displays).toEqual([])
      act(() => { vi.advanceTimersByTime(1) })
      expect(displays).toEqual(['none'])
      flushFrames()
      expect(displays).toEqual(['none', ''])
      vi.advanceTimersByTime(1000)
      flushFrames()
      expect(displays).toEqual(['none', ''])
    })

    it('does nothing with no shortfall', () => {
      mount(true)
      flushFrames()
      blur()
      act(() => { vi.advanceTimersByTime(500) })
      flushFrames()
      expect(displays).toEqual([])
    })

    it('does nothing while a helper textarea is focused again', () => {
      mount(true)
      flushFrames()
      vv.height = 500
      blur()
      ta.focus()
      act(() => { vi.advanceTimersByTime(500) })
      flushFrames()
      expect(displays).toEqual([])
    })
  })
})
