import type { ITheme } from '@xterm/xterm'

// THEM-01: values copied verbatim from 02-UI-SPEC.md "Terminal palettes". Module
// constants, so the identity is stable and TerminalPanel's theme effect only fires
// on a real light/dark change.
export const LIGHT_PALETTE: ITheme = {
  background: '#FFFFFF',
  foreground: '#1D1D1F',
  cursor: '#1D1D1F',
  cursorAccent: '#FFFFFF',
  selectionBackground: 'rgba(0,99,204,0.25)',
  black: '#1D1D1F',
  red: '#CF222E',
  green: '#1A7F37',
  yellow: '#9A6700',
  blue: '#0969DA',
  magenta: '#8250DF',
  cyan: '#1B7C83',
  white: '#636366',
  brightBlack: '#6E6E73',
  brightRed: '#A40E26',
  brightGreen: '#116329',
  brightYellow: '#7D4E00',
  brightBlue: '#0550AE',
  brightMagenta: '#6639BA',
  brightCyan: '#0F5E64',
  brightWhite: '#8E8E93',
}

export const DARK_PALETTE: ITheme = {
  background: '#1C1C1E',
  foreground: '#D8D8DC',
  cursor: '#D8D8DC',
  cursorAccent: '#1C1C1E',
  selectionBackground: 'rgba(77,163,255,0.35)',
  black: '#48484A',
  red: '#FF7B72',
  green: '#7EE787',
  yellow: '#E3B341',
  blue: '#79C0FF',
  magenta: '#D2A8FF',
  cyan: '#76E3EA',
  white: '#D8D8DC',
  brightBlack: '#8E8E93',
  brightRed: '#FFA198',
  brightGreen: '#A6F4B0',
  brightYellow: '#F2CC60',
  brightBlue: '#A5D6FF',
  brightMagenta: '#E2C5FF',
  brightCyan: '#B3F0FF',
  brightWhite: '#FFFFFF',
}
