import React from 'react'
import { ExclamationTriangleIcon } from '@heroicons/react/24/outline'
import { TAILSCALE_OFF_REASON } from '../lib/tailscale'
import { OpenTailscale } from '../wailsjs/go/main/App'

/**
 * Persistent banner shown while Tailscale is not connected (NET-01). It has no
 * dismiss button: it clears only when a recheck finds the connection back.
 * Portty 02-08: the toolbar Share button points aria-describedby at
 * #ts-off-reason, so the reason is visible and announced (UI-REVIEW 01 #2/#3).
 * Portty 02-10: the action button launches the installed app, or its download
 * page; it never installs or configures anything.
 */
export function TailscaleBanner(): React.ReactElement {
  return (
    <div className="banner banner--warning" role="status">
      <ExclamationTriangleIcon className="banner__icon" aria-hidden="true" />
      <span className="banner__message" id="ts-off-reason">{TAILSCALE_OFF_REASON}</span>
      <button
        type="button"
        className="banner__action"
        onClick={() => {
          OpenTailscale().catch((err) => console.error('OpenTailscale failed:', err))
        }}
      >
        Open Tailscale
      </button>
    </div>
  )
}
