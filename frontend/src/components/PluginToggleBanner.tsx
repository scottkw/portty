import React, { useEffect } from 'react'
import { InformationCircleIcon, XMarkIcon } from '@heroicons/react/24/outline'

/**
 * PluginToggleBanner — Phase 99 PUI-02.
 *
 * One-shot informational banner rendered inside the existing `.banner-stack`
 * (Phase 81 BAN-01/BAN-02 vocabulary) when the user toggles a plugin that
 * cannot hot-swap (Unicode 11 or Inline Images) and saves. Both variants
 * auto-dismiss after 6000ms (unconditional — differs from WebGLRecoveryBanner
 * where 'software-rasterized' is persistent).
 *
 * Verbatim copy locked by 99-RESEARCH.md "Claude's Discretion" — DO NOT
 * paraphrase. Portty 02-10: the shared `.banner` style, info variant.
 */
export interface PluginToggleBannerProps {
  kind: 'unicode11' | 'image'
  onDismiss: () => void
  className?: string
}

export function PluginToggleBanner({
  kind,
  onDismiss,
  className,
}: PluginToggleBannerProps): React.ReactElement {
  useEffect(() => {
    const id = window.setTimeout(onDismiss, 6000)
    return () => window.clearTimeout(id)
  }, [onDismiss])

  const message =
    kind === 'unicode11'
      ? 'Open a new terminal session to apply the Unicode 11 change.'
      : 'Open a new terminal session to apply the Inline Images change.'

  const cls = ['banner', 'banner--info', className].filter(Boolean).join(' ')

  return (
    <div className={cls} role="status" aria-live="polite">
      <InformationCircleIcon className="banner__icon" aria-hidden="true" />
      <span className="banner__message">{message}</span>
      <button
        type="button"
        className="banner__dismiss"
        aria-label="Dismiss"
        onClick={onDismiss}
      >
        <XMarkIcon aria-hidden="true" />
      </button>
    </div>
  )
}
