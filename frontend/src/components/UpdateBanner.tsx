import React from 'react'
import { InformationCircleIcon, XMarkIcon } from '@heroicons/react/24/outline'
import type { UpdateStatus } from '../wailsjs/go/main/App'

export type { UpdateStatus } from '../wailsjs/go/main/App'

interface UpdateBannerProps {
  status: UpdateStatus
  onDownload: () => void
  onDismiss: () => void
  className?: string
}

// Portty 06-03: the in-app half of the update flow. Go owns the download, verify and
// reveal; this only renders the Status it reports and never opens a URL.
export function UpdateBanner({ status, onDownload, onDismiss, className }: UpdateBannerProps): React.ReactElement {
  const { state, latest, done, total } = status
  let message: string
  if (state === 'downloading') {
    message = `Downloading Portty ${latest}… ${total > 0 ? Math.floor((done * 100) / total) : 0}%`
  } else if (state === 'verifying') {
    message = `Verifying Portty ${latest}…`
  } else if (state === 'revealed') {
    message = `Portty ${latest} is verified and shown in Finder.`
  } else {
    message = `Portty ${latest} is available.`
  }
  return (
    <div
      className={`banner banner--info${className ? ' ' + className : ''}`}
      role="alert"
      aria-live="polite"
    >
      <InformationCircleIcon className="banner__icon" aria-hidden="true" />
      <span className="banner__message">{message}</span>
      {state === 'downloading' && (
        <progress className="banner__progress" max={total} value={done} aria-label="Download progress" />
      )}
      {state === 'available' && (
        <button type="button" className="banner__action" onClick={onDownload}>
          Download
        </button>
      )}
      <button type="button" className="banner__dismiss" aria-label="Dismiss" onClick={onDismiss}>
        <XMarkIcon aria-hidden="true" />
      </button>
    </div>
  )
}
