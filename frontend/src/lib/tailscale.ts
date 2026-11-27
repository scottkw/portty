// NET-01: the one copy of the not-connected sentence. The banner, the disabled
// Share controls and Settings all show exactly this text.
export const TAILSCALE_OFF_REASON = "Tailscale isn't connected. Remote access and sharing are off until it is."

// The userspace-networking sentence (19-UI-SPEC): the Settings desktop switch and
// the Share Desktop sheet block with exactly this text.
export const USERSPACE_SENTENCE =
  "Tailscale is running in userspace-networking mode, so Portty can't open a desktop connection. Switch Tailscale to its standard mode, then try again."

// Mirrors Go webserver.TailscaleHealth (LocalAPI connected/not connected only).
export interface TailscaleHealth {
  connected: boolean
  hasCerts: boolean
  ip: string
  domain: string
  acceptDns?: boolean
}
