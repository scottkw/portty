/**
 * Phase 11 (PREV-02): Markdown from a remote shell to an HTML string that is NOT
 * yet safe. It runs only inside markdownWorker.ts (11-07 fix): marked's inline
 * tokenizer can take minutes on hostile input, so the parse runs off the main
 * thread under a hard timeout (parseMarkdownInWorker.ts). The main thread then
 * passes the result through sanitizeMarkdown (markdownSanitize.ts); unsanitized
 * HTML never reaches a DOM node.
 *
 * marked drops raw HTML and never emits <img> or <input>: images become
 * placeholder spans (nothing is fetched) and task boxes become glyphs
 * (11-RESEARCH "Sanitizer configuration", Pitfalls 8-9).
 */
import { Marked, Tokenizer, type Tokens } from 'marked'

const ENTITIES: Record<string, string> = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }
const escapeHtml = (s: string): string => s.replace(/[&<>"']/g, (c) => ENTITIES[c])

/**
 * marked recurses once per blockquote or list level, and its time grows with the
 * square of the depth (about 1 s at 1,000 levels; the stack overflows near 2,000).
 * Past MAX_DEPTH a container line stays literal paragraph text, as markdown-it's
 * maxNesting does (its strict presets use 20).
 */
export const MAX_DEPTH = 20
let depth = 0

type Tok = (this: Tokenizer, src: string) => Tokens.Blockquote | Tokens.List | Tokens.Paragraph | undefined

function capped(kind: 'blockquote' | 'list'): Tok {
  const original = Tokenizer.prototype[kind] as Tok
  return function (this: Tokenizer, src: string) {
    if (depth >= MAX_DEPTH) {
      if (!this.rules.block[kind].exec(src)) return undefined
      const raw = /^[^\n]*\n?/.exec(src)![0]
      const text = raw.trim()
      return { type: 'paragraph', raw, text, tokens: this.lexer.inline(text) }
    }
    depth++
    try {
      return original.call(this, src)
    } finally {
      depth--
    }
  }
}

const md = new Marked({
  gfm: true,
  async: false,
  // A capped container returns a paragraph token; marked's per-key types don't allow that, hence the casts.
  tokenizer: {
    blockquote: capped('blockquote') as unknown as Tokenizer['blockquote'],
    list: capped('list') as unknown as Tokenizer['list'],
  },
  renderer: {
    html: () => '',
    image({ text, tokens }) {
      const alt = tokens ? this.parser.parseInline(tokens, this.parser.textRenderer) : text
      return `<span class="md-image">${escapeHtml(alt || 'Image')}</span>`
    },
    checkbox: ({ checked }) => `<span class="md-task">${checked ? '☑' : '☐'}</span> `,
  },
})

/**
 * The formatted-render budget (11-REVIEW WR-03). The main thread sanitizes, inserts and lays out
 * every element parseMarkdown returns, and the worker's time limit covers none of that, so output
 * past either cap throws here, in the worker, and the pane shows "Couldn't Show File".
 */
export const MARKDOWN_MAX_HTML_LENGTH = 2_000_000
export const MARKDOWN_MAX_ELEMENTS = 50_000

/** Unsanitized HTML: always pass it through sanitizeMarkdown before it reaches the DOM. */
export function parseMarkdown(src: string): string {
  const html = md.parse(src) as string
  if (html.length > MARKDOWN_MAX_HTML_LENGTH) throw new Error('Markdown too large to format')
  // marked escapes '<' in text, so every '<' not followed by '/' opens an element.
  let elements = 0
  for (let i = html.indexOf('<'); i >= 0; i = html.indexOf('<', i + 1)) {
    if (html[i + 1] !== '/' && ++elements > MARKDOWN_MAX_ELEMENTS) throw new Error('Markdown too large to format')
  }
  return html
}
