export type SpeechOwner = Readonly<{sessionKey: string; requestID: string}>;
export type SpeechSource = Readonly<{kind: 'canonical' | 'session'; session: string; turnID: string; conversationID?: string; runID?: string}>;
export type SpeechChunk = Readonly<{sequence: number; text: string}>;
export type SpeechDelivery = Readonly<{
  playbackID: string; owner: SpeechOwner; source?: SpeechSource;
  state: 'active' | 'completed' | 'interrupted' | 'failed'; committed: boolean;
  generatedCharacters: number; unpreparedCharacters: number;
  scheduledChunks: number; synthesizedChunks: number;
  queued?: SpeechChunk & {state: 'queued' | 'synthesizing' | 'synthesized'};
  playing?: SpeechChunk & {state: 'playback_requested' | 'playback_started'};
  devicePlaybackCompleted: Readonly<{chunks: number; text: string}>;
  uncertainPartial?: SpeechChunk & {state: 'partial_unknown'; playbackEvidence: 'playback_requested' | 'playback_started'};
  discardedChunks: number; discardedUnpreparedCharacters: number;
}>;

export function sameSpeechSource(first: SpeechSource | undefined, second: SpeechSource): boolean {
  return !!first && first.kind === second.kind && first.session === second.session && first.turnID === second.turnID && first.conversationID === second.conversationID && (!first.runID || first.runID === second.runID);
}

// Session-local delivery accounting. Only a media ended event records a whole
// chunk as device playback completed. Neither that event nor wall-clock time
// proves human hearing, and partial audio has no inferred text alignment.
export class SpeechDeliveryLedger {
  private value: SpeechDelivery;
  constructor(owner: SpeechOwner, private changed?: (value: SpeechDelivery) => void) {
    this.value = {playbackID: crypto.randomUUID(), owner: {...owner}, state: 'active', committed: false, generatedCharacters: 0, unpreparedCharacters: 0, scheduledChunks: 0, synthesizedChunks: 0, devicePlaybackCompleted: {chunks: 0, text: ''}, discardedChunks: 0, discardedUnpreparedCharacters: 0};
  }
  snapshot(): SpeechDelivery {
    const value = this.value;
    return Object.freeze({...value, owner: Object.freeze({...value.owner}), source: value.source && Object.freeze({...value.source}), queued: value.queued && Object.freeze({...value.queued}), playing: value.playing && Object.freeze({...value.playing}), uncertainPartial: value.uncertainPartial && Object.freeze({...value.uncertainPartial}), devicePlaybackCompleted: Object.freeze({...value.devicePlaybackCompleted})});
  }
  private publish() { this.changed?.(this.snapshot()); }
  bind(source: SpeechSource): boolean {
    if (this.value.state !== 'active' || !source.session || !source.turnID || source.kind === 'canonical' && !source.conversationID || this.value.source && !sameSpeechSource(this.value.source, source)) return false;
    this.value = {...this.value, source: {...source}}; this.publish(); return true;
  }
  text(generatedCharacters: number, unpreparedCharacters: number, committed: boolean) {
    if (this.value.state !== 'active') return;
    this.value = {...this.value, generatedCharacters, unpreparedCharacters, committed}; this.publish();
  }
  queue(chunk: SpeechChunk) {
    if (this.value.state !== 'active') return;
    this.value = {...this.value, scheduledChunks: this.value.scheduledChunks + 1, queued: {...chunk, state: 'queued'}}; this.publish();
  }
  synthesizing(chunk: SpeechChunk) {
    if (this.value.state !== 'active' || this.value.queued?.sequence !== chunk.sequence) return;
    this.value = {...this.value, queued: {...chunk, state: 'synthesizing'}}; this.publish();
  }
  synthesized(chunk: SpeechChunk) {
    if (this.value.state !== 'active' || this.value.queued?.sequence !== chunk.sequence) return;
    this.value = {...this.value, synthesizedChunks: this.value.synthesizedChunks + 1, queued: {...chunk, state: 'synthesized'}}; this.publish();
  }
  requestPlayback(chunk: SpeechChunk) {
    if (this.value.state !== 'active') return;
    this.value = {...this.value, queued: undefined, playing: {...chunk, state: 'playback_requested'}}; this.publish();
  }
  started(chunk: SpeechChunk) {
    if (this.value.state !== 'active' || this.value.playing?.sequence !== chunk.sequence) return;
    this.value = {...this.value, playing: {...chunk, state: 'playback_started'}}; this.publish();
  }
  ended(chunk: SpeechChunk) {
    if (this.value.state !== 'active' || this.value.playing?.sequence !== chunk.sequence) return;
    const completed = this.value.devicePlaybackCompleted;
    this.value = {...this.value, playing: undefined, devicePlaybackCompleted: {chunks: completed.chunks + 1, text: completed.text ? `${completed.text} ${chunk.text}` : chunk.text}}; this.publish();
  }
  complete() {
    if (this.value.state !== 'active' || this.value.queued || this.value.playing || !this.value.committed) return;
    this.value = {...this.value, state: 'completed'}; this.publish();
  }
  stop(state: 'interrupted' | 'failed') {
    if (this.value.state !== 'active') return;
    this.value = {...this.value, state, uncertainPartial: this.value.playing && {...this.value.playing, state: 'partial_unknown', playbackEvidence: this.value.playing.state}, playing: undefined, discardedChunks: this.value.queued ? 1 : 0, queued: undefined, discardedUnpreparedCharacters: this.value.unpreparedCharacters, unpreparedCharacters: 0}; this.publish();
  }
}
