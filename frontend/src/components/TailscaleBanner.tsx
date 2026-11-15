import React from 'react'
import { ExclamationTriangleIcon } from '@heroicons/react/24/outline'
import { TAILSCALE_OFF_REASON } from '../lib/tailscale'

/**
 * Persistent banner shown while Tailscale is not connected (NET-01). It has no
 * dismiss button: it clears only when a recheck finds the connection back.
 * Portty 02-08: the toolbar Share button points aria-describedby at
 * #ts-off-reason, so the reason is visible and announced (UI-REVIEW 01 #2/#3).
 */
export function TailscaleBanner(): React.ReactElement {
  return (
    <div className="tailscale-banner" role="status">
      <ExclamationTriangleIcon className="tailscale-banner__icon" aria-hidden="true" />
      <span className="tailscale-banner__message" id="ts-off-reason">{TAILSCALE_OFF_REASON}</span>
    </div>
  )
}
