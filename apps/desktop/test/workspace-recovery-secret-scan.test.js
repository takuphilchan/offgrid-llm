'use strict';
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs/promises'),os=require('node:os'),path=require('node:path');
const {assertNoPersistedSecret}=require('./fixtures/recovery-secret-scan.cjs');

test('fixture secret scanner detects UTF-8 and both UTF-16 byte orders across read boundaries',async t=>{
 const root=await fs.mkdtemp(path.join(os.tmpdir(),'offgrid-recovery-scan-'));
 t.after(()=>{assert.equal(path.dirname(path.resolve(root)),os.tmpdir());assert.ok(path.basename(root).startsWith('offgrid-recovery-scan-'));return fs.rm(root,{recursive:true});});
 const secret='isolated-secret-sentinel';
 for(const encoded of [Buffer.from(secret),Buffer.from(secret,'utf16le'),Buffer.from(secret,'utf16le').swap16()]){
  await fs.writeFile(path.join(root,'fixture.bin'),Buffer.concat([Buffer.alloc(1024*1024-7),encoded,Buffer.from('end')]));
  await assert.rejects(assertNoPersistedSecret(root,secret),/Secret persisted in isolated fixture.bin/);
 }
 await fs.writeFile(path.join(root,'fixture.bin'),'public fixture bytes');
 assert.deepEqual(await assertNoPersistedSecret(root,secret),{files:1,bytes:20});
});
