// Phase 20-06 — the viewer's input controller: golden-shaped messages, pointer mapping and
// coalescing, keys, text, the Cmd-chord tap rule, swap, release triggers and the heartbeat.
// Everything runs in jsdom against injected fakes; no channel, timer or frame is real.

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import {
  HB,
  RELEASE,
  HEARTBEAT_MS,
  HINT_KEY,
  SWAP_KEY,
  createDesktopInput,
  isReleaseChord,
  splitText,
  swapCode,
  encodeBtn,
  encodeKey,
  encodeMove,
  encodeRmove,
  encodeText,
  encodeWheel,
  type DesktopInput,
  type DesktopInputEvents,
  type InputMsg,
} from '../desktopInput'

interface GoldenLine {
  name: string
  dir: 'viewer' | 'server'
  valid: boolean
  wire: Record<string, unknown>
}

const golden: GoldenLine[] = readFileSync(
  resolve(__dirname, '../../../../internal/desktop/testdata/input-v1.jsonl'),
  'utf8',
)
  .trim()
  .split('\n')
  .map((l) => JSON.parse(l))

/** The encoder the viewer uses for a golden wire message, driven by the wire's own fields. */
function encodeFromWire(w: Record<string, unknown>): InputMsg {
  switch (w.t) {
    case 'move': return encodeMove(w.x as number, w.y as number, w.e as number)
    case 'rmove': return encodeRmove(w.dx as number, w.dy as number, w.e as number)
    case 'btn': return encodeBtn(w.b as number, w.d as boolean, w.x as number, w.y as number, w.e as number, w.ts as number)
    case 'wheel': return encodeWheel(w.dx as number, w.dy as number, w.e as number)
    case 'key': return encodeKey(w.code as string, w.d as boolean, w.r as boolean, w.m as number, w.caps as boolean, w.k as string, w.ts as number)
    case 'text': return encodeText(w.s as string)
    case 'hb': return HB
    case 'release': return RELEASE
  }
  throw new Error(`no encoder for ${String(w.t)}`)
}

describe('encoders against the Go golden file', () => {
  // move-clamped and move-extra-field test the daemon's tolerance, not what a viewer sends.
  const skip = new Set(['move-clamped', 'move-extra-field'])
  const lines = golden.filter((l) => l.dir === 'viewer' && l.valid && !skip.has(l.name))

  it('covers every kind a viewer sends', () => {
    expect(new Set(lines.map((l) => l.wire.t))).toEqual(
      new Set(['move', 'rmove', 'btn', 'wheel', 'key', 'text', 'hb', 'release']),
    )
  })

  it.each(lines.map((l) => [l.name, l] as const))('%s is byte-shape identical to the wire line', (_n, l) => {
    expect(encodeFromWire(l.wire)).toEqual(l.wire)
  })
})

// A stage with a 1000x500 video showing 1600x1000: the picture is x 100, width 800, height 500.
const RECT = { x: 100, y: 0, width: 800, height: 500 }

type Store = Pick<Storage, 'getItem' | 'setItem'>
function memStore(): Store & { data: Map<string, string> } {
  const data = new Map<string, string>()
  return { data, getItem: (k) => data.get(k) ?? null, setItem: (k, v) => void data.set(k, v) }
}

function harness(over: { epoch?: number | null; events?: DesktopInputEvents; storage?: Store | null } = {}) {
  const sent: { msg: InputMsg; droppable: boolean }[] = []
  const frames: (() => void)[] = []
  const intervals = new Map<number, () => void>()
  let ivSeq = 0
  const storage = over.storage === undefined ? memStore() : over.storage
  const stage = document.createElement('div')
  const input = document.createElement('textarea')
  stage.appendChild(input)
  document.body.appendChild(stage)
  const state = { epoch: over.epoch === undefined ? 7 : over.epoch, rect: RECT, t: 1000 }
  const ctl = createDesktopInput(
    stage,
    input,
    {
      send: (msg, droppable = false) => {
        sent.push({ msg, droppable })
        return true
      },
      getRect: () => state.rect,
      getEpoch: () => state.epoch,
      now: () => state.t,
      raf: (cb) => frames.push(cb),
      cancelRaf: () => {},
      setInterval: (fn) => {
        intervals.set(++ivSeq, fn)
        return ivSeq
      },
      clearInterval: (id) => void intervals.delete(id as number),
      storage,
    },
    over.events ?? {},
  )
  ctl.setControl(true)
  sent.length = 0
  const msgs = () => sent.map((s) => s.msg)
  const frame = () => frames.splice(0).forEach((f) => f())
  const tick = () => [...intervals.values()].forEach((f) => f())
  return { stage, input, ctl, sent, msgs, frame, state, frames, intervals, tick, storage }
}

