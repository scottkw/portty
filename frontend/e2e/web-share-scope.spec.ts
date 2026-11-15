// Phase 159 Plan 02 (gap-closure) — web-share-scope.spec.ts
//
// WEBCHAT-03: a remote web-share guest who lands on /app/?session=&cap= (via the
// 159-01 redirect) must see ONLY the scoped session surface — the terminal (chat
// was removed in Portty 02-07, CHAT-01) — and NOT the desktop app chrome (the Sidebar with
// Home / Hub / Settings / session groups).
//
// Discovered during 159 live UAT: /app/ rendered the full desktop shell because
// <Sidebar> was rendered unconditionally in App.tsx (no `mode === 'web'` gate).
// A guest given a capability for one session could navigate to Home / Hub /
// Settings and reach the open /api/sessions/meta enumeration surface.
//
// Selector contract:
//   nav[aria-label="Sessions"]         — the desktop Sidebar since Portty 02-04 (MUST be absent in web mode)
//   nav[aria-label="Main navigation"]  — the pre-02-04 Sidebar label (kept as a regression guard)
//   .xterm                             — xterm terminal (session surface present)
//   button named /chat/i               — MUST be absent (Portty 02-07, CHAT-01)

import { test, expect } from '@playwright/test'
import { appUrl, viewerAppUrl, loadFixtureEnv } from './fixture-env'

test.describe('Phase 159-02 — web-share guest scope (WEBCHAT-03)', () => {
  test('web-share /app/ guest sees the scoped session surface, NOT the desktop Sidebar', async ({ browser }) => {
    const env = loadFixtureEnv()
    const ctx = await browser.newContext({ ignoreHTTPSErrors: true })
    try {
      const page = await ctx.newPage()
      await page.goto(appUrl(env), { waitUntil: 'domcontentloaded' })

      // Scoped surface IS present: the terminal mount, and no chat control (CHAT-01).
      await expect(page.locator('.xterm').first()).toBeVisible({ timeout: 15_000 })
      await expect(page.getByRole('button', { name: /chat/i })).toHaveCount(0)

      // Desktop chrome is NOT present: the Sidebar nav must not render in web mode.
      await expect(page.locator('nav[aria-label="Sessions"]')).toHaveCount(0)
      await expect(page.locator('nav[aria-label="Main navigation"]')).toHaveCount(0)
      // Portty 02-04: no peer list or This Mac group either.
      await expect(page.getByText('This Mac')).toHaveCount(0)

      // Belt-and-suspenders: the Sidebar's nav entry points must be unreachable.
      await expect(page.getByRole('button', { name: 'Hub' })).toHaveCount(0)
      await expect(page.getByRole('button', { name: 'Settings' })).toHaveCount(0)
    } finally {
      await ctx.close()
    }
  })

  // WEBCHAT-05 + Phase 01-02: a web-share guest gets no session-menu chevron and
  // no file tab, whatever perms the cap carries. Rename / Save Terminal As are
  // Wails RPCs with no browser bridge, and the file browser is gone, so the menu
  // has nothing to offer. Double-click rename is disabled too.
  for (const [label, urlFor] of [['owner', appUrl], ['viewer', viewerAppUrl]] as const) {
    test(`${label} cap: no session-menu chevron, no file tab, no double-click rename`, async ({ browser }) => {
      const env = loadFixtureEnv()
      const ctx = await browser.newContext({ ignoreHTTPSErrors: true })
      try {
        const page = await ctx.newPage()
        await page.goto(urlFor(env), { waitUntil: 'domcontentloaded' })
        await expect(page.locator('.xterm').first()).toBeVisible({ timeout: 15_000 })
        await expect(page.getByRole('button', { name: /chat/i })).toHaveCount(0)
        await page.waitForTimeout(2_000) // let any late bootstrap work settle
        await expect(page.locator('[data-testid="tab-chevron"]')).toHaveCount(0)
        await expect(page.getByRole('button', { name: /— Files$/ })).toHaveCount(0)

        await page.locator('.tab__name', { hasText: 'Session' }).first().dblclick()
        await page.waitForTimeout(300)
        await expect(page.locator('input[aria-label="Session name"]')).toHaveCount(0)
      } finally {
        await ctx.close()
      }
    })
  }
})
