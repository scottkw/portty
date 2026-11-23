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

  // tap() in WebKit's iPhone emulation dispatches pointerdown (and a click with detail 1), so no
  // synthetic pointerdown fallback is needed. Each hex is exact: one tap sends once.
  const key = (page: Page, name: string) => page.getByRole('button', { name, exact: true })
  const expectHex = (page: Page, hex: string) => expect.poll(() => inputHex(page), { timeout: 2_000 }).toBe(hex)

  test('tapping Esc reaches the shell as 1b', async ({ page }) => {
    await key(page, 'Esc').tap()
    await expectHex(page, '1b')
  })

  test('Tab sends 09', async ({ page }) => {
    await key(page, 'Tab').tap()
    await expectHex(page, '09')
  })

  test('the pipe key sends 7c', async ({ page }) => {
    await key(page, 'Vertical bar').tap()
    await expectHex(page, '7c')
  })

  test('Up sends 1b5b41', async ({ page }) => {
    await key(page, 'Up arrow').tap()
    await expectHex(page, '1b5b41')
  })

  test('Ctrl then Up sends 1b5b313b3541 and Ctrl releases', async ({ page }) => {
    await key(page, 'Ctrl').tap()
    await expect(key(page, 'Ctrl')).toHaveAttribute('aria-pressed', 'true')
    await key(page, 'Up arrow').tap()
    await expectHex(page, '1b5b313b3541')
    await expect(key(page, 'Ctrl')).toHaveAttribute('aria-pressed', 'false')
  })

  test('Ctrl then typing c on the keyboard sends 03 and Ctrl releases', async ({ page }) => {
    await key(page, 'Ctrl').tap()
    await page.locator('.xterm-helper-textarea').focus()
    await page.keyboard.type('c')
    await expectHex(page, '03')
    await expect(key(page, 'Ctrl')).toHaveAttribute('aria-pressed', 'false')
  })

  test('the eight keys fit one row at 44 pt or more', async ({ page }) => {
    const boxes = await page.getByRole('toolbar', { name: 'Terminal keys' }).getByRole('button').evaluateAll(
      (els) => els.map((el) => el.getBoundingClientRect()).map((r) => ({ y: r.y, w: r.width, h: r.height, right: r.right })),
    )
    expect(boxes).toHaveLength(8)
    for (const b of boxes) {
      expect(b.w).toBeGreaterThanOrEqual(44)
      expect(b.h).toBeGreaterThanOrEqual(44)
      expect(b.y).toBe(boxes[0].y)
    }
    const vw = page.viewportSize()!.width
    expect(Math.max(...boxes.map((b) => b.right))).toBeLessThanOrEqual(vw)
  })
})

test.describe('Phase 14-11 — phone chrome on an iPhone (MOB-01)', () => {
  test.beforeEach(async ({ page }) => { await openShare(page) })

  test('with only the session tab, the tab strip is not shown', async ({ page }) => {
    await expect(page.locator('.tab-bar')).toBeHidden()
  })

  test('terminal padding is 4px 8px', async ({ page }) => {
    const pad = await page.locator('.terminal-session-container').evaluate((el) => getComputedStyle(el).padding)
    expect(pad).toBe('4px 8px')
  })

  // WR-03: the pinch block is the terminal's alone. The page and the in-app PDF layer keep native zoom.
  test('page zoom is blocked on the terminal container only', async ({ page }) => {
    const ta = (sel: string) => page.locator(sel).evaluate((el) => getComputedStyle(el).touchAction)
    expect(await ta('.terminal-session-container')).toBe('pan-y') // no pinch-zoom: the base rule, not a phone override
    expect(await ta('html')).toBe('auto')
    expect(await ta('.app')).toBe('auto')
  })

  test('the xterm helper textarea is 16px, so iOS does not zoom on focus', async ({ page }) => {
    const size = await page.locator('.xterm-helper-textarea').evaluate((el) => getComputedStyle(el).fontSize)
    expect(size).toBe('16px')
  })

  test('the key bar is the last row, below the terminal area', async ({ page }) => {
    const bar = await page.getByRole('toolbar', { name: 'Terminal keys' }).boundingBox()
    const term = await page.locator('.terminal-session-container').boundingBox()
    expect(bar!.y).toBeGreaterThanOrEqual(term!.y + term!.height - 1)
  })
})
