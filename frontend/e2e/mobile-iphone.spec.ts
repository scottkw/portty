// Phase 14 plan 10 (MOB-01): the phone key bar under WebKit's iPhone 15 emulation (coarse pointer,
// no hover). Bytes are read from the fixture's PTY input stub (/__test__/input).

import { test, expect, type Page } from '@playwright/test'
import { appUrl, loadFixtureEnv } from './fixture-env'

async function openShare(page: Page) {
  const env = loadFixtureEnv()
  await page.request.post(`${env.adminURL}/__test__/input/reset`)
  await page.goto(appUrl(env), { waitUntil: 'domcontentloaded' })
  await expect(page.locator('.xterm').first()).toBeVisible({ timeout: 15_000 })
  await expect(page.getByRole('toolbar', { name: 'Terminal keys' })).toBeVisible()
}

async function inputHex(page: Page): Promise<string> {
  const res = await page.request.get(`${loadFixtureEnv().adminURL}/__test__/input`)
  return (await res.json()).hex as string
}

test.describe('Phase 14-10 — key bar on an iPhone (MOB-01)', () => {
  test.beforeEach(async ({ page }) => { await openShare(page) })

  test('tapping Esc reaches the shell as 1b', async ({ page }) => {
    await page.getByRole('button', { name: 'Esc', exact: true }).tap()
    await expect.poll(() => inputHex(page), { timeout: 2_000 }).toContain('1b')
  })
})
