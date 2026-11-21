import type { PreviewFrame } from './relayClient'

/** relay.PreviewMaxBytes: the largest preview a viewer accepts (UI copy says "20 MB"). */
export const PREVIEW_MAX_BYTES = 20_000_000

export type PreviewKind = 'image' | 'svg' | 'pdf' | 'markdown'

// The viewer's own MIME table (relay.PreviewMIMEs). The Blob type is taken from here, never the header.
const PREVIEW_MIMES = [
  ['image/png', 'image'],
  ['image/jpeg', 'image'],
  ['image/gif', 'image'],
  ['image/webp', 'image'],
  ['image/svg+xml', 'svg'],
  ['application/pdf', 'pdf'],
  ['text/markdown', 'markdown'],
] as const

export type PreviewMime = (typeof PREVIEW_MIMES)[number][0]

export type PreviewEvent =
  | { type: 'start'; id: number; name: string; kind: PreviewKind; mime: PreviewMime; size: number; key: string; focus: boolean }
  | { type: 'progress'; id: number; received: number }
  | { type: 'complete'; id: number; blob: Blob }
  | { type: 'failed'; id: number; reason: 'tooLarge' | 'incomplete' }

const PROGRESS_MS = 100

/**
 * The name as shown: C0/C1 controls and bidi override and isolate characters removed (so
 * "gnp.exe" can't pose as "exe.png"), at most 255 characters, never empty.
 */
export function cleanPreviewName(name: string): string {
  const cleaned = Array.from(name.replace(/[\u0000-\u001f\u007f-\u009f‪-‮⁦-⁩]/g, '')).slice(0, 255).join('')
  return cleaned || 'file'
}

/**
 * One assembler per RelayClient connection: turns MSG_PREVIEW frames into preview events.
 * The stream comes from a remote host, so it is bounded here.
 */
export function createPreviewAssembler(emit: (e: PreviewEvent) => void) {
  let open: { id: number; size: number; mime: PreviewMime; chunks: Uint8Array[]; received: number; lastProgress: number } | null = null

  const fail = (reason: 'tooLarge' | 'incomplete') => {
    const id = open!.id
    open = null
    emit({ type: 'failed', id, reason })
  }

  const feed = (f: PreviewFrame): void => {
    if (f.kind === 'header') {
      if (open) fail('incomplete') // one preview at a time per hub: a new header means the old one is dead
      const entry = PREVIEW_MIMES.find(([m]) => m === f.mime)
      if (!entry) {
        console.warn('[preview] ignored a preview of an unsupported type')
        return
      }
      if (!/^[0-9a-f]{32}$/.test(f.key)) return
      const [mime, kind] = entry
      emit({ type: 'start', id: f.id, name: cleanPreviewName(f.name), kind, mime, size: f.size, key: f.key, focus: f.focus })
      // Checked before any byte is accepted; nothing is ever preallocated from the header.
      if (f.size > PREVIEW_MAX_BYTES) emit({ type: 'failed', id: f.id, reason: 'tooLarge' })
      else open = { id: f.id, size: f.size, mime, chunks: [], received: 0, lastProgress: -Infinity }
      return
    }
    if (!open || f.id !== open.id) return
    if (f.kind === 'chunk') {
      open.received += f.payload.length
      if (open.received > PREVIEW_MAX_BYTES) return fail('tooLarge')
      if (open.received > open.size) return fail('incomplete')
      open.chunks.push(f.payload)
      const now = Date.now()
      if (now - open.lastProgress >= PROGRESS_MS || open.received === open.size) {
        open.lastProgress = now
        emit({ type: 'progress', id: open.id, received: open.received })
      }
      return
    }
    if (f.status === 2) return fail('tooLarge')
    if (f.status !== 0 || open.received !== open.size) return fail('incomplete')
    const { id, chunks, mime } = open
    open = null // the chunk array goes with it once the Blob holds the bytes
    emit({ type: 'complete', id, blob: new Blob(chunks as BlobPart[], { type: mime }) })
  }

  const discard = (): void => {
    if (open) fail('incomplete')
  }

  return { feed, discard }
}