function ptr(type: string, x: number, y: number, init: PointerEventInit = {}) {
  return new PointerEvent(type, { clientX: x, clientY: y, bubbles: true, cancelable: true, button: 0, pointerId: 1, ...init })
}

let created: DesktopInput[] = []
afterEach(() => {
  for (const c of created) c.destroy()
  created = []
  document.body.innerHTML = ''
})

describe('pointer press', () => {
  let h: ReturnType<typeof harness>
  beforeEach(() => {
    h = harness()
    created.push(h.ctl)
  })

  it('a press in the middle of the picture is a golden-shaped btn with normalised x, y, the epoch and the clock', () => {
    h.stage.dispatchEvent(ptr('pointerdown', 500, 250))
    expect(h.msgs()).toEqual([{ t: 'btn', b: 0, d: true, x: 0.5, y: 0.5, e: 7, ts: 1000 }])
    expect(h.sent[0].droppable).toBe(false)
  })

  it('a press in the letterbox bar sends nothing', () => {
    h.stage.dispatchEvent(ptr('pointerdown', 50, 250))
    expect(h.msgs()).toEqual([])
  })

  it('a click on the picture focuses the hidden input', () => {
    h.stage.dispatchEvent(ptr('pointerdown', 500, 250))
    expect(document.activeElement).toBe(h.input)
  })

  it('sends nothing until the surface epoch is known', () => {
    h.state.epoch = null
    h.stage.dispatchEvent(ptr('pointerdown', 500, 250))
    h.stage.dispatchEvent(ptr('pointermove', 500, 250))
    h.frame()
    expect(h.msgs()).toEqual([])
  })

  it('drops buttons 3 and 4 and sends the right button as index 2', () => {
    h.stage.dispatchEvent(ptr('pointerdown', 500, 250, { button: 3 }))
    h.stage.dispatchEvent(ptr('pointerdown', 500, 250, { button: 4 }))
    expect(h.msgs()).toEqual([])
    h.stage.dispatchEvent(ptr('pointerdown', 700, 125, { button: 2 }))
    expect(h.msgs()).toEqual([{ t: 'btn', b: 2, d: true, x: 0.75, y: 0.25, e: 7, ts: 1000 }])
  })

  it('suppresses the context menu while control is on, and only then', () => {
    const on = new MouseEvent('contextmenu', { bubbles: true, cancelable: true })
    h.stage.dispatchEvent(on)
    expect(on.defaultPrevented).toBe(true)
    h.ctl.setControl(false)
    const off = new MouseEvent('contextmenu', { bubbles: true, cancelable: true })
    h.stage.dispatchEvent(off)
    expect(off.defaultPrevented).toBe(false)
  })
})

