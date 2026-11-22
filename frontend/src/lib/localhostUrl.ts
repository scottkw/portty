/**
 * Phase 13 (URL-01): recognise a printed loopback http URL a remote session shows (a dev server banner).
 * Mirrors Go `classifyLoopbackURL` (internal/daemon/portforward_api.go); both are pinned by the shared
 * fixture lib/__tests__/fixtures/localhost-url.json. No `new URL`: it folds case and drops `:80`.
 */

const HOSTS = ['localhost', '127.0.0.1', '[::1]', '0.0.0.0', '[::]']
const TRAILING = ']}>,;.\'"'
// Go's unicode.IsSpace is White_Space and unicode.IsControl is Cc: the same set the Go classifier refuses.
const BAD_REST = /[\p{White_Space}\p{Cc}]/u

/** Drops trailing punctuation a terminal prints around a URL; a `)` goes only when unbalanced. */
export function trimLinkTail(raw: string): string {
  let end = raw.length
  while (end > 0) {
    const last = raw[end - 1]
    const s = raw.slice(0, end)
    const unbalanced = last === ')' && s.split('(').length < s.split(')').length
    if (!TRAILING.includes(last) && !unbalanced) break
    end--
  }
  return raw.slice(0, end)
}

/** `{ port, rest }` for http://{localhost,127.0.0.1,[::1],0.0.0.0,[::]}:PORT plus rest, else null. */
export function classifyLoopbackUrl(raw: string): { port: number; rest: string } | null {
  const trimmed = trimLinkTail(raw)
  if (!trimmed.startsWith('http://')) return null
  let s = trimmed.slice('http://'.length)
  const host = HOSTS.find((h) => s.startsWith(h + ':'))
  if (!host) return null
  s = s.slice(host.length + 1)
  let n = 0
  while (n < s.length && s[n] >= '0' && s[n] <= '9') n++
  if (n === 0 || n > 5 || s[0] === '0') return null
  const port = Number(s.slice(0, n))
  if (port < 1 || port > 65535) return null
  const rest = s.slice(n)
  if (rest !== '' && !'/?#'.includes(rest[0])) return null
  if (BAD_REST.test(rest)) return null
  return { port, rest }
}
