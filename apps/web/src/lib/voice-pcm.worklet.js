// Bundled locally; no model, network or microphone permission acquisition here.
class OffGridPCM extends AudioWorkletProcessor {
  constructor() {
    super();
    this.frame = new Float32Array(Math.round(sampleRate * .02));
    this.at = 0; this.sequence = 0; this.outstanding = 0; this.closed = false;
    this.port.onmessage = event => {
      if (event.data === 'ack') this.outstanding = Math.max(0, this.outstanding - 1);
      if (event.data === 'close') this.closed = true;
    };
  }
  process(inputs) {
    if (this.closed) return false;
    const channels = inputs[0];
    if (!channels?.length) return true;
    for (let i = 0; i < channels[0].length; i++) {
      let sample = 0;
      for (const channel of channels) sample += channel[i];
      this.frame[this.at++] = sample / channels.length;
      if (this.at !== this.frame.length) continue;
      const sequence = this.sequence++;
      // Bound the port queue while the renderer is busy. A sequence gap is
      // explicit and invalidates a partial utterance rather than hiding loss.
      if (this.outstanding < 4) {
        this.port.postMessage({sequence, samples: this.frame}, [this.frame.buffer]);
        this.outstanding++;
        this.frame = new Float32Array(Math.round(sampleRate * .02));
      }
      this.at = 0;
    }
    return true;
  }
}
registerProcessor('offgrid-pcm', OffGridPCM);
