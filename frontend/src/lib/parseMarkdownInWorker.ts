/**
 * Phase 11 (PREV-02, 11-07 fix): run parseMarkdown in a fresh Web Worker with a
 * hard time limit. marked 18.1.3's emphasis parsing time grows with the square of
 * the unclosed delimiters in a paragraph (80 KB took 23 s; 2 MB never finished),
 * and a frozen main thread freezes the whole window, terminal tabs included. The
 * limit bounds any parser pathology, not just that one.
 *
 * Each call gets its own worker and terminates it when it settles (result,
 * error, timeout or abort), so a runaway parse never outlives its call. The
 * result is NOT sanitized: pass it through sanitizeMarkdown.
 */

/** Hard limit for one parse, worker start-up included. */
export const MARKDOWN_PARSE_TIMEOUT_MS = 3000

// Vite emits markdownWorker.ts as its own same-origin chunk (worker-src 'self' on the share page).
const defaultWorker = (): Worker =>
  new Worker(new URL('./markdownWorker.ts', import.meta.url), { type: 'module' })

export function parseMarkdownInWorker(
  src: string,
  signal?: AbortSignal,
  makeWorker: () => Worker = defaultWorker,
  timeoutMs: number = MARKDOWN_PARSE_TIMEOUT_MS,
): Promise<string> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) return reject(signal.reason)
    const worker = makeWorker()
    const settle = (finish: () => void): void => {
      clearTimeout(timer)
      signal?.removeEventListener('abort', onAbort)
      worker.onmessage = worker.onerror = null
      worker.terminate()
      finish()
    }
    const onAbort = (): void => settle(() => reject(signal!.reason))
    const timer = setTimeout(() => settle(() => reject(new Error('Markdown parse timed out'))), timeoutMs)
    signal?.addEventListener('abort', onAbort)
    worker.onmessage = (e: MessageEvent) => settle(() => resolve(String(e.data)))
    worker.onerror = () => settle(() => reject(new Error('Markdown worker failed')))
    worker.postMessage(src)
  })
}
