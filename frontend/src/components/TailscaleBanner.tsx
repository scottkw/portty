import React from 'react'
import { TAILSCALE_OFF_REASON } from '../lib/tailscale'

/**
 * Persistent banner shown while Tailscale is not connected (NET-01). It has no
 * dismiss button: it clears only when a recheck finds the connection back.
 */
export function TailscaleBanner(): React.ReactElement {
  return (
    <div className="tailscale-banner" role="status">
      <span className="tailscale-banner__icon" aria-hidden="true">{'⚠'}</span>
      <span className="tailscale-banner__message">{TAILSCALE_OFF_REASON}</span>
    </div>
  )
}
