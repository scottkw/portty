import React, { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { LinkConfirmPopover } from './LinkConfirmPopover'
import { getRisk, isAllowedScheme, type RiskKind } from '../lib/urlSafety'
import { openLink } from '../lib/openLink'

interface MarkdownPreviewProps {
  blob: Blob
  name: string
  /** Part of the renderer contract 11-09 composes; openLink already picks desktop or browser. */
  webMode: boolean
  onFail: () => void
  autoFocus?: boolean
}

const SVG_NS = 'http://www.w3.org/2000/svg'
// Heroicons 24/outline PhotoIcon, built with trusted DOM calls (the sanitizer allows no svg).
const PHOTO_PATH =
  'm2.25 15.75 5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5 1.409-1.409a2.25 2.25 0 0 1 3.182 0l2.909 2.909m-18 3.75h16.5a1.5 1.5 0 0 0 1.5-1.5V6a1.5 1.5 0 0 0-1.5-1.5H3.75A1.5 1.5 0 0 0 2.25 6v12a1.5 1.5 0 0 0 1.5 1.5Zm10.5-11.25h.008v.008h-.008V8.25Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z'

function photoIcon(): SVGSVGElement {
  const svg = document.createElementNS(SVG_NS, 'svg')
  svg.setAttribute('viewBox', '0 0 24 24')
  svg.setAttribute('fill', 'none')
  svg.setAttribute('stroke', 'currentColor')
  svg.setAttribute('stroke-width', '1.5')
  svg.setAttribute('aria-hidden', 'true')
  svg.setAttribute('class', 'md-image__icon')
  const path = document.createElementNS(SVG_NS, 'path')
  path.setAttribute('stroke-linecap', 'round')
  path.setAttribute('stroke-linejoin', 'round')
  path.setAttribute('d', PHOTO_PATH)
  svg.appendChild(path)
  return svg
}

/** Trusted decoration of the already-sanitized, attached content. */
function decorate(root: HTMLElement): void {
  for (const a of root.querySelectorAll('a[href]')) a.setAttribute('title', a.getAttribute('href')!)
  for (const span of root.querySelectorAll('.md-image')) {
    span.setAttribute('role', 'img')
    span.setAttribute('aria-label', `Image: ${span.textContent}`)
    span.prepend(photoIcon())
  }
  for (const table of root.querySelectorAll('table')) {
    const wrap = document.createElement('div')
    wrap.className = 'md-table-wrap'
    table.replaceWith(wrap)
    wrap.appendChild(table)
  }
}

/** Heading slug: lowercase, spaces to hyphens, punctuation dropped. */
const slug = (s: string): string =>
  s.trim().toLowerCase().replace(/[^\p{L}\p{N}\s_-]/gu, '').replace(/\s/g, '-')

interface Confirm {
  url: string
  risk: RiskKind
  x: number
  y: number
}

/**
 * Phase 11 (PREV-02): a Markdown preview. The text comes from blob.text(), never
 * from fetching the blob URL (the share page's connect-src has no blob:). The
 * renderer is imported lazily so marked and DOMPurify stay out of the entry chunk,
 * and its sanitized string is assigned once. Links never navigate the pane
 * (RESEARCH Pitfall 7): every anchor event is prevented, #fragments scroll to the
 * matching heading, and other links go through the terminal's scheme and risk gate.
 */
export function MarkdownPreview({ blob, name, onFail, autoFocus }: MarkdownPreviewProps): React.ReactElement {
  const [html, setHtml] = useState<string | null>(null)
  const [confirm, setConfirm] = useState<Confirm | null>(null)
  const articleRef = useRef<HTMLElement>(null)
  const onFailRef = useRef(onFail)
  onFailRef.current = onFail

  useEffect(() => {
    let live = true
    Promise.all([blob.text(), import('../lib/markdown')])
      .then(([src, { renderMarkdown }]) => {
        if (live) setHtml(renderMarkdown(src))
      })
      .catch(() => {
        if (live) onFailRef.current()
      })
    return () => {
      live = false
    }
  }, [blob])

  // React 19 reassigns innerHTML whenever this object changes identity, which would drop the decoration.
  const inner = useMemo(() => ({ __html: html ?? '' }), [html])

  // Once per string: StrictMode re-runs effects.
  const decorated = useRef<string | null>(null)
  useLayoutEffect(() => {
    if (html && articleRef.current && decorated.current !== html) {
      decorate(articleRef.current)
      decorated.current = html
    }
  }, [html])

  useEffect(() => {
    if (html !== null && autoFocus) articleRef.current?.focus()
  }, [html, autoFocus])

  const activate = (a: HTMLAnchorElement, x: number, y: number): void => {
    const href = a.getAttribute('href')
    if (!href) return
    if (href.startsWith('#')) {
      let target = href.slice(1)
      try {
        target = decodeURIComponent(target)
      } catch {
        // A malformed escape: match the raw fragment.
      }
      const want = slug(target)
      const heading = [...articleRef.current!.querySelectorAll('h1, h2, h3, h4, h5, h6')].find(
        (h) => slug(h.textContent ?? '') === want,
      )
      heading?.scrollIntoView({ block: 'start' })
      return
    }
    if (!isAllowedScheme(href)) return
    const risk = getRisk(a.textContent ?? '', href)
    if (risk) setConfirm({ url: href, risk, x, y })
    else openLink(href)
  }

  const anchorOf = (e: React.SyntheticEvent): HTMLAnchorElement | null =>
    (e.target as Element).closest?.('a') ?? null

  const onClick = (e: React.MouseEvent): void => {
    const a = anchorOf(e)
    if (!a) return
    e.preventDefault()
    activate(a, e.clientX, e.clientY)
  }

  // Middle and other buttons never open or navigate anything.
  const onAuxClick = (e: React.MouseEvent): void => {
    if (anchorOf(e)) e.preventDefault()
  }

  // Enter would otherwise synthesise a navigating click.
  const onKeyDown = (e: React.KeyboardEvent): void => {
    if (e.key !== 'Enter') return
    const a = anchorOf(e)
    if (!a) return
    e.preventDefault()
    const r = a.getBoundingClientRect()
    activate(a, r.left, r.bottom)
  }

  return (
    <div className="preview-markdown-pane">
      <article
        ref={articleRef}
        className="preview-markdown"
        aria-label={name}
        tabIndex={0}
        onClick={onClick}
        onAuxClick={onAuxClick}
        onKeyDown={onKeyDown}
        dangerouslySetInnerHTML={inner}
      />
      {confirm && (
        <LinkConfirmPopover
          {...confirm}
          onContinue={() => {
            openLink(confirm.url)
            setConfirm(null)
          }}
          onCancel={() => setConfirm(null)}
        />
      )}
    </div>
  )
}
