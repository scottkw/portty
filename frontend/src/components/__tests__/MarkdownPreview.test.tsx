// Phase 11 (PREV-02): the Markdown preview pane.
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import { MarkdownPreview } from '../MarkdownPreview'
import { openLink } from '../../lib/openLink'
import { parseMarkdown } from '../../lib/markdown'
import { MARKDOWN_PARSE_TIMEOUT_MS } from '../../lib/parseMarkdownInWorker'

vi.mock('../../lib/openLink', () => ({ openLink: vi.fn() }))
;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const here = dirname(fileURLToPath(import.meta.url))

/** jsdom has no Worker: this one runs the real parse on a later tick. */
class InlineWorker {
  static all: InlineWorker[] = []
  onmessage: ((e: { data: unknown }) => void) | null = null
  onerror: ((e: unknown) => void) | null = null
  terminated = false
  constructor() {
    InlineWorker.all.push(this)
  }
  postMessage(src: string): void {
    setTimeout(() => {
      if (!this.terminated) this.onmessage?.({ data: parseMarkdown(src) })
    })
  }
  terminate(): void {
    this.terminated = true
  }
}

/** A parse that never finishes, as marked does on hostile emphasis. */
class HangingWorker extends InlineWorker {
  postMessage(): void {}
}

beforeEach(() => {
  InlineWorker.all = []
  vi.stubGlobal('Worker', InlineWorker)
})

let root: Root | null = null
let container: HTMLDivElement | null = null

/** Render and wait for blob.text() and the lazy import to settle. */
async function render(el: React.ReactElement): Promise<HTMLDivElement> {
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  act(() => root!.render(el))
  await vi.waitFor(() => {
    if (!container!.querySelector('article')?.hasChildNodes()) throw new Error('not rendered yet')
  })
  return container
}

afterEach(() => {
  act(() => root?.unmount())
  container?.remove()
  root = null
  container = null
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  vi.mocked(openLink).mockReset()
})

const md = (s: string) => new Blob([s], { type: 'text/markdown' })

