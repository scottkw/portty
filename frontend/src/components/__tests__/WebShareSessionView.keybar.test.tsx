// Phase 14-10 (MOB-01): when WebShareSessionView renders the key bar, and its one-shot Ctrl on
// the keyboard input path (inputTransform). TerminalPanel is mocked; its props are captured.
import { describe, it, expect, vi, afterEach } from 'vitest'
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const hoisted = vi.hoisted(() => ({ props: null as null | { inputTransform?: (d: string) => string; keyInputRef?: { current: unknown } } }))
vi.mock('../TerminalPanel', () => ({
  TerminalPanel: (props: typeof hoisted.props) => {
    hoisted.props = props
    return null
  },
}))

import { WebShareSessionView } from '../WebShareSessionView'
import { PHONE_QUERY } from '../../lib/phoneQuery'

const realMatchMedia = window.matchMedia
let root: Root | null = null
let container: HTMLDivElement | null = null

function phone(on: boolean) {
  window.matchMedia = ((media: string) => ({
    matches: on && media === PHONE_QUERY, media, addEventListener() {}, removeEventListener() {},
  })) as unknown as typeof window.matchMedia
}
function mount(props: Partial<React.ComponentProps<typeof WebShareSessionView>> = {}) {
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  const render = (p: typeof props) => act(() => root!.render(
    <WebShareSessionView sessionId="s" capToken="c" relayPort={0} pluginConfig={null} {...p} />,
  ))
  render(props)
  return render
}
const bar = () => container!.querySelector('[role="toolbar"]')
const ctrlBtn = () => container!.querySelector<HTMLButtonElement>('button[aria-pressed]')!
const arm = () => act(() => { ctrlBtn().dispatchEvent(new MouseEvent('pointerdown', { bubbles: true, cancelable: true })) })

afterEach(() => {
  act(() => root?.unmount()); container?.remove(); root = null; container = null
  window.matchMedia = realMatchMedia
  hoisted.props = null
})

describe('WebShareSessionView key bar', () => {
  it('renders on a phone with write', () => {
    phone(true)
    mount({ canWrite: true })
    expect(bar()).not.toBeNull()
  })
  it('renders on a phone when the capability is unknown (info call failed)', () => {
    phone(true)
    mount()
    expect(bar()).not.toBeNull()
  })
  it('is absent for a capability without write', () => {
    phone(true)
    mount({ canWrite: false })
    expect(bar()).toBeNull()
  })
  it('is absent on a desktop peer tab (remote)', () => {
    phone(true)
    mount({ remote: true, canWrite: true })
    expect(bar()).toBeNull()
  })
  it('is absent while a preview is active (isActive false), and returns when active again', () => {
    phone(true)
    const render = mount({ isActive: false })
    expect(bar()).toBeNull()
    render({ isActive: true })
    expect(bar()).not.toBeNull()
  })
  it('is absent when the phone query does not match', () => {
    phone(false)
    mount({ canWrite: true })
    expect(bar()).toBeNull()
  })

  it('keyboardUp reaches the bar as keybar--kb-up', () => {
    phone(true)
    const render = mount({ canWrite: true })
    expect(bar()!.classList.contains('keybar--kb-up')).toBe(false)
    render({ canWrite: true, keyboardUp: true })
    expect(bar()!.classList.contains('keybar--kb-up')).toBe(true)
  })

  describe('inputTransform', () => {
    it('passes data through when Ctrl is not armed', () => {
      phone(true)
      mount({ canWrite: true })
      expect(hoisted.props!.inputTransform!('c')).toBe('c')
    })
    // The transform runs inside xterm's onData (outside React), and sets state: wrap it in act.
    const typed = (d: string) => { let out = ''; act(() => { out = hoisted.props!.inputTransform!(d) }); return out }

    it('maps c to 0x03 once when armed, then Ctrl is released', () => {
      phone(true)
      mount({ canWrite: true })
      arm()
      expect(ctrlBtn().getAttribute('aria-pressed')).toBe('true')
      expect(typed('c')).toBe('\x03')
      expect(ctrlBtn().getAttribute('aria-pressed')).toBe('false')
      expect(typed('c')).toBe('c')
    })
    it('a character with no control code goes as typed and still releases Ctrl', () => {
      phone(true)
      mount({ canWrite: true })
      arm()
      expect(typed('é')).toBe('é')
      expect(ctrlBtn().getAttribute('aria-pressed')).toBe('false')
    })
    it('releases Ctrl when a preview takes focus', () => {
      phone(true)
      const render = mount({ canWrite: true })
      arm()
      render({ canWrite: true, isActive: false })
      render({ canWrite: true, isActive: true })
      expect(ctrlBtn().getAttribute('aria-pressed')).toBe('false')
    })
  })
})
