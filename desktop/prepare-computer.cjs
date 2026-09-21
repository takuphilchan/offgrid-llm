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
  for (const file of ['managed.cjs','session.mjs','journal.mjs','network.mjs','browser.mjs','demo.mjs','pack.cjs','package.json']) await fs.copyFile(path.join(source,file),path.join(target,file));
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
