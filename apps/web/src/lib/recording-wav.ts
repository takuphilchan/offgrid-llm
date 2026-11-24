import {recognitionWav} from './voice-pcm';

// Decode browser codecs at the ASR input rate, then mix to mono PCM16. A device's
// 48/96kHz stereo default must not multiply a bounded recording's upload size.
export async function recordingWav(blob: Blob): Promise<Blob> {
  const Context = window.AudioContext ?? (window as typeof window & {webkitAudioContext?: typeof AudioContext}).webkitAudioContext;
  if (!Context || blob.type.includes('wav')) return blob;
  const context = new Context({sampleRate: 16000});
  try {
    const decoded = await context.decodeAudioData(await blob.arrayBuffer());
    if (!decoded.numberOfChannels || decoded.duration > 121 || decoded.length * 2 + 44 > 12 * 1024 * 1024) throw new RangeError('audio_capture_limit');
    const mono = new Float32Array(decoded.length);
    for (let channel = 0; channel < decoded.numberOfChannels; channel++) {
      const samples = decoded.getChannelData(channel);
      for (let index = 0; index < mono.length; index++) mono[index] += samples[index] / decoded.numberOfChannels;
    }
    return recognitionWav(mono, decoded.sampleRate);
  } finally {await context.close();}
}
