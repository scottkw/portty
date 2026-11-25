import React from 'react'
import { ExclamationTriangleIcon, InformationCircleIcon, XMarkIcon } from '@heroicons/react/24/outline'
import type { UpdateStatus as WailsUpdateStatus } from '../wailsjs/go/main/App'

// Declared here, not in the bindings stub: the stub is marked generated, and Wails generates a plain
// string for Go's state field, so the literal union belongs to the frontend.
export type UpdateStatus = Omit<WailsUpdateStatus, 'state'> & {
  state: WailsUpdateStatus['state'] | 'installing' | 'relaunching'
}

interface UpdateBannerProps {
  status: UpdateStatus
  onDownload: () => void
  onDismiss: () => void
  className?: string
}

// Portty 06-03: the in-app half of the update flow. Go owns the download, verify, install,
// relaunch and (when an install fails) reveal; this only renders the Status it reports and
// never opens a URL or a file.
export function UpdateBanner({ status, onDownload, onDismiss, className }: UpdateBannerProps): React.ReactElement {
  const { state, latest, done, total, error } = status
  const warn = error !== '' && (state === 'available' || state === 'revealed')
  const Icon = warn ? ExclamationTriangleIcon : InformationCircleIcon
  let message: string
  if (warn) {
    message = error
  } else if (state === 'downloading') {
    message = `Downloading Portty ${latest}… ${total > 0 ? Math.floor((done * 100) / total) : 0}%`
  } else if (state === 'verifying') {
    message = `Verifying Portty ${latest}…`
  } else if (state === 'installing') {
    message = `Installing Portty ${latest}…`
  } else if (state === 'relaunching') {
    message = `Restarting Portty ${latest}…`
  } else if (state === 'revealed') {
    message = `Portty ${latest} is verified and shown in Finder.`
  } else {
    message = `Portty ${latest} is available.`
  }
  return (
    <div
      className={`banner ${warn ? 'banner--warning' : 'banner--info'}${className ? ' ' + className : ''}`}
      role="alert"
      aria-live="polite"
    >
      <Icon className="banner__icon" aria-hidden="true" />
      <span className="banner__message">{message}</span>
      {state === 'downloading' && (
        <progress className="banner__progress" max={total} value={done} aria-label="Download progress" />
      )}
      {state === 'available' && (
        <button type="button" className="banner__action" onClick={onDownload}>
          {warn ? 'Try Again' : 'Update'}
        </button>
      )}
      <button type="button" className="banner__dismiss" aria-label="Dismiss" onClick={onDismiss}>
        <XMarkIcon aria-hidden="true" />
      </button>
    </div>
  )
}
