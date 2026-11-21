// Phase 11 (PREV-02): the Markdown renderer and its sanitizer configuration.
import { describe, it, expect } from 'vitest'
import { renderMarkdown } from '../markdown'

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
