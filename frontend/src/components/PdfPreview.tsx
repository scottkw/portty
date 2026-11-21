import React, { useEffect, useRef } from 'react'
import { DocumentIcon } from '@heroicons/react/24/outline'

interface PdfPreviewProps {
  url: string
  name: string
  webMode: boolean
  onFail: () => void
  autoFocus?: boolean
}

/**
 * Phase 11 (PREV-02, PREV-04): a PDF preview. The PDF element and the browser
 * mode come from 11-SPIKE.md's decision block (verdict b):
 *  - Desktop: an <iframe src=blob:> shows WKWebView's own PDF view (S-01, S-02).
 *  - Browser share viewer, Browser PDF mode open-pdf: Safari shows no framed
 *    blob PDF under the share CSP (S-03), so the pane offers Open PDF, which
 *    opens the blob top-level (S-04). Only a PDF is ever opened this way.
 */
export function PdfPreview({ url, name, webMode, onFail, autoFocus }: PdfPreviewProps): React.ReactElement {
  const focusRef = useRef<HTMLIFrameElement & HTMLButtonElement>(null)

  useEffect(() => {
    if (autoFocus) focusRef.current?.focus()
  }, [autoFocus])

  if (!webMode) {
    // An iframe reports no error event. Its load is the signal (11-SPIKE.md S-01):
    // WKWebView loads a readable application/pdf plugin document; any other
    // readable document means the PDF view did not take it.
    const onLoad = (e: React.SyntheticEvent<HTMLIFrameElement>): void => {
      const doc = e.currentTarget.contentDocument
      if (doc && doc.contentType !== 'application/pdf') onFail()
    }
    // index.html's frame-src blob: keeps a PDF link from navigating this frame away (11-REVIEW CR-01).
    // No sandbox: it would null contentDocument (breaking the check above) and is unproven with the PDF view.
    return <iframe ref={focusRef} className="preview-pdf" src={url} title={name} onLoad={onLoad} />
  }

  return (
    <div className="empty-state preview-pdf-open">
      <div className="empty-state__content">
        <DocumentIcon className="empty-state__icon" aria-hidden="true" />
        <h2 className="empty-state__heading">{name}</h2>
        <p className="empty-state__body">This browser can't show the PDF inside the page.</p>
        <div className="empty-state__actions">
          <button
            ref={focusRef}
            type="button"
            className="empty-state__button"
            // noopener makes window.open return null; that is not a failure (11-SPIKE.md S-04).
            onClick={() => window.open(url, '_blank', 'noopener,noreferrer')}
          >
            Open PDF
          </button>
        </div>
      </div>
    </div>
  )
}
