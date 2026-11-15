import { useEffect, useState } from 'react'

const QUERY = '(prefers-color-scheme: dark)'

// THEM-01: live system appearance. WKWebView and browsers fire 'change' when
// macOS (or the browser) switches Light/Dark.
export function useColorScheme(): 'light' | 'dark' {
  const [dark, setDark] = useState(() => window.matchMedia(QUERY).matches)
  useEffect(() => {
    const mql = window.matchMedia(QUERY)
    const onChange = (e: MediaQueryListEvent) => setDark(e.matches)
    setDark(mql.matches) // catch a change between first render and subscribe
    mql.addEventListener('change', onChange)
    return () => mql.removeEventListener('change', onChange)
  }, [])
  return dark ? 'dark' : 'light'
}
