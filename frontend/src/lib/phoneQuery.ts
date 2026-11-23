import { useEffect, useState } from 'react'

// Phase 14 (MOB-03, UI-SPEC "What phone means"): a phone is a coarse pointer with no hover. Never the
// user agent or 'ontouchstart' in window. The test setup stubs matchMedia, so there is no fallback branch.
export const PHONE_QUERY = '(pointer: coarse) and (hover: none)'

export function isPhone(): boolean {
  return window.matchMedia(PHONE_QUERY).matches
}

// Shaped like useColorScheme: state, a change listener, cleanup.
export function usePhone(): boolean {
  const [phone, setPhone] = useState(isPhone)
  useEffect(() => {
    const mql = window.matchMedia(PHONE_QUERY)
    const onChange = (e: MediaQueryListEvent) => setPhone(e.matches)
    setPhone(mql.matches) // catch a change between first render and subscribe
    mql.addEventListener('change', onChange)
    return () => mql.removeEventListener('change', onChange)
  }, [])
  return phone
}

// A Home Screen web app: the standard display-mode, or iOS Safari's own navigator.standalone flag.
export function isStandalone(): boolean {
  return window.matchMedia('(display-mode: standalone)').matches ||
    (navigator as Navigator & { standalone?: boolean }).standalone === true
}
