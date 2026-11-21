// Phase 11 (PREV-02, UI-SPEC "States", "Size display", "Accessibility Contract"): the preview
// pane's states. The renderers are stubbed here (PreviewRenderers.test.tsx and
// MarkdownPreview.test.tsx cover them); each stub shows its url and can fail, throw or focus.

import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { flushSync } from 'react-dom'

import { PreviewPanel } from '../PreviewPanel'
import type { PreviewEntry } from '../../lib/previewStore'

type StubProps = { url?: string; blob?: Blob; name: string; onFail: () => void; autoFocus?: boolean }

const stub = vi.hoisted(() => ({ target: true }))

function makeStub(label: string) {
  return function Stub(p: StubProps) {
    const ref = React.useRef<HTMLButtonElement>(null)
    React.useEffect(() => { if (p.autoFocus && stub.target) ref.current?.focus() }, [p.autoFocus])
    if (p.name === 'boom.png') throw new Error('render failed')
    return React.createElement('div', { 'data-testid': label, 'data-url': p.url ?? '' },
      stub.target ? React.createElement('button', { ref, type: 'button', 'data-testid': `${label}-target` }) : null,
      React.createElement('button', { type: 'button', 'data-testid': `${label}-fail`, onClick: p.onFail }))
  }
}

vi.mock('../ImagePreview', () => ({ ImagePreview: makeStub('image') }))
vi.mock('../PdfPreview', () => ({ PdfPreview: makeStub('pdf') }))
vi.mock('../MarkdownPreview', () => ({ MarkdownPreview: makeStub('markdown') }))

let root: Root | undefined
let container: HTMLDivElement

beforeEach(() => {
  stub.target = true
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
})

afterEach(() => {
  flushSync(() => root?.unmount())
  container.remove()
  vi.useRealTimers()
})

function entry(patch: Partial<PreviewEntry> = {}): PreviewEntry {
  return {
    tabId: '__preview__s1/' + 'a'.repeat(32),
    ownerTabId: 's1',
    key: 'a'.repeat(32),
    name: 'diagram.png',
    kind: 'image',
    mime: 'image/png',
    size: 4,
    state: 'rendered',
    received: 4,
    blob: new Blob(['x']),
    url: 'blob:test/1',
    arrivedAt: Date.now(),
    refreshing: false,
    ...patch,
  }
}

function render(e: PreviewEntry, props: { webMode?: boolean; autoFocus?: boolean } = {}) {
  flushSync(() => root!.render(React.createElement(PreviewPanel, { entry: e, webMode: props.webMode ?? false, autoFocus: props.autoFocus })))
}

function status(): { heading: string; body: string } | null {
  const s = container.querySelector('[role="status"]')
  return s ? { heading: s.querySelector('h2')?.textContent ?? '', body: s.querySelector('p')?.textContent ?? '' } : null
}

describe('PreviewPanel: receiving (UI-SPEC "States")', () => {
  it('shows nothing for 300 ms, then the name, a labelled <progress> and the decimal caption', () => {
    vi.useFakeTimers()
    render(entry({ state: 'receiving', size: 18_600_000, received: 4_200_000, blob: undefined, url: undefined, arrivedAt: Date.now() }))
    expect(container.querySelector('progress')).toBeNull()
    expect(container.textContent).toBe('')
    act(() => { vi.advanceTimersByTime(299) })
    expect(container.querySelector('progress')).toBeNull()
    act(() => { vi.advanceTimersByTime(1) })
    const bar = container.querySelector('progress')!
    expect(bar.getAttribute('aria-label')).toBe('Receiving diagram.png')
    expect(bar.max).toBe(18_600_000)
    expect(bar.value).toBe(4_200_000)
    expect(container.querySelector('h2')?.textContent).toBe('diagram.png')
    expect(container.querySelector('.preview-receiving__caption')?.textContent).toBe('Receiving… 4.2 MB of 18.6 MB')
    expect(container.querySelector('[aria-live]')).toBeNull() // the caption is not live
  })

  it('uses whole KB below 1 MB', () => {
    vi.useFakeTimers()
    render(entry({ state: 'receiving', size: 900_000, received: 640_000, blob: undefined, url: undefined, arrivedAt: Date.now() - 1000 }))
    act(() => { vi.advanceTimersByTime(0) })
    expect(container.querySelector('.preview-receiving__caption')?.textContent).toBe('Receiving… 640 KB of 900 KB')
  })

  it('never renders a partial file while receiving', () => {
    render(entry({ state: 'receiving', blob: undefined, url: undefined, received: 2 }))
    expect(container.querySelector('[data-testid="image"]')).toBeNull()
  })
})

