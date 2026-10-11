import test from 'node:test';
import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {mkdtemp,mkdir,writeFile,readFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
const {createManifest,verifyNativePack,runtimeFiles}=createRequire(import.meta.url)('../pack.cjs');

test('current desktop payload includes every first-party imported runtime module',async()=>{
  assert.equal(new Set(runtimeFiles).size,runtimeFiles.length);
  assert.ok(runtimeFiles.includes('document-session.mjs'));
  for(const file of runtimeFiles.filter(f=>/\.(mjs|cjs)$/.test(f))){
    const source=await readFile(new URL('../'+file,import.meta.url),'utf8');
    // Current first-party files use literal static imports/requires. This is a
    // build regression check, not a parser or execution of downloaded code.
    for(const match of source.matchAll(/(?:\bfrom\s*|\b(?:import|require)\s*\(\s*)['"]\.\/([^'"]+)['"]/g)){
      assert.ok(runtimeFiles.includes(match[1]),`${file} imports unpackaged ${match[1]}`);
    }
  }
  const hook=await readFile(new URL('../../desktop/prepare-computer.cjs',import.meta.url),'utf8');
  assert.match(hook,/for \(const file of runtimeFiles\)/);
});

for(const [platform,arch,driver] of [['win32','x64','windows-uia'],['darwin','x64','macos-accessibility'],['darwin','arm64','macos-accessibility'],['linux','x64','linux-atspi']]) {
  test(`native ${platform}/${arch} manifest cannot invent qualification or lose worker integrity`,async t=>{
    const root=await mkdtemp(join(tmpdir(),'offgrid-native-pack-'));
    t.after(()=>rm(root,{recursive:true,force:true}));
    await mkdir(join(root,'browsers'));await mkdir(join(root,'native'));
    for(const file of ['managed.cjs','native-managed.cjs','session.mjs','native-session.mjs','desktop-session.mjs','native-features.cjs','approval-policy.mjs','browser.mjs','pack.cjs'])await writeFile(join(root,file),'// test fixture');
    await writeFile(join(root,'browsers','fixture'),'not an executable');
    const worker=join(root,'native',platform==='win32'?'offgrid-computer.exe':'offgrid-computer');
    await writeFile(worker,'not an executable');
    await createManifest(root,platform,arch,'1.62.1');
    await verifyNativePack(root,platform,arch);
    const manifest=JSON.parse(await readFile(join(root,'manifest.json'),'utf8'));
    assert.equal(manifest.native.driver,driver);assert.equal(manifest.native.qualified,false);
    for(const changed of [{qualified:true},{vision:true},{scope:'whole-desktop'},{driver:'browser'},{contracts:{native_documents:2}},{contracts:{native_documents:1,shell:1}}]) {
      await writeFile(join(root,'manifest.json'),JSON.stringify({...manifest,native:{...manifest.native,...changed}}));
      await assert.rejects(verifyNativePack(root,platform,arch),/pack_invalid/);
    }
    await writeFile(join(root,'manifest.json'),JSON.stringify(manifest));
    const legacy={...manifest.native};delete legacy.contracts;
    await writeFile(join(root,'manifest.json'),JSON.stringify({...manifest,native:legacy}));
    assert.equal((await verifyNativePack(root,platform,arch)).contracts,undefined);
    await writeFile(join(root,'manifest.json'),JSON.stringify(manifest));
    await writeFile(worker,'tampered executable');
    await assert.rejects(verifyNativePack(root,platform,arch),/pack_invalid/);
  });
}
