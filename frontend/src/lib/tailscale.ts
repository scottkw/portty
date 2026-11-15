// NET-01: the one copy of the not-connected sentence. The banner, the disabled
// Share controls and Settings all show exactly this text.
export const TAILSCALE_OFF_REASON = "Tailscale isn't connected. Remote access and sharing are off until it is."

// Mirrors Go webserver.TailscaleHealth (LocalAPI connected/not connected only).
export interface TailscaleHealth {
  connected: boolean
  hasCerts: boolean
  ip: string
  domain: string
  acceptDns?: boolean
}
