import * as App from '../wailsjs/go/main/App'
import type { ReconnectVerdict } from './relayClient'

interface HostReconnectDeps {
  GetRelayPort: () => Promise<number>
  ListSessions: () => Promise<{ id: string; state: string }[]>
}

/**
 * 09-06 (UPGR-02): a host tab's re-auth before each redial. After a handoff the
 * old daemon closes viewers with 1012 and the session lives on under the new
 * one, on the same relay port. After a crash the respawned daemon (on a new
 * port) lists no such session, so the tab ends as before. Any read failure is
 * a retry: the daemon may still be coming up. The bindings are read only here,
 * at call time, so component tests that mock App without them still load.
 */
export async function hostReconnectVerdict(sessionId: string, deps: HostReconnectDeps = App): Promise<ReconnectVerdict> {
  let port: number
  let sessions: { id: string; state: string }[]
  try {
    port = await deps.GetRelayPort()
    sessions = await deps.ListSessions()
  } catch {
    return { kind: 'retry' }
  }
  const s = sessions.find((x) => x.id === sessionId)
  if (!s || s.state === 'stopped') return { kind: 'stop', reason: 'ended' }
  return { kind: 'ok', port }
}
