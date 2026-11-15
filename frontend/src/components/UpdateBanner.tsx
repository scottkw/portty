import React from 'react'
import { InformationCircleIcon, XMarkIcon } from '@heroicons/react/24/outline'
import { BrowserOpenURL } from '../wailsjs/wailsjs/runtime/runtime'

export interface UpdateInfo {
  currentVersion: string
  latestVersion: string
  releaseURL: string
}

interface UpdateBannerProps {
  update: UpdateInfo
  onDismiss: () => void
  className?: string
}

export function UpdateBanner({ update, onDismiss, className }: UpdateBannerProps): React.ReactElement {
  return (
    <div
      className={`banner banner--info${className ? ' ' + className : ''}`}
      role="alert"
      aria-live="polite"
    >
      <InformationCircleIcon className="banner__icon" aria-hidden="true" />
      <span className="banner__message">
        Update available: {update.currentVersion} &rarr; {update.latestVersion}
      </span>
      <button
        type="button"
        className="banner__action"
        onClick={() => BrowserOpenURL(update.releaseURL)}
      >
        Download Update
      </button>
      <button type="button" className="banner__dismiss" aria-label="Dismiss" onClick={onDismiss}>
        <XMarkIcon aria-hidden="true" />
      </button>
    </div>
  )
}
