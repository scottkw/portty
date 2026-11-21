// Phase 11 (PREV-02, PREV-04): the image and PDF preview renderers.
// jsdom has no layout and no URL.createObjectURL (RESEARCH Pitfall 10), so the
// pane size and the image's natural size are stubbed per test.
import { describe, it, expect, vi, afterEach } from 'vitest'
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import { ImagePreview } from '../ImagePreview'
import { PdfPreview } from '../PdfPreview'
;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const fixtureDir = resolve(dirname(fileURLToPath(import.meta.url)), '../../lib/__tests__/fixtures/preview')

let root: Root | null = null
let container: HTMLDivElement | null = null

function render(el: React.ReactElement): HTMLDivElement {
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  act(() => root!.render(el))
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

/** Give the pane a size and the image a natural size, then fire load. */
function loadImage(c: HTMLElement, natural: [number, number], pane: [number, number] = [400, 300]): HTMLImageElement {
  const paneEl = c.querySelector('.preview-image') as HTMLElement
  Object.defineProperty(paneEl, 'clientWidth', { configurable: true, value: pane[0] })
  Object.defineProperty(paneEl, 'clientHeight', { configurable: true, value: pane[1] })
  const img = c.querySelector('img')!
  Object.defineProperty(img, 'naturalWidth', { configurable: true, value: natural[0] })
  Object.defineProperty(img, 'naturalHeight', { configurable: true, value: natural[1] })
  act(() => {
    img.dispatchEvent(new Event('load'))
  })
  return img
}

describe('ImagePreview (PREV-02)', () => {
  it('renders exactly one img from the blob URL with the file name as alt, on the checkerboard class', () => {
    const c = render(<ImagePreview url="blob:test/1" name="photo.png" kind="image" onFail={() => {}} />)
    const imgs = c.querySelectorAll('img')
    expect(imgs).toHaveLength(1)
    expect(imgs[0].getAttribute('src')).toBe('blob:test/1')
    expect(imgs[0].getAttribute('alt')).toBe('photo.png')
    expect(imgs[0].classList.contains('preview-image__img')).toBe(true)
  })

  it('offers the Actual Size toggle when the natural size exceeds the fitted size; click flips aria-pressed', () => {
    const c = render(<ImagePreview url="blob:test/2" name="big.png" kind="image" onFail={() => {}} />)
    expect(c.querySelector('button')).toBeNull()
    loadImage(c, [3000, 2000])
    const btn = c.querySelector('button.preview-image__toggle') as HTMLButtonElement
    expect(btn).not.toBeNull()
    expect(btn.getAttribute('aria-label')).toBe('Actual Size')
    expect(btn.getAttribute('aria-pressed')).toBe('false')
    expect(btn.contains(c.querySelector('img'))).toBe(true)
    act(() => btn.click())
    expect(btn.getAttribute('aria-pressed')).toBe('true')
    expect(c.querySelector('.preview-image')!.classList.contains('preview-image--actual')).toBe(true)
    act(() => btn.click())
    expect(btn.getAttribute('aria-pressed')).toBe('false')
  })

  it('the toggle is a native button, so Enter and Space activate it as a click', () => {
    // jsdom does not synthesise keyboard activation; the browser does for a
    // <button type="button">, which is why no keydown handler is added.
    const c = render(<ImagePreview url="blob:test/3" name="big.png" kind="image" onFail={() => {}} />)
    loadImage(c, [3000, 2000])
    const btn = c.querySelector('button.preview-image__toggle')
    expect(btn).toBeInstanceOf(HTMLButtonElement)
    expect(btn!.getAttribute('type')).toBe('button')
  })

  it('a pane resize re-measures: the toggle goes when the image now fits, and Actual Size resets', () => {
    let fire: () => void = () => {}
    vi.stubGlobal(
      'ResizeObserver',
      class {
        constructor(cb: () => void) {
          fire = cb
        }
        observe() {}
        disconnect() {}
      },
    )
    const c = render(<ImagePreview url="blob:test/4" name="big.png" kind="image" onFail={() => {}} />)
    loadImage(c, [800, 600])
    act(() => (c.querySelector('button') as HTMLButtonElement).click())
    const pane = c.querySelector('.preview-image') as HTMLElement
    Object.defineProperty(pane, 'clientWidth', { configurable: true, value: 2000 })
    Object.defineProperty(pane, 'clientHeight', { configurable: true, value: 2000 })
    act(() => fire())
    expect(c.querySelector('button')).toBeNull()
    expect(pane.classList.contains('preview-image--actual')).toBe(false)
  })

  it('a small image has no toggle (shown at natural size)', () => {
    const c = render(<ImagePreview url="blob:test/5" name="icon.png" kind="image" onFail={() => {}} />)
    loadImage(c, [64, 64])
    expect(c.querySelector('button')).toBeNull()
    expect(c.querySelectorAll('img')).toHaveLength(1)
  })

  it('kind svg never shows the toggle, even when large', () => {
    const c = render(<ImagePreview url="blob:test/6" name="d.svg" kind="svg" onFail={() => {}} />)
    loadImage(c, [5000, 5000])
    expect(c.querySelector('button')).toBeNull()
    expect(c.querySelector('img')!.classList.contains('preview-image__img--svg')).toBe(true)
  })

  it('autoFocus focuses the toggle once it is offered', () => {
    const c = render(<ImagePreview url="blob:test/7" name="big.png" kind="image" onFail={() => {}} autoFocus />)
    loadImage(c, [3000, 2000])
    expect(document.activeElement).toBe(c.querySelector('button.preview-image__toggle'))
  })

  it('an img error calls onFail', () => {
    const onFail = vi.fn()
    const c = render(<ImagePreview url="blob:test/8" name="bad.png" kind="image" onFail={onFail} />)
    act(() => {
      c.querySelector('img')!.dispatchEvent(new Event('error'))
    })
    expect(onFail).toHaveBeenCalledTimes(1)
  })

  it.each(['hostile-script.svg', 'hostile-external.svg'])(
    '%s is shown only as one img from its blob URL: no svg, script, iframe or foreignObject in the document (T-11-21)',
    (file) => {
      const text = readFileSync(resolve(fixtureDir, file), 'utf8')
      expect(text).toContain('BEACON')
      const blob = new Blob([text], { type: 'image/svg+xml' })
      const createURL = vi.fn((_b: Blob) => 'blob:test/hostile')
      const url = createURL(blob)
      const c = render(<ImagePreview url={url} name={file} kind="svg" onFail={() => {}} />)
      loadImage(c, [240, 120])
      expect(createURL).toHaveBeenCalledWith(blob)
      const imgs = document.querySelectorAll('img')
      expect(imgs).toHaveLength(1)
      expect(imgs[0].getAttribute('src')).toBe('blob:test/hostile')
      expect(document.querySelector('svg, script, iframe, foreignObject, object, embed')).toBeNull()
      expect(c.innerHTML).not.toContain('BEACON')
      expect(document.title).not.toMatch(/RAN/)
    },
  )
})

// 11-SPIKE.md decision: **PDF element:** iframe; **Browser PDF mode:** open-pdf.
describe('PdfPreview (PREV-02, PREV-04)', () => {
  it('desktop: one full-bleed iframe from the blob URL, titled with the file name', () => {
    const c = render(<PdfPreview url="blob:test/pdf" name="report.pdf" webMode={false} onFail={() => {}} />)
    const frames = c.querySelectorAll('iframe')
    expect(frames).toHaveLength(1)
    expect(frames[0].getAttribute('src')).toBe('blob:test/pdf')
    expect(frames[0].getAttribute('title')).toBe('report.pdf')
    expect(frames[0].classList.contains('preview-pdf')).toBe(true)
    expect(c.querySelector('object, embed, button')).toBeNull()
  })

  it.each([
    ['application/pdf', 0],
    ['text/html', 1],
    [null, 0],
  ] as const)('desktop: an iframe load whose document is %s calls onFail %d times', (contentType, calls) => {
    const onFail = vi.fn()
    const c = render(<PdfPreview url="blob:test/pdf" name="report.pdf" webMode={false} onFail={onFail} />)
    const frame = c.querySelector('iframe')!
    onFail.mockClear()
    // null is a document the page can't read (cross-origin): not treated as a failure.
    Object.defineProperty(frame, 'contentDocument', {
      configurable: true,
      value: contentType === null ? null : { contentType },
    })
    act(() => {
      frame.dispatchEvent(new Event('load'))
    })
    expect(onFail).toHaveBeenCalledTimes(calls)
  })

  it('desktop: autoFocus focuses the frame', () => {
    const c = render(<PdfPreview url="blob:test/pdf" name="report.pdf" webMode={false} onFail={() => {}} autoFocus />)
    expect(document.activeElement).toBe(c.querySelector('iframe'))
  })

  it('web mode (open-pdf): no frame; the pane shows the accepted copy and Open PDF opens the blob top-level', () => {
    const open = vi.spyOn(window, 'open').mockReturnValue(null)
    const c = render(<PdfPreview url="blob:test/pdf" name="report.pdf" webMode onFail={() => {}} autoFocus />)
    expect(c.querySelector('iframe, object, embed')).toBeNull()
    expect(c.querySelector('h2')?.textContent).toBe('report.pdf')
    expect(c.textContent).toContain("This browser can't show the PDF inside the page.")
    const btn = c.querySelector('button') as HTMLButtonElement
    expect(btn.textContent).toBe('Open PDF')
    expect(document.activeElement).toBe(btn)
    expect(open).not.toHaveBeenCalled()
    act(() => btn.click())
    // noopener makes window.open return null; that is not a failure (11-SPIKE.md S-04).
    expect(open).toHaveBeenCalledTimes(1)
    expect(open).toHaveBeenCalledWith('blob:test/pdf', '_blank', 'noopener,noreferrer')
  })
})
