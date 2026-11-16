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
