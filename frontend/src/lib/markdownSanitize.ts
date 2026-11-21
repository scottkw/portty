/**
 * Phase 11 (PREV-02): the sanitizer for parseMarkdown's output. Loaded only through
 * import() from MarkdownPreview, so DOMPurify stays out of the entry chunk. It runs
 * on the main thread, never in the worker: DOMPurify needs a DOM, and in a worker
 * (no document) it reports isSupported = false and sanitize() returns its input
 * unchanged. Sanitizing is linear in the input, so it can't freeze the window.
 *
 * DOMPurify in plain string mode with explicit allowlists (11-RESEARCH "Sanitizer
 * configuration"). No profile (a profile replaces the allowlists and turns
 * SVG/MathML on), no hooks, no ADD_TAGS, and none of the IN_PLACE,
 * SAFE_FOR_TEMPLATES or RETURN_DOM modes, where the 2026 advisories live.
 * The caller assigns the returned string once.
 */
import DOMPurify from 'dompurify'

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

export function sanitizeMarkdown(html: string): string {
  return DOMPurify.sanitize(html, SANITIZE)
}
