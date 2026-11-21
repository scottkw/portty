// Phase 11 (PREV-02): the Markdown renderer and its sanitizer configuration.
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import { parseMarkdown, MAX_DEPTH } from '../markdown'
import { sanitizeMarkdown } from '../markdownSanitize'

// The two steps MarkdownPreview runs: parse (in the worker), then sanitize (on the main thread).
const renderMarkdown = (src: string): string => sanitizeMarkdown(parseMarkdown(src))

const hostile = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), 'fixtures/preview/hostile.md'),
  'utf8',
).replace(/BEACON/g, '127.0.0.1:8765')

/** hostile.md split at its "## <label>" headings. */
const sections: Array<[string, string]> = hostile
  .split(/^(?=## )/m)
  .slice(1)
  .map((s: string): [string, string] => [s.slice(3, s.indexOf('\n')).trim(), s])

const BANNED = ['<img', '<script', '<style', '<iframe', '<object', '<embed', '<form', '<input', '<meta', '<base',
  '<link', '<svg', '<math', 'style=', 'src=', 'srcset', 'url(', '@import']

function expectInert(html: string): void {
  // Markup only: text between tags is escaped (a '<' there is '&lt;'), so a dropped <script>'s body showing as text is inert.
  const markup = html.replace(/>[^<]*</g, '><').toLowerCase()
  for (const b of BANNED) expect(markup, b).not.toContain(b)
  for (const el of dom(html).querySelectorAll('*')) {
    for (const a of el.attributes) {
      expect(a.name.startsWith('on'), `${el.tagName} ${a.name}`).toBe(false)
      expect(['href', 'class', 'title', 'align']).toContain(a.name)
    }
    const href = el.getAttribute('href')
    if (href !== null) expect(href).toMatch(/^(?:https?:|mailto:|#)/i)
  }
}

/** Parse the sanitizer's output string into a detached template (never live, never fetched). */
function dom(html: string): DocumentFragment {
  const t = document.createElement('template')
  t.innerHTML = html
  return t.content
}

describe('renderMarkdown (PREV-02)', () => {
  it('renders headings, lists, emphasis, inline code, a code block and a GFM table to the allowed tags', () => {
    const out = renderMarkdown(
      [
        '# Title',
        '## Sub',
        '',
        'Some *em* and **strong** and ~~del~~ and `code`.',
        '',
        '- one',
        '- two',
        '',
        '1. first',
        '',
        '> quoted',
        '',
        '```',
        'block',
        '```',
        '',
        '| a | b |',
        '|---|---|',
        '| 1 | 2 |',
        '',
        '---',
      ].join('\n'),
    )
    const d = dom(out)
    expect(d.querySelector('h1')?.textContent).toBe('Title')
    expect(d.querySelector('h2')?.textContent).toBe('Sub')
    expect(d.querySelector('em')?.textContent).toBe('em')
    expect(d.querySelector('strong')?.textContent).toBe('strong')
    expect(d.querySelector('del')?.textContent).toBe('del')
    expect(d.querySelector('p code')?.textContent).toBe('code')
    expect(d.querySelectorAll('ul li')).toHaveLength(2)
    expect(d.querySelector('ol li')?.textContent).toBe('first')
    expect(d.querySelector('blockquote')?.textContent?.trim()).toBe('quoted')
    expect(d.querySelector('pre code')?.textContent).toBe('block\n')
    expect(d.querySelectorAll('table thead th')).toHaveLength(2)
    expect(d.querySelectorAll('table tbody td')).toHaveLength(2)
    expect(d.querySelector('hr')).not.toBeNull()
  })

  it('drops raw HTML, block and inline', () => {
    const out = renderMarkdown('<div class="x">block</div>\n\ntext <b>bold</b> <span>s</span> end')
    expect(out).not.toMatch(/<div|<b>|class="x"/)
    const d = dom(out)
    expect(d.textContent).not.toContain('block')
    expect(d.querySelector('p')?.textContent).toBe('text bold s end')
  })

  it('turns an image into an md-image span with its alt text, or Image when there is none', () => {
    const d = dom(renderMarkdown('![a cat](http://example.com/cat.png) ![](cat.png)'))
    const spans = d.querySelectorAll('span.md-image')
    expect(spans).toHaveLength(2)
    expect(spans[0].textContent).toBe('a cat')
    expect(spans[1].textContent).toBe('Image')
    expect(d.querySelector('img')).toBeNull()
  })

  it('escapes HTML inside alt text', () => {
    const out = renderMarkdown('![<b onclick=x>hi</b>](x.png)')
    const d = dom(out)
    expect(d.querySelector('b')).toBeNull()
    expect(d.querySelector('span.md-image')?.textContent).toContain('hi')
  })

  it('turns task items into md-task glyph spans, never an input', () => {
    const out = renderMarkdown('- [x] done\n- [ ] todo')
    const d = dom(out)
    const tasks = d.querySelectorAll('span.md-task')
    expect(tasks).toHaveLength(2)
    expect(tasks[0].textContent).toBe('☑')
    expect(tasks[1].textContent).toBe('☐')
    expect(out).not.toMatch(/<input/i)
  })
})

describe('renderMarkdown hostile fixture (PREV-02)', () => {
  it('has the sections the plan lists', () => {
    const labels = sections.map(([l]) => l)
    for (const l of ['script', 'img-onerror', 'javascript-links', 'data-links', 'images', 'iframe', 'object', 'embed',
      'svg-onload', 'math-mxss', 'style-import', 'form', 'meta-refresh', 'base-href', 'link-stylesheet', 'safe-links',
      'task-list', 'table']) {
      expect(labels).toContain(l)
    }
  })

  it.each(sections)('section %s renders inert', (_label, src) => {
    expectInert(renderMarkdown(src))
  })

  it('the whole document renders inert', () => {
    expectInert(renderMarkdown(hostile))
  })

  it('javascript:, data:, vbscript:, file: and relative links keep no href', () => {
    for (const label of ['javascript-links', 'data-links', 'vbscript-and-file-links']) {
      const src = sections.find(([l]) => l === label)![1]
      const anchors = [...dom(renderMarkdown(src)).querySelectorAll('a')]
      expect(anchors.length, label).toBeGreaterThan(0)
      for (const a of anchors) {
        // GFM autolinks the beacon URL inside the hostile text; that link is a plain http link to what it shows.
        if (a.textContent?.startsWith('http://127.0.0.1:8765/')) expect(a.getAttribute('href')).toBe(a.textContent)
        else expect(a.hasAttribute('href'), `${label}: ${a.textContent}`).toBe(false)
      }
    }
  })

  it('https, mailto and #fragment links keep their href', () => {
    const src = sections.find(([l]) => l === 'safe-links')![1]
    const hrefs = [...dom(renderMarkdown(src)).querySelectorAll('a')].map((a) => a.getAttribute('href'))
    expect(hrefs).toEqual(['https://example.com/safe', 'https://example.com/autolink', 'mailto:someone@example.com', '#intro'])
  })

  it('images become placeholders, also inside a link, and nothing names the beacon', () => {
    const src = sections.find(([l]) => l === 'images')![1]
    const out = renderMarkdown(src)
    const d = dom(out)
    expect([...d.querySelectorAll('span.md-image')].map((s) => s.textContent)).toEqual([
      'remote image', 'relative image', 'Image', 'linked image', 'ref image',
    ])
    expect(d.querySelector('a[href="https://example.com/linked"] span.md-image')).not.toBeNull()
    expect(out).not.toContain('127.0.0.1')
  })

  it('tables keep align', () => {
    const src = sections.find(([l]) => l === 'table')![1]
    const ths = [...dom(renderMarkdown(src)).querySelectorAll('th')]
    expect(ths.map((t) => t.getAttribute('align'))).toEqual(['left', 'center', 'right'])
  })
})

describe('renderMarkdown DoS bounds (PREV-02, T-11-25)', () => {
  const depthOf = (html: string, tag: string) => (html.match(new RegExp(`<${tag}>`, 'g')) ?? []).length

  it('renders a 10,000-level nested blockquote without throwing, nesting capped at MAX_DEPTH', () => {
    const out = renderMarkdown('>'.repeat(10_000) + ' deep')
    expect(depthOf(out, 'blockquote')).toBe(MAX_DEPTH)
    expect(out).toContain('deep')
  })

  it('renders a 10,000-level nested list without throwing, nesting capped at MAX_DEPTH', () => {
    const out = renderMarkdown('- '.repeat(10_000) + 'x')
    expect(depthOf(out, 'ul')).toBe(MAX_DEPTH)
  })

  it('caps an indentation-nested list too', () => {
    const src = Array.from({ length: 2_000 }, (_, i) => '  '.repeat(i) + '- x').join('\n')
    expect(depthOf(renderMarkdown(src), 'ul')).toBeLessThanOrEqual(MAX_DEPTH)
  })

  it('keeps ordinary nesting below the cap', () => {
    const out = renderMarkdown('> > > three\n\n- a\n  - b\n    - c')
    expect(depthOf(out, 'blockquote')).toBe(3)
    expect(depthOf(out, 'ul')).toBe(3)
  })

  it('renders a 2 MB single line without throwing', () => {
    const out = renderMarkdown('a'.repeat(2_000_000))
    expect(out.length).toBeGreaterThanOrEqual(2_000_000)
  })
})
