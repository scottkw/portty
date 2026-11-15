import { describe, it, expect } from 'vitest'
import raw from '../../App.tsx?raw'
import hubRaw from '../Hub/HubPanel.tsx?raw'

// Source-inspection tests for App.tsx Hub integration (Phase 131 Plan 05).
// These verify the wiring contract without mounting the full Wails component tree.
// Pattern: mirrors App.nav.test.tsx (source inspection via ?raw import).

describe('HUB-TAB: HUB_TAB constant exists with correct id and type (HUB-01)', () => {
  it("defines HUB_TAB constant", () => {
    expect(raw).toContain('HUB_TAB')
  })

  it("HUB_TAB has id '__hub__'", () => {
    expect(raw).toContain("id: '__hub__'")
  })

  it("HUB_TAB has type 'hub'", () => {
    expect(raw).toContain("type: 'hub'")
  })

  it('HUB_TAB has name Hub', () => {
    expect(raw).toContain("name: 'Hub'")
  })
})

// Portty 02-04: HUB-WIRE and HUB-POLL removed with the Hub sidebar item and the Hub-gated poll.

describe('HUB-RENDER: HubPanel is rendered with correct props (HUB-01)', () => {
  it('imports HubPanel from Hub/HubPanel', () => {
    expect(raw).toContain("from './components/Hub/HubPanel'")
  })

  it('renders HubPanel when activeId === HUB_TAB.id', () => {
    expect(raw).toContain('activeId === HUB_TAB.id')
  })

  it('passes sessions={sessions} to HubPanel', () => {
    expect(raw).toContain('sessions={sessions}')
  })

  it('passes error={hubError} to HubPanel', () => {
    expect(raw).toContain('error={hubError}')
  })

  it('HubPanel onNewSession calls setShowNewSessionModal(true)', () => {
    expect(raw).toContain('setShowNewSessionModal(true)')
  })

  it('passes onRename={handleRenameTab} to HubPanel', () => {
    expect(raw).toContain('onRename={handleRenameTab}')
  })
})

// Phase 138 / NAV-03/04: DaemonManagerPanel and RemoteSessionsPanel are deleted.
// App.tsx must NOT reference them or their constants after Plan 02.
describe('NAV-03/04: DaemonManagerPanel and RemoteSessionsPanel are removed (Phase 138)', () => {
  it('does not define DAEMON_MANAGER_TAB constant', () => {
    expect(raw).not.toContain('DAEMON_MANAGER_TAB')
  })
  it('does not define REMOTE_SESSIONS_TAB constant', () => {
    expect(raw).not.toContain('REMOTE_SESSIONS_TAB')
  })
  it('does not render DaemonManagerPanel', () => {
    expect(raw).not.toContain('DaemonManagerPanel')
  })
  it('does not render RemoteSessionsPanel', () => {
    expect(raw).not.toContain('RemoteSessionsPanel')
  })
})

// Phase 138 / CARD-01: HubPanel receives new affordance props; header-less.
// These assertions are RED until Plan 02 wires onKill/onOpenInBrowser in App.tsx
// and Plan 03 removes the .hub__header block from HubPanel.tsx.
describe('CARD-01: HubPanel receives onKill, onOpenInBrowser (Phase 138)', () => {
  it('wires onKill to handleCloseTab', () => {
    expect(raw).toContain('onKill=')
  })
  it('wires onOpenInBrowser to handleOpenRemoteSession', () => {
    expect(raw).toContain('onOpenInBrowser=')
  })
  it('HubPanel source does NOT contain hub__header (header removed)', () => {
    expect(hubRaw).not.toContain('hub__header')
  })
})

describe('HUB-02: Hub coexists with other tabs (terminal-exclusion sites)', () => {
  it("'hub' is excluded from the daemonError empty-filter (first terminal-exclusion site)", () => {
    // The daemonError check filters out non-terminal tabs to decide whether to show the error.
    // 'hub' must be excluded so the Hub tab doesn't trigger daemon-error display.
    const daemonErrorIdx = raw.indexOf('daemonError && tabs.filter')
    expect(daemonErrorIdx).toBeGreaterThan(-1)
    const surroundingBlock = raw.slice(daemonErrorIdx, daemonErrorIdx + 300)
    expect(surroundingBlock).toContain("t.type !== 'hub'")
  })

  it("'hub' is excluded from the terminal map (second terminal-exclusion site)", () => {
    // The terminal map skips non-terminal tabs via early-return.
    // 'hub' must be in that exclusion list so it doesn't try to render a TerminalPanel.
    const terminalMapIdx = raw.indexOf("tab.type === 'welcome'")
    expect(terminalMapIdx).toBeGreaterThan(-1)
    const surroundingBlock = raw.slice(terminalMapIdx, terminalMapIdx + 300)
    expect(surroundingBlock).toContain("tab.type === 'hub'")
  })

  it("'hub' appears in the daemonError terminal-exclusion filter (t.type !== 'hub')", () => {
    // First exclusion site uses t.type !== 'hub'
    const matches1 = raw.match(/t\.type !== 'hub'/g)
    expect(matches1).not.toBeNull()
    expect(matches1!.length).toBeGreaterThanOrEqual(1)
  })

  it("'hub' appears in the terminal map exclusion (tab.type === 'hub')", () => {
    // Second exclusion site uses tab.type === 'hub' in the early-return guard
    const matches2 = raw.match(/tab\.type === 'hub'/g)
    expect(matches2).not.toBeNull()
    expect(matches2!.length).toBeGreaterThanOrEqual(1)
  })
})

describe('HUB-REATTACH: open/focus a running session terminal tab (Phase 131 UAT follow-up)', () => {
  it('defines handleOpenSessionTab callback', () => {
    expect(raw).toContain('handleOpenSessionTab')
  })

  it('handleOpenSessionTab focuses an existing tab by sessionId (find-or-create)', () => {
    const idx = raw.indexOf('const handleOpenSessionTab')
    expect(idx).toBeGreaterThan(-1)
    const block = raw.slice(idx, idx + 500)
    // focuses existing tab when one already exists for this session id
    expect(block).toContain('tabs.find((t) => t.id === sessionId)')
    expect(block).toContain('setActiveId(existing.id)')
    // otherwise creates a terminal tab (no `type` → terminal) keyed by sessionId
    expect(block).toContain('sessionId,')
    expect(block).toContain('setActiveId(newTab.id)')
  })

  it('wires onOpenSession={handleOpenSessionTab} to HubPanel', () => {
    const idx = raw.indexOf('<HubPanel')
    expect(idx).toBeGreaterThan(-1)
    const block = raw.slice(idx, idx + 300)
    expect(block).toContain('onOpenSession={handleOpenSessionTab}')
  })
})
