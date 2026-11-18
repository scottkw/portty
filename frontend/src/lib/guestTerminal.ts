import type { Terminal } from '@xterm/xterm'

// RED stub (05-12 Task 1): still the synchronous calls TerminalPanel makes today.
export function applyHostGrid(term: Terminal, cols: number, rows: number, after?: () => void): void {
  term.resize(Math.max(1, cols), Math.max(1, rows))
  after?.()
}

export function resetForFullReplay(term: Terminal): void {
  term.reset()
}
