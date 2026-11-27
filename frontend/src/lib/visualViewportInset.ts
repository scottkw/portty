import { useEffect, useState } from 'react'

// Phase 14-11 (MOB-01, T-14-27): on a phone the on-screen keyboard shrinks the VISUAL viewport but
// not the layout viewport, so a 100%-tall root slides under it. This sizes the app root to the
// visual viewport (CSS variables read by `.app`), so the key bar rides directly above the keyboard
// and the cursor line stays above the bar. Nothing resizes xterm here: its ResizeObserver reports,
// and the next keystroke claims (INPT-06).
//
// Both the viewport math and the stuck-viewport heal are [ASSUMED] to hold on iOS 26 standalone
// until the simulator (14-13) and a real phone show it.

// The keyboard is at least this much of the screen; anything smaller is browser chrome moving.
const KEYBOARD_MIN_PX = 100
// iOS reports the final visualViewport a beat after the helper textarea blurs.
const HEAL_DELAY_MS = 140

export interface ViewportInsetOptions {
  /** The element sized to the visual viewport (default the terminal app's `.app`). */
  rootSelector?: string
  /** The field whose blur heals a stuck viewport (default the terminal's `.xterm-helper-textarea`). */
  fieldSelector?: string
}

// Phase 21-10: the /desktop/ phone page passes its own root and hidden field; every default is the terminal's.
export function useVisualViewportInset(
  enabled: boolean,
  { rootSelector = '.app', fieldSelector = '.xterm-helper-textarea' }: ViewportInsetOptions = {},
): { keyboardUp: boolean } {
  const [keyboardUp, setKeyboardUp] = useState(false)

  useEffect(() => {
    const vv = window.visualViewport
    if (!enabled || !vv) return
    const root = document.querySelector<HTMLElement>(rootSelector)
    if (!root) return

    let frame = 0
    let healFrame = 0
    let healTimer: ReturnType<typeof setTimeout> | null = null

    const update = (): void => {
      frame = 0
      root.style.setProperty('--vv-height', `${vv.height}px`)
      root.style.setProperty('--vv-top', `${vv.offsetTop}px`)
      setKeyboardUp(window.innerHeight - vv.height > KEYBOARD_MIN_PX)
    }
    const schedule = (): void => { if (!frame) frame = requestAnimationFrame(update) }

    // A standalone Home Screen app can stay at the keyboard's height after it closes. A display
    // toggle makes WebKit lay the page out again.
    const heal = (): void => {
      healTimer = null
      if (document.activeElement?.matches(fieldSelector)) return
      // The layout viewport does not shrink for the keyboard, so the live innerHeight is the baseline
      // (a remembered maximum would read a landscape rotation as a stuck keyboard).
      if (window.innerHeight - vv.height <= KEYBOARD_MIN_PX) return
      root.style.display = 'none'
      void root.offsetHeight // commit the hidden state before restoring it
      healFrame = requestAnimationFrame(() => { healFrame = 0; root.style.display = ''; update() })
    }
    const onFocusOut = (e: FocusEvent): void => {
      if (!(e.target as Element | null)?.matches?.(fieldSelector)) return
      if (healTimer !== null) clearTimeout(healTimer)
      healTimer = setTimeout(heal, HEAL_DELAY_MS)
    }

    vv.addEventListener('resize', schedule)
    vv.addEventListener('scroll', schedule)
    document.addEventListener('focusout', onFocusOut)
    schedule()

    return () => {
      vv.removeEventListener('resize', schedule)
      vv.removeEventListener('scroll', schedule)
      document.removeEventListener('focusout', onFocusOut)
      if (frame) cancelAnimationFrame(frame)
      if (healFrame) { cancelAnimationFrame(healFrame); root.style.display = '' } // never leave the app hidden
      if (healTimer !== null) clearTimeout(healTimer)
      root.style.removeProperty('--vv-height')
      root.style.removeProperty('--vv-top')
      setKeyboardUp(false)
    }
  }, [enabled, rootSelector, fieldSelector])

  return { keyboardUp }
}
