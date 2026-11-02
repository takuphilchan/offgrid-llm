import { api, type DocumentImport } from '../../api/client';

// Status polling is observation only. The explicit POST remains the sole
// operation; neither a failed poll nor a terminal observation replays it.
export async function runDocumentImport(id: string, signal: AbortSignal, progress: (state: DocumentImport['state']) => void) {
 const observation = new AbortController();
 let timer: ReturnType<typeof setTimeout> | undefined;
 const abort = () => observation.abort();
 signal.addEventListener('abort', abort, { once: true });
 if (signal.aborted) abort();
 const poll = async () => {
  try {
   const item = await api.documentImport(id, observation.signal);
   if (!observation.signal.aborted && item.id === id && ['queued', 'extracting', 'indexing'].includes(item.state)) progress(item.state);
  } catch { /* The mutation receipt and saved recovery remain authoritative. */ }
  finally { if (!observation.signal.aborted) timer = setTimeout(() => void poll(), 1500); }
 };
 timer = setTimeout(() => void poll(), 750);
 try { return await api.retryDocumentImport(id, signal); }
 finally { clearTimeout(timer); observation.abort(); signal.removeEventListener('abort', abort); }
}
