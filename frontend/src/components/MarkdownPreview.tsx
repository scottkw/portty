import React, { useEffect, useRef, useState } from 'react'

interface MarkdownPreviewProps {
  blob: Blob
  name: string
  /** Part of the renderer contract 11-09 composes; openLink already picks desktop or browser. */
  webMode: boolean
  onFail: () => void
  autoFocus?: boolean
}

/**
 * Phase 11 (PREV-02): a Markdown preview. The text comes from blob.text(), never
 * from fetching the blob URL (the share page's connect-src has no blob:). The
 * renderer is imported lazily so marked and DOMPurify stay out of the entry chunk,
 * and its sanitized string is assigned once.
 */
export function MarkdownPreview({ blob, name, onFail, autoFocus }: MarkdownPreviewProps): React.ReactElement {
  const [html, setHtml] = useState<string | null>(null)
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

  useEffect(() => {
    if (html !== null && autoFocus) articleRef.current?.focus()
  }, [html, autoFocus])

  return (
    <div className="preview-markdown-pane">
      <article
        ref={articleRef}
        className="preview-markdown"
        aria-label={name}
        tabIndex={0}
        dangerouslySetInnerHTML={{ __html: html ?? '' }}
      />
    </div>
  )
}