describe('PreviewPanel: message panes (UI-SPEC "States")', () => {
  it('too large', () => {
    render(entry({ state: 'tooLarge', blob: undefined, url: undefined }))
    expect(status()).toEqual({ heading: 'Too Large to Preview', body: 'diagram.png is larger than 20 MB.' })
  })

  it("couldn't receive", () => {
    render(entry({ state: 'failed', blob: undefined, url: undefined }))
    expect(status()).toEqual({ heading: "Couldn't Receive File", body: "diagram.png didn't arrive completely. Run portty open again." })
  })

  it("a renderer's onFail shows Couldn't Show File", () => {
    render(entry())
    act(() => { container.querySelector<HTMLButtonElement>('[data-testid="image-fail"]')!.click() })
    expect(status()).toEqual({ heading: "Couldn't Show File", body: 'diagram.png may be damaged, or its format isn\'t supported.' })
  })

  it("a renderer that throws shows Couldn't Show File through the ErrorBoundary", () => {
    const err = vi.spyOn(console, 'error').mockImplementation(() => {})
    render(entry({ name: 'boom.png' }))
    expect(status()).toEqual({ heading: "Couldn't Show File", body: "boom.png may be damaged, or its format isn't supported." })
    err.mockRestore()
  })

  it('a refreshing entry keeps rendering its old content', () => {
    render(entry({ refreshing: true, received: 1, size: 10 }))
    expect(container.querySelector('[data-testid="image"]')?.getAttribute('data-url')).toBe('blob:test/1')
    expect(container.querySelector('progress')).toBeNull()
  })

  it('a refresh after a failed render shows the new copy', () => {
    render(entry())
    act(() => { container.querySelector<HTMLButtonElement>('[data-testid="image-fail"]')!.click() })
    render(entry({ url: 'blob:test/2' }))
    expect(container.querySelector('[data-testid="image"]')?.getAttribute('data-url')).toBe('blob:test/2')
  })

  it('dispatches by kind: pdf and markdown', () => {
    render(entry({ kind: 'pdf', mime: 'application/pdf', name: 'a.pdf' }))
    expect(container.querySelector('[data-testid="pdf"]')).not.toBeNull()
    render(entry({ kind: 'markdown', mime: 'text/markdown', name: 'a.md', tabId: 'other' }))
    expect(container.querySelector('[data-testid="markdown"]')).not.toBeNull()
  })

  it('a hostile name is shown as text and adds no element', () => {
    const name = '<img src=x onerror=alert(1)>‮gnp.exe'
    render(entry({ state: 'tooLarge', blob: undefined, url: undefined, name }))
    expect(container.querySelector('img')).toBeNull()
    expect(status()?.heading).toBe('Too Large to Preview')
    expect(status()?.body).toBe(`${name} is larger than 20 MB.`)
  })
})

describe('PreviewPanel: focus (UI-SPEC "Accessibility Contract")', () => {
  it('autoFocus moves focus to the renderer target', () => {
    render(entry(), { autoFocus: true })
    expect(document.activeElement).toBe(container.querySelector('[data-testid="image-target"]'))
  })

  it('autoFocus with no renderer target focuses the pane', () => {
    stub.target = false
    render(entry(), { autoFocus: true })
    const pane = container.querySelector<HTMLElement>('.preview-panel')!
    expect(document.activeElement).toBe(pane)
    expect(pane.getAttribute('role')).toBe('tabpanel')
    expect(pane.id).toBe('tabpanel-__preview__s1/' + 'a'.repeat(32))
  })

  it('without autoFocus, focus stays where it was', () => {
    const outside = document.createElement('button')
    document.body.appendChild(outside)
    outside.focus()
    render(entry())
    expect(document.activeElement).toBe(outside)
    outside.remove()
  })
})
