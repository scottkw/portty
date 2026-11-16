// Quick 261116-ku9 (R1): Settings ▸ General ▸ Terminal Font.

// Nerd Font fallbacks after SF Mono: prompt themes (Powerlevel10k) draw their icons from them.
export const DEFAULT_TERMINAL_FONT_STACK =
  'ui-monospace, "SF Mono", Menlo, "MesloLGS NF", "Symbols Nerd Font Mono", "Hack Nerd Font Mono", monospace'

export const DEFAULT_TERMINAL_FONT_SIZE = 13

export interface TerminalFont {
  family: string
  size: number
}

// Same rules as the daemon's validateTerminalFont (defense in depth): nothing
// that could escape the double-quoted family in a font string.
function isSafeFamily(family: string): boolean {
  // eslint-disable-next-line no-control-regex
  return [...family].length <= 64 && !/["';{}\\\u0000-\u001f\u007f-\u009f]/.test(family)
}

/** The xterm fontFamily for a stored family: quoted family plus the default stack, or the stack alone. */
export function terminalFontFamily(family: string): string {
  const f = family.trim()
  if (f === '' || !isSafeFamily(f)) return DEFAULT_TERMINAL_FONT_STACK
  return `"${f}", ${DEFAULT_TERMINAL_FONT_STACK}`
}

/** A host terminalFont from /api/plugin-config, or null unless it is a valid {family, size}. */
export function parseTerminalFont(value: unknown): TerminalFont | null {
  if (typeof value !== 'object' || value === null) return null
  const { family, size } = value as { family?: unknown; size?: unknown }
  if (typeof family !== 'string' || !isSafeFamily(family.trim())) return null
  if (typeof size !== 'number' || !Number.isInteger(size) || size < 6 || size > 32) return null
  return { family: family.trim(), size }
}
