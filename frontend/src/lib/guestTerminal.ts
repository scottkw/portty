import type { Terminal } from '@xterm/xterm'

/**
 * Apply the host's grid (MsgResize) in stream order. xterm's resize() runs at
 * once while earlier output may still be queued, so old-width bytes would be
 * parsed at the new width (planner probe 2026-11-17). An empty write's callback
 * runs after everything queued before it and before anything queued after.
 * Clamped to at least 1 (T-157-03: xterm rejects a zero grid).
 */
export function applyHostGrid(term: Terminal, cols: number, rows: number, after?: () => void): void {
  term.write('', () => {
    term.resize(Math.max(1, cols), Math.max(1, rows))
    after?.()
  })
}

/**
 * Clear the terminal before a full replay, in stream order. xterm's reset()
 * does not drop writes still queued, so stale output would be drawn after it;
 * RIS (ESC c) does the same full reset (buffer, scrollback, modes) in order.
 */
export function resetForFullReplay(term: Terminal): void {
  term.write('\x1bc')
}