describe('pointer moves', () => {
  it('ten moves in one frame send one droppable move, from the latest point', () => {
    const h = harness()
    created.push(h.ctl)
    for (let i = 0; i < 10; i++) h.stage.dispatchEvent(ptr('pointermove', 200 + i * 10, 250))
    expect(h.msgs()).toEqual([])
    h.frame()
    expect(h.msgs()).toEqual([{ t: 'move', x: 0.2375, y: 0.5, e: 7 }])
    expect(h.sent[0].droppable).toBe(true)
  })

  it('a move in the bar is not forwarded unless a button is held, and then it clamps to the picture edge', () => {
    const h = harness()
    created.push(h.ctl)
    h.stage.dispatchEvent(ptr('pointermove', 50, 250))
    h.frame()
    expect(h.msgs()).toEqual([])
    h.stage.dispatchEvent(ptr('pointerdown', 500, 250))
    h.sent.length = 0
    h.stage.dispatchEvent(ptr('pointermove', 20, 600, { button: -1, buttons: 1 }))
    h.frame()
    expect(h.msgs()).toEqual([{ t: 'move', x: 0, y: 1, e: 7 }])
  })

  it('a left button lifted while the right is still down is released, and nothing stays held', () => {
    const h = harness()
    created.push(h.ctl)
    h.stage.dispatchEvent(ptr('pointerdown', 500, 250))
    // Pointer Events report the second button's press and the left release as pointermove.
    h.stage.dispatchEvent(ptr('pointermove', 500, 250, { button: 2, buttons: 3 }))
    h.stage.dispatchEvent(ptr('pointermove', 500, 250, { button: 0, buttons: 2 }))
    h.stage.dispatchEvent(ptr('pointerup', 500, 250, { button: 2, buttons: 0 }))
    expect(h.msgs()).toEqual([
      { t: 'btn', b: 0, d: true, x: 0.5, y: 0.5, e: 7, ts: 1000 },
      { t: 'btn', b: 0, d: false, x: 0.5, y: 0.5, e: 7, ts: 1000 },
    ])
    expect(h.ctl.held()).toBe(0)
    expect(h.intervals.size).toBe(0)
  })

  it('the release of a drag that ended outside the picture still arrives, clamped', () => {
    const h = harness()
    created.push(h.ctl)
    h.stage.dispatchEvent(ptr('pointerdown', 500, 250))
    h.state.t = 1080
    h.stage.dispatchEvent(ptr('pointerup', 990, 250))
    expect(h.msgs().slice(1)).toEqual([{ t: 'btn', b: 0, d: false, x: 1, y: 0.5, e: 7, ts: 1080 }])
    h.stage.dispatchEvent(ptr('pointerup', 500, 250))
    expect(h.msgs()).toHaveLength(2) // nothing is released twice
  })

  it('reports the dot on the animation frame of the move and hides it on leave', () => {
    const dots: unknown[] = []
    const h = harness({ events: { onDot: (p) => dots.push(p) } })
    created.push(h.ctl)
    dots.length = 0
    h.stage.dispatchEvent(ptr('pointermove', 500, 250))
    expect(dots).toEqual([])
    h.frame()
    expect(dots).toEqual([{ x: 500, y: 250 }])
    h.stage.dispatchEvent(new PointerEvent('pointerleave'))
    expect(dots[dots.length - 1]).toBeNull()
  })
})

