import React, { useEffect, useRef } from 'react'
import { ConnectPeerDesktop } from '../wailsjs/go/main/App'
import { createPeerDesktopOptions } from '../lib/peerDesktop'
import { DesktopView } from './DesktopView'
import type { Tab } from './TabBar'

/**
 * Phase 21-08: the Portty app's Desktop tab. It mounts the shared DesktopView (the one the /desktop/
 * browser page uses) over the loopback connection 21-05 builds from the ConnectPeerDesktop binding.
 * The tab only ever knows the host, the loopback URL and error codes: the desktop token stays in the daemon.
 */
export function DesktopTab({ tab }: { tab: Tab }): React.ReactElement {
  // Built once per mount: the options own the first connect, so a re-render must not make another.
  const built = useRef<ReturnType<typeof createPeerDesktopOptions> | null>(null)
  if (built.current === null) {
    built.current = createPeerDesktopOptions({
      host: tab.connectHost ?? tab.peerHost ?? '',
      connect: ConnectPeerDesktop,
      onLocalNetwork: () => {},
    })
  }
  const peer = built.current

  // StrictMode (dev) runs this effect's cleanup and then the effect again on the same instance; dispose
  // is final, so it waits a tick and the second run cancels it.
  const disposeTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  useEffect(() => {
    if (disposeTimer.current !== null) clearTimeout(disposeTimer.current)
    disposeTimer.current = null
    return () => {
      disposeTimer.current = setTimeout(() => peer.dispose(), 0)
    }
  }, [peer])

  return <DesktopView embedded clientOptions={peer.options} />
}
