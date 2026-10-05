import { api } from '../api/client';
import { defaultVoicePreferences, selectSpeech, type SpeechSelection, type VoicePreferences } from './voice-preferences';

export const stopSpeechEvent = 'offgrid-stop-read-aloud';

// Strip incomplete constructs too: streamed code/reasoning must never be spoken
// merely because the closing delimiter has not arrived yet.
export function spokenProse(text: string): string {
  return text.replace(/<(think|analysis)\b[^>]*>[\s\S]*?(?:<\/\1>|$)/gi, ' ')
    .replace(/```[\s\S]*?(?:```|$)/g, ' ')
    .replace(/~~~[\s\S]*?(?:~~~|$)/g, ' ')
    .replace(/`[^`]*(?:`|$)/g, ' ')
    .replace(/!?\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/!?\[[^\]]*$/, '').replace(/!?\[[^\]]*\]\([^)]*$/, '')
    .replace(/https?:\/\/\S+/g, ' ').replace(/<[^>]*(?:>|$)/g, ' ')
    .replace(/^[#>*\-]+\s*/gm, '').replace(/[*_~]/g, '').replace(/\s+/g, ' ').trim();
}

export type SpeechState = 'idle' | 'waiting' | 'preparing' | 'playing';

// One turn owns capture of text, synthesis and playback. It never owns the chat
// request: failure/backpressure stops audio, not generation or persistence.
export class ResponseSpeech {
  private controller = new AbortController();
  private player = new Audio();
  private url?: string;
  private raw = '';
  private consumed = 0;
  private busy = false;
  private finished = false;
  private timer: ReturnType<typeof setTimeout>;
  private first = true;
  private ready = false;
  private selection?: SpeechSelection;
  private pending?: Promise<{ blob?: Blob; error?: unknown }>;
  constructor(private state: (state: SpeechState) => void, private error: (message: string) => void, private preferences: VoicePreferences = defaultVoicePreferences) {
    this.timer = setTimeout(() => this.fail('Speech took too long. Audio stopped; the text response is unaffected.'), 300_000);
    this.state('waiting');
  }
  append(delta: string) {
    if (this.controller.signal.aborted || this.finished) return;
    this.raw += delta;
    if (this.raw.length > 64_000 || spokenProse(this.raw).length - this.consumed > 2400) {
      this.fail('Speech cannot keep up with this response. Audio stopped; read the text or use Read aloud afterward.'); return;
    }
    this.prepareNext(); void this.pump();
  }
  finish(committedText: string) {
    if (this.controller.signal.aborted) return;
    if (!committedText.startsWith(this.raw)) { this.fail('The saved response changed. Audio stopped; use Read aloud for the saved answer.'); return; }
    this.append(committedText.slice(this.raw.length));
    this.finished = true; this.prepareNext(); void this.pump();
  }
  stop() {
    this.controller.abort(); clearTimeout(this.timer);
    this.player.pause(); this.player.removeAttribute('src'); this.player.load();
    if (this.url) { URL.revokeObjectURL(this.url); this.url = undefined; }
    this.state('idle');
  }
  private fail(message: string) { this.stop(); this.error(message); }
  private prepareNext() {
    if (this.pending || this.controller.signal.aborted) return;
    const rest = spokenProse(this.raw).slice(this.consumed);
    const limit = this.first ? 160 : 280;
    const head = rest.slice(0, limit);
    const boundary = [...head.matchAll(/[.!?。！？](?=\s)/g)].pop();
    let cut = boundary ? boundary.index! + 1 : rest.length > limit ? head.lastIndexOf(' ') : 0;
    if (cut <= 0 && rest.length > limit) cut = limit;
    if (!cut && this.finished) cut = Math.min(limit, rest.length);
    if (!cut) return;
    const chunk = rest.slice(0, cut).trim();
    this.consumed += cut;
    if (!chunk) return;
    this.first = false;
    // Only one pending request. Settle failures immediately, including aborts
    // while the previous audio chunk is still playing.
    this.pending = (async () => {
      if (!this.ready) {
        const status = await api.audioStatus(this.controller.signal);
        this.selection = selectSpeech(status, this.preferences, 'tts');
        this.ready = true;
      }
      this.controller.signal.throwIfAborted();
      return { blob: await api.synthesizeSpeech(chunk, this.controller.signal, this.selection) };
    })().catch(error => ({ error }));
  }
  private async pump() {
    if (this.busy || this.controller.signal.aborted) return;
    this.busy = true;
    try {
      while (!this.controller.signal.aborted) {
        this.prepareNext();
        if (!this.pending) { this.state(this.finished ? 'idle' : 'waiting'); if (this.finished) clearTimeout(this.timer); break; }
        this.state('preparing');
        const result = await this.pending;
        this.pending = undefined;
        if (this.controller.signal.aborted) break;
        if (result.error) throw result.error;
        if (!result.blob) throw new Error('Speech returned no audio.');
        this.prepareNext();
        this.url = URL.createObjectURL(result.blob); this.player.src = this.url;
        await new Promise<void>((resolve, reject) => {
          const cleanup = () => { this.player.onended = null; this.player.onerror = null; this.controller.signal.removeEventListener('abort', aborted); };
          const aborted = () => { cleanup(); reject(new DOMException('Stopped', 'AbortError')); };
          this.player.onended = () => { cleanup(); resolve(); };
          this.player.onerror = () => { cleanup(); reject(new Error('Audio playback failed. Check your output device.')); };
          this.controller.signal.addEventListener('abort', aborted, { once: true });
          void this.player.play().then(() => { if (!this.controller.signal.aborted) this.state('playing'); }).catch(e => { cleanup(); reject(e); });
        });
        if (this.url) { URL.revokeObjectURL(this.url); this.url = undefined; }
      }
    } catch (reason) {
      if (!this.controller.signal.aborted) this.fail(reason instanceof Error ? reason.message : 'Speech playback failed.');
    } finally { this.busy = false; }
  }
}
