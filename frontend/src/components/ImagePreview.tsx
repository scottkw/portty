import React, { useCallback, useEffect, useRef, useState } from 'react'

// 16px pane padding on each side (UI-SPEC "Images"; mirrors .preview-image).
const PANE_PADDING = 32

interface ImagePreviewProps {
  url: string
  name: string
  kind: 'image' | 'svg'
  onFail: () => void
  autoFocus?: boolean
}

/**
 * Phase 11 (PREV-02): an image or SVG preview. The file is only ever an
 * <img src=blob:>, so an SVG runs no script and fetches nothing (CONTEXT
 * "Image rendering"). Raster images fit without upscaling and toggle to Actual
 * Size when larger than the fit; SVG always fits (UI-SPEC "Images", "SVG").
 */
export function ImagePreview({ url, name, kind, onFail, autoFocus }: ImagePreviewProps): React.ReactElement {
  const paneRef = useRef<HTMLDivElement>(null)
  // The img remounts when the toggle wraps it, so keep the size from its first load.
  const naturalRef = useRef<[number, number] | null>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)
  const focusedRef = useRef(false)
  const [canToggle, setCanToggle] = useState(false)
  const [actual, setActual] = useState(false)

  const measure = useCallback(() => {
    const pane = paneRef.current
    const natural = naturalRef.current
    if (kind === 'svg' || !pane || !natural) return
    const larger = natural[0] > pane.clientWidth - PANE_PADDING || natural[1] > pane.clientHeight - PANE_PADDING
    setCanToggle(larger)
    if (!larger) setActual(false)
  }, [kind])

  useEffect(() => {
    const pane = paneRef.current
    if (!pane || kind === 'svg') return
    const ro = new ResizeObserver(() => measure())
    ro.observe(pane)
    return () => ro.disconnect()
  }, [kind, measure])

  useEffect(() => {
    if (autoFocus && canToggle && !focusedRef.current) {
      focusedRef.current = true
      buttonRef.current?.focus()
    }
  }, [autoFocus, canToggle])

  const onLoad = (e: React.SyntheticEvent<HTMLImageElement>): void => {
    const { naturalWidth, naturalHeight } = e.currentTarget
    if (naturalWidth) naturalRef.current = [naturalWidth, naturalHeight]
    measure()
  }

  const toggle = (): void => {
    setActual((a) => !a)
    // Actual Size opens at the top-left (UI-SPEC "Images").
    paneRef.current?.scrollTo?.(0, 0)
  }

  const img = (
    <img
      className={kind === 'svg' ? 'preview-image__img preview-image__img--svg' : 'preview-image__img'}
      src={url}
      alt={name}
      onLoad={onLoad}
      onError={onFail}
    />
  )

  return (
    <div ref={paneRef} className={actual ? 'preview-image preview-image--actual' : 'preview-image'}>
      {canToggle ? (
        <button
          ref={buttonRef}
          type="button"
          className="preview-image__toggle"
          aria-label="Actual Size"
          aria-pressed={actual}
          onClick={toggle}
        >
          {img}
        </button>
      ) : (
        img
      )}
    </div>
  )
}
