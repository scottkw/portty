import React, { useEffect, useRef, useState } from 'react'
import { DocumentIcon, DocumentTextIcon, ExclamationTriangleIcon, PhotoIcon } from '@heroicons/react/24/outline'
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

/** UI-SPEC "Size display": decimal units as Finder counts them; one decimal from 1 MB, whole KB below. */
function formatSize(n: number): string {
  return n >= 1_000_000 ? `${(n / 1_000_000).toFixed(1)} MB` : `${Math.round(n / 1000)} KB`
}

/** UI-SPEC: the receiving stack shows only if bytes are still arriving 300 ms after the header. */
const RECEIVING_DELAY_MS = 300

function Receiving({ entry }: { entry: PreviewEntry }): React.ReactElement | null {
  const [shown, setShown] = useState(() => Date.now() - entry.arrivedAt >= RECEIVING_DELAY_MS)
  useEffect(() => {
    if (shown) return
    const t = setTimeout(() => setShown(true), Math.max(0, entry.arrivedAt + RECEIVING_DELAY_MS - Date.now()))
    return () => clearTimeout(t)
  }, [shown, entry.arrivedAt])
  if (!shown) return null
  const Icon = entry.kind === 'pdf' ? DocumentIcon : entry.kind === 'markdown' ? DocumentTextIcon : PhotoIcon
  return (
    <div className="empty-state preview-message">
      <div className="empty-state__content">
        <Icon className="empty-state__icon" aria-hidden="true" />
        <h2 className="empty-state__heading">{entry.name}</h2>
        <progress className="preview-receiving__bar" max={entry.size} value={entry.received} aria-label={`Receiving ${entry.name}`} />
        <p className="preview-receiving__caption">{`Receiving… ${formatSize(entry.received)} of ${formatSize(entry.size)}`}</p>
      </div>
    </div>
  )
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
  if (entry.state === 'tooLarge') {
    content = <MessagePane heading="Too Large to Preview" body={`${name} is larger than 20 MB.`} />
  } else if (entry.state === 'failed') {
    content = <MessagePane heading="Couldn't Receive File" body={`${name} didn't arrive completely. Run portty open again.`} />
  } else if (entry.state === 'receiving') {
    content = <Receiving entry={entry} />
  } else if (url && blob) {
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
