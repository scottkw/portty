// Phase 93 Plan 05 — fixture-env loader for Playwright specs.
// Reads .playwright/fixture-env.json (written by global-setup.ts) and
// returns the BASE_URL / CAP / ADMIN_URL the test fixture exposed.

import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

export interface FixtureEnv {
  baseURL: string
  cap: string
  adminURL: string
  pid: number
}

let cached: FixtureEnv | null = null

export function loadFixtureEnv(): FixtureEnv {
  if (cached) return cached
  const path = resolve(__dirname, '..', '.playwright', 'fixture-env.json')
  const data = readFileSync(path, 'utf8')
  cached = JSON.parse(data) as FixtureEnv
  return cached
}

export function sessionURL(env: FixtureEnv = loadFixtureEnv()): string {
  return `${env.baseURL}/sessions/playwright-test-session?cap=${encodeURIComponent(env.cap)}`
}

/**
 * appUrl builds the /app/ entry URL with session + cap query params. Phase
 * 120-06 wired App.tsx to consult lib/webMode.detectMode() and, when the
 * pathname starts with `/app/`, skip the Wails RPC suite and source
 * session/cap from URL params + window.location.origin. The fixture binary
 * now embeds frontend/dist under -tags=playwrightfixture,wailsassets (see
 * cmd/playwright-fixture/assets_prod.go), so loading this URL in a browser
 * mounts a fully-functional file-browser tab — scenarios 13 + 14 in the
 * files-browser spec exercise that DOM path on all three browsers.
 */
export function appUrl(env: FixtureEnv = loadFixtureEnv()): string {
  return `${env.baseURL}/app/?session=playwright-test-session&cap=${encodeURIComponent(env.cap)}`
}