describe('scope', () => {
  it('attaches nothing to window or document', () => {
    const h = harness()
    created.push(h.ctl)
    // The source gate: no key or pointer listener on window or document, ever.
    const src = readFileSync(resolve(__dirname, '../desktopInput.ts'), 'utf8')
    expect(src).not.toMatch(/(window|document|win|doc)\.addEventListener\('(keydown|keyup|pointerdown|contextmenu)/)
  })
})

// ---- Task 2: keyboard, text, swap, chord, wheel, release triggers, heartbeat ----

function key(type: 'keydown' | 'keyup', code: string, k: string, init: KeyboardEventInit = {}) {
  return new KeyboardEvent(type, { code, key: k, bubbles: true, cancelable: true, ...init })
}
const keyMsg = (code: string, d: boolean, over: Partial<Extract<InputMsg, { t: 'key' }>> = {}): InputMsg =>
  ({ t: 'key', code, d, r: false, m: 0, caps: false, k: '', ts: 1000, ...over })

describe('keys', () => {
  let h: ReturnType<typeof harness>
  beforeEach(() => {
    h = harness()
    created.push(h.ctl)
  })

  it('keydown then keyup of KeyA send key down then up with the typed character and Caps Lock state', () => {
    const down = key('keydown', 'KeyA', 'a', { modifierCapsLock: true })
    h.input.dispatchEvent(down)
    h.input.dispatchEvent(key('keyup', 'KeyA', 'a', { modifierCapsLock: true }))
    expect(h.msgs()).toEqual([
      keyMsg('KeyA', true, { k: 'a', caps: true }),
      keyMsg('KeyA', false, { k: 'a', caps: true }),
    ])
    expect(down.defaultPrevented).toBe(true)
  })

  it('a browser repeat is forwarded as a repeat down', () => {
    h.input.dispatchEvent(key('keydown', 'ArrowDown', 'ArrowDown', { repeat: true }))
    expect(h.msgs()).toEqual([keyMsg('ArrowDown', true, { r: true, k: 'ArrowDown' })])
  })

  it('a keydown with Meta held is a press at once followed by a release, and its keyup sends nothing', () => {
    h.input.dispatchEvent(key('keydown', 'MetaLeft', 'Meta', { metaKey: true }))
    h.input.dispatchEvent(key('keydown', 'KeyC', 'c', { metaKey: true }))
    expect(h.msgs()).toEqual([
      keyMsg('MetaLeft', true, { m: 8, k: 'Meta' }),
      keyMsg('KeyC', true, { m: 8, k: 'c' }),
      keyMsg('KeyC', false, { m: 8, k: 'c' }),
    ])
    h.sent.length = 0
    h.input.dispatchEvent(key('keyup', 'KeyC', 'c'))
    expect(h.msgs()).toEqual([])
    h.input.dispatchEvent(key('keyup', 'MetaLeft', 'Meta'))
    expect(h.msgs()).toEqual([keyMsg('MetaLeft', false, { k: 'Meta' })])
    expect(h.ctl.held()).toBe(0)
  })

  it('a key held when Cmd goes down is released first, since macOS swallows its keyup while Cmd is down', () => {
    h.input.dispatchEvent(key('keydown', 'KeyS', 's'))
    h.input.dispatchEvent(key('keydown', 'ShiftLeft', 'Shift', { shiftKey: true }))
    h.input.dispatchEvent(key('keydown', 'MetaLeft', 'Meta', { metaKey: true, shiftKey: true }))
    // No keyup for KeyS ever arrives.
    h.input.dispatchEvent(key('keyup', 'MetaLeft', 'Meta', { shiftKey: true }))
    h.input.dispatchEvent(key('keyup', 'ShiftLeft', 'Shift'))
    expect(h.msgs()).toEqual([
      keyMsg('KeyS', true, { k: 's' }),
      keyMsg('ShiftLeft', true, { m: 1, k: 'Shift' }),
      keyMsg('KeyS', false, { m: 9 }),
      keyMsg('MetaLeft', true, { m: 9, k: 'Meta' }),
      keyMsg('MetaLeft', false, { m: 1, k: 'Meta' }),
      keyMsg('ShiftLeft', false, { k: 'Shift' }),
    ])
    expect(h.ctl.held()).toBe(0)
  })

  it('a Cmd keyup whose down was never forwarded still releases held non-modifier keys', () => {
    h.input.dispatchEvent(key('keydown', 'KeyA', 'a'))
    h.input.dispatchEvent(key('keyup', 'MetaRight', 'Meta'))
    expect(h.msgs()).toEqual([keyMsg('KeyA', true, { k: 'a' }), keyMsg('KeyA', false)])
    expect(h.ctl.held()).toBe(0)
  })

  it('a keyup whose down was never forwarded sends nothing', () => {
    h.input.dispatchEvent(key('keyup', 'KeyQ', 'q'))
    expect(h.msgs()).toEqual([])
  })

  it('sends no key while composing (isComposing, keyCode 229, Dead)', () => {
    h.input.dispatchEvent(key('keydown', 'KeyN', 'n', { isComposing: true }))
    h.input.dispatchEvent(key('keydown', 'KeyN', 'Process', { keyCode: 229 }))
    h.input.dispatchEvent(key('keydown', 'KeyE', 'Dead'))
    expect(h.msgs()).toEqual([])
  })

  it('sends nothing for keys on the stage, on window or while the epoch is unknown, paused or control is off', () => {
    h.stage.dispatchEvent(key('keydown', 'KeyA', 'a'))
    window.dispatchEvent(key('keydown', 'KeyA', 'a'))
    expect(h.msgs()).toEqual([])
    h.state.epoch = null
    h.input.dispatchEvent(key('keydown', 'KeyA', 'a'))
    expect(h.msgs()).toEqual([])
    h.state.epoch = 7
    h.ctl.setPaused(true)
    h.input.dispatchEvent(key('keydown', 'KeyA', 'a'))
    expect(h.msgs()).toEqual([])
    h.ctl.setPaused(false)
    h.ctl.setControl(false)
    h.sent.length = 0
    h.input.dispatchEvent(key('keydown', 'KeyA', 'a'))
    expect(h.msgs()).toEqual([])
  })
})

describe('text', () => {
  let h: ReturnType<typeof harness>
  beforeEach(() => {
    h = harness()
    created.push(h.ctl)
  })

  it('compositionend sends the committed text once and clears the input', () => {
    h.input.value = '日本'
    h.input.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true }))
    h.input.dispatchEvent(new CompositionEvent('compositionend', { data: '日本', bubbles: true }))
    expect(h.msgs()).toEqual([{ t: 'text', s: '日本' }])
    expect(h.input.value).toBe('')
  })

  it('a composed string of 3000 bytes goes out as 3 text messages of at most 1024 bytes that concatenate back', () => {
    const s = 'あ'.repeat(1000)
    h.input.dispatchEvent(new CompositionEvent('compositionend', { data: s, bubbles: true }))
    const texts = h.msgs().map((m) => (m as { s: string }).s)
    expect(texts).toHaveLength(3)
    for (const t of texts) expect(new TextEncoder().encode(t).length).toBeLessThanOrEqual(1024)
    expect(texts.join('')).toBe(s)
  })

  it('an insertText that did not come from a key (emoji picker, dictation) is sent as text and not inserted', () => {
    const ev = new InputEvent('beforeinput', { data: '😀', inputType: 'insertText', bubbles: true, cancelable: true })
    h.input.dispatchEvent(ev)
    expect(h.msgs()).toEqual([{ t: 'text', s: '😀' }])
    expect(ev.defaultPrevented).toBe(true)
    h.sent.length = 0
    h.input.dispatchEvent(new InputEvent('beforeinput', { data: 'に', inputType: 'insertCompositionText', bubbles: true, cancelable: true }))
    expect(h.msgs()).toEqual([])
  })

  it('a paste on the hidden input is prevented', () => {
    const ev = new Event('paste', { bubbles: true, cancelable: true })
    h.input.dispatchEvent(ev)
    expect(ev.defaultPrevented).toBe(true)
  })
})

