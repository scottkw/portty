// Regression for debug selection-offset-copy. A pane that follows a larger session
// grid is drawn with transform: scale(s < 1); a drag must still select the row under
// the pointer, a TUI must get that row in its mouse reports, and Option-drag must make
// an xterm selection (for Cmd-C) while a TUI such as Claude Code captures the mouse.
// Output and grid (0x02) frames are injected into the /app/ web view's relay socket;
// the fixture's PTY is a silent stub. navigator.platform is forced to MacIntel so
// xterm takes its macOS paths (Option forces a selection only there).

import { test, expect, type Page, type WebSocketRoute } from '@playwright/test'
import { appUrl } from './fixture-env'

const ROWS_OF_TEXT = 60

// Capture every xterm Terminal the page creates (Terminal's constructor assigns this._core).
const HOOK = `
  if (!/Mac/.test(navigator.platform)) Object.defineProperty(Navigator.prototype, 'platform', { get: () => 'MacIntel' });
  window.__terms = [];
  Object.defineProperty(Object.prototype, '_core', {
    configurable: true,
    set(v) {
      Object.defineProperty(this, '_core', { value: v, writable: true, configurable: true, enumerable: false });
      if (v && typeof this.getSelection === 'function') window.__terms.push(this);
    },
  });
`

function frame(type: number, payload: Uint8Array): Buffer {
  const b = Buffer.alloc(payload.length + 1)
  b[0] = type
  b.set(payload, 1)
  return b
}
const output = (s: string) => frame(0x01, new TextEncoder().encode(s))
const grid = (cols: number, rows: number) => frame(0x02, new Uint8Array([cols >> 8, cols & 255, rows >> 8, rows & 255]))

async function setup(page: Page): Promise<{ sock: () => WebSocketRoute }> {
  let route: WebSocketRoute | null = null
  await page.routeWebSocket(/\/ws/, (ws) => {
    ws.connectToServer()
    route = ws
  })
  await page.addInitScript(HOOK)
  await page.goto(appUrl())
  await page.locator('.xterm-screen').waitFor()
  await expect.poll(() => route !== null).toBe(true)
  await page.waitForTimeout(500)
  return { sock: () => route! }
}

interface Geo {
  cols: number
  rows: number
  cellW: number
  cellH: number
  top: number
  left: number
  visH: number
  transform: string
}

async function geo(page: Page): Promise<Geo> {
  return page.evaluate(() => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const t = (window as any).__terms.find((x: any) => x.element?.isConnected)
    const cell = t._core._renderService.dimensions.css.cell
    const r = t._core.screenElement.getBoundingClientRect()
    return { cols: t.cols, rows: t.rows, cellW: cell.width, cellH: cell.height, top: r.top, left: r.left, visH: r.height, transform: t.element.style.transform }
  })
}

async function fitOf(page: Page): Promise<{ cols: number; rows: number }> {
  return page.evaluate(() => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const t = (window as any).__terms.find((x: any) => x.element?.isConnected)
    const cell = t._core._renderService.dimensions.css.cell
    const p = t.element.parentElement
    const cs = getComputedStyle(p)
    const w = p.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight)
    const h = p.clientHeight - parseFloat(cs.paddingTop) - parseFloat(cs.paddingBottom)
    return { cols: Math.floor(w / cell.width), rows: Math.floor(h / cell.height) }
  })
}

async function fill(page: Page, sock: WebSocketRoute) {
  let s = ''
  for (let i = 1; i <= ROWS_OF_TEXT; i++) s += `line-${String(i).padStart(2, '0')} abcdefghij\r\n`
  sock.send(output(s))
  await page.waitForTimeout(300)
}

// Drag across visual row `vrow` (1-based, counted from the top of the visible screen).
async function dragRow(page: Page, g: Geo, vrow: number) {
  const visCell = g.visH / g.rows // what the eye sees, after any transform
  const y = g.top + (vrow - 0.5) * visCell
  const x0 = g.left + 2
  const x1 = g.left + (g.visH / g.rows / g.cellH) * g.cellW * 15
  await page.mouse.move(x0, y)
  await page.mouse.down()
  await page.mouse.move((x0 + x1) / 2, y, { steps: 5 })
  await page.mouse.move(x1, y, { steps: 5 })
  await page.mouse.up()
}

async function selection(page: Page): Promise<{ text: string; viewportY: number }> {
  return page.evaluate(() => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const t = (window as any).__terms.find((x: any) => x.element?.isConnected)
    return { text: t.getSelection(), viewportY: t.buffer.active.viewportY }
  })
}

async function visibleRowText(page: Page, vrow: number): Promise<string> {
  return page.evaluate((v) => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const t = (window as any).__terms.find((x: any) => x.element?.isConnected)
    const b = t.buffer.active
    return b.getLine(b.viewportY + v - 1)?.translateToString(true) ?? ''
  }, vrow)
}

