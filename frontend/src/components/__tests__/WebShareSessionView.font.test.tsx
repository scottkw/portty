// Phase 14-11 (MOB-01): the browser guest's font (a pinch, or Shift+=/-) lives in memory only.
// TerminalPanel is mocked; its props are captured.
import { describe, it, expect, vi, afterEach } from 'vitest'
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const hoisted = vi.hoisted(() => ({ props: null as null | { fontSize: number; onFontSizeChange: (d: number) => void } }))
vi.mock('../TerminalPanel', () => ({
  TerminalPanel: (props: typeof hoisted.props) => {
    hoisted.props = props
    return null
  },
}))

import { WebShareSessionView } from '../WebShareSessionView'
import { DEFAULT_TERMINAL_FONT_SIZE } from '../../lib/terminalFont'

let root: Root | null = null
let container: HTMLDivElement | null = null

function mount(props: Partial<React.ComponentProps<typeof WebShareSessionView>> = {}) {
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  act(() => root!.render(<WebShareSessionView sessionId="s" capToken="c" relayPort={0} pluginConfig={null} {...props} />))
}
const unmount = () => { act(() => root?.unmount()); container?.remove(); root = null; container = null }

afterEach(() => { unmount(); hoisted.props = null })

describe('WebShareSessionView browser-guest font', () => {
  it('+3 from the host font renders the panel 3 px larger, and a remount returns to the host size', () => {
    mount()
    expect(hoisted.props!.fontSize).toBe(DEFAULT_TERMINAL_FONT_SIZE)
    act(() => hoisted.props!.onFontSizeChange(3))
    expect(hoisted.props!.fontSize).toBe(DEFAULT_TERMINAL_FONT_SIZE + 3)
    unmount()
    mount()
    expect(hoisted.props!.fontSize).toBe(DEFAULT_TERMINAL_FONT_SIZE)
  })

  it('several deltas before a render add up, and the size clamps to 6..32', () => {
    mount()
    const change = hoisted.props!.onFontSizeChange // the first closure, as Shift+= and a pinch hold it
    act(() => { change(1); change(1); change(1) })
    expect(hoisted.props!.fontSize).toBe(DEFAULT_TERMINAL_FONT_SIZE + 3)
    act(() => change(100))
    expect(hoisted.props!.fontSize).toBe(32)
    act(() => change(-100))
    expect(hoisted.props!.fontSize).toBe(6)
  })

  it('an explicit fontSize prop (a desktop peer tab) wins and its onFontSizeChange is used as given', () => {
    const spy = vi.fn()
    mount({ fontSize: 15, fontFamily: 'x', onFontSizeChange: spy })
    expect(hoisted.props!.fontSize).toBe(15)
    act(() => hoisted.props!.onFontSizeChange(1))
    expect(spy).toHaveBeenCalledWith(1)
    expect(hoisted.props!.fontSize).toBe(15)
  })
})
