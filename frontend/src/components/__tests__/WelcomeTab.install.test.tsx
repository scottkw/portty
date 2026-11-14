// frontend/src/components/__tests__/WelcomeTab.install.test.tsx
// Phase 156 — INSTALL-02 source-gate test (repo link; Linux/WinGet rows removed in Portty Phase 1).
// Pattern: reads WelcomeTab.tsx as a raw string (no render), asserts exact install strings.
// Consistent with style.hub.test.ts, style.redesign.test.ts source-gate pattern.
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, it, expect } from 'vitest'

const SOURCE = readFileSync(
  join(__dirname, '../../components/WelcomeTab.tsx'),
  'utf-8'
)

describe('WelcomeTab — install string source gates (INSTALL-02)', () => {
  it('INSTALL-02: repo link points to scottkw/agenthub', () => {
    expect(SOURCE).toContain('github.com/scottkw/agenthub')
  })

  it('INSTALL-02: repo link does NOT contain the wrong agenthub-dev org', () => {
    expect(SOURCE).not.toContain('agenthub-dev/agenthub')
  })
})