describe('splitText', () => {
  const bytes = (s: string) => new TextEncoder().encode(s).length

  it('keeps a combining mark with the letter before it, even at the byte limit', () => {
    const s = 'a'.repeat(1023) + 'é' + 'b'
    const parts = splitText(s, 1024)
    expect(parts.join('')).toBe(s)
    for (const p of parts) expect(bytes(p)).toBeLessThanOrEqual(1024)
    for (const p of parts) expect(p).not.toMatch(/^[̀-ͯ‍️]/)
    expect(parts[1].startsWith('é')).toBe(true)
  })

  it('keeps a ZWJ sequence and a variation selector whole', () => {
    const family = '\u{1F469}‍\u{1F4BB}'
    const s = 'a'.repeat(1020) + family + '❤️'
    const parts = splitText(s, 1024)
    expect(parts.join('')).toBe(s)
    expect(parts).toEqual(['a'.repeat(1020), family + '❤️'])
  })

  it('cuts a run longer than a piece between code points, never inside a surrogate pair', () => {
    const s = '\u{1F600}'.repeat(600)
    const parts = splitText(s, 1024)
    expect(parts.join('')).toBe(s)
    for (const p of parts) {
      expect(bytes(p)).toBeLessThanOrEqual(1024)
      expect(p).not.toMatch(/[\ud800-\udbff]$/)
    }
  })
})

