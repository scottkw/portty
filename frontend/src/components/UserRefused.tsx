import React from 'react'

/**
 * Portty 14-04 (MOB-05): shown on the web share page (/app/) when the share
 * server refuses this device because it is signed in to another Tailscale
 * account. Takes no props and renders nothing from the URL. Same copy as the
 * server's web/user-refused.html.
 */
export function UserRefused(): React.ReactElement {
  return (
    <div className="link-dead">
      <h1 className="link-dead__heading">This link only works on your own devices</h1>
      <p className="link-dead__body">Open it on a device signed in to the same Tailscale account as the Mac that shared it.</p>
    </div>
  )
}
