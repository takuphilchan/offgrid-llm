import {recognitionWav, VoicePCM} from './voice-pcm';
import pcmWorkletURL from './voice-pcm.worklet.js?url&no-inline';
import {microphoneConstraints} from './microphone';
import {configureMicrophoneEcho, type MicrophoneEchoMode} from './microphone-echo';

export type VoiceCaptureEvent = {sessionID: string; captureID: string; utteranceID: string};
export type VoiceCaptureCapability = {automatic: boolean; echoCancellation?: MicrophoneEchoMode; noiseSuppression?: boolean; reason?: 'processing_unavailable' | 'echo_unverified'};
export type ContinuousVoiceConfig = {
  sessionID: string;
  monitoring: boolean;
  fallbackListening?: boolean;
  playbackActive: boolean;
  onSpeechStart: (event: VoiceCaptureEvent) => boolean;
  onTranscript: (text: string, event: VoiceCaptureEvent) => Promise<boolean>;
  onSettled?: (event: VoiceCaptureEvent) => void;
  onCapability?: (capability: VoiceCaptureCapability) => void;
};
export type ContinuousCapturePhase = 'permission' | 'monitoring' | 'recording' | 'transcribing' | 'closed';
type Clip = {owner: VoiceCaptureEvent; samples: Float32Array};

// One explicit Talk lifetime. The current config can change eligibility, never
// the session identity. Only accepted final clips enter the existing batch ASR.
export class VoiceCapture {
  readonly controller = new AbortController();
  readonly captureID = crypto.randomUUID();
  private stream?: MediaStream;
  private context?: AudioContext;
  private source?: MediaStreamAudioSourceNode;
  private node?: AudioWorkletNode;
  private silent?: GainNode;
  private pcm?: VoicePCM;
  private owner?: VoiceCaptureEvent;
  private pending?: Clip;
  private processing = false;
  private expected = 0;
  private ended?: () => void;
  private automatic = false;
  private closed = false;
  unsupported = false;
  private phase: ContinuousCapturePhase = 'permission';

  constructor(readonly sessionID: string, private config: () => ContinuousVoiceConfig,
    private transcribe: (blob: Blob, signal: AbortSignal) => Promise<string>,
    private changed: (phase: ContinuousCapturePhase, level: number, seconds: number) => void,
    private failed: (reason: unknown) => void) {}

  private active() { return !this.closed && !this.controller.signal.aborted && this.config().sessionID === this.sessionID; }
  private publish(level = 0) {
    if (!this.active()) return;
    this.phase = this.pcm?.recording ? 'recording' : this.processing ? 'transcribing' : 'monitoring';
    this.changed(this.phase, level, this.pcm?.seconds ?? 0);
  }