async function setGrid(page: Page, sock: WebSocketRoute, mult: number): Promise<number> {
  const fit = await fitOf(page)
  sock.send(grid(Math.round(fit.cols * mult), Math.round(fit.rows * mult)))
  await page.waitForTimeout(300)
  const t = (await geo(page)).transform
  return Number(/scale\(([\d.]+)\)/.exec(t)?.[1] ?? NaN)
}

function mouseTracking(page: Page, sock: WebSocketRoute): Promise<string[]> {
  sock.send(output('\x1b[?1002h\x1b[?1006h')) // button-event tracking + SGR, as a TUI sets
  const sent: string[] = []
  sock.onMessage((m) => {
    const b = Buffer.from(m as Buffer)
    if (b[0] === 0x10) sent.push(b.subarray(1).toString()) // input frames
  })
  return page.waitForTimeout(200).then(() => sent)
}

// What xterm puts on the clipboard for a copy event (the handler Cmd-C reaches).
async function copied(page: Page): Promise<string> {
  return page.evaluate(() => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const t = (window as any).__terms.find((x: any) => x.element?.isConnected)
    const ev = new ClipboardEvent('copy', { clipboardData: new DataTransfer(), bubbles: true, cancelable: true })
    t.element.dispatchEvent(ev)
    // Read the event's own clipboardData: Firefox replaces the DataTransfer passed in the init dict.
    return ev.clipboardData?.getData('text/plain') ?? ''
  })
}

test.describe('selection under the pointer (selection-offset-copy)', () => {
  for (const [name, mult] of [['grid equal to the fit', 1], ['grid 1.2x the fit (lone viewer on a stale grid)', 1.2]] as const) {
    test(`${name}: drags on the top, middle and bottom rows select the pointer row`, async ({ page }) => {
      const { sock } = await setup(page)
      const s = await setGrid(page, sock(), mult)
      if (mult === 1) expect(s).toBe(1)
      else expect(s).toBeLessThan(0.95)
      await fill(page, sock())
      const g = await geo(page)
      for (const vrow of [3, Math.round(g.rows / 2), g.rows - 2]) {
        await dragRow(page, g, vrow)
        const sel = await selection(page)
        const want = await visibleRowText(page, vrow)
        expect(sel.text.trim().slice(0, 7), `scale ${s}, visual row ${vrow}`).toBe(want.slice(0, 7))
      }
    })
  }

  test('scaled pane: a TUI\'s mouse report carries the pointer row', async ({ page }) => {
    const { sock } = await setup(page)
    expect(await setGrid(page, sock(), 1.2)).toBeLessThan(0.95)
    const sent = await mouseTracking(page, sock())
    const g = await geo(page)
    const vrow = g.rows - 2
    await page.mouse.click(g.left + 2, g.top + (vrow - 0.5) * (g.visH / g.rows))
    await expect.poll(() => sent.length).toBeGreaterThan(0)
    expect(sent[0]).toMatch(new RegExp(`^\\x1b\\[<0;1;${vrow}M$`))
  })

  test('mouse tracking on (Claude Code fullscreen), scaled pane: plain drag goes to the app; Option-drag selects the pointer row and copy carries it', async ({ page }) => {
    const { sock } = await setup(page)
    expect(await setGrid(page, sock(), 1.2)).toBeLessThan(0.95)
    await fill(page, sock())
    const sent = await mouseTracking(page, sock())
    const g = await geo(page)
    const vrow = g.rows - 2
    const want = await visibleRowText(page, vrow)

    await dragRow(page, g, vrow)
    expect((await selection(page)).text).toBe('')
    expect(sent.length).toBeGreaterThan(0)

    const before = sent.length
    await page.keyboard.down('Alt')
    await dragRow(page, g, vrow)
    await page.keyboard.up('Alt')
    const sel = await selection(page)
    expect(sel.text.trim().slice(0, 7)).toBe(want.slice(0, 7))
    expect(sent.length).toBe(before) // the Option-drag went to xterm, not the app
    expect(await copied(page)).toBe(sel.text)
  })

  test('Cmd-C puts the Option-drag selection on the clipboard (macOS host, chromium)', async ({ page, context, browserName }) => {
    // ControlOrMeta is Meta only on a macOS host, matching the forced Mac platform; clipboard read is chromium-only.
    test.skip(process.platform !== 'darwin' || browserName !== 'chromium', 'needs a macOS host and chromium')
    await context.grantPermissions(['clipboard-read', 'clipboard-write'])
    const { sock } = await setup(page)
    expect(await setGrid(page, sock(), 1.2)).toBeLessThan(0.95)
    await fill(page, sock())
    await mouseTracking(page, sock())
    const g = await geo(page)
    await page.keyboard.down('Alt')
    await dragRow(page, g, g.rows - 2)
    await page.keyboard.up('Alt')
    const sel = await selection(page)
    expect(sel.text.trim().slice(0, 7)).toBe((await visibleRowText(page, g.rows - 2)).slice(0, 7))
    await page.evaluate(() => navigator.clipboard.writeText('SENTINEL'))
    await page.keyboard.press('ControlOrMeta+C')
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(sel.text)
  })
})
