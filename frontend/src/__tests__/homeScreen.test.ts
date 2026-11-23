/**
 * Phase 14 MOB-02 — Home Screen source gate: the manifest fields, the head
 * tags and the icon PNGs (size and opacity). Read straight from the source tree.
 */

import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

const __dirname = dirname(fileURLToPath(import.meta.url))
const frontendDir = resolve(__dirname, '..', '..')
const repoDir = resolve(frontendDir, '..')
const html = readFileSync(resolve(frontendDir, 'index.html'), 'utf8')
const manifest = JSON.parse(readFileSync(resolve(frontendDir, 'public/manifest.json'), 'utf8'))

/** Every <meta> or <link> tag whose attribute set contains name="value". */
function tags(tagName: string): Record<string, string>[] {
  const out: Record<string, string>[] = []
  for (const m of html.matchAll(new RegExp(`<${tagName}\\s([^>]*?)/?>`, 'g'))) {
    const attrs: Record<string, string> = {}
    for (const a of m[1].matchAll(/([\w-]+)="([^"]*)"/g)) attrs[a[1]] = a[2]
    out.push(attrs)
  }
  return out
}
const metas = tags('meta')
const links = tags('link')
const meta = (name: string) => metas.filter((t) => t.name === name)

describe('manifest.json', () => {
  it('has the Home Screen fields and no start_url, id or orientation', () => {
    expect(manifest.name).toBe('Portty')
    expect(manifest.short_name).toBe('Portty')
    expect(manifest.display).toBe('standalone')
    expect(manifest.scope).toBe('./')
    expect(manifest.background_color).toBe('#1C1C1E')
    expect(manifest.theme_color).toBe('#1C1C1E')
    for (const k of ['start_url', 'id', 'orientation']) expect(manifest).not.toHaveProperty(k)
  })

  it('lists icon-192 and icon-512 with relative src and no purpose', () => {
    expect(manifest.icons).toEqual([
      { src: 'icon-192.png', sizes: '192x192', type: 'image/png' },
      { src: 'icon-512.png', sizes: '512x512', type: 'image/png' },
    ])
  })
})

describe('index.html head tags', () => {
  it('links the manifest and the apple-touch-icon', () => {
    expect(links.filter((t) => t.rel === 'manifest').map((t) => t.href)).toEqual(['/manifest.json'])
    expect(links.filter((t) => t.rel === 'apple-touch-icon').map((t) => t.href)).toEqual(['/apple-touch-icon.png'])
  })

  it('sets the Apple web-app meta tags', () => {
    expect(meta('apple-mobile-web-app-capable').map((t) => t.content)).toEqual(['yes'])
    expect(meta('apple-mobile-web-app-title').map((t) => t.content)).toEqual(['Portty'])
  })

  it('has a light and a dark theme-color', () => {
    const got = meta('theme-color').map((t) => [t.content, t.media])
    expect(got).toEqual([
      ['#FFFFFF', '(prefers-color-scheme: light)'],
      ['#1C1C1E', '(prefers-color-scheme: dark)'],
    ])
  })

  it('ends the viewport content with viewport-fit=cover', () => {
    const [viewport] = meta('viewport')
    expect(viewport.content.endsWith('viewport-fit=cover')).toBe(true)
  })

  it('leaves the Wails meta CSP untouched', () => {
    const csp = metas.find((t) => t['http-equiv'] === 'Content-Security-Policy')
    expect(csp?.content).toBe("frame-src blob:; child-src blob:; worker-src 'self'")
  })
})

describe('icons', () => {
  const icons: [string, number][] = [
    [resolve(repoDir, 'build/appicon-fullbleed.png'), 1024],
    [resolve(frontendDir, 'public/icon-512.png'), 512],
    [resolve(frontendDir, 'public/icon-192.png'), 192],
    [resolve(frontendDir, 'public/apple-touch-icon.png'), 180],
  ]
  it.each(icons)('%s is %i px square and opaque RGB', (path, size) => {
    const png = readFileSync(path)
    expect(png.subarray(1, 4).toString('ascii')).toBe('PNG')
    expect(png.readUInt32BE(16)).toBe(size) // IHDR width
    expect(png.readUInt32BE(20)).toBe(size) // IHDR height
    expect(png[24]).toBe(8) // bit depth
    expect(png[25]).toBe(2) // colour type 2: RGB, no alpha
  })
})
