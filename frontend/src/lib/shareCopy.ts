// Portty 04-05: the fixed share copy (04-UI-SPEC Copywriting Contract). Error
// copy is picked by ShareResult.errorCode, never parsed from text; the only
// interpolated detail comes from the local binding, never from a remote peer.

export const SHARE_SWITCH_LABEL = 'Share this session'
export const SHARE_DESCRIPTION =
  'Macs signed in to your Tailscale account see it in their sidebar, and you get a link for any browser on your tailnet.'
export const SHARE_STARTING = 'Getting a link…'
export const SHARE_WARNING = 'Anyone on your tailnet with this link has full control.'
export const COPY = 'Copy'
export const COPIED = 'Copied!'
export const COPY_FAILED = "Couldn't copy the link. Select it and press ⌘C."

export function shareCtLine(domain: string): string {
  return `The first share gets an HTTPS certificate for ${domain}. That name is published in public Certificate Transparency logs.`
}

// A thrown Error stringifies as "Error: …" and the detail often ends with its own
// period; strip both so the fixed sentence reads "…: <detail>. Check…".
function clean(detail: string): string {
  return detail.trim().replace(/^Error:\s*/, '').replace(/\.+$/, '')
}

export function shareStartError(code: string, detail: string): string {
  if (code === 'https_off') {
    return "Couldn't start sharing: HTTPS certificates are off for your tailnet. Turn on HTTPS on the DNS page of the Tailscale admin console, then try again."
  }
  return `Couldn't start sharing: ${clean(detail)}. Check that Tailscale is connected, then try again.`
}

/** Shared, but the link couldn't be read back (popover reopened on a shared session). */
export function shareLinkLoadError(detail: string): string {
  return `Couldn't get the link: ${clean(detail)}. Check that Tailscale is connected, then try again.`
}

export function shareStopError(detail: string): string {
  return `Couldn't stop sharing: ${clean(detail)}. Try again.`
}

export function menuShareError(name: string, detail: string): string {
  return `Couldn't share “${name}”: ${clean(detail)}. Check that Tailscale is connected, then try again.`
}

export function menuStopError(name: string, detail: string): string {
  return `Couldn't stop sharing “${name}”: ${clean(detail)}. Try again.`
}
