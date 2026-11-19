/**
 * 08-REVIEW WR-01: xterm 6.0.0's paste handler (Clipboard.ts handlePasteEvent)
 * reads clipboardData, sends it and clears its helper textarea, but never calls
 * preventDefault(). The browser then inserts the whole paste into that textarea,
 * where WebKit re-shapes it on every caret blink (20 MB: ~100% CPU, 1.8 GB for
 * 15+ minutes). Cancel only the default insertion: xterm's listener, registered
 * first on the same textarea, already ran (it stops propagation, not immediate
 * propagation), so bracketed paste and the input splitting are unchanged.
 * Returns the remover for the mount effect's cleanup.
 */
export function guardPasteInsert(textarea: HTMLTextAreaElement | undefined): () => void {
  const stop = (e: Event) => e.preventDefault()
  textarea?.addEventListener('paste', stop)
  return () => textarea?.removeEventListener('paste', stop)
}
