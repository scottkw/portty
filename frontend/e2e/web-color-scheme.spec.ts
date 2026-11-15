// Portty Phase 2 plan 02-03 (THEM-01): the web share page follows the browser's
// light/dark setting live, with the same built-in palettes as the desktop app.
// xterm 6 writes theme.background inline on .xterm-scrollable-element, so the
// computed background tells us which palette is in use.

import { test, expect } from '@playwright/test'
import { appUrl, loadFixtureEnv } from './fixture-env'

test.describe('Portty 02-03 — web share page follows prefers-color-scheme (THEM-01)', () => {
  test('terminal palette switches dark -> light without a reload', async ({ browser }) => {
    const env = loadFixtureEnv()
    const ctx = await browser.newContext({ ignoreHTTPSErrors: true, colorScheme: 'dark' })
    try {
      const page = await ctx.newPage()
      await page.goto(appUrl(env), { waitUntil: 'domcontentloaded' })
      const viewport = page.locator('.xterm-scrollable-element').first()
      await expect(viewport).toBeVisible({ timeout: 15_000 })
      const bg = () => viewport.evaluate((el) => getComputedStyle(el).backgroundColor)

      await expect.poll(bg).toBe('rgb(28, 28, 30)')
      await page.emulateMedia({ colorScheme: 'light' })
      await expect.poll(bg).toBe('rgb(255, 255, 255)')
    } finally {
      await ctx.close()
    }
  })
})
