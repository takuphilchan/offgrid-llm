import {test, expect} from '@playwright/test';
import {pcmWav, recognitionWav, VoicePCM} from '../src/lib/voice-pcm';

test('short explicit completion preserves captured speech and pads only the silent tail for recognition', async () => {
  const samples = new Float32Array(8000).fill(.25);
  const wav = new DataView(await recognitionWav(samples, 16000).arrayBuffer());
  expect(wav.getUint32(40, true)).toBe(35200);
  for (let i = 0; i < 17600; i++) expect(wav.getInt16(44 + i * 2, true)).toBe(i < 8000 ? 8191 : 0);
  const long = new Float32Array(32000).fill(.25);
  expect(await recognitionWav(long, 16000).arrayBuffer()).toEqual(await pcmWav(long, 16000).arrayBuffer());
});

const rate = 16000;
const frame = (level: number) => new Float32Array(320).fill(level);
function frames(pcm: VoicePCM, count: number, level: number, eligible = true) {
  for (let index = 0; index < count; index++) pcm.push(frame(level), eligible);
}

test('quiet speech and short unvoiced consonant gaps trigger interruption', () => {
  for (const level of [.012, .006]) {
    let starts = 0;
    const pcm = new VoicePCM(rate, () => {starts++; return true;}, () => {});
    frames(pcm, 25, .001);
    frames(pcm, 7, level); frames(pcm, 2, .001); frames(pcm, 7, level);
    expect(starts).toBe(1);
    expect(pcm.recording).toBe(true);
    frames(pcm, 100, level);
    expect(pcm.recording).toBe(true);
  }
});

test('isolated spikes separated by silence do not accumulate into speech', () => {
  let starts = 0;
  const pcm = new VoicePCM(rate, () => {starts++; return true;}, () => {});
  frames(pcm, 100, .001);
  for (let i = 0; i < 30; i++) {frames(pcm, 3, .06); frames(pcm, 10, .001);}
  expect(starts).toBe(0);
});

test('periodic short spikes cannot accumulate beyond the onset window', () => {
  let starts = 0;
  const pcm = new VoicePCM(rate, () => {starts++; return true;}, () => {});
  frames(pcm, 50, .001);
  for (let i = 0; i < 100; i++) {frames(pcm, 1, .06); frames(pcm, 4, .001);}
  expect(starts).toBe(0);
});

test('qualified onset preserves exact pre-roll and explicit completion is single-use', async () => {
  const events: string[] = [], clips: Float32Array[] = [];
  const pcm = new VoicePCM(rate, () => {events.push('onset'); return true;}, samples => {events.push('final'); clips.push(samples);});
  frames(pcm, 10, .001); frames(pcm, 8, .08);
  expect(events).toEqual([]);
  frames(pcm, 1, .08);
  expect(events).toEqual(['onset']);
  expect(pcm.recording).toBe(true);
  expect(pcm.finish()).toBe(true); expect(pcm.finish()).toBe(false);
  expect(events).toEqual(['onset', 'final']);
  expect(clips[0].length).toBe(4800);
  expect(Array.from(clips[0].slice(0, 1920))).toEqual(Array(1920).fill(Math.fround(.001)));
  expect(Array.from(clips[0].slice(1920))).toEqual(Array(2880).fill(Math.fround(.08)));
  const wav = new DataView(await pcmWav(clips[0], rate).arrayBuffer());
  expect(wav.getUint32(40, true)).toBe(9600); expect(wav.getUint32(24, true)).toBe(rate);
});

test('short spikes, ineligible playback and refused onset never become clips', () => {
  let starts = 0, ends = 0;
  const pcm = new VoicePCM(rate, () => {starts++; return false;}, () => ends++);
  frames(pcm, 500, .002); frames(pcm, 4, .09); frames(pcm, 100, .002);
  expect(starts).toBe(0);
  frames(pcm, 200, .09, false); expect(starts).toBe(0);
  frames(pcm, 9, .09); expect(starts).toBe(1);
  expect(pcm.finish()).toBe(false); expect(ends).toBe(0);
});

test('resumed hesitation extends the next endpoint within bounded silence', () => {
  const clips: Float32Array[] = [];
  const pcm = new VoicePCM(rate, () => true, samples => clips.push(samples));
  frames(pcm, 30, .08); frames(pcm, 40, .002); // 800ms pause then speech resumes
  expect(clips).toHaveLength(0);
  frames(pcm, 15, .08); frames(pcm, 50, .002);
  expect(clips).toHaveLength(0); // longer than the initial900ms window
  frames(pcm, 40, .002); expect(clips).toHaveLength(1);
  frames(pcm, 500, .002); expect(clips).toHaveLength(1);
});

test('monitoring withdrawal retains accepted utterance; discard erases pre-roll', () => {
  const clips: Float32Array[] = [];
  const pcm = new VoicePCM(rate, () => true, samples => clips.push(samples));
  frames(pcm, 10, .08); frames(pcm, 10, .09, false);
  expect(pcm.recording).toBe(true); pcm.finish();
  expect(clips[0].length).toBe(6400);
  frames(pcm, 9, .08); pcm.reset(); expect(pcm.finish()).toBe(false);
  frames(pcm, 9, .07); pcm.finish();
  expect(clips[1].length).toBe(2880);
});

test('continuous speech is capped by PCM bytes, with no duplicate limit clip', () => {
  const clips: Float32Array[] = [], reasons: string[] = [];
  const pcm = new VoicePCM(192000, () => true, (samples, reason) => {clips.push(samples); reasons.push(reason);});
  const input = new Float32Array(3840).fill(.08);
  while (!clips.length) pcm.push(input, true);
  expect(clips[0].length).toBe(pcm.limit);
  expect(reasons).toEqual(['limit']); expect(pcm.finish()).toBe(false);
});

test('invalid rate, oversized frames and nonfinite samples reject', () => {
  expect(() => new VoicePCM(0, () => true, () => {})).toThrow();
  const pcm = new VoicePCM(rate, () => true, () => {});
  expect(() => pcm.push(new Float32Array(1000), true)).toThrow();
  expect(() => pcm.push(new Float32Array([NaN]), true)).toThrow();
});
