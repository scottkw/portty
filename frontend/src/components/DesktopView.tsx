import React, { useEffect, useRef, useState } from 'react'
import { ComputerDesktopIcon } from '@heroicons/react/24/outline'
import { createDesktopClient } from '../lib/desktopClient'

const TITLE = 'Portty Desktop'

/**
 * Phase 19-06 — the /desktop/ viewer root. Owns one DesktopClient and one
 * <video> for the life of the page; no Wails imports, no input.
 */
export function DesktopView(): React.ReactElement {
  const videoRef = useRef<HTMLVideoElement>(null)
  const [hasFrame, setHasFrame] = useState(false)

  useEffect(() => {
    document.title = TITLE
    let meta = document.querySelector<HTMLMetaElement>('meta[name="apple-mobile-web-app-title"]')
    if (!meta) {
      meta = document.createElement('meta')
      meta.name = 'apple-mobile-web-app-title'
      document.head.appendChild(meta)
    }
    meta.content = TITLE
  }, [])

  useEffect(() => {
    const client = createDesktopClient()
    const off = client.on('track', (stream) => {
      const v = videoRef.current
      if (!v) return
      v.srcObject = stream
      void Promise.resolve(v.play()).catch(() => {})
    })
    client.start()
    return () => {
      off()
      client.stop()
    }
  }, [])

  return (
    <div className="desktop-view">
      <video
        ref={videoRef}
        className="desktop-view__video"
        aria-label="Mac desktop"
        muted
        playsInline
        autoPlay
        onPlaying={() => setHasFrame(true)}
        onLoadedData={() => setHasFrame(true)}
      />
      <div role="status" aria-live="polite" className="desktop-status">
        {!hasFrame && (
          <div className="desktop-card">
            <ComputerDesktopIcon className="desktop-card__icon" aria-hidden="true" />
            <h1 className="desktop-card__heading">Connecting…</h1>
          </div>
        )}
      </div>
    </div>
  )
}
