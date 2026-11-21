// Phase 11 (PREV-02, 11-07 fix): the Markdown parse, off the main thread. One
// message in (the source), one out (unsanitized HTML). A throw surfaces as the
// Worker's error event; parseMarkdownInWorker terminates this worker either way.
import { parseMarkdown } from './markdown'

self.onmessage = (e: MessageEvent<string>) => self.postMessage(parseMarkdown(e.data))
