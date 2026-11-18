/**
 * 05-12 (SHAR-07 / VERF-01): the guest half of the reattach-fidelity proof.
 *
 * A real xterm.js Terminal (jsdom, Unicode 11 widths as TerminalPanel uses by
 * default) is fed the Go-generated wrapped full replay from 05-08 and must show
 * the host's rows, cursor and scrollback tail, before and after the next live
 * redraw. The two ordering cases pin why guestTerminal.ts exists: xterm's
 * synchronous reset() and resize() do not wait for writes still queued.
 */
import { describe, it, expect } from 'vitest'
import { Terminal } from '@xterm/xterm'
import { Unicode11Addon } from '@xterm/addon-unicode11'
import fixture from './fixtures/wrapped-replay.json'
import { applyHostGrid, resetForFullReplay } from '../guestTerminal'

const decode = (b64: string) => Uint8Array.from(atob(b64), (c) => c.charCodeAt(0))
const flush = (term: Terminal, data: string | Uint8Array = '') =>
  new Promise<void>((resolve) => term.write(data, resolve))

function guestTerm(cols = 80, rows = 24): Terminal {
  const term = new Terminal({ cols, rows, scrollback: 10000, allowProposedApi: true })
  term.loadAddon(new Unicode11Addon())
  term.unicode.activeVersion = '11'
  return term
}

// Rows are right-trimmed of spaces, as the Go fixture's plainRows does: a styled
// ESC[K line is replayed as bg-colored spaces, which xterm counts as content.
function lines(term: Terminal, from: number, to: number): string[] {
  const buf = term.buffer.active
  const out: string[] = []
  for (let i = from; i < to; i++) out.push((buf.getLine(i)?.translateToString(true) ?? '').replace(/ +$/, ''))
  return out
}
const screen = (term: Terminal) => lines(term, term.buffer.active.baseY, term.buffer.active.baseY + term.rows)
const cursor = (term: Terminal) => ({ x: term.buffer.active.cursorX, y: term.buffer.active.cursorY })
const allLines = (term: Terminal) => lines(term, 0, term.buffer.active.length)

describe('wrapped replay renders in real xterm.js like the host (05-08 fixture)', () => {
  it('rows, cursor and scrollback tail match after the replay, and again after the live redraw', async () => {
    const term = guestTerm() // a fresh guest starts at 80x24, then the 0x02 grid frame arrives
    applyHostGrid(term, fixture.cols, fixture.rows)
    await flush(term, decode(fixture.replay))

    expect([term.cols, term.rows]).toEqual([fixture.cols, fixture.rows])
    expect(screen(term)).toEqual(fixture.screen)
    expect(cursor(term)).toEqual(fixture.cursor)
    const baseY = term.buffer.active.baseY
    expect(lines(term, baseY - fixture.historyTail.length, baseY)).toEqual(fixture.historyTail)

    await flush(term, decode(fixture.live))
    expect(screen(term)).toEqual(fixture.screenAfter)
    expect(cursor(term)).toEqual(fixture.cursorAfter)
    term.dispose()
  })
})

const burst = Array.from({ length: 400 }, (_, i) => `stale burst line ${i} ${'x'.repeat(40)}`).join('\r\n')

describe('full replay reset is applied in stream order', () => {
  it('resetForFullReplay drops the queued burst: only the replay remains, no scrollback', async () => {
    const term = guestTerm(80, 10)
    term.write(burst)
    resetForFullReplay(term)
    await flush(term, 'replay row 0\r\nreplay row 1')

    expect(term.buffer.active.length).toBe(term.rows) // no scrollback
    expect(allLines(term).filter((l) => l !== '')).toEqual(['replay row 0', 'replay row 1'])
    term.dispose()
  })

  it('the synchronous API reset does not (why the helper exists)', async () => {
    const term = guestTerm(80, 10)
    term.write(burst)
    term.reset()
    await flush(term, 'replay row 0\r\nreplay row 1')

    expect(allLines(term).some((l) => l.startsWith('stale burst line'))).toBe(true)
    term.dispose()
  })
})

// At 30 cols: a burst, then a status line drawn with CHA past column 20; the
// host then narrows to 20 cols and redraws.
const before = Array.from({ length: 60 }, (_, i) => `out ${i} ${'y'.repeat(30)}`).join('\r\n') +
  '\r\n\x1b[2Kleft\x1b[25Gright-status'
const redraw = '\r\n\x1b[2Kafter resize\x1b[15Gend'

async function reference(rows: number) {
  const term = guestTerm(30, rows)
  await flush(term, before)
  term.resize(20, rows) // strictly between the two byte runs
  await flush(term, redraw)
  return term
}

describe('host grid change is applied in stream order', () => {
  it('applyHostGrid lands between old-width and new-width bytes', async () => {
    const ref = await reference(8)
    const term = guestTerm(30, 8)
    term.write(before)
    let afterRan = false
    applyHostGrid(term, 20, 8, () => { afterRan = true })
    await flush(term, redraw)

    expect(afterRan).toBe(true)
    expect([term.cols, term.rows]).toEqual([20, 8])
    // the status line and redraw rows first (one-line diff), then everything
    expect(allLines(term).slice(-3).join(' | ')).toBe(allLines(ref).slice(-3).join(' | '))
    expect(allLines(term)).toEqual(allLines(ref))
    expect(cursor(term)).toEqual(cursor(ref))
    term.dispose()
    ref.dispose()
  })

  it('a synchronous resize parses old-width bytes at the new width (why the helper exists)', async () => {
    const ref = await reference(8)
    const term = guestTerm(30, 8)
    term.write(before)
    term.resize(20, 8)
    await flush(term, redraw)

    expect(allLines(term)).not.toEqual(allLines(ref))
    term.dispose()
    ref.dispose()
  })
})
