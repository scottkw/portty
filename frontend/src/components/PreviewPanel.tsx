import React, { useEffect, useRef, useState } from 'react'
import { ExclamationTriangleIcon } from '@heroicons/react/24/outline'
import type { PreviewEntry } from '../lib/previewStore'
import { ErrorBoundary } from './ErrorBoundary'
import { ImagePreview } from './ImagePreview'
import { PdfPreview } from './PdfPreview'
import { MarkdownPreview } from './MarkdownPreview'

interface PreviewPanelProps {
  entry: PreviewEntry
  webMode: boolean
  /** A focused arrival: move keyboard focus into the pane (UI-SPEC "Accessibility Contract"). */
  autoFocus?: boolean
}

function MessagePane({ heading, body }: { heading: string; body: string }): React.ReactElement {
  return (
    <div className="empty-state preview-message" role="status">
      <div className="empty-state__content">
        <ExclamationTriangleIcon className="empty-state__icon" aria-hidden="true" />
        <h2 className="empty-state__heading">{heading}</h2>
        <p className="empty-state__body">{body}</p>
      </div>
    </div>
  )
}

/**
 * Phase 11 (UI-SPEC "Preview Pane", "States"): one preview tab's pane. App mounts it only
 * while its tab is visible, so a slow renderer never runs for a background tab.
 */
export function PreviewPanel({ entry, webMode, autoFocus = false }: PreviewPanelProps): React.ReactElement {
  const rootRef = useRef<HTMLDivElement>(null)
  // The url whose renderer reported it can't show the file; a refresh brings a new url.
  const [failedUrl, setFailedUrl] = useState<string | null>(null)

  // Children's effects run first: a renderer that focused its own target keeps it.
  useEffect(() => {
    const root = rootRef.current
    if (autoFocus && root && !root.contains(document.activeElement)) root.focus()
  }, [autoFocus])

  const { name, url, blob } = entry
  const cantShow = <MessagePane heading="Couldn't Show File" body={`${name} may be damaged, or its format isn't supported.`} />

  let content: React.ReactNode = null
  if (url && blob) {
    const onFail = () => setFailedUrl(url)
    content = failedUrl === url ? cantShow : (
      <ErrorBoundary label="preview" resetKey={url} fallback={cantShow}>
        {entry.kind === 'pdf' ? (
          <PdfPreview key={url} url={url} name={name} webMode={webMode} onFail={onFail} autoFocus={autoFocus} />
        ) : entry.kind === 'markdown' ? (
          <MarkdownPreview key={url} blob={blob} name={name} webMode={webMode} onFail={onFail} autoFocus={autoFocus} />
        ) : (
          <ImagePreview key={url} url={url} name={name} kind={entry.kind} onFail={onFail} autoFocus={autoFocus} />
        )}
      </ErrorBoundary>
    )
  }

  return (
    <div
      ref={rootRef}
      className="preview-panel"
      id={`tabpanel-${entry.tabId}`}
      role="tabpanel"
      aria-labelledby={`tab-${entry.tabId}`}
      tabIndex={-1}
    >
      {content}
    </div>
  )
}
