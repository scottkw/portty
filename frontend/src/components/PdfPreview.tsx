import React, { useEffect, useRef, useState } from 'react'
import { DocumentIcon } from '@heroicons/react/24/outline'
import { isStandalone } from '../lib/phoneQuery'

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
 *  - Phase 14 (MOB-03, T-14-13): in a Home Screen web app there are no browser controls, so a top-level
 *    blob could strand the user. There Open PDF shows an in-app layer instead: the file name, "Open full PDF"
 *    (the old window.open, for the whole document) and "Done" (also Esc) back to the pane. The PDF is drawn
 *    as an <img> (img-src already allows blob:, no CSP change); iOS may draw only its first page (14-13 S-06),
 *    which is why Open full PDF stays. Done is the way back whatever the image shows.
 */
export function PdfPreview({ url, name, webMode, onFail, autoFocus }: PdfPreviewProps): React.ReactElement {
  const focusRef = useRef<HTMLIFrameElement & HTMLButtonElement>(null)
  const doneRef = useRef<HTMLButtonElement>(null)
  const [inApp, setInApp] = useState(false)

  useEffect(() => {
    if (autoFocus) focusRef.current?.focus()
  }, [autoFocus])

  useEffect(() => {
    if (!inApp) return
    doneRef.current?.focus()
    const onKey = (e: KeyboardEvent): void => {
      if (e.key === 'Escape') close()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [inApp])

  const close = (): void => {
    setInApp(false)
    focusRef.current?.focus()
  }
  // noopener makes window.open return null; that is not a failure (11-SPIKE.md S-04).
  const openFull = (): void => { window.open(url, '_blank', 'noopener,noreferrer') }

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
            onClick={() => (isStandalone() ? setInApp(true) : openFull())}
          >
            Open PDF
          </button>
        </div>
      </div>
      {inApp && (
        <div className="pdf-layer" role="dialog" aria-modal="true" aria-label={name}>
          <div className="pdf-layer__bar">
            <span className="pdf-layer__name">{name}</span>
            <button type="button" className="pdf-layer__button" onClick={openFull}>Open full PDF</button>
            <button ref={doneRef} type="button" className="pdf-layer__button" onClick={close}>Done</button>
          </div>
          <div className="pdf-layer__body">
            <img className="pdf-layer__img" src={url} alt={name} />
          </div>
        </div>
      )}
    </div>
  )
}
