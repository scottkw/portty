// Phase 11 (PREV-02): the Markdown preview pane.
import { describe, it, expect, vi, afterEach } from 'vitest'
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import { MarkdownPreview } from '../MarkdownPreview'
;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const here = dirname(fileURLToPath(import.meta.url))

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

  it('imports marked and DOMPurify only dynamically', () => {
    const src = readFileSync(resolve(here, '../MarkdownPreview.tsx'), 'utf8')
    expect(src).not.toMatch(/^import .*(marked|dompurify)/m)
    expect(src).not.toMatch(/^import .*lib\/markdown['"]/m)
    expect(src).toMatch(/import\(['"]\.\.\/lib\/markdown['"]\)/)
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
})
