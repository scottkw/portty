import { useEffect, useState } from 'react'

const QUERY = '(prefers-color-scheme: dark)'

// THEM-01: live system appearance. WKWebView and browsers fire 'change' when
// macOS (or the browser) switches Light/Dark. Portty 02-13: a 'light' or 'dark'
// override wins, so terminals never wait on the media query after a switch.
export function useColorScheme(override: 'system' | 'light' | 'dark' = 'system'): 'light' | 'dark' {
  const [dark, setDark] = useState(() => window.matchMedia(QUERY).matches)
  useEffect(() => {
    const mql = window.matchMedia(QUERY)
    const onChange = (e: MediaQueryListEvent) => setDark(e.matches)
    setDark(mql.matches) // catch a change between first render and subscribe
    mql.addEventListener('change', onChange)
    return () => mql.removeEventListener('change', onChange)
  }, [])
  if (override !== 'system') return override
  return dark ? 'dark' : 'light'
}