describe('swap Ctrl and Cmd', () => {
  it('exchanges the Control and Meta pairs and nothing else', () => {
    expect(swapCode('ControlLeft')).toBe('MetaLeft')
    expect(swapCode('MetaRight')).toBe('ControlRight')
    expect(swapCode('KeyA')).toBe('KeyA')
  })

  it('with swap on, ControlLeft goes out as MetaLeft with mask bit 8, and its keyup follows the code it went out with', () => {
    const h = harness()
    created.push(h.ctl)
    h.ctl.setSwap(true)
    h.input.dispatchEvent(key('keydown', 'ControlLeft', 'Control', { ctrlKey: true }))
    h.ctl.setSwap(false) // toggled while held
    h.input.dispatchEvent(key('keyup', 'ControlLeft', 'Control'))
    expect(h.msgs()).toEqual([
      keyMsg('MetaLeft', true, { m: 8, k: 'Control' }),
      keyMsg('MetaLeft', false, { k: 'Control' }),
    ])
  })

  it('is off by default, persists under portty:desktop-swap-ctrl-cmd, and a storage that throws reads as off', () => {
    const store = memStore()
    const h = harness({ storage: store })
    created.push(h.ctl)
    expect(h.ctl.swap()).toBe(false)
    h.ctl.setSwap(true)
    expect(store.data.get(SWAP_KEY)).toBe('1')
    expect(SWAP_KEY).toBe('portty:desktop-swap-ctrl-cmd')
    const on = harness({ storage: store })
    created.push(on.ctl)
    expect(on.ctl.swap()).toBe(true)
    const boom: Store = { getItem: () => { throw new Error('private mode') }, setItem: () => { throw new Error('private mode') } }
    const safe = harness({ storage: boom })
    created.push(safe.ctl)
    expect(safe.ctl.swap()).toBe(false)
    expect(() => safe.ctl.setSwap(true)).not.toThrow()
    expect(safe.ctl.swap()).toBe(true)
  })

  it('keeps the first-capture hint key name', () => {
    expect(HINT_KEY).toBe('portty:desktop-release-hint-seen')
  })
})

describe('release chord', () => {
  it('Ctrl+Option+Cmd+Return is never forwarded: it releases everything and calls the host once', () => {
    let chords = 0
    const h = harness({ events: { onReleaseChord: () => chords++ } })
    created.push(h.ctl)
    h.input.dispatchEvent(key('keydown', 'KeyA', 'a'))
    h.sent.length = 0
    const chord = key('keydown', 'Enter', 'Enter', { ctrlKey: true, altKey: true, metaKey: true })
    h.input.dispatchEvent(chord)
    expect(h.msgs().filter((m) => m.t === 'key')).toEqual([])
    expect(h.msgs()).toContainEqual({ t: 'release' })
    expect(chords).toBe(1)
    expect(chord.defaultPrevented).toBe(true)
    expect(h.ctl.held()).toBe(0)
    h.input.dispatchEvent(key('keyup', 'KeyA', 'a'))
    expect(h.msgs().filter((m) => m.t === 'key')).toEqual([])
  })

  it('is read on the physical modifiers, before the swap', () => {
    expect(isReleaseChord({ ctrlKey: true, altKey: true, metaKey: true, code: 'Enter' })).toBe(true)
    expect(isReleaseChord({ ctrlKey: true, altKey: true, metaKey: false, code: 'Enter' })).toBe(false)
    expect(isReleaseChord({ ctrlKey: true, altKey: true, metaKey: true, code: 'KeyA' })).toBe(false)
  })
})

describe('wheel', () => {
  const wheel = (init: WheelEventInit) =>
    new WheelEvent('wheel', { clientX: 500, clientY: 250, bubbles: true, cancelable: true, ...init })
  let h: ReturnType<typeof harness>
  beforeEach(() => {
    h = harness()
    created.push(h.ctl)
    Object.defineProperty(h.stage, 'clientHeight', { configurable: true, value: 500 })
  })

  it('converts lines to 16 px and pages to the stage height, and never coalesces', () => {
    h.stage.dispatchEvent(wheel({ deltaMode: 1, deltaY: 3 }))
    h.stage.dispatchEvent(wheel({ deltaMode: 2, deltaY: 1 }))
    h.stage.dispatchEvent(wheel({ deltaMode: 0, deltaX: 5, deltaY: 120 }))
    expect(h.msgs()).toEqual([
      { t: 'wheel', dx: 0, dy: 48, e: 7 },
      { t: 'wheel', dx: 0, dy: 500, e: 7 },
      { t: 'wheel', dx: 5, dy: 120, e: 7 },
    ])
    expect(h.sent.every((s) => !s.droppable)).toBe(true)
  })

  it('swallows a pinch (ctrlKey) without sending, and clamps a huge delta to 2400', () => {
    const pinch = wheel({ ctrlKey: true, deltaY: -40 })
    h.stage.dispatchEvent(pinch)
    expect(h.msgs()).toEqual([])
    expect(pinch.defaultPrevented).toBe(true)
    h.stage.dispatchEvent(wheel({ deltaY: 5000 }))
    h.stage.dispatchEvent(wheel({ deltaY: -5000 }))
    expect(h.msgs()).toEqual([
      { t: 'wheel', dx: 0, dy: 2400, e: 7 },
      { t: 'wheel', dx: 0, dy: -2400, e: 7 },
    ])
  })

  it('does not forward a wheel over the letterbox bar', () => {
    h.stage.dispatchEvent(wheel({ clientX: 50, deltaY: 100 }))
    expect(h.msgs()).toEqual([])
  })
})

