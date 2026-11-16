import { describe, it, expect } from 'vitest'
import { Terminal } from '@xterm/xterm'
import { gridForContentBox, measureInitialGrid } from '../initialGrid'

// Quick 261116-ku9 (R2): the inverted "%" on a new session's first line is
// zsh's PROMPT_SP mark. zsh prints the standout "%", then COLUMNS−1 spaces, CR,
// a space, CR. If the xterm has fewer columns than the COLUMNS zsh used, the
// spaces wrap, the erase lands on the next row, and the mark stays on row 0.
// Byte shape captured from /bin/zsh -f -i on a 20-column pty (zsh 5.9).
function promptSpPreamble(shellCols: number): string {
  return '\x1b[1m\x1b[7m%\x1b[27m\x1b[1m\x1b[0m' + ' '.repeat(shellCols - 1) + '\r \r' +
    '\r\x1b[0m\x1b[27m\x1b[24m\x1b[J' + 'host% ' + '\x1b[K'
}

async function row0After(termCols: number, shellCols: number): Promise<string> {
  const term = new Terminal({ cols: termCols, rows: 24, allowProposedApi: true })
  await new Promise<void>((r) => term.write(promptSpPreamble(shellCols), r))
  const line = term.buffer.active.getLine(0)!.translateToString(true)
  term.dispose()
  return line
}

describe('PROMPT_SP mark mechanism (headless xterm)', () => {
  it('an 80-col xterm fed a 132-col shell keeps the mark on row 0 (the artifact)', async () => {
    expect((await row0After(80, 132)).startsWith('%')).toBe(true)
  })

  it('one column short is enough to keep the mark', async () => {
    expect((await row0After(131, 132)).startsWith('%')).toBe(true)
  })

  it('an xterm at least as wide as the shell erases the mark and the prompt sits on row 0', async () => {
    for (const cols of [132, 140]) {
      const row0 = await row0After(cols, 132)
      expect(row0.trimEnd()).toBe('host%')
    }
  })
})

describe('gridForContentBox', () => {
  it('floors the content box by the cell size', () => {
    expect(gridForContentBox(1028, 696, 7.8, 17)).toEqual({ cols: 131, rows: 40 })
  })

  it('clamps tiny or zero boxes to at least 2 cols and 1 row', () => {
    expect(gridForContentBox(0, 0, 7.8, 17)).toEqual({ cols: 2, rows: 1 })
    expect(gridForContentBox(5, 3, 7.8, 17)).toEqual({ cols: 2, rows: 1 })
  })
})

describe('measureInitialGrid', () => {
  it('returns null under jsdom (no layout) and leaves no probe behind', () => {
    const host = document.createElement('div')
    document.body.appendChild(host)
    expect(measureInitialGrid(host, 'monospace', 13)).toBeNull()
    expect(host.childElementCount).toBe(0)
    host.remove()
  })
})