describe('MarkdownPreview (PREV-02)', () => {
  it('renders the blob text into an article labelled with the file name', async () => {
    const c = await render(<MarkdownPreview blob={md('# Hello\n\nworld')} name="README.md" webMode={false} onFail={() => {}} />)
    const article = c.querySelector('article.preview-markdown')!
    expect(article.getAttribute('aria-label')).toBe('README.md')
    expect(article.getAttribute('tabindex')).toBe('0')
    expect(article.querySelector('h1')?.textContent).toBe('Hello')
  })

  it('reads the Blob with text() and never fetches', async () => {
    const fetchSpy = vi.fn()
    vi.stubGlobal('fetch', fetchSpy)
    const blob = md('# T')
    const textSpy = vi.spyOn(blob, 'text')
    await render(<MarkdownPreview blob={blob} name="a.md" webMode={false} onFail={() => {}} />)
    expect(textSpy).toHaveBeenCalledTimes(1)
    expect(fetchSpy).not.toHaveBeenCalled()
  })

  it('imports marked and DOMPurify only dynamically: marked in the worker, the sanitizer through import()', () => {
    const src = readFileSync(resolve(here, '../MarkdownPreview.tsx'), 'utf8')
    expect(src).not.toMatch(/^import .*(marked|dompurify)/m)
    expect(src).not.toMatch(/^import .*lib\/markdown(Sanitize)?['"]/m)
    expect(src).toMatch(/import\(['"]\.\.\/lib\/markdownSanitize['"]\)/)
    const client = readFileSync(resolve(here, '../../lib/parseMarkdownInWorker.ts'), 'utf8')
    expect(client).not.toMatch(/^import /m)
    expect(client).toContain("new Worker(new URL('./markdownWorker.ts', import.meta.url), { type: 'module' })")
  })

  it('calls onFail when the blob cannot be read', async () => {
    const onFail = vi.fn()
    const blob = md('x')
    vi.spyOn(blob, 'text').mockRejectedValue(new Error('boom'))
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
    act(() => root!.render(<MarkdownPreview blob={blob} name="a.md" webMode={false} onFail={onFail} />))
    await vi.waitFor(() => expect(onFail).toHaveBeenCalledTimes(1))
  })

  it('a parse that never finishes: onFail within 3.5 s, the worker is terminated, and the main thread keeps running', async () => {
    vi.stubGlobal('Worker', HangingWorker)
    const onFail = vi.fn()
    let ticks = 0
    const timer = setInterval(() => ticks++, 50)
    const start = performance.now()
    try {
      container = document.createElement('div')
      document.body.appendChild(container)
      root = createRoot(container)
      act(() =>
        root!.render(<MarkdownPreview blob={md('*a **b _c'.repeat(9_000))} name="a.md" webMode={false} onFail={onFail} />),
      )
      await vi.waitFor(() => expect(onFail).toHaveBeenCalledTimes(1), { timeout: 3500, interval: 20 })
    } finally {
      clearInterval(timer)
    }
    expect(performance.now() - start).toBeGreaterThanOrEqual(MARKDOWN_PARSE_TIMEOUT_MS - 50)
    expect(InlineWorker.all).toHaveLength(1)
    expect(InlineWorker.all[0].terminated).toBe(true)
    expect(ticks).toBeGreaterThanOrEqual(20)
    expect(container.querySelector('article')!.hasChildNodes()).toBe(false)
  }, 10_000)

  it('unmounting terminates a running parse and never calls onFail', async () => {
    vi.stubGlobal('Worker', HangingWorker)
    const onFail = vi.fn()
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
    act(() => root!.render(<MarkdownPreview blob={md('# T')} name="a.md" webMode={false} onFail={onFail} />))
    await vi.waitFor(() => expect(InlineWorker.all).toHaveLength(1))
    act(() => root!.unmount())
    root = null
    expect(InlineWorker.all[0].terminated).toBe(true)
    await new Promise((r) => setTimeout(r, 50))
    expect(onFail).not.toHaveBeenCalled()
  })
})

const hostile = readFileSync(resolve(here, '../../lib/__tests__/fixtures/preview/hostile.md'), 'utf8')

/** Dispatch an event and return it so defaultPrevented can be read. */
function fire<E extends Event>(el: Element, ev: E): E {
  act(() => {
    el.dispatchEvent(ev)
  })
  return ev
}
const click = (el: Element, init: MouseEventInit = {}) =>
  fire(el, new MouseEvent('click', { bubbles: true, cancelable: true, button: 0, ...init }))

describe('MarkdownPreview links (PREV-02, Pitfall 7)', () => {
  it('a safe https link whose text is its URL opens through openLink, and the click is prevented', async () => {
    const c = await render(<MarkdownPreview blob={md('<https://example.com/a>')} name="a.md" webMode={false} onFail={() => {}} />)
    const a = c.querySelector('a[href="https://example.com/a"]')!
    expect(click(a).defaultPrevented).toBe(true)
    expect(openLink).toHaveBeenCalledWith('https://example.com/a')
    expect(document.querySelector('.link-confirm-popover')).toBeNull()
  })

  it('a display/href mismatch shows LinkConfirmPopover first; Continue opens, Cancel does not', async () => {
    const c = await render(<MarkdownPreview blob={md('[apple.com](https://evil.example)')} name="a.md" webMode={false} onFail={() => {}} />)
    const a = c.querySelector('a')!
    expect(click(a, { clientX: 10, clientY: 20 }).defaultPrevented).toBe(true)
    expect(openLink).not.toHaveBeenCalled()
    const pop = document.querySelector('.link-confirm-popover')!
    expect(pop.querySelector('.link-confirm-popover__url')?.textContent).toBe('https://evil.example')
    act(() => (pop.querySelector('.link-confirm-popover__btn--cancel') as HTMLButtonElement).click())
    expect(document.querySelector('.link-confirm-popover')).toBeNull()
    expect(openLink).not.toHaveBeenCalled()
    // The popover's re-render must not reassign the article's HTML (the same anchor still works).
    expect(a.isConnected).toBe(true)
    click(a)
    act(() => (document.querySelector('.link-confirm-popover__btn--continue') as HTMLButtonElement).click())
    expect(openLink).toHaveBeenCalledWith('https://evil.example')
  })

  it('auxclick and Enter on a link are prevented; auxclick opens nothing', async () => {
    const c = await render(<MarkdownPreview blob={md('<https://example.com/a>')} name="a.md" webMode={false} onFail={() => {}} />)
    const a = c.querySelector('a')!
    const aux = fire(a, new MouseEvent('auxclick', { bubbles: true, cancelable: true, button: 1 }))
    expect(aux.defaultPrevented).toBe(true)
    expect(openLink).not.toHaveBeenCalled()
    const enter = fire(a, new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))
    expect(enter.defaultPrevented).toBe(true)
    expect(openLink).toHaveBeenCalledWith('https://example.com/a')
  })

  it('a #fragment link scrolls the matching heading and adds no id', async () => {
    const scroll = vi.fn()
    Element.prototype.scrollIntoView = scroll
    const c = await render(
      <MarkdownPreview blob={md('[go](#second-part)\n\n# First\n\n## Second Part!\n')} name="a.md" webMode={false} onFail={() => {}} />,
    )
    expect(click(c.querySelector('a')!).defaultPrevented).toBe(true)
    expect(scroll).toHaveBeenCalledTimes(1)
    expect(scroll.mock.contexts[0]).toBe(c.querySelector('h2'))
    expect(c.querySelector('[id]')).toBeNull()
    expect(openLink).not.toHaveBeenCalled()
  })

  it('a #fragment with no matching heading does nothing', async () => {
    const scroll = vi.fn()
    Element.prototype.scrollIntoView = scroll
    const c = await render(<MarkdownPreview blob={md('[go](#nowhere)\n\n# First')} name="a.md" webMode={false} onFail={() => {}} />)
    expect(click(c.querySelector('a')!).defaultPrevented).toBe(true)
    expect(scroll).not.toHaveBeenCalled()
  })

  it('a stripped javascript: anchor has no href, is not focusable and opens nothing', async () => {
    const c = await render(<MarkdownPreview blob={md('[x](javascript:alert(1))')} name="a.md" webMode={false} onFail={() => {}} />)
    const a = c.querySelector('a')!
    expect(a.hasAttribute('href')).toBe(false)
    expect(a.hasAttribute('tabindex')).toBe(false)
    a.focus()
    expect(document.activeElement).not.toBe(a)
    expect(click(a).defaultPrevented).toBe(true)
    expect(openLink).not.toHaveBeenCalled()
  })
})

describe('MarkdownPreview decoration (PREV-02)', () => {
  it('images are role img placeholders with an icon; links carry title = href; tables sit in .md-table-wrap', async () => {
    const c = await render(
      <MarkdownPreview
        blob={md('![a cat](http://x/cat.png) ![](y.png)\n\n[link](https://example.com/t "shown")\n\n| a |\n|---|\n| b |')}
        name="a.md"
        webMode={false}
        onFail={() => {}}
      />,
    )
    const imgs = [...c.querySelectorAll('.md-image')]
    expect(imgs.map((s) => [s.getAttribute('role'), s.getAttribute('aria-label')])).toEqual([
      ['img', 'Image: a cat'],
      ['img', 'Image: Image'],
    ])
    expect(imgs[0].querySelectorAll('svg.md-image__icon')).toHaveLength(1)
    expect(c.querySelector('a')!.getAttribute('title')).toBe('https://example.com/t')
    expect(c.querySelector('table')!.parentElement!.className).toBe('md-table-wrap')
  })

  it('the hostile fixture leaves no fetching, scripting or styled element in the pane', async () => {
    const fetchSpy = vi.fn()
    vi.stubGlobal('fetch', fetchSpy)
    const c = await render(<MarkdownPreview blob={md(hostile)} name="hostile.md" webMode={false} onFail={() => {}} />)
    const article = c.querySelector('article')!
    expect(article.querySelector('img, script, style, iframe, object, embed, form, input, meta, base, link, math')).toBeNull()
    // The only svg elements are the trusted placeholder icons.
    for (const svg of article.querySelectorAll('svg')) expect(svg.getAttribute('class')).toBe('md-image__icon')
    for (const el of article.querySelectorAll('*')) {
      for (const attr of el.attributes) expect(attr.name.startsWith('on'), attr.name).toBe(false)
      expect(el.hasAttribute('style')).toBe(false)
    }
    expect(document.title).not.toContain('BEACON')
    expect(fetchSpy).not.toHaveBeenCalled()
  })

  it('autoFocus focuses the article once rendered', async () => {
    const c = await render(<MarkdownPreview blob={md('# T')} name="a.md" webMode={false} onFail={() => {}} autoFocus />)
    expect(document.activeElement).toBe(c.querySelector('article'))
  })
})
