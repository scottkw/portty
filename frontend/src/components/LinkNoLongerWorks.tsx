import React from 'react'

/**
 * Portty 04-06 (SHAR-03): shown on the web share page (/app/) when the link is
 * incomplete or its capability is refused. Takes no props and renders nothing
 * from the URL, so it never reveals the host, session name or ID. Same copy as
 * the server's web/link-invalid.html.
 */
export function LinkNoLongerWorks(): React.ReactElement {
  return (
    <div className="link-dead">
      <h1 className="link-dead__heading">Link No Longer Works</h1>
      <p className="link-dead__body">This link was turned off or is incomplete. Copy a fresh link from the Share button on the Mac that runs the session.</p>
    </div>
  )
}
