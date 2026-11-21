// Phase 11 (PREV-02, 11-07 fix): the Markdown parse runs in a worker under a hard time limit.
// Every pathological parse here runs in a thread that the limit terminates, inside a bounded test.
import { describe, it, expect, vi, afterEach } from 'vitest'
import { once } from 'node:events'
import { Worker as NodeWorker } from 'node:worker_threads'
import { dirname, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { parseMarkdownInWorker, MARKDOWN_PARSE_TIMEOUT_MS } from '../parseMarkdownInWorker'

class FakeWorker {
  onmessage: ((e: { data: unknown }) => void) | null = null
  onerror: ((e: unknown) => void) | null = null
  posted: string[] = []
  terminated = 0
  postMessage(src: string): void {
    this.posted.push(src)
  }
  terminate(): void {
    this.terminated++
  }
}

function fake(): { w: FakeWorker; make: () => Worker } {
  const w = new FakeWorker()
  return { w, make: () => w as unknown as Worker }
}

afterEach(() => {
  vi.useRealTimers()
})

describe('parseMarkdownInWorker (PREV-02, 11-07 fix)', () => {
  it('the limit is 3 s', () => {
    expect(MARKDOWN_PARSE_TIMEOUT_MS).toBe(3000)
  })

  it('posts the source, resolves with the reply and terminates the worker', async () => {
    const { w, make } = fake()
    const p = parseMarkdownInWorker('# Hi', undefined, make)
    expect(w.posted).toEqual(['# Hi'])
    w.onmessage!({ data: '<h1>Hi</h1>' })
    await expect(p).resolves.toBe('<h1>Hi</h1>')
    expect(w.terminated).toBe(1)
  })

  it('no reply: rejects at the limit, not before, and terminates the worker', async () => {
    vi.useFakeTimers()
    const { w, make } = fake()
    const settled = vi.fn()
    const p = parseMarkdownInWorker('x', undefined, make).catch((e: Error) => {
      settled(e.message)
    })
    await vi.advanceTimersByTimeAsync(MARKDOWN_PARSE_TIMEOUT_MS - 1)
    expect(settled).not.toHaveBeenCalled()
    expect(w.terminated).toBe(0)
    await vi.advanceTimersByTimeAsync(1)
    await p
    expect(settled).toHaveBeenCalledWith('Markdown parse timed out')
    expect(w.terminated).toBe(1)
    // A late reply has nowhere to go.
    expect(w.onmessage).toBeNull()
    expect(w.onerror).toBeNull()
  })

  it('a worker error rejects and terminates the worker', async () => {
    const { w, make } = fake()
    const p = parseMarkdownInWorker('x', undefined, make)
    w.onerror!(new Event('error'))
    await expect(p).rejects.toThrow('Markdown worker failed')
    expect(w.terminated).toBe(1)
  })

  it('abort rejects and terminates the worker; an already-aborted signal starts no worker', async () => {
    const { w, make } = fake()
    const stop = new AbortController()
    const p = parseMarkdownInWorker('x', stop.signal, make)
    stop.abort()
    await expect(p).rejects.toBeDefined()
    expect(w.terminated).toBe(1)

    const makeSpy = vi.fn(make)
    await expect(parseMarkdownInWorker('x', stop.signal, makeSpy)).rejects.toBeDefined()
    expect(makeSpy).not.toHaveBeenCalled()
  })

  it('a worker that cannot be created rejects', async () => {
    const make = (): Worker => {
      throw new Error('no Worker')
    }
    await expect(parseMarkdownInWorker('x', undefined, make)).rejects.toThrow('no Worker')
  })
})

// A real thread: node's worker_threads running the same parseMarkdown (node strips the types),
// adapted to the Worker surface parseMarkdownInWorker uses.
const markdownUrl = pathToFileURL(resolve(dirname(fileURLToPath(import.meta.url)), '../markdown.ts')).href
const threads: NodeWorker[] = []

function nodeThread(): { make: () => Worker; exited: () => Promise<unknown> } {
  let exited: Promise<unknown> = Promise.resolve()
  const make = (): Worker => {
    const t = new NodeWorker(
      `const { parentPort } = require('node:worker_threads');
       import(${JSON.stringify(markdownUrl)}).then(({ parseMarkdown }) =>
         parentPort.on('message', (src) => parentPort.postMessage(parseMarkdown(src))))`,
      { eval: true },
    )
    threads.push(t)
    exited = once(t, 'exit')
    const w = new FakeWorker()
    w.postMessage = (src) => t.postMessage(src)
    w.terminate = () => void t.terminate()
    t.on('message', (data) => w.onmessage?.({ data }))
    t.on('error', (err) => w.onerror?.(err))
    return w as unknown as Worker
  }
  return { make, exited: () => exited }
}

describe('parseMarkdownInWorker in a real thread (PREV-02, 11-07 fix)', () => {
  afterEach(async () => {
    // Belt and braces: never leave a parse running past its test.
    await Promise.all(threads.splice(0).map((t) => t.terminate()))
  })

  it('normal Markdown parses, and the thread is gone afterwards', async () => {
    const thread = nodeThread()
    await expect(parseMarkdownInWorker('# Hi\n\n*em*', undefined, thread.make)).resolves.toBe(
      '<h1>Hi</h1>\n<p><em>em</em></p>\n',
    )
    await thread.exited()
  }, 10_000)

  it('81 KB of unclosed emphasis (11-07 measured 23 s at 80 KB): rejected within 3.5 s, the thread is terminated, and the main thread keeps running', async () => {
    const thread = nodeThread()
    let ticks = 0
    const timer = setInterval(() => ticks++, 50)
    const start = performance.now()
    try {
      await expect(parseMarkdownInWorker('*a **b _c'.repeat(9_000), undefined, thread.make)).rejects.toThrow(
        'Markdown parse timed out',
      )
    } finally {
      clearInterval(timer)
    }
    const elapsed = performance.now() - start
    expect(elapsed).toBeGreaterThanOrEqual(MARKDOWN_PARSE_TIMEOUT_MS - 50)
    expect(elapsed).toBeLessThan(3500)
    // About 60 ticks at 50 ms over 3 s; a frozen main thread would manage at most one.
    expect(ticks).toBeGreaterThanOrEqual(20)
    await thread.exited()
  }, 10_000)
})
