// Build-time only. End users never install Node/npm or run a terminal.
const fs=require('node:fs/promises');
const path=require('node:path');
const {spawn}=require('node:child_process');
const {createManifest}=require('../computer/pack.cjs');

module.exports=async function prepareComputer(context) {
  const platform=context.electronPlatformName;
  const arch=require('builder-util').Arch[context.arch];
  const host={win32:{x64:'win64'},darwin:{x64:'mac15',arm64:'mac15-arm64'},linux:{x64:'ubuntu24.04-x64'}}[platform]?.[arch];
  if (!host) throw Error(`Computer browser package is not configured for ${platform}/${arch}`);
  const source=path.resolve(__dirname,'../computer');
  const version=require(path.join(source,'package.json')).dependencies.playwright;
  if (version!=='1.62.1' || require(path.join(source,'node_modules/playwright/package.json')).version!==version) throw Error('Install the locked computer build dependencies with npm ci --prefix computer --ignore-scripts');
  const root=path.resolve(__dirname,'../build/computer-runtime');
  const target=path.join(root,`${{win32:'win',darwin:'mac',linux:'linux'}[platform]}-${arch}`);
  if (!target.startsWith(root+path.sep) || target===root) throw Error('Invalid build target');
  await fs.mkdir(target,{recursive:true});
  for (const file of ['managed.cjs','session.mjs','journal.mjs','network.mjs','native-worker.cjs','browser.mjs','demo.mjs','pack.cjs','package.json']) await fs.copyFile(path.join(source,file),path.join(target,file));
  // Ship the real host worker with its pack digest, but do not advertise a
  // native driver until the coordinated service/UI and platform gates pass.
  // Cross-compilation is build-time only; users never install a Go toolchain.
  if(platform==='win32' && arch==='x64') {
    await fs.mkdir(path.join(target,'native'),{recursive:true});
    await new Promise((resolve,reject)=>{
      const child=spawn('go',['build','-trimpath','-ldflags=-H=windowsgui','-o',path.join(target,'native','offgrid-computer.exe'),'./cmd/offgrid-computer'],{cwd:path.resolve(__dirname,'..'),stdio:'inherit',shell:false,env:{...process.env,GOOS:'windows',GOARCH:'amd64',CGO_ENABLED:'0'}});
      child.once('error',reject);child.once('exit',code=>code===0?resolve():reject(Error('Native computer worker packaging failed')));
    });
  }
  if(platform==='darwin') {
    if(process.platform!=='darwin') throw Error('macOS native components require the macOS SDK on a Mac build runner');
    const native=path.join(target,'native');
    const bundle=path.join(native,'OffGrid Computer Controls.app','Contents');
    await fs.mkdir(path.join(bundle,'MacOS'),{recursive:true});
    const run=(program,args,env=process.env)=>new Promise((resolve,reject)=>{
      const child=spawn(program,args,{cwd:path.resolve(__dirname,'..'),stdio:'inherit',shell:false,env});
      child.once('error',reject);child.once('exit',code=>code===0?resolve():reject(Error('macOS native computer component build failed')));
    });
    await run('go',['build','-trimpath','-o',path.join(native,'offgrid-computer'),'./cmd/offgrid-computer'],{...process.env,GOOS:'darwin',GOARCH:arch==='x64'?'amd64':'arm64',CGO_ENABLED:'1',MACOSX_DEPLOYMENT_TARGET:'15.0'});
    await run('xcrun',['clang','-fobjc-arc','-arch',arch==='x64'?'x86_64':'arm64','-mmacosx-version-min=15.0','-framework','AppKit','-framework','Carbon',path.join(source,'native','macos','consent.m'),'-o',path.join(bundle,'MacOS','offgrid-computer-ui')]);
    await fs.copyFile(path.join(source,'native','macos','Info.plist'),path.join(bundle,'Info.plist'));
  }
  if(platform==='linux') {
    if(process.platform!=='linux'||arch!=='x64') throw Error('Linux native components require an x64 Linux build runner');
    const native=path.join(target,'native');await fs.mkdir(native,{recursive:true});
    const {execFile}=require('node:child_process');
    const run=(program,args,env=process.env)=>new Promise((resolve,reject)=>{
      const child=spawn(program,args,{cwd:path.resolve(__dirname,'..'),stdio:'inherit',shell:false,env});child.once('error',reject);child.once('exit',code=>code===0?resolve():reject(Error('Linux native computer component build failed')));
    });
    const flags=await new Promise((resolve,reject)=>execFile('pkg-config',['--cflags','--libs','gtk+-3.0','json-glib-1.0'],(err,stdout)=>err?reject(Error('Install Linux build dependencies: libatspi2.0-dev libgtk-3-dev libjson-glib-dev')):resolve(stdout.trim().split(/\s+/))));
    await run('go',['build','-trimpath','-tags','offgrid_native','-o',path.join(native,'offgrid-computer'),'./cmd/offgrid-computer'],{...process.env,GOOS:'linux',GOARCH:'amd64',CGO_ENABLED:'1'});
    await run('cc',[path.join(source,'native','linux','consent.c'),'-o',path.join(native,'offgrid-computer-ui'),...flags]);
  }
  for (const module of ['playwright','playwright-core']) await fs.cp(path.join(source,'node_modules',module),path.join(target,'node_modules',module),{recursive:true});
  await new Promise((resolve,reject)=>{
    const env={...process.env,PLAYWRIGHT_BROWSERS_PATH:path.join(target,'browsers')};
    delete env.PLAYWRIGHT_HOST_PLATFORM_OVERRIDE;
    if(platform!==process.platform || arch!==process.arch) env.PLAYWRIGHT_HOST_PLATFORM_OVERRIDE=host;
    const child=spawn(process.execPath,[path.join(source,'node_modules/playwright/cli.js'),'install','chromium','--no-shell'],{stdio:'inherit',env});
    child.once('error',reject);child.once('exit',code=>code===0?resolve():reject(Error('Pinned browser packaging failed')));
  });
  await createManifest(target,platform,arch,version);
};
