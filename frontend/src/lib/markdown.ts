/**
 * Phase 11 (PREV-02): Markdown from a remote shell to a sanitized HTML string.
 * Loaded only through import() from MarkdownPreview, so marked and DOMPurify
 * stay out of the entry chunk.
 *
 * Two layers (11-RESEARCH "Sanitizer configuration", Pitfalls 8-9):
 *  1. marked drops raw HTML and never emits <img> or <input>: images become
 *     placeholder spans (nothing is fetched) and task boxes become glyphs.
 *  2. DOMPurify in plain string mode with explicit allowlists. No profile (a
 *     profile replaces the allowlists and turns SVG/MathML on), no hooks, no
 *     ADD_TAGS, and none of the IN_PLACE, SAFE_FOR_TEMPLATES or RETURN_DOM
 *     modes, where the 2026 advisories live.
 * The caller assigns the returned string once; unsanitized HTML never reaches a DOM node.
 */
import { Marked } from 'marked'
import DOMPurify from 'dompurify'

const ENTITIES: Record<string, string> = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }
const escapeHtml = (s: string): string => s.replace(/[&<>"']/g, (c) => ENTITIES[c])

const md = new Marked({
  gfm: true,
  async: false,
  renderer: {
    html: () => '',
    image({ text, tokens }) {
      const alt = tokens ? this.parser.parseInline(tokens, this.parser.textRenderer) : text
      return `<span class="md-image">${escapeHtml(alt || 'Image')}</span>`
    },
    checkbox: ({ checked }) => `<span class="md-task">${checked ? '☑' : '☐'}</span> `,
  },
})

const SANITIZE = {
  ALLOWED_TAGS: ['h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'p', 'br', 'hr', 'ul', 'ol', 'li', 'blockquote', 'pre', 'code',
    'em', 'strong', 'del', 'a', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'span'],
  ALLOWED_ATTR: ['href', 'class', 'title', 'align'],
  // Every allowed attribute not marked URI-safe is checked against ALLOWED_URI_REGEXP, so align must be marked.
  ADD_URI_SAFE_ATTR: ['align'],
  ALLOWED_URI_REGEXP: /^(?:https?:|mailto:|#)/i,
  ALLOW_DATA_ATTR: false,
  ALLOW_ARIA_ATTR: false,
  FORBID_ATTR: ['style'],
}

export function renderMarkdown(src: string): string {
  return DOMPurify.sanitize(md.parse(src) as string, SANITIZE)
}