describe('release triggers and the heartbeat', () => {
  let h: ReturnType<typeof harness>
  beforeEach(() => {
    h = harness()
    created.push(h.ctl)
    h.input.dispatchEvent(key('keydown', 'KeyA', 'a'))
    h.stage.dispatchEvent(ptr('pointerdown', 500, 250))
    h.sent.length = 0
  })

  it('window blur, pagehide, a hidden tab and a closed channel each send release and clear what the viewer holds', () => {
    const triggers: [string, () => void][] = [
      ['blur', () => window.dispatchEvent(new Event('blur'))],
      ['pagehide', () => window.dispatchEvent(new Event('pagehide'))],
      ['hidden', () => {
        Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'hidden' })
        document.dispatchEvent(new Event('visibilitychange'))
        Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' })
      }],
      ['channel', () => h.ctl.releaseAll('channel')],
    ]
    for (const [name, fire] of triggers) {
      expect(h.ctl.held(), name).toBe(2)
      fire()
      expect(h.msgs(), name).toEqual([{ t: 'release' }])
      expect(h.ctl.held(), name).toBe(0)
      h.input.dispatchEvent(key('keydown', 'KeyA', 'a'))
      h.stage.dispatchEvent(ptr('pointerdown', 500, 250))
      h.sent.length = 0
    }
  })

  it('a tab that becomes visible again releases nothing', () => {
    document.dispatchEvent(new Event('visibilitychange'))
    expect(h.msgs()).toEqual([])
    expect(h.ctl.held()).toBe(2)
  })

  it('reports what was held to the host with the reason, once', () => {
    const got: unknown[] = []
    const g = harness({ events: { onReleased: (reason, n) => got.push([reason, n]) } })
    created.push(g.ctl)
    g.input.dispatchEvent(key('keydown', 'KeyA', 'a'))
    g.ctl.releaseAll('lock')
    g.ctl.releaseAll('lock')
    expect(got).toEqual([['lock', 1]])
  })

  it('sends hb every 1000 ms while anything is held and stops once nothing is', () => {
    expect(HEARTBEAT_MS).toBe(1000)
    expect(h.intervals.size).toBe(1)
    h.tick()
    h.tick()
    expect(h.msgs()).toEqual([{ t: 'hb' }, { t: 'hb' }])
    h.input.dispatchEvent(key('keyup', 'KeyA', 'a'))
    expect(h.intervals.size).toBe(1) // the button is still down
    h.stage.dispatchEvent(ptr('pointerup', 500, 250))
    expect(h.intervals.size).toBe(0)
  })

  it('choosing View Only releases, and removes the heartbeat', () => {
    h.ctl.setControl(false)
    expect(h.msgs()).toContainEqual({ t: 'release' })
    expect(h.ctl.held()).toBe(0)
    expect(h.intervals.size).toBe(0)
  })
})

describe('Send Keys chords', () => {
  it('taps modifiers down in order, the key, then everything up in reverse, with the mask accumulating', () => {
    const h = harness()
    created.push(h.ctl)
    h.ctl.sendChord(['MetaLeft', 'Tab'])
    expect(h.msgs()).toEqual([
      keyMsg('MetaLeft', true, { m: 8, k: 'Meta' }),
      keyMsg('Tab', true, { m: 8, k: 'Tab' }),
      keyMsg('Tab', false, { m: 8, k: 'Tab' }),
      keyMsg('MetaLeft', false, { m: 0, k: 'Meta' }),
    ])
    expect(h.ctl.held()).toBe(0)
  })
})
