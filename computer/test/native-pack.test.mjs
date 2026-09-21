import test from 'node:test';
import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {mkdtemp,mkdir,writeFile,readFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
const {createManifest,verifyNativePack}=createRequire(import.meta.url)('../pack.cjs');

for(const [platform,arch,driver] of [['win32','x64','windows-uia'],['darwin','x64','macos-accessibility'],['darwin','arm64','macos-accessibility'],['linux','x64','linux-atspi']]) {
  test(`native ${platform}/${arch} manifest cannot invent qualification or lose worker integrity`,async t=>{
    const root=await mkdtemp(join(tmpdir(),'offgrid-native-pack-'));
    t.after(()=>rm(root,{recursive:true,force:true}));
    await mkdir(join(root,'browsers'));await mkdir(join(root,'native'));
    await writeFile(join(root,'managed.cjs'),'// test fixture');
    await writeFile(join(root,'browsers','fixture'),'not an executable');
    const worker=join(root,'native',platform==='win32'?'offgrid-computer.exe':'offgrid-computer');
    await writeFile(worker,'not an executable');
    await createManifest(root,platform,arch,'1.62.1');
    await verifyNativePack(root,platform,arch);
    const manifest=JSON.parse(await readFile(join(root,'manifest.json'),'utf8'));
    assert.equal(manifest.native.driver,driver);assert.equal(manifest.native.qualified,false);
    for(const changed of [{qualified:true},{vision:true},{scope:'whole-desktop'},{driver:'browser'}]) {
      await writeFile(join(root,'manifest.json'),JSON.stringify({...manifest,native:{...manifest.native,...changed}}));
      await assert.rejects(verifyNativePack(root,platform,arch),/pack_invalid/);
    }
    await writeFile(join(root,'manifest.json'),JSON.stringify(manifest));
    await writeFile(worker,'tampered executable');
    await assert.rejects(verifyNativePack(root,platform,arch),/pack_invalid/);
  });
}
