import {expect,test,type Page} from '@playwright/test';
import {configureMicrophoneEcho} from '../src/lib/microphone-echo';

for(const [mode,noise,automatic] of [[true,false,true],[true,undefined,true],['all',false,true],[false,true,false],['remote-only',true,false],[undefined,true,false],['unexpected',true,false]] as const){
  test(`applied AEC ${String(mode)} with NS ${String(noise)} qualifies ${automatic}`,async()=>{
    const track={readyState:'live',getSettings:()=>({echoCancellation:mode,noiseSuppression:noise})} as unknown as MediaStreamTrack;
    expect((await configureMicrophoneEcho(track)).automatic).toBe(automatic);
  });
}

for(const fails of [false,true])test(`all-mode upgrade ${fails?'rejects while preserving true':'preserves the other constraints'}`,async()=>{
  let mode:unknown=true, applied:unknown;
  const constraints={noiseSuppression:{ideal:false},deviceId:{exact:'isolated-device'}};
  const track={readyState:'live',getCapabilities:()=>({echoCancellation:[true,false,'all']}),getConstraints:()=>constraints,
    applyConstraints:async(value:unknown)=>{applied=value;if(fails)throw new Error('Unsupported device configuration');mode='all';},
    getSettings:()=>({echoCancellation:mode,noiseSuppression:false})} as unknown as MediaStreamTrack;
  expect(await configureMicrophoneEcho(track)).toEqual({automatic:true,echoCancellation:fails?true:'all',noiseSuppression:false});
  expect(applied).toEqual({...constraints,echoCancellation:{exact:'all'}});
});

test('failed upgrade cannot qualify a disabled device or a track that ended',async()=>{
  const track={readyState:'live',getCapabilities:()=>({echoCancellation:[true,false,'all']}),getConstraints:()=>({}),
    applyConstraints:async()=>{throw new Error('Unsupported');},getSettings:()=>({echoCancellation:false})} as unknown as MediaStreamTrack;
  expect((await configureMicrophoneEcho(track)).automatic).toBe(false);
  Object.assign(track,{readyState:'ended',getSettings:()=>({echoCancellation:true})});
  expect((await configureMicrophoneEcho(track)).automatic).toBe(false);
});

async function captureFixture(page:Page,mode:unknown,noise:boolean|undefined,delayed=false){
  test.skip(!!process.env.OFFGRID_E2E_URL,'Source capture harness requires isolated Vite; no installed service or microphone.');
  await page.route('**/ui/echo-capture.html',route=>route.fulfill({contentType:'text/html',body:'<html><body><p>Isolated synthetic capture</p></body></html>'}));
  await page.goto('/ui/echo-capture.html');
  await page.evaluate(async({mode,noise,delayed})=>{
    const f=window as any;
    // Real browser graph and AudioWorklet, supplied exclusively by an oscillator.
    const context=new AudioContext(),source=context.createOscillator(),gain=context.createGain(),output=context.createMediaStreamDestination();
    gain.gain.value=0;source.connect(gain);gain.connect(output);source.start();await context.resume();
    const track=output.stream.getAudioTracks()[0],stop=track.stop.bind(track);
    f.echoTest={context,gain,starts:0,stops:0,capabilities:[],failures:[],phases:[],session:'owned-session'};
    const state=f.echoTest;
    track.getSettings=()=>({echoCancellation:mode,noiseSuppression:noise}) as MediaTrackSettings;
    track.getCapabilities=()=>({echoCancellation:delayed?[true,false,'all']:[]}) as MediaTrackCapabilities;
    track.applyConstraints=()=>new Promise<void>(resolve=>{state.resolveConfiguration=resolve;});
    track.stop=()=>{state.stops++;stop();source.stop();void context.close();};
    Object.defineProperty(navigator,'mediaDevices',{configurable:true,value:{getUserMedia:async()=>output.stream}});
    const {VoiceCapture}=await import('/ui/src/lib/voice-capture.ts');
    state.capture=new VoiceCapture('owned-session',()=>({sessionID:state.session,monitoring:true,playbackActive:true,
      onSpeechStart:()=>{state.starts++;return true;},onTranscript:async()=>true,onCapability:(value:unknown)=>state.capabilities.push(value)}),
      async()=>'',(phase:string)=>state.phases.push(phase),(reason:Error)=>state.failures.push(reason.message));
    state.start=state.capture.start();
  },{mode,noise,delayed});
}

for(const [mode,noise,automatic] of [[true,false,true],['all',undefined,true],[false,true,false],['remote-only',true,false]] as const){
  test(`real AudioWorklet during playback with applied ${mode} and NS ${String(noise)}`,async({page})=>{
    await captureFixture(page,mode,noise);
    expect(await page.evaluate(()=>(window as any).echoTest.start)).toBe(true);
    expect(await page.evaluate(()=>(window as any).echoTest.capabilities[0].automatic)).toBe(automatic);
    const time=await page.evaluate(()=>{const f=(window as any).echoTest;f.gain.gain.value=.15;return f.context.currentTime;});
    await expect.poll(()=>page.evaluate(()=>(window as any).echoTest.context.currentTime)).toBeGreaterThan(time+.6);
    expect(await page.evaluate(()=>(window as any).echoTest.starts)).toBe(automatic?1:0);
    await page.evaluate(()=>(window as any).echoTest.capture.close());
    await expect.poll(()=>page.evaluate(()=>(window as any).echoTest.stops)).toBe(1);
    expect(await page.evaluate(()=>(window as any).echoTest.failures)).toEqual([]);
  });
}

for(const changedSession of [false,true])test(`${changedSession?'changed session':'Stop'} during echo configuration releases capture without late capability publication`,async({page})=>{
  await captureFixture(page,true,false,true);
  await expect.poll(()=>page.evaluate(()=>typeof (window as any).echoTest.resolveConfiguration)).toBe('function');
  await page.evaluate(changedSession=>{const f=(window as any).echoTest;if(changedSession)f.session='another-session';else f.capture.close();},changedSession);
  if(!changedSession)expect(await page.evaluate(()=>(window as any).echoTest.stops)).toBe(1);
  await page.evaluate(()=>(window as any).echoTest.resolveConfiguration());
  expect(await page.evaluate(()=>(window as any).echoTest.start)).toBe(false);
  expect(await page.evaluate(()=>(window as any).echoTest.stops)).toBe(1);
  expect(await page.evaluate(()=>(window as any).echoTest.capabilities)).toEqual([]);
  expect(await page.evaluate(()=>(window as any).echoTest.phases)).not.toContain('monitoring');
});
