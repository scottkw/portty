// Quick 261116-ku9 (R2): spawn a new session's PTY at the grid its xterm will
// fit to, so zsh pads its first PROMPT_SP mark to the same column count the
// xterm renders it at (otherwise an inverted "%" line stays on row 0).
import { Terminal } from '@xterm/xterm'

/** cols/rows that fit a content box, at least 2×1. The math fitTerminal uses. */
export function gridForContentBox(contentWidth: number, contentHeight: number, cellWidth: number, cellHeight: number): { cols: number; rows: number } {
  return {
    cols: Math.max(2, Math.floor(contentWidth / cellWidth)),
    rows: Math.max(1, Math.floor(contentHeight / cellHeight)),
  }
}

/**
 * Measure the grid a new tab's TerminalPanel will fit to inside `host`
 * (the `.terminal-container`), or null when there is no layout yet.
 *
 * A probe `.terminal-session-container` gets the real panel padding (PAD-01),
 * and a throwaway DOM-renderer Terminal reports xterm's own cell metrics for
 * the configured font. The DOM renderer's cell is the CSS-measured glyph
 * width; the WebGL renderer floors that to whole device pixels, so its cell
 * is never wider. The measured grid is therefore never wider than the grid
 * the real panel fits to, and zsh's padding never wraps there.
 */
export function measureInitialGrid(host: HTMLElement, fontFamily: string, fontSize: number): { cols: number; rows: number } | null {
  const probe = document.createElement('div')
  probe.className = 'terminal-session-container'
  probe.style.cssText = 'position:absolute;inset:0;visibility:hidden;pointer-events:none'
  host.appendChild(probe)
  const term = new Terminal({ fontFamily, fontSize })
  try {
    term.open(probe)
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const cell = (term as any)._core?._renderService?.dimensions?.css?.cell
    const style = window.getComputedStyle(probe)
    const w = probe.clientWidth - parseFloat(style.paddingLeft || '0') - parseFloat(style.paddingRight || '0')
    const h = probe.clientHeight - parseFloat(style.paddingTop || '0') - parseFloat(style.paddingBottom || '0')
    if (!cell || !(cell.width > 0) || !(cell.height > 0) || !(w > 0) || !(h > 0)) return null
    return gridForContentBox(w, h, cell.width, cell.height)
  } catch {
    return null // no layout (jsdom) or no renderer: the daemon falls back to 80×24
  } finally {
    term.dispose()
    probe.remove()
  }
}
