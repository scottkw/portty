// Phase 14 (MOB-01, RESEARCH Pattern 5): the bytes the phone key bar sends. A closed table, so a
// character with no control code goes as typed rather than as a guess.
export const ESC = '\x1b'
export const TAB = '\t'
export const PIPE = '|'

export type ArrowDir = 'up' | 'down' | 'right' | 'left'
const ARROW_FINAL: Record<ArrowDir, string> = { up: 'A', down: 'B', right: 'C', left: 'D' }

/** The control byte for one character, or null when it has none (digits, multi-character strings). */
export function ctrlByte(ch: string): string | null {
  if (ch.length !== 1) return null
  const c = ch.charCodeAt(0)
  if ((c >= 0x41 && c <= 0x5a) || (c >= 0x61 && c <= 0x7a)) return String.fromCharCode(c & 0x1f)
  switch (ch) {
    case '@':
    case ' ': return '\x00'
    case '[': return '\x1b'
    case '\\': return '\x1c'
    case ']': return '\x1d'
    case '^': return '\x1e'
    case '_': return '\x1f'
    default: return null
  }
}

/** ESC [ x normally, ESC O x in application cursor mode, ESC [ 1 ; 5 x with Ctrl in either mode. */
export function arrowSeq(dir: ArrowDir, applicationCursor: boolean, ctrl: boolean): string {
  const f = ARROW_FINAL[dir]
  if (ctrl) return `${ESC}[1;5${f}`
  return `${ESC}${applicationCursor ? 'O' : '['}${f}`
}
