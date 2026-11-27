import React from 'react'
import { InformationCircleIcon, ShieldExclamationIcon, XMarkIcon } from '@heroicons/react/24/outline'
import * as copy from '../lib/shareDesktopCopy'

type ShareBannerProps =
  /** A live condition (Secure Event Input on while someone can control): no action, no dismiss. */
  | { kind: 'secure-input' }
  /** The share ended under the sharer (daemon restart): the action opens the sheet, the dismiss hides it. */
  | { kind: 'stopped'; onShare: () => void; onDismiss: () => void }

/** The two click-to-share banners (22-UI-SPEC Surface 5), in TailscaleBanner's pattern. The caller decides when each shows. */
export function ShareBanner(props: ShareBannerProps): React.ReactElement {
  if (props.kind === 'secure-input') {
    return (
      <div className="banner banner--warning" role="status">
        <ShieldExclamationIcon className="banner__icon" aria-hidden="true" />
        <span className="banner__message">{copy.BANNER_SECURE_INPUT}</span>
      </div>
    )
  }
  return (
    <div className="banner banner--info" role="status">
      <InformationCircleIcon className="banner__icon" aria-hidden="true" />
      <span className="banner__message">{copy.BANNER_SHARE_STOPPED}</span>
      <button type="button" className="banner__action" onClick={props.onShare}>
        {copy.SHARE_DESKTOP_ACTION}
      </button>
      <button type="button" className="banner__dismiss" onClick={props.onDismiss} aria-label="Dismiss">
        <XMarkIcon aria-hidden="true" />
      </button>
    </div>
  )
}
