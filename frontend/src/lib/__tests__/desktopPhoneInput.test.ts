import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { HEARTBEAT_MS, MAX_TEXT_BYTES, MAX_WHEEL } from '../desktopInput'
import { createPhoneInput, type PhoneInputDeps } from '../desktopPhoneInput'

type Msg = { t: string; [k: string]: unknown }

function rig(epoch: number | null = 1) {
  const sent: { msg: Msg; droppable?: boolean }[] = []
  const win = new EventTarget() as unknown as Window
  const doc = Object.assign(new EventTarget(), { visibilityState: 'visible' }) as unknown as Document
  let e = epoch
  const deps: PhoneInputDeps = {
    sendInput: (msg, droppable) => {
      sent.push({ msg: msg as Msg, droppable })
      return true
    },
    epoch: () => e,
    now: () => 1234,
    setInterval: (fn, ms) => setInterval(fn, ms),
    clearInterval: (id) => clearInterval(id as ReturnType<typeof setInterval>),
    win,
    doc,
  }
  const pi = createPhoneInput(deps)
  return {
    pi,
    win,
    doc,
    sent,
    msgs: () => sent.map((s) => s.msg),
    setEpoch: (v: number | null) => {
      e = v
    },
  }
}

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

describe('createPhoneInput', () => {
  it('rmove: nothing while the epoch is null, then Phase 20 rmove with the epoch, droppable', () => {
    const r = rig(null)
    r.pi.rmove(0.01, -0.02)
    expect(r.sent).toEqual([])
    r.setEpoch(3)
    r.pi.rmove(0.01, -0.02)
    expect(r.sent).toEqual([{ msg: { t: 'rmove', dx: 0.01, dy: -0.02, e: 3 }, droppable: true }])
  })

  it('click is a button down then up at the position with the given time, never droppable', () => {
    const r = rig()
    r.pi.click(0.5, 0.25, 0, 1000)
    expect(r.msgs()).toEqual([
      { t: 'btn', b: 0, d: true, x: 0.5, y: 0.25, e: 1, ts: 1000 },
      { t: 'btn', b: 0, d: false, x: 0.5, y: 0.25, e: 1, ts: 1000 },
    ])
    expect(r.sent.every((s) => !s.droppable)).toBe(true)
    expect(r.pi.held()).toBe(0)
  })

  it('doubleClick carries the two real tap times, one pair each', () => {
    const r = rig()
    r.pi.doubleClick(0.5, 0.25, [1000, 1180])
    expect(r.msgs().map((m) => [m.d, m.ts])).toEqual([[true, 1000], [false, 1000], [true, 1180], [false, 1180]])
  })

  it('a click with no epoch sends nothing', () => {
    const r = rig(null)
    r.pi.click(0.5, 0.25, 0, 1000)
    expect(r.sent).toEqual([])
  })

  it('press holds the button with a heartbeat every HEARTBEAT_MS, none once released', () => {
    const r = rig()
    r.pi.press(0.1, 0.2, 0, 10)
    expect(r.pi.held()).toBe(1)
    vi.advanceTimersByTime(HEARTBEAT_MS * 3)
    expect(r.msgs().filter((m) => m.t === 'hb')).toHaveLength(3)
    r.pi.release(0.1, 0.2, 0, 20)
    expect(r.pi.held()).toBe(0)
    r.sent.length = 0
    vi.advanceTimersByTime(HEARTBEAT_MS * 3)
    expect(r.sent).toEqual([])
  })

  it('a release with no epoch still goes out, as the daemon release', () => {
    const r = rig()
    r.pi.press(0.1, 0.2, 0, 10)
    r.setEpoch(null)
    r.sent.length = 0
    r.pi.release(0.1, 0.2, 0, 20)
    expect(r.msgs()).toEqual([{ t: 'release' }])
    expect(r.pi.held()).toBe(0)
  })

  it('wheel is clamped to the wire limit and sent with the epoch', () => {
    const r = rig()
    r.pi.wheel(0, 99999)
    r.pi.wheel(-5, 12)
    expect(r.msgs()).toEqual([
      { t: 'wheel', dx: 0, dy: MAX_WHEEL, e: 1 },
      { t: 'wheel', dx: -5, dy: 12, e: 1 },
    ])
  })

  it('keyTap sends one down and one up with the mask', () => {
    const r = rig()
    r.pi.keyTap('Tab', 0)
    r.pi.keyTap('KeyC', 8)
    const keys = r.msgs().map((m) => [m.code, m.d, m.m])
    expect(keys).toEqual([['Tab', true, 0], ['Tab', false, 0], ['KeyC', true, 8], ['KeyC', false, 8]])
  })

  it('chord is modifiers down in order, the key, then everything up in reverse with the mask following', () => {
    const r = rig()
    r.pi.chord(['ControlLeft', 'MetaLeft', 'KeyQ'])
    expect(r.msgs().map((m) => [m.code, m.d, m.m])).toEqual([
      ['ControlLeft', true, 2],
      ['MetaLeft', true, 10],
      ['KeyQ', true, 10],
      ['KeyQ', false, 10],
      ['MetaLeft', false, 2],
      ['ControlLeft', false, 0],
    ])
  })

  it('text of 3000 bytes goes out as pieces of at most 1 KiB that concatenate back', () => {
    const r = rig()
    const s = 'x'.repeat(3000)
    r.pi.text(s)
    const pieces = r.msgs().map((m) => m.s as string)
    expect(pieces).toHaveLength(3)
    expect(pieces.every((p) => new TextEncoder().encode(p).length <= MAX_TEXT_BYTES)).toBe(true)
    expect(pieces.join('')).toBe(s)
    expect(r.msgs().every((m) => m.t === 'text')).toBe(true)
  })

  it.each([
    ['window blur', (r: ReturnType<typeof rig>) => r.win.dispatchEvent(new Event('blur'))],
    ['pagehide', (r: ReturnType<typeof rig>) => r.win.dispatchEvent(new Event('pagehide'))],
    [
      'visibilitychange to hidden',
      (r: ReturnType<typeof rig>) => {
        ;(r.doc as unknown as { visibilityState: string }).visibilityState = 'hidden'
        r.doc.dispatchEvent(new Event('visibilitychange'))
      },
    ],
    ['releaseAll', (r: ReturnType<typeof rig>) => r.pi.releaseAll()],
  ])('%s sends release, clears what is held and stops the heartbeat', (_name, trigger) => {
    const r = rig()
    r.pi.press(0.1, 0.2, 0, 10)
    r.sent.length = 0
    trigger(r)
    expect(r.msgs()).toEqual([{ t: 'release' }])
    expect(r.pi.held()).toBe(0)
    vi.advanceTimersByTime(HEARTBEAT_MS * 2)
    expect(r.msgs()).toEqual([{ t: 'release' }])
  })

  it('visibilitychange to visible releases nothing', () => {
    const r = rig()
    r.pi.press(0.1, 0.2, 0, 10)
    r.sent.length = 0
    r.doc.dispatchEvent(new Event('visibilitychange'))
    expect(r.sent).toEqual([])
    expect(r.pi.held()).toBe(1)
  })

  it('destroy removes the release triggers and the heartbeat', () => {
    const r = rig()
    r.pi.press(0.1, 0.2, 0, 10)
    r.pi.destroy()
    r.sent.length = 0
    r.win.dispatchEvent(new Event('blur'))
    r.win.dispatchEvent(new Event('pagehide'))
    vi.advanceTimersByTime(HEARTBEAT_MS * 2)
    expect(r.sent).toEqual([])
  })
})
