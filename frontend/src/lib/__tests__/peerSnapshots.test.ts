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
import { PEER_SNAPSHOT_MAX, putPeerSnapshot, takePeerSnapshot, watchPeerTerm, type PeerSnapshot } from '../peerSnapshots'

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

describe('watchPeerTerm: what serialize() leaves out (WR-01)', () => {
  const terms: Terminal[] = []
  afterEach(() => { terms.splice(0).forEach((t) => t.dispose()) })

  // A live peer tab: output is fed to the watch and written to xterm, as TerminalPanel does.
  function live() {
    const term = guestTerm()
    terms.push(term)
    const serializer = new SerializeAddon()
    term.loadAddon(serializer)
    const watch = watchPeerTerm(term)
    const out = (d: string | Uint8Array) => {
      const bytes = typeof d === 'string' ? new TextEncoder().encode(d) : d
      watch.feed(bytes)
      return new Promise<void>((resolve) => term.write(bytes, resolve))
    }
    return { term, serializer, watch, out }
  }
  // xterm keeps cursor visibility and the mouse encoding off its public API.
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const core = (t: Terminal) => (t as any)._core
  const restoreFrom = async (a: ReturnType<typeof live>) => {
    const extra = a.watch.suffix()
    expect(extra).not.toBeNull()
    const b = live()
    await flush(b.term, a.serializer.serialize() + extra)
    return b
  }

  it('restores SGR mouse encoding, mouse tracking, a hidden cursor and bracketed paste', async () => {
    const a = live()
    await a.out('$ vim\r\n\x1b[?1049h\x1b[?1000h\x1b[?1006h\x1b[?25l\x1b[?2004h\x1b[5;5Hbuffer')
    expect(core(a.term).coreMouseService.activeEncoding).toBe('SGR')
    expect(core(a.term).coreService.isCursorHidden).toBe(true)

    const b = await restoreFrom(a)
    expect(b.term.modes.mouseTrackingMode).toBe('vt200')
    expect(core(b.term).coreMouseService.activeEncoding).toBe('SGR')
    expect(core(b.term).coreService.isCursorHidden).toBe(true)
    expect(b.term.modes.bracketedPasteMode).toBe(true)
    expect(b.watch.suffix()).toBe(a.watch.suffix()) // the restored tab tracks the same state
  })

  it('a reset mode is not re-emitted, and a mode list is read whole', async () => {
    const a = live()
    await a.out('\x1b[?25;1016l\x1b[?1016;25h\x1b[?25h\x1b[?1016l')
    expect(a.watch.suffix()).toBe('')
    await a.out('\x1b[?2004;1016;25l')
    expect(a.watch.suffix()).toBe('\x1b[?25l')
    await a.out('\x1b[?1016h')
    const b = await restoreFrom(a)
    expect(core(b.term).coreMouseService.activeEncoding).toBe('SGR_PIXELS')
  })

  it('RIS clears the tracked state', async () => {
    const a = live()
    await a.out('\x1b[?25l\x1b[?1006h\x1b[1;4r')
    await a.out('\x1bc')
    expect(a.watch.suffix()).toBe('')
  })

  it.each([
    ['a CSI cut after its parameters', '$ \x1b[3'],
    ['a bare ESC', 'text\x1b'],
    ['an ESC charset designation missing its final byte', '\x1b('],
    ['an unterminated OSC', '\x1b]0;title'],
    ['an OSC cut inside its ST', '\x1b]0;title\x1b'],
    ['an unterminated DCS', '\x1bPq#0;2;0;0;0~'],
    ['a C1 CSI (U+009B) without its final byte', 'a\u009b3'],
  ])('no snapshot when the output ends inside %s', async (_name, tail) => {
    const a = live()
    await a.out(`hello\r\n${tail}`)
    expect(a.watch.suffix()).toBeNull()
  })

  it('no snapshot mid-UTF-8, and the next read completing the character clears it', async () => {
    const a = live()
    await a.out(new Uint8Array([0x24, 0x20, 0xe2, 0x94]))
    expect(a.watch.suffix()).toBeNull()
    await a.out(new Uint8Array([0x80, 0x6f, 0x6b]))
    expect(a.watch.suffix()).toBe('')
  })

  it('complete sequences end in ground: BEL- and ST-terminated OSC, CAN aborting a CSI', async () => {
    const a = live()
    await a.out('\x1b]0;t\x07\x1b]8;;http://x\x1b\\link\x1b]8;;\x1b\\ \x1b[3\x18 \x1b(B\x1b[1;32mok\x1b[0m')
    expect(a.watch.suffix()).toBe('')
  })

  it('a scroll region vetoes the snapshot until a full-screen DECSTBM or leaving the alt screen', async () => {
    const a = live()
    await a.out('\x1b[1;4r')
    expect(a.watch.suffix()).toBeNull()
    await a.out('\x1b[r')
    expect(a.watch.suffix()).toBe('')
    await a.out('\x1b[1;24r') // the whole screen is no region
    expect(a.watch.suffix()).toBe('')
    await a.out('\x1b[?1049h\x1b[2;20r')
    expect(a.watch.suffix()).toBeNull()
    await a.out('\x1b[?1049l') // xterm clears the alt buffer, region and all
    expect(a.watch.suffix()).toBe('')
  })
})