  async start(): Promise<boolean> {
    const Context = window.AudioContext;
    if (!Context || typeof AudioWorkletNode === 'undefined' || !navigator.mediaDevices?.getUserMedia) {
      this.unsupported = true;
      this.config().onCapability?.({automatic: false, reason: 'processing_unavailable'});
      return false;
    }
    try {
      this.changed('permission', 0, 0);
      const stream = await navigator.mediaDevices.getUserMedia(microphoneConstraints());
      if (!this.active()) { stream.getTracks().forEach(track => track.stop()); return false; }
      this.stream = stream;
      const track = stream.getAudioTracks()[0];
      if (!track || track.readyState !== 'live') throw new Error('Microphone unavailable');
      this.ended = () => this.reject(new Error('Microphone disconnected or muted'));
      track.addEventListener('ended', this.ended);
      track.addEventListener('mute', this.ended);
      const settings = await configureMicrophoneEcho(track);
      if (!this.active()) { this.close(); return false; }
      if (track.readyState !== 'live') throw new Error('Microphone unavailable');
      this.automatic = settings.automatic;
      this.config().onCapability?.({...settings, reason: this.automatic ? undefined : 'echo_unverified'});
      if (!this.active()) { this.close(); return false; }
      const context = new Context(); this.context = context;
      try {
        if (!context.audioWorklet) throw new Error('AudioWorklet unavailable');
        await context.audioWorklet.addModule(pcmWorkletURL);
      } catch {
        if (this.active()) { this.unsupported = true; this.close(); this.config().onCapability?.({automatic: false, reason: 'processing_unavailable'}); }
        return false;
      }
      if (!this.active()) return false;
      this.pcm = new VoicePCM(context.sampleRate, () => this.begin(), samples => this.enqueue(samples));
      this.node = new AudioWorkletNode(context, 'offgrid-pcm', {numberOfInputs: 1, numberOfOutputs: 1, outputChannelCount: [1]});
      this.node.onprocessorerror = () => this.reject(new Error('Audio processing unavailable'));
      this.node.port.onmessage = event => {
        this.node?.port.postMessage('ack');
        if (!this.active()) return;
        try {
          const {sequence, samples} = event.data;
          if (!Number.isSafeInteger(sequence) || sequence < this.expected || !(samples instanceof Float32Array)) throw new Error('Invalid capture frame');
          if (sequence !== this.expected) {
            if (this.pcm?.recording) throw new Error('Audio capture interrupted before completion');
            this.pcm?.reset();
          }
          this.expected = sequence + 1;
          const config = this.config();
          const eligible = config.monitoring && !this.pending && (!config.playbackActive || this.automatic);
          const level = this.pcm!.push(samples, eligible);
          this.publish(level);
        } catch (reason) { this.reject(reason); }
      };
      this.source = context.createMediaStreamSource(stream);
      this.silent = context.createGain(); this.silent.gain.value = 0;
      this.source.connect(this.node); this.node.connect(this.silent); this.silent.connect(context.destination);
      await context.resume();
      if (!this.active()) return false;
      this.publish(); return true;
    } catch (reason) { if (this.active()) this.reject(reason); else this.close(); return false; }
  }

  private begin() {
    if (!this.active()) return false;
    const owner = {sessionID: this.sessionID, captureID: this.captureID, utteranceID: crypto.randomUUID()};
    if (!this.config().onSpeechStart(owner) || !this.active()) return false;
    this.owner = owner;
    return true;
  }

  private enqueue(samples: Float32Array) {
    const owner = this.owner; this.owner = undefined;
    if (!owner || !this.active()) return;
    if (this.pending) { this.reject(new Error('Voice input queue full')); return; }
    this.pending = {owner, samples};
    void this.drain();
  }

  private async drain() {
    if (this.processing || !this.active()) return;
    this.processing = true; this.publish();
    try {
      while (this.pending && this.active()) {
        const clip = this.pending; this.pending = undefined;
        try {
          const text = (await this.transcribe(recognitionWav(clip.samples, this.pcm!.sampleRate), this.controller.signal)).trim();
          if (!this.active()) return;
          if (text && !await this.config().onTranscript(text, clip.owner)) {
            this.reject(new Error('Voice input needs review before continuing')); return;
          }
        } finally { if (this.active()) this.config().onSettled?.(clip.owner); }
      }
    } catch (reason) { if (this.active()) this.reject(reason); }
    finally { this.processing = false; this.publish(); }
  }

  finish() { if (this.active()) this.pcm?.finish(); this.publish(); }
  discard() {
    const owner = this.owner; this.owner = undefined; this.pcm?.reset();
    if (owner && this.active()) this.config().onSettled?.(owner);
    this.publish();
  }
  private reject(reason: unknown) { this.close(); this.failed(reason); }
  close() {
    if (this.closed) return;
    this.closed = true; this.controller.abort();
    this.owner = undefined; this.pending = undefined; this.pcm?.reset();
    this.node?.port.postMessage('close');
    if (this.node) { this.node.port.onmessage = null; this.node.onprocessorerror = null; this.node.disconnect(); this.node.port.close(); }
    this.source?.disconnect(); this.silent?.disconnect();
    this.stream?.getTracks().forEach(track => {
      if (this.ended) { track.removeEventListener('ended', this.ended); track.removeEventListener('mute', this.ended); }
      track.stop();
    });
    void this.context?.close().catch(() => {});
    this.changed('closed', 0, 0);
  }
}
