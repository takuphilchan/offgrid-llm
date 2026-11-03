// A bounded local level gate. It detects an utterance followed by a pause;
// no audio samples leave this observer or enter persistent state.
export class SpeechPauseGate {
  private noise = 0.006;
  private aboveSince = 0;
  private lastVoice = 0;
  private heard = false;

  sample(level: number, at: number): boolean {
    if (!Number.isFinite(level) || !Number.isFinite(at)) return false;
    const threshold = Math.max(0.018, this.noise * 3);
    if (!this.heard) {
      if (level < threshold) {
        this.noise = Math.min(0.025, this.noise * 0.98 + level * 0.02);
        this.aboveSince = 0;
      } else {
        if (!this.aboveSince) this.aboveSince = at;
        if (at - this.aboveSince >= 180) { this.heard = true; this.lastVoice = at; }
      }
      return false;
    }
    if (level >= threshold * 0.7) this.lastVoice = at;
    return at - this.lastVoice >= 1050;
  }
}

export function observeSpeechPause(stream: MediaStream, onPause: () => void): () => void {
  const AudioContextCtor = window.AudioContext ?? (window as typeof window & { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
  if (!AudioContextCtor) return () => {};
  let context: AudioContext | undefined;
  let timer: number | undefined;
  try {
    context = new AudioContextCtor();
    const source = context.createMediaStreamSource(stream);
    const analyser = context.createAnalyser();
    analyser.fftSize = 1024;
    source.connect(analyser);
    const samples = new Float32Array(analyser.fftSize);
    const gate = new SpeechPauseGate();
    timer = window.setInterval(() => {
      analyser.getFloatTimeDomainData(samples);
      let energy = 0;
      for (const sample of samples) energy += sample * sample;
      if (gate.sample(Math.sqrt(energy / samples.length), performance.now())) {
        if (timer !== undefined) window.clearInterval(timer);
        timer = undefined;
        onPause();
      }
    }, 90);
  } catch {
    // Unsupported audio graph or test media: keep manual Stop and the deadline.
  }
  return () => {
    if (timer !== undefined) window.clearInterval(timer);
    void context?.close().catch(() => {});
  };
}
