import type { IBufferLine, IDisposable, ILink, Terminal } from '@xterm/xterm'
import { isModifierPressed, type ModifierMode } from './openLink'

// Phase 12 (PATH-02): file-path links for the terminal. Detection mirrors the host's token alphabet
// (internal/relay/pathclick.go) and is pinned by frontend/src/lib/__tests__/fixtures/path-click.json.
// The host decides every outcome; this file only finds, underlines and reports the click.

export interface PathLink {
  start: number // offsets into the scanned string, end exclusive
  end: number
  text: string // what gets underlined: the token plus a trailing :line[:col]
  request: string // what is sent to the host: the token alone
}

const RUN = /[\p{L}\p{N}_.~@%+#\-/]+/gu
const URLISH = /[A-Za-z][A-Za-z0-9+.-]*:\/\/\S+|mailto:\S+/gi
const ANCHORED = /^(?:\/|~\/|\.\.?\/)[^/]/
const EXTENSION = /\.[\p{L}\p{N}]+$/u
const LINE_COL = /^:\d+(?::\d+)?(?![\p{L}\p{N}])/u

/** The token without a trailing :line[:col] (what the host is asked for). */
export function stripLineCol(text: string): string {
  return text.replace(/:\d+(?::\d+)?$/, '')
}

export function findPathLinks(text: string): PathLink[] {
  const urls = [...text.matchAll(URLISH)].map((m) => [m.index!, m.index! + m[0].length] as const)
  const out: PathLink[] = []
  for (const m of text.matchAll(RUN)) {
    const start = m.index!
    const token = m[0].replace(/\.+$/, '')
    if (!token) continue
    const lastSegment = token.slice(token.lastIndexOf('/') + 1)
    const bare = token.includes('/') && !token.startsWith('/') && !token.startsWith('~') && EXTENSION.test(lastSegment)
    if (!ANCHORED.test(token) && !bare) continue
    let end = start + token.length
    const suffix = LINE_COL.exec(text.slice(end))
    if (suffix) end += suffix[0].length
    // URLs keep their own click behaviour (confirm popover, risk checks); never a path link.
    if (urls.some(([us, ue]) => start < ue && end > us)) continue
    out.push({ start, end, text: text.slice(start, end), request: token })
  }
  return out
}

const MAX_LOGICAL_CHARS = 2048

interface Cell { x: number; y: number; w: number } // 1-based x and y, width in cells

// The logical line row `y` (1-based) belongs to: wrapped rows joined, with a map from string offset to cell.
function logicalLine(term: Terminal, y: number): { text: string; cells: Cell[] } {
  const buf = term.buffer.active
  let first = y - 1
  while (first > 0 && buf.getLine(first)?.isWrapped) first--
  let text = ''
  const cells: Cell[] = []
  for (let r = first; text.length < MAX_LOGICAL_CHARS; r++) {
    const line: IBufferLine | undefined = buf.getLine(r)
    if (!line || (r > first && !line.isWrapped)) break
    for (let x = 0; x < line.length && text.length < MAX_LOGICAL_CHARS; x++) {
      const cell = line.getCell(x)
      if (!cell || cell.getWidth() === 0) continue // spacer after a wide character
      const chars = cell.getChars() || ' '
      for (let i = 0; i < chars.length; i++) cells.push({ x: x + 1, y: r + 1, w: cell.getWidth() })
      text += chars
    }
  }
  return { text, cells }
}

export interface PathLinkOptions {
  /** True only while the host has said path clicks are available on this socket. */
  enabled: () => boolean
  /** Read at event time, like the URL handler, so a settings change needs no re-registration. */
  modifier: () => ModifierMode
  /** Called with the request text on a modifier-click. */
  onClick: (request: string) => void
}

export function registerPathLinks(term: Terminal, opts: PathLinkOptions): IDisposable {
  let modDown = false
  let hovered: ILink | null = null
  const held = () => modDown || opts.modifier() === 'none'

  const paint = () => {
    // Always write through link.decorations: xterm replaces it with a tracked object after hover.
    if (hovered?.decorations) {
      hovered.decorations.underline = held()
      hovered.decorations.pointerCursor = held()
    }
  }
  const setMod = (down: boolean) => {
    if (down === modDown) return
    modDown = down
    paint()
  }
  const onKey = (e: KeyboardEvent) => setMod(isModifierPressed(e, opts.modifier()))
  const onMove = (e: MouseEvent) => setMod(isModifierPressed(e, opts.modifier()))
  const onLost = () => setMod(false) // a Cmd keyup is lost on Cmd-Tab

  window.addEventListener('keydown', onKey, true)
  window.addEventListener('keyup', onKey, true)
  window.addEventListener('blur', onLost)
  document.addEventListener('visibilitychange', onLost)
  const el = term.element!
  el.addEventListener('mousemove', onMove, true)

  const provider = term.registerLinkProvider({
    provideLinks(y, callback) {
      if (!opts.enabled()) return callback(undefined)
      const { text, cells } = logicalLine(term, y)
      const links: ILink[] = []
      for (const m of findPathLinks(text)) {
        const a = cells[m.start]
        const z = cells[m.end - 1]
        if (y < a.y || y > z.y) continue
        const link: ILink = {
          range: { start: { x: a.x, y: a.y }, end: { x: z.x + z.w - 1, y: z.y } },
          text: m.text,
          decorations: { underline: held(), pointerCursor: held() },
          hover: () => { hovered = link },
          leave: () => { if (hovered === link) hovered = null },
          activate: (event) => {
            if (isModifierPressed(event, opts.modifier())) opts.onClick(m.request)
          },
        }
        links.push(link)
      }
      callback(links.length ? links : undefined)
    },
  })

  return {
    dispose() {
      provider.dispose()
      window.removeEventListener('keydown', onKey, true)
      window.removeEventListener('keyup', onKey, true)
      window.removeEventListener('blur', onLost)
      document.removeEventListener('visibilitychange', onLost)
      el.removeEventListener('mousemove', onMove, true)
      hovered = null
    },
  }
}
