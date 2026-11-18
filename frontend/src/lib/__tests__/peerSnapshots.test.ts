/**
 * HARD-05 (07-02): the peer-tab snapshot store, and the proof that a snapshot plus
 * the host's delta equals the live terminal.
 *
 * Fidelity uses REAL xterm.js and the REAL SerializeAddon in jsdom (as
 * replayFidelity.test.ts does): terminal A is the live tab, B is the tab restored
 * from A's serialize() output; both then get the same delta and must match.
 */
import { describe, it, expect, afterEach } from 'vitest'
import { Terminal } from '@xterm/xterm'
import { Unicode11Addon } from '@xterm/addon-unicode11'
import { SerializeAddon } from '@xterm/addon-serialize'
import { PEER_SNAPSHOT_MAX, putPeerSnapshot, takePeerSnapshot, type PeerSnapshot } from '../peerSnapshots'

const snap = (offset: number): PeerSnapshot => ({ data: `d${offset}`, cols: 80, rows: 24, offset })

describe('peerSnapshots store (HARD-05)', () => {
  const ids = Array.from({ length: PEER_SNAPSHOT_MAX + 1 }, (_, i) => `s${i}`)
  afterEach(() => { for (const id of ids) takePeerSnapshot(id) })

  it('take returns the entry once, then undefined', () => {
    putPeerSnapshot('s0', snap(1))
    expect(takePeerSnapshot('s0')).toEqual(snap(1))
    expect(takePeerSnapshot('s0')).toBeUndefined()
  })

  it('a second put for the same id replaces the first', () => {
    putPeerSnapshot('s0', snap(1))
    putPeerSnapshot('s0', snap(2))
    expect(takePeerSnapshot('s0')).toEqual(snap(2))
  })

  it('keeps at most 8: after 9 puts the first id is gone and the other 8 remain', () => {
    expect(PEER_SNAPSHOT_MAX).toBe(8)
    ids.forEach((id, i) => putPeerSnapshot(id, snap(i)))
    expect(takePeerSnapshot('s0')).toBeUndefined()
    ids.slice(1).forEach((id, i) => expect(takePeerSnapshot(id)).toEqual(snap(i + 1)))
  })

  it('a re-put refreshes the position: the oldest untouched entry is evicted instead', () => {
    ids.slice(0, PEER_SNAPSHOT_MAX).forEach((id, i) => putPeerSnapshot(id, snap(i)))
    putPeerSnapshot('s0', snap(100)) // s0 is now the newest
    putPeerSnapshot(ids[PEER_SNAPSHOT_MAX], snap(8))
    expect(takePeerSnapshot('s1')).toBeUndefined()
    expect(takePeerSnapshot('s0')).toEqual(snap(100))
  })
})

const flush = (term: Terminal, data: string = '') => new Promise<void>((resolve) => term.write(data, resolve))

function guestTerm(): Terminal {
  const term = new Terminal({ cols: 80, rows: 24, scrollback: 10000, allowProposedApi: true })
  term.loadAddon(new Unicode11Addon())
  term.unicode.activeVersion = '11'
  return term
}

// Rows right-trimmed of spaces, as replayFidelity.test.ts does.
function lines(term: Terminal, from: number, to: number): string[] {
  const buf = term.buffer.active
  const out: string[] = []
  for (let i = from; i < to; i++) out.push((buf.getLine(i)?.translateToString(true) ?? '').replace(/ +$/, ''))
  return out
}
const screen = (term: Terminal) => lines(term, term.buffer.active.baseY, term.buffer.active.baseY + term.rows)
const cursor = (term: Terminal) => ({ x: term.buffer.active.cursorX, y: term.buffer.active.cursorY })
const allLines = (term: Terminal) => lines(term, 0, term.buffer.active.length)
// Wrapped-line flags: a lost flag would change how later rows reflow and select.
const wrapped = (term: Terminal) => {
  const buf = term.buffer.active
  return Array.from({ length: buf.length }, (_, i) => buf.getLine(i)?.isWrapped ?? false)
}

// A Claude-Code-like relative redraw: up 2, CR, erase below, a new line, a prompt.
const DELTA = '\x1b[2A\r\x1b[Jredrawn status line\r\n$ '

describe('snapshot + delta equals the live terminal in real xterm.js (HARD-05)', () => {
  const terms: Terminal[] = []
  const make = () => { const t = guestTerm(); terms.push(t); return t }
  afterEach(() => { terms.splice(0).forEach((t) => t.dispose()) })

  // Restore as TerminalPanel does: the snapshot is written into a fresh terminal.
  async function restore(serializer: SerializeAddon): Promise<Terminal> {
    const restored = make()
    await flush(restored, serializer.serialize())
    return restored
  }

  it('main screen: scrollback, a wrapped line, SGR, bracketed paste and a mid-line cursor', async () => {
    const a = make()
    const serializer = new SerializeAddon()
    a.loadAddon(serializer)
    const numbered = Array.from({ length: 300 }, (_, i) => `line ${String(i).padStart(3, '0')}`).join('\r\n')
    await flush(a, `${numbered}\r\n${'w'.repeat(200)}\r\n\x1b[1;32mgreen\x1b[0m and \x1b[38;5;208morange\x1b[0m\r\n`)
    await flush(a, '\x1b[?2004h$ echo hi\x1b[3D')
    expect(a.buffer.active.baseY).toBeGreaterThan(0) // it has scrollback
    expect(cursor(a).x).toBe(6) // mid-line

    const b = await restore(serializer)
    expect(allLines(b)).toEqual(allLines(a))
    expect(wrapped(b)).toEqual(wrapped(a))
    expect(cursor(b)).toEqual(cursor(a))
    expect(b.modes.bracketedPasteMode).toBe(true)

    await flush(a, DELTA)
    await flush(b, DELTA)
    expect(allLines(b)).toEqual(allLines(a))
    expect(wrapped(b)).toEqual(wrapped(a))
    expect(cursor(b)).toEqual(cursor(a))
    expect(b.modes.bracketedPasteMode).toBe(a.modes.bracketedPasteMode)
    // Attributes too: B serializes to exactly what A does.
    const bSerializer = new SerializeAddon()
    b.loadAddon(bSerializer)
    expect(bSerializer.serialize()).toBe(serializer.serialize())
  })

  it('alternate screen: restored in the alt buffer, and leaving it gives the same normal buffer', async () => {
    const a = make()
    const serializer = new SerializeAddon()
    a.loadAddon(serializer)
    const main = Array.from({ length: 40 }, (_, i) => `main ${i}`).join('\r\n')
    await flush(a, `${main}\r\n$ vim`)
    await flush(a, '\x1b[?1049h\x1b[1;1Htop row\x1b[10;5Hmiddle\x1b[24;1Hbottom\x1b[12;3H')
    expect(a.buffer.active.type).toBe('alternate')

    const b = await restore(serializer)
    expect(b.buffer.active.type).toBe('alternate')
    expect(screen(b)).toEqual(screen(a))
    expect(cursor(b)).toEqual(cursor(a))

    const delta = `${DELTA}\x1b[?1049l`
    await flush(a, delta)
    await flush(b, delta)
    expect(a.buffer.active.type).toBe('normal')
    expect(b.buffer.active.type).toBe('normal')
    expect(allLines(b)).toEqual(allLines(a))
    expect(cursor(b)).toEqual(cursor(a))
  })
})
