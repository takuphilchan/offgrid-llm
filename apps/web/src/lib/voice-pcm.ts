// Audio-only endpointing. This is not a semantic turn detector or a VAD model.
export class VoicePCM {
  private clock = 0;
  private noise = .003;
  private candidate = 0;
  private candidateGap = 0;
  private onsetWindow: {duration: number; voiced: boolean}[] = [];
  private onsetDuration = 0;
  private voiced = 0;
  private lastVoice = 0;
  private longestPause = 0;
  private ring: Float32Array;
  private ringAt = 0;
  private ringSize = 0;
  private chunks: Float32Array[] | null = null;
  private length = 0;
  readonly limit: number;

  constructor(readonly sampleRate: number, private started: () => boolean,
    private finished: (samples: Float32Array, reason: 'endpoint' | 'explicit' | 'limit') => void) {
    if (!Number.isFinite(sampleRate) || sampleRate < 8000 || sampleRate > 192000) throw new Error('Unsupported capture sample rate');
    this.ring = new Float32Array(Math.ceil(sampleRate * .3));
    this.limit = Math.min(Math.floor(sampleRate * 120), Math.floor((12 * 1024 * 1024 - 44) / 2));
  }

  get recording() { return this.chunks !== null; }
  get seconds() { return this.length / this.sampleRate; }

  push(frame: Float32Array, eligible: boolean): number {
    if (!frame.length || frame.length > Math.ceil(this.sampleRate * .05)) throw new Error('Invalid capture frame');
    let energy = 0;
    for (const sample of frame) {
      if (!Number.isFinite(sample)) throw new Error('Invalid capture sample');
      energy += sample * sample;
    }
    const level = Math.sqrt(energy / frame.length);
    const duration = frame.length / this.sampleRate;
    this.clock += duration;
    if (!this.chunks) {
      if (!eligible) { this.reset(); return level; }
      for (const sample of frame) {
        this.ring[this.ringAt] = sample;
        this.ringAt = (this.ringAt + 1) % this.ring.length;
        this.ringSize = Math.min(this.ring.length, this.ringSize + 1);
      }
      const voiced = level >= Math.max(.004, this.noise * 3);
      this.onsetWindow.push({duration, voiced}); this.onsetDuration += duration;
      if (voiced) {
        this.candidate += duration; this.candidateGap = 0;
      } else {
        // Consonants and brief pauses do not erase the preceding voiced frames.
        // Separate clicks/spikes still expire; the gap contributes no speech.
        this.candidateGap += duration;
        if (this.candidateGap > .08 + 1e-8) {
          this.candidate = 0; this.onsetWindow = []; this.onsetDuration = 0;
        }
        if (!this.candidate) {
          const weight = level < this.noise ? .15 : .02;
          this.noise = Math.min(.025, this.noise * (1 - weight) + level * weight);
        }
      }
      // Require recent voiced evidence, rather than accumulating periodic noise
      // forever. Match the retained pre-roll so onset never omits its evidence.
      while (this.onsetDuration > .3 + 1e-8 && this.onsetWindow.length) {
        const oldest = this.onsetWindow[0], trim = Math.min(oldest.duration, this.onsetDuration - .3);
        if (oldest.voiced) this.candidate = Math.max(0, this.candidate - trim);
        oldest.duration -= trim; this.onsetDuration -= trim;
        if (oldest.duration < 1e-8) this.onsetWindow.shift();
      }
      if (this.candidate + 1e-8 < .18) return level;
      // The callback cancels owned output synchronously, before any ASR work.
      if (!this.started()) { this.reset(); return level; }
      const preRoll = new Float32Array(this.ringSize);
      for (let i = 0; i < preRoll.length; i++) preRoll[i] = this.ring[(this.ringAt - this.ringSize + i + this.ring.length) % this.ring.length];
      this.chunks = [preRoll]; this.length = preRoll.length;
      this.voiced = this.candidate; this.lastVoice = this.clock;
      return level;
    }
    const remaining = this.limit - this.length;
    const selected = frame.slice(0, remaining);
    this.chunks.push(selected); this.length += selected.length;
    if (level >= Math.max(.0025, this.noise * 2.1)) {
      this.longestPause = Math.max(this.longestPause, this.clock - duration - this.lastVoice);
      this.lastVoice = this.clock; this.voiced += duration;
    }
    // A resumed hesitation extends the next pause window, within fixed bounds.
    // Short utterances receive extra time; no language completeness is inferred.
    const silence = Math.min(1.8, Math.max(this.voiced < .4 ? 1.4 : .9, this.longestPause * 1.5 + .3));
    if (this.length >= this.limit) this.finish('limit');
    else if (this.clock - this.lastVoice >= silence) this.finish('endpoint');
    return level;
  }

  finish(reason: 'endpoint' | 'explicit' | 'limit' = 'explicit') {
    if (!this.chunks) return false;
    const samples = new Float32Array(this.length);
    let at = 0;
    for (const chunk of this.chunks) { samples.set(chunk, at); at += chunk.length; }
    this.reset();
    this.finished(samples, reason);
    return true;
  }

  reset() {
    this.chunks = null; this.length = 0; this.candidate = 0; this.candidateGap = 0; this.voiced = 0;
    this.onsetWindow = []; this.onsetDuration = 0;
    this.longestPause = 0; this.ringSize = 0; this.ringAt = 0;
    this.ring.fill(0);
  }
}

export function pcmWav(samples: Float32Array, sampleRate: number): Blob {
  const bytes = new ArrayBuffer(44 + samples.length * 2), view = new DataView(bytes);
  const word = (offset: number, value: string) => [...value].forEach((char, i) => view.setUint8(offset + i, char.charCodeAt(0)));
  word(0, 'RIFF'); view.setUint32(4, bytes.byteLength - 8, true); word(8, 'WAVE'); word(12, 'fmt ');
  view.setUint32(16, 16, true); view.setUint16(20, 1, true); view.setUint16(22, 1, true);
  view.setUint32(24, sampleRate, true); view.setUint32(28, sampleRate * 2, true);
  view.setUint16(32, 2, true); view.setUint16(34, 16, true); word(36, 'data'); view.setUint32(40, samples.length * 2, true);
  samples.forEach((value, index) => { const sample = Math.max(-1, Math.min(1, value)); view.setInt16(44 + index * 2, sample < 0 ? sample * 32768 : sample * 32767, true); });
  return new Blob([bytes], {type: 'audio/wav'});
}

export function recognitionWav(samples: Float32Array, sampleRate: number): Blob {
  // The pinned Whisper runtime returns success with no segments below 1s.
  // Preserve every captured sample and pad only the tail; Stop stays immediate.
  const minimum = Math.ceil(sampleRate * 1.1);
  if (samples.length && samples.length < minimum) {
    const padded = new Float32Array(minimum); padded.set(samples);
    return pcmWav(padded, sampleRate);
  }
  return pcmWav(samples, sampleRate);
}
