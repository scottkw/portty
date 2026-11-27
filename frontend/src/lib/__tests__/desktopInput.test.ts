// Phase 20-06 — the viewer's input controller: golden-shaped messages, pointer mapping and
// coalescing, keys, text, the Cmd-chord tap rule, swap, release triggers and the heartbeat.
// Everything runs in jsdom against injected fakes; no channel, timer or frame is real.

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import {
  HB,
  RELEASE,
  createDesktopInput,
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

function harness(over: { epoch?: number | null; events?: DesktopInputEvents } = {}) {
  const sent: { msg: InputMsg; droppable: boolean }[] = []
  const frames: (() => void)[] = []
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
    },
    over.events ?? {},
  )
  ctl.setControl(true)
  sent.length = 0
  const msgs = () => sent.map((s) => s.msg)
  const frame = () => frames.splice(0).forEach((f) => f())
  return { stage, input, ctl, sent, msgs, frame, state, frames }
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
    h.stage.dispatchEvent(ptr('pointermove', 20, 600))
    h.frame()
    expect(h.msgs()).toEqual([{ t: 'move', x: 0, y: 1, e: 7 }])
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
