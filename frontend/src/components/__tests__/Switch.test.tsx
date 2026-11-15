import { describe, it, expect, afterEach } from 'vitest'
import React, { useState } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { act } from 'react'
import { Switch } from '../Switch'

let container: HTMLDivElement
let root: Root

function Harness({ saving = false }: { saving?: boolean }): React.ReactElement {
  const [on, setOn] = useState(false)
  return (
    <>
      <label htmlFor="sw">Thing</label>
      <Switch id="sw" checked={on} saving={saving} onChange={() => setOn((v) => !v)} />
    </>
  )
}

async function render(el: React.ReactElement) {
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  await act(async () => root.render(el))
  return container.querySelector<HTMLButtonElement>('#sw')!
}

afterEach(async () => {
  await act(async () => root.unmount())
  container.remove()
})

describe('LOOK-01: Switch', () => {
  it('is a role="switch" button whose aria-checked flips on click', async () => {
    const sw = await render(<Harness />)
    expect(sw.tagName).toBe('BUTTON')
    expect(sw.getAttribute('role')).toBe('switch')
    expect(sw.getAttribute('aria-checked')).toBe('false')
    await act(async () => sw.click())
    expect(sw.getAttribute('aria-checked')).toBe('true')
  })

  it('clicking its <label for> toggles it', async () => {
    const sw = await render(<Harness />)
    await act(async () => container.querySelector('label')!.click())
    expect(sw.getAttribute('aria-checked')).toBe('true')
  })

  it('blocks clicks while saving and shows the saving class', async () => {
    const sw = await render(<Harness saving />)
    expect(sw.className).toContain('switch--saving')
    await act(async () => sw.click())
    expect(sw.getAttribute('aria-checked')).toBe('false')
  })
})
