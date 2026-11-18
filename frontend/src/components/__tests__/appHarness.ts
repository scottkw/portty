// Test harness that mounts the real App with the Wails bridge faked on window.
// The generated stubs call window.go.main.App[method](...); every method resolves
// to impl[method](...args) when given, else null. Runtime calls are no-ops; EventsOn callbacks land in wailsEvents.
import React from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { act } from 'react'
import App from '../../App'

type Impl = Record<string, (...args: unknown[]) => unknown>

/** The callback App registered per Wails event name (EventsOn), so tests can fire Go events. */
export const wailsEvents = new Map<string, (...args: unknown[]) => void>()

let root: Root | undefined
let container: HTMLDivElement | undefined

// Bindings whose null result App can't take.
const defaults: Impl = {
  GetDaemonError: () => '',
  GetTerminalFont: () => ({ family: '', size: 13 }),
}

export async function mountApp(overrides: Impl): Promise<HTMLDivElement> {
  const impl = { ...defaults, ...overrides }
  const w = window as unknown as Record<string, unknown>
  w.go = {
    main: {
      App: new Proxy({}, {
        get: (_t, k: string) => (...args: unknown[]) => Promise.resolve(impl[k] ? impl[k](...args) : null),
      }),
    },
  }
  w.runtime = new Proxy({}, {
    get: (_t, k: string) => k === 'EventsOn' // the test runtime stub (src/wailsjs/runtime) calls window.runtime.EventsOn
      ? (name: string, cb: (...args: unknown[]) => void) => { wailsEvents.set(name, cb); return () => {} }
      : () => () => {},
  })
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  await act(async () => { root!.render(React.createElement(App)) })
  await flush()
  return container
}

/** Let pending promises and the effects they trigger settle. */
export async function flush(): Promise<void> {
  for (let i = 0; i < 5; i++) {
    await act(async () => { await Promise.resolve() })
  }
}

export async function unmountApp(): Promise<void> {
  if (root) await act(async () => { root!.unmount() })
  root = undefined
  container?.remove()
  container = undefined
  const w = window as unknown as Record<string, unknown>
  delete w.go
  delete w.runtime
  wailsEvents.clear()
}
