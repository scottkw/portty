// Portty 14-04 (MOB-05): the in-app refusal screen shows only its fixed copy.
import { describe, it, expect, afterEach } from 'vitest'
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { UserRefused } from '../UserRefused'

const HEADING = 'This link only works on your own devices'
const BODY = 'Open it on a device signed in to the same Tailscale account as the Mac that shared it.'

let root: Root | undefined
let container: HTMLDivElement | undefined

afterEach(() => {
  if (root) act(() => root!.unmount())
  container?.remove()
  root = undefined
  container = undefined
})

describe('UserRefused', () => {
  it('renders the heading and body copy and nothing else', () => {
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
    act(() => root!.render(React.createElement(UserRefused)))
    expect(container.querySelector('h1')?.textContent).toBe(HEADING)
    expect(container.querySelector('p')?.textContent).toBe(BODY)
    expect(container.textContent).toBe(HEADING + BODY)
  })
})
