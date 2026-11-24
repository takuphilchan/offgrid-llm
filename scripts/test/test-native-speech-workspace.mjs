// Opt-in real engines + existing model packages, in fresh private state only.
// Downloads nothing, does not record a microphone, and never updates an install.
import {mkdir, readFile, writeFile, cp, copyFile} from 'node:fs/promises';
import {createWriteStream} from 'node:fs';
import {resolve, join, basename} from 'node:path';
import {spawn} from 'node:child_process';
import {createRequire} from 'node:module';
import {createHash} from 'node:crypto';
import {setTimeout as delay} from 'node:timers/promises';
import {privateFixtureDirectory} from './private-fixture-directory.mjs';

const root = resolve(import.meta.dirname, '../..');
const require = createRequire(import.meta.url);
const {verify, files} = require('../../apps/desktop/speech-pack.cjs');
const {availablePort} = require('../../apps/desktop/runtime.js');
const [binary, runtime, asr, tts, recording] = process.argv.slice(2);
if (process.argv.length !== 7) throw Error('Usage: node scripts/test/test-native-speech-workspace.mjs <built-service> <speech-runtime> <ASR-package-revision-dir> <TTS-package-revision-dir> <known-WAV>');
const digest = bytes => createHash('sha256').update(bytes).digest('hex');
const pack = await verify(resolve(runtime), process.platform, process.arch);
const evidence = await privateFixtureDirectory();
console.log(`Isolated native speech qualification: ${evidence}`);
for (const dir of ['bin', 'data', 'models/.packages']) await mkdir(join(evidence, dir), {recursive: true});
const executable = join(evidence, 'bin', process.platform === 'win32' ? 'offgrid.exe' : 'offgrid');
await copyFile(resolve(binary), executable);
await cp(resolve(runtime), join(evidence, 'speech-runtime'), {recursive: true, force: false, errorOnExist: true});
await verify(join(evidence, 'speech-runtime'), process.platform, process.arch);
const models = [];
for (const [directory, architecture] of [[asr, 'whisper'], [tts, 'piper']]) {
  const source = resolve(directory);
  const manifest = JSON.parse(await readFile(join(source, 'manifest.json'), 'utf8'));
  if (manifest.architecture !== architecture || !/^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$/.test(manifest.id) || !/^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$/.test(manifest.revision)) throw Error(`Expected an existing ${architecture} package revision`);
  const before = await files(source); // Reject links and record every copied byte.
  const target = join(evidence, 'models/.packages', manifest.id, manifest.revision);
  await cp(source, target, {recursive: true, force: false, errorOnExist: true});
  if (JSON.stringify(before) !== JSON.stringify(await files(target)) || JSON.stringify(before) !== JSON.stringify(await files(source))) throw Error('Model source changed during fixture copy');
  models.push({id: manifest.id, revision: manifest.revision, architecture, files: before});
}
const wav = await readFile(resolve(recording));
if (wav.length < 44 || wav.length > 12 * 1024 * 1024 || wav.toString('ascii', 0, 4) !== 'RIFF' || wav.toString('ascii', 8, 12) !== 'WAVE') throw Error('Supply an existing bounded known WAV recording');
await writeFile(join(evidence, 'known.wav'), wav);
await writeFile(join(evidence, 'config.yaml'), '{}');
const provenance = {service_sha256: digest(await readFile(executable)), runtime: pack, models, recording: {name: basename(recording), bytes: wav.length, sha256: digest(wav)}};
await writeFile(join(evidence, 'source.json'), JSON.stringify(provenance, null, 2));
const port = await availablePort(), url = `http://127.0.0.1:${port}`;
const env = Object.fromEntries(Object.entries(process.env).filter(([key]) => !key.startsWith('OFFGRID_')));
Object.assign(env, {OFFGRID_CONFIG: join(evidence, 'config.yaml'), OFFGRID_DATA_DIR: join(evidence, 'data'), OFFGRID_MODELS_DIR: join(evidence, 'models'), OFFGRID_BIN_DIR: join(evidence, 'bin'), OFFGRID_HOST: '127.0.0.1', OFFGRID_PORT: String(port), OFFGRID_REQUIRE_AUTH: 'false', OFFGRID_ENABLE_P2P: 'false', OFFGRID_UI_DIR: join(root, 'apps/web/dist')});
const log = createWriteStream(join(evidence, 'service.log'));
const service = spawn(executable, ['serve'], {cwd: evidence, env, windowsHide: true, stdio: ['ignore', 'pipe', 'pipe']});
service.stdout.pipe(log, {end: false}); service.stderr.pipe(log, {end: false});
let spawnError;
service.once('error', error => {spawnError = error;});
const closed = new Promise(resolve => service.once('close', resolve));
try {
  let status;
  for (let i = 0; i < 100; i++) {
    if (spawnError) throw spawnError;
    if (service.exitCode !== null) throw Error('Isolated service exited; inspect service.log');
    try { const reply = await fetch(`${url}/v1/audio/status`, {signal: AbortSignal.timeout(500)}); if (reply.ok) {status = await reply.json(); break;} } catch { /* Bounded startup wait. */ }
    await delay(100);
  }
  if (!status) throw Error('Isolated speech service did not become ready');
  for (const model of models) {
    if (!status.profiles?.some(profile => profile.id === model.id && profile.revision === model.revision && profile.available)) throw Error(`Copied speech profile unavailable: ${model.id}`);
  }
  // Do not install/resolve anything if readiness is missing. No download fallback.
  await writeFile(join(evidence, 'status.json'), JSON.stringify(status, null, 2));
  const asrModel = models[0], ttsModel = models[1], form = new FormData();
  form.set('file', new Blob([wav], {type: 'audio/wav'}), 'known.wav'); form.set('model', `${asrModel.id}@${asrModel.revision}`);
  const started = performance.now();
  const recognized = await fetch(`${url}/v1/audio/transcriptions`, {method: 'POST', body: form, signal: AbortSignal.timeout(120_000)});
  const transcript = await recognized.json(), asrMS = performance.now() - started;
  if (!recognized.ok || !/your country/i.test(transcript.text ?? '')) throw Error('Known JFK transcript did not match; inspect retained state');
  const synthesisStart = performance.now();
  const response = await fetch(`${url}/v1/audio/speech`, {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({model: `${ttsModel.id}@${ttsModel.revision}`, input: 'Welcome to OffGrid. This is an isolated speech qualification.', response_format: 'wav'}), signal: AbortSignal.timeout(120_000)});
  const output = Buffer.from(await response.arrayBuffer()), ttsMS = performance.now() - synthesisStart;
  if (!response.ok || output.toString('ascii', 0, 4) !== 'RIFF' || output.toString('ascii', 8, 12) !== 'WAVE' || output.length < 10000) throw Error('Speech response is not the required WAV');
  await writeFile(join(evidence, 'speech.wav'), output);
  const helloResponse = await fetch(`${url}/v1/audio/speech`, {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({model:`${ttsModel.id}@${ttsModel.revision}`, input:'Hello.', response_format:'wav'}), signal:AbortSignal.timeout(120_000)});
  const hello = Buffer.from(await helloResponse.arrayBuffer());
  if (!helloResponse.ok || hello.toString('ascii', 0, 4) !== 'RIFF') throw Error('Short speech fixture synthesis failed');
  let format, data;
  for (let at = 12; at + 8 <= hello.length;) {
    const size = hello.readUInt32LE(at + 4), kind = hello.toString('ascii', at, at + 4);
    if (kind === 'fmt ') format = hello.subarray(at + 8, at + 8 + size);
    if (kind === 'data') data = hello.subarray(at + 8, at + 8 + size);
    at += 8 + size + (size % 2);
  }
  if (!format || !data || format.readUInt16LE(0) !== 1 || format.readUInt16LE(2) !== 1 || format.readUInt16LE(14) !== 16) throw Error('Expected mono PCM16 short fixture');
  const rate = format.readUInt32LE(4), margin = Math.ceil(rate * .04);
  let first = 0, last = data.length / 2 - 1;
  while (first < last && Math.abs(data.readInt16LE(first * 2)) < 350) first++;
  while (last > first && Math.abs(data.readInt16LE(last * 2)) < 350) last--;
  const clip = data.subarray(Math.max(0, first - margin) * 2, Math.min(data.length / 2, last + margin + 1) * 2);
  if (clip.length / 2 / rate >= .9) throw Error('Short fixture must exercise Whisper subsecond rejection');
  const short = Buffer.alloc(44 + clip.length);
  short.write('RIFF'); short.writeUInt32LE(short.length - 8, 4); short.write('WAVEfmt ', 8);
  short.writeUInt32LE(16, 16); format.copy(short, 20, 0, 16); short.write('data', 36); short.writeUInt32LE(clip.length, 40); clip.copy(short, 44);
  await writeFile(join(evidence, 'short.wav'), short);
  const shortForm = new FormData(); shortForm.set('file', new Blob([short], {type:'audio/wav'}), 'short.wav'); shortForm.set('model', `${asrModel.id}@${asrModel.revision}`);
  const shortReply = await fetch(`${url}/v1/audio/transcriptions`, {method:'POST', body:shortForm, signal:AbortSignal.timeout(120_000)});
  const unpadded = await shortReply.json();
  if (!shortReply.ok || unpadded.text?.trim()) throw Error('Expected pinned Whisper short-input negative control');
  await writeFile(join(evidence, 'short-negative-control.json'), JSON.stringify({seconds:clip.length / 2 / rate, unpadded}, null, 2));
  await writeFile(join(evidence, 'result.json'), JSON.stringify({transcript, asr_ms: asrMS, tts_ms: ttsMS, wav_bytes: output.length, scope: 'Known recording and native synthesis; no microphone, acoustic, interruption or human qualification'}, null, 2));
  const browser = spawn(process.execPath, [require.resolve('../../apps/web/node_modules/@playwright/test/cli'), 'test', 'tests/voice-native.spec.ts', 'tests/speech-packages.spec.ts', '--workers=1', `--output=${join(evidence, 'browser-results')}`], {cwd: join(root, 'apps/web'), env: {...env, OFFGRID_E2E_URL: url, OFFGRID_E2E_SPEECH_FIXTURE: join(evidence, 'known.wav'), OFFGRID_E2E_SHORT_SPEECH_FIXTURE: join(evidence, 'short.wav')}, windowsHide: true, stdio: 'inherit'});
  const code = await new Promise((resolve, reject) => {browser.once('error', reject); browser.once('close', resolve);});
  if (code !== 0) throw Error(`Native speech browser checks failed (${code})`);
  console.log(`PASS native ASR ${asrMS.toFixed(1)} ms, TTS ${ttsMS.toFixed(1)} ms; evidence ${evidence}`);
} finally {
  if (service.exitCode === null && !spawnError) {
    service.kill('SIGTERM');
    const deadline = setTimeout(() => service.kill('SIGKILL'), 5000); deadline.unref();
    await closed; clearTimeout(deadline);
  } else await closed;
  await new Promise(resolve => log.end(resolve));
}
