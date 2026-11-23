// Phase 14-10 (MOB-01): KeyBar firing rule, bytes and one-shot Ctrl.
import { describe, it, expect, vi, afterEach } from 'vitest'
import { act, useState } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { KeyBar } from '../KeyBar'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

let root: Root | null = null
let container: HTMLDivElement | null = null
afterEach(() => { act(() => root?.unmount()); container?.remove(); root = null; container = null })

function mount(opts: { appCursor?: boolean } = {}) {
  const send = vi.fn()
  function Harness() {
    const [armed, setArmed] = useState(false)
    return <KeyBar send={send} applicationCursor={() => opts.appCursor ?? false} ctrlArmed={armed} onCtrlChange={setArmed} />
  }
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  act(() => root!.render(<Harness />))
  const btn = (name: string) => Array.from(container!.querySelectorAll<HTMLButtonElement>('button')).find((b) => (b.getAttribute('aria-label') ?? b.textContent) === name)!
  return { send, btn }
}

// jsdom has no PointerEvent: a MouseEvent named pointerdown reaches React's onPointerDown.
function pointerDown(el: HTMLElement) {
  const ev = new MouseEvent('pointerdown', { bubbles: true, cancelable: true })
  act(() => { el.dispatchEvent(ev) })
  return ev
}
// React derives onPointerLeave from a native pointerout with no relatedTarget, not from a native pointerleave.
const fireEvt = (el: HTMLElement, type: string) => act(() => { el.dispatchEvent(new MouseEvent(type, { bubbles: true, cancelable: true })) })
const click = (el: HTMLElement, detail: number) => act(() => { el.dispatchEvent(new MouseEvent('click', { bubbles: true, detail })) })

describe('KeyBar', () => {
  it('adds keybar--kb-up only while the keyboard is up', () => {
    const render = (keyboardUp?: boolean) => {
      container = document.createElement('div')
      document.body.appendChild(container)
      root = createRoot(container)
      act(() => root!.render(<KeyBar send={() => {}} applicationCursor={() => false} ctrlArmed={false} onCtrlChange={() => {}} keyboardUp={keyboardUp} />))
      return container.querySelector('.keybar')!
    }
    expect(render().className).toBe('keybar')
    act(() => root!.unmount()); container!.remove()
    expect(render(true).className).toBe('keybar keybar--kb-up')
  })

  it('is a labelled horizontal toolbar of eight keys in order', () => {
    mount()
    const bar = container!.querySelector('[role="toolbar"]')!
    expect(bar.getAttribute('aria-label')).toBe('Terminal keys')
    expect(bar.getAttribute('aria-orientation')).toBe('horizontal')
    const names = Array.from(bar.querySelectorAll('button')).map((b) => b.getAttribute('aria-label') ?? b.textContent)
    expect(names).toEqual(['Esc', 'Tab', 'Ctrl', 'Left arrow', 'Up arrow', 'Down arrow', 'Right arrow', 'Vertical bar'])
  })

  const table: Array<[string, string]> = [
    ['Esc', '\x1b'], ['Tab', '\t'], ['Left arrow', '\x1b[D'], ['Up arrow', '\x1b[A'],
    ['Down arrow', '\x1b[B'], ['Right arrow', '\x1b[C'], ['Vertical bar', '|'],
  ]
  for (const [name, bytes] of table) {
    it(`pointerdown on ${name} sends its bytes once and prevents default`, () => {
      const { send, btn } = mount()
      const ev = pointerDown(btn(name))
      expect(send).toHaveBeenCalledTimes(1)
      expect(send).toHaveBeenCalledWith(bytes)
      expect(ev.defaultPrevented).toBe(true)
    })
  }

  it('pointerdown adds keybar__key--down; pointerup, pointercancel and leave (pointerout) remove it', () => {
    const { btn } = mount()
    for (const end of ['pointerup', 'pointercancel', 'pointerout']) {
      expect(btn('Esc').classList.contains('keybar__key--down')).toBe(false)
      pointerDown(btn('Esc'))
      expect(btn('Esc').classList.contains('keybar__key--down')).toBe(true)
      expect(btn('Tab').classList.contains('keybar__key--down')).toBe(false)
      fireEvt(btn('Esc'), end)
      expect(btn('Esc').classList.contains('keybar__key--down')).toBe(false)
    }
  })

  it('the | glyph is aria-hidden and the accessible name stays Vertical bar', () => {
    const { btn } = mount()
    const glyph = btn('Vertical bar').querySelector('span')!
    expect(glyph.textContent).toBe('|')
    expect(glyph.getAttribute('aria-hidden')).toBe('true')
    expect(btn('Vertical bar').getAttribute('aria-label')).toBe('Vertical bar')
  })

  it('application cursor mode sends ESC O x', () => {
    const { send, btn } = mount({ appCursor: true })
    pointerDown(btn('Up arrow'))
    expect(send).toHaveBeenCalledWith('\x1bOA')
  })

  it('a touch tap (pointerdown, then click with detail 1) sends once; a click with detail 0 sends once', () => {
    const { send, btn } = mount()
    pointerDown(btn('Tab'))
    click(btn('Tab'), 1)
    expect(send).toHaveBeenCalledTimes(1)
    click(btn('Tab'), 0)
    expect(send).toHaveBeenCalledTimes(2)
  })

  it('Ctrl toggles aria-pressed and the armed class, and tapping again cancels without sending', () => {
    const { send, btn } = mount()
    expect(btn('Ctrl').getAttribute('aria-pressed')).toBe('false')
    pointerDown(btn('Ctrl'))
    expect(btn('Ctrl').getAttribute('aria-pressed')).toBe('true')
    expect(btn('Ctrl').classList.contains('keybar__key--armed')).toBe(true)
    pointerDown(btn('Ctrl'))
    expect(btn('Ctrl').getAttribute('aria-pressed')).toBe('false')
    expect(send).not.toHaveBeenCalled()
  })

  it('armed Ctrl plus Up sends CSI 1;5 A and disarms', () => {
    const { send, btn } = mount()
    pointerDown(btn('Ctrl'))
    pointerDown(btn('Up arrow'))
    expect(send).toHaveBeenCalledWith('\x1b[1;5A')
    expect(btn('Ctrl').getAttribute('aria-pressed')).toBe('false')
  })

  it('armed Ctrl plus Esc sends ESC and disarms', () => {
    const { send, btn } = mount()
    pointerDown(btn('Ctrl'))
    pointerDown(btn('Esc'))
    expect(send).toHaveBeenCalledWith('\x1b')
    expect(btn('Ctrl').getAttribute('aria-pressed')).toBe('false')
  })
})
