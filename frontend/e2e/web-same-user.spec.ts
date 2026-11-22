// Phase 14 Plan 03 — web-same-user.spec.ts
//
// MOB-05: a browser on another Tailscale account that opens the share view gets
// the static refusal page and no session data (no WebSocket, no /api/sessions
// request). The fixture's WhoIs fake (POST /__test__/whois) picks the identity;
// every test puts it back to `same`.

import { test, expect } from '@playwright/test'
import { appUrl, loadFixtureEnv } from './fixture-env'

async function setIdentity(as: string): Promise<void> {
  const res = await fetch(`${loadFixtureEnv().adminURL}/__test__/whois`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ as }),
  })
  expect(res.status).toBe(200)
}

test.describe('Phase 14-03 — same-account gate on the page routes (MOB-05)', () => {
  test.afterEach(async () => {
    await setIdentity('same')
  })

  test('another account gets the refusal page and no session data', async ({ page }) => {
    await setIdentity('other')
    const sockets: string[] = []
    const requests: string[] = []
    page.on('websocket', (ws) => sockets.push(ws.url()))
    page.on('request', (req) => requests.push(req.url()))

    const resp = await page.goto(appUrl(), { waitUntil: 'domcontentloaded' })
    expect(resp?.status()).toBe(403)
    expect(resp?.headers()['x-portty']).toBe('1')
    await expect(page.getByRole('heading', { name: 'This link only works on your own devices' })).toBeVisible()
    await expect(page.getByText('Open it on a device signed in to the same Tailscale account as the Mac that shared it.')).toBeVisible()
    await expect(page.locator('.xterm')).toHaveCount(0)
    expect(sockets).toEqual([])
    expect(requests.filter((u) => u.includes('/api/sessions/'))).toEqual([])
  })

  test('the same account opens the share view', async ({ page }) => {
    await page.goto(appUrl(), { waitUntil: 'domcontentloaded' })
    await expect(page.locator('.xterm').first()).toBeVisible({ timeout: 15_000 })
  })
})
