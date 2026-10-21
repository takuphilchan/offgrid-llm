const fs = require('node:fs/promises');
const {createReadStream} = require('node:fs');
const path = require('node:path');
const {createHash} = require('node:crypto');

// Current build payload, shared with the desktop package hook. Older verified
// packs keep their own complete payload; this is not a new runtime requirement
// retroactively imposed on legacy manifests.
const runtimeFiles=Object.freeze(['managed.cjs','native-managed.cjs','session.mjs','native-session.mjs','desktop-session.mjs','document-session.mjs','native-features.cjs','approval-policy.mjs','journal.mjs','network.mjs','native-worker.cjs','browser.mjs','demo.mjs','pack.cjs','package.json']);

async function digest(file) {
  const hash=createHash('sha256');
  for await (const chunk of createReadStream(file)) hash.update(chunk);
  return hash.digest('hex');
}
async function entries(root, folder = root) {
  const result=[];
  for (const entry of await fs.readdir(folder,{withFileTypes:true})) {
    const file=path.join(folder,entry.name), relative=path.relative(root,file).split(path.sep).join('/');
    if (relative === 'manifest.json') continue;
    if (entry.isDirectory()) result.push(...await entries(root,file));
    else if (entry.isFile()) result.push({path:relative,sha256:await digest(file)});
    else if (entry.isSymbolicLink()) {
      const target=await fs.readlink(file);
      const resolved=path.resolve(path.dirname(file),target);
      if (!resolved.startsWith(path.resolve(root)+path.sep)) throw Error('pack_invalid');
      result.push({path:relative,link:target});
    } else throw Error('pack_invalid');
  }
  return result.sort((a,b)=>a.path.localeCompare(b.path));
}
async function createManifest(root, platform, arch, version) {
  const manifest={protocol:1,platform,arch,playwright:version,files:await entries(root)};
  const nativeFile=`native/offgrid-computer${platform==='win32'?'.exe':''}`;
  if(manifest.files.some(file=>file.path===nativeFile)) manifest.native=nativeProfile(platform);
  await fs.writeFile(path.join(root,'manifest.json'),JSON.stringify(manifest));
}
function nativeProfile(platform) {
  const driver={win32:'windows-uia',darwin:'macos-accessibility',linux:'linux-atspi'}[platform];
  if(!driver) throw Error('pack_invalid');
  return {protocol:2,driver,scope:'selected-window',operations:['observe','replace_text','activate','set_checked','shortcut','verify_text','verify_checked'],vision:false,qualified:false,contracts:{native_documents:1},...(platform==='win32'?{desktop_protocol:3,desktop_operations:['applications','open_application','surfaces','focus_surface','verify_application']}: {})};
}
async function verifyPack(root, platform = process.platform, arch = process.arch) {
  try {
    const manifest=JSON.parse(await fs.readFile(path.join(root,'manifest.json'),'utf8'));
    if (manifest.protocol!==1 || manifest.platform!==platform || manifest.arch!==arch || manifest.playwright!=='1.62.1') throw Error('pack_invalid');
    if (JSON.stringify(manifest.files)!==JSON.stringify(await entries(root))) throw Error('pack_invalid');
    for (const required of ['managed.cjs','native-managed.cjs','session.mjs','native-session.mjs','desktop-session.mjs','approval-policy.mjs','browser.mjs','pack.cjs']) {
      if (!manifest.files.some(file=>file.path===required)) throw Error('pack_invalid');
    }
    if (!manifest.files.some(file=>file.path.startsWith('browsers/'))) throw Error('pack_invalid');
    const hasNative=manifest.files.some(file=>file.path===`native/offgrid-computer${platform==='win32'?'.exe':''}`);
    if(hasNative) {
      const current=nativeProfile(platform),legacy={...current};delete legacy.contracts;
      if(JSON.stringify(manifest.native)!==JSON.stringify(current)&&JSON.stringify(manifest.native)!==JSON.stringify(legacy))throw Error('pack_invalid');
      if(manifest.native.contracts&&!manifest.files.some(file=>file.path==='native-features.cjs'))throw Error('pack_invalid');
    }
    if(!hasNative && manifest.native) throw Error('pack_invalid');
  } catch { throw Error('pack_invalid'); }
}
async function verifyNativePack(root,platform=process.platform,arch=process.arch) {
  await verifyPack(root,platform,arch);
  const manifest=JSON.parse(await fs.readFile(path.join(root,'manifest.json'),'utf8'));
  if(!manifest.native) throw Error('computer_driver_unavailable');
  return manifest.native;
}
module.exports={createManifest,verifyPack,verifyNativePack,runtimeFiles};
