import { api } from '../api/client';
import type { LocaleCode } from '../i18n';
import { speechFeedback } from '../i18n/speech-feedback';
import { defaultVoicePreferences, selectSpeech, type SpeechSelection, type VoicePreferences } from './voice-preferences';
import { SpeechDeliveryLedger, sameSpeechSource, type SpeechChunk, type SpeechDelivery, type SpeechOwner, type SpeechSource } from './speech-delivery';

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
  private completionEmitted = false;
  private timer: ReturnType<typeof setTimeout>;
  private first = true;
  private ready = false;
  private selection?: SpeechSelection;
  private pending?: {chunk: SpeechChunk; work: Promise<{ blob?: Blob; error?: unknown }>};
  private sequence = 0;
  private ledger: SpeechDeliveryLedger;
  constructor(private state: (state: SpeechState) => void, private error: (message: string) => void, private preferences: VoicePreferences = defaultVoicePreferences, private locale: LocaleCode = 'en', private completed?: () => void, delivery?: {owner: SpeechOwner; changed: (value: SpeechDelivery) => void}) {
    this.ledger = new SpeechDeliveryLedger(delivery?.owner ?? {sessionKey: '', requestID: crypto.randomUUID()}, delivery?.changed);
    this.timer = setTimeout(() => this.fail(speechFeedback(this.locale).timeout), 300_000);
    this.state('waiting');
  }
  delivery() { return this.ledger.snapshot(); }
  bindSource(source: SpeechSource) { return this.ledger.bind(source); }
  matchesSource(source: SpeechSource) { return sameSpeechSource(this.delivery().source, source); }
  private trackText() { this.ledger.text(this.raw.length, Math.max(0, spokenProse(this.raw).length - this.consumed), this.finished); }
  append(delta: string) {
    if (this.controller.signal.aborted || this.finished) return;
    this.raw += delta;
    this.trackText();
    if (this.raw.length > 64_000 || spokenProse(this.raw).length - this.consumed > 2400) {
      this.fail(speechFeedback(this.locale).lag); return;
    }
    this.prepareNext(); void this.pump();
  }
  finish(committedText: string) {
    if (this.controller.signal.aborted || this.finished) return;
    if (!committedText.startsWith(this.raw)) { this.fail(speechFeedback(this.locale).changed); return; }
    this.append(committedText.slice(this.raw.length));
    this.finished = true; this.trackText(); this.prepareNext(); void this.pump();
  }
  stop(reason: 'interrupted' | 'failed' = 'interrupted') {
    this.controller.abort(); clearTimeout(this.timer);
    this.player.pause(); this.player.removeAttribute('src'); this.player.load();
    if (this.url) { URL.revokeObjectURL(this.url); this.url = undefined; }
    this.pending = undefined; this.ledger.stop(reason);
    this.state('idle');
    return this.delivery();
  }
  private fail(message: string) { this.stop('failed'); this.error(message); }
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
    this.trackText();
    if (!chunk) return;
    this.first = false;
    const item = {sequence: ++this.sequence, text: chunk};
    this.ledger.queue(item);
    // Only one pending request. Settle failures immediately, including aborts
    // while the previous audio chunk is still playing.
    this.pending = {chunk: item, work: (async () => {
      if (!this.ready) {
        const status = await api.audioStatus(this.controller.signal);
        this.selection = selectSpeech(status, this.preferences, 'tts', this.locale);
        this.ready = true;
      }
      this.controller.signal.throwIfAborted();
      this.ledger.synthesizing(item);
      const blob = await api.synthesizeSpeech(chunk, this.controller.signal, this.selection);
      this.controller.signal.throwIfAborted();
      this.ledger.synthesized(item);
      return {blob};
    })().catch(error => ({ error }))};
  }
  private async pump() {
    if (this.busy || this.controller.signal.aborted) return;
    this.busy = true;
    try {
      while (!this.controller.signal.aborted) {
        this.prepareNext();
        if (!this.pending) {
          this.state(this.finished ? 'idle' : 'waiting');
          if (this.finished) {
            clearTimeout(this.timer);
            this.ledger.complete();
            if (!this.completionEmitted) { this.completionEmitted = true; this.completed?.(); }
          }
          break;
        }
        this.state('preparing');
        const pending = this.pending;
        const result = await pending.work;
        this.pending = undefined;
        if (this.controller.signal.aborted) break;
        if (result.error) throw result.error;
        if (!result.blob) throw new Error(speechFeedback(this.locale).empty);
        this.url = URL.createObjectURL(result.blob); this.player.src = this.url;
        await new Promise<void>((resolve, reject) => {
          let settled = false;
          const cleanup = () => { settled = true; this.player.onended = null; this.player.onerror = null; this.controller.signal.removeEventListener('abort', aborted); };
          const aborted = () => { cleanup(); reject(new DOMException('Stopped', 'AbortError')); };
          this.player.onended = () => { cleanup(); this.ledger.ended(pending.chunk); resolve(); };
          this.player.onerror = () => { cleanup(); reject(new Error(speechFeedback(this.locale).playback)); };
          this.controller.signal.addEventListener('abort', aborted, { once: true });
          this.ledger.requestPlayback(pending.chunk);
          this.prepareNext();
          void this.player.play().then(() => { if (!settled && !this.controller.signal.aborted) {this.ledger.started(pending.chunk); this.state('playing');} }).catch(e => { if (!settled) {cleanup(); reject(e);} });
        });
        if (this.url) { URL.revokeObjectURL(this.url); this.url = undefined; }
      }
    } catch (reason) {
      if (!this.controller.signal.aborted) this.fail(reason instanceof Error ? reason.message : speechFeedback(this.locale).playback);
    } finally { this.busy = false; }
  }
}
