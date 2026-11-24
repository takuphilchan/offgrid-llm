'use strict';
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs/promises'),path=require('node:path'),os=require('node:os');
const files=require('../workspace-maintenance-files');
async function fixture(t){const root=await fs.mkdtemp(path.join(os.tmpdir(),'offgrid-maintenance-files-'));t.after(()=>fs.rm(root,{recursive:true,force:true}));return root;}
test('a regular executable replaced by a FIFO cannot block the reviewed opener',{skip:process.platform==='win32',timeout:2000},async t=>{
 const root=await fixture(t),file=path.join(root,'offgrid');await fs.writeFile(file,'reviewed executable');const pin=await files.existing(file);
 await fs.rename(file,file+'.old');require('node:child_process').execFileSync('mkfifo',[file]);
 await assert.rejects(files.openBinary(pin),/maintenance_binary_changed/);
});
test('native selections pin canonical existing files and reject changed identities',async t=>{
 const root=await fixture(t),file=path.join(root,'archive.age');await fs.writeFile(file,'original');
 const pin=await files.existing(file);await files.check(pin);
 await fs.rename(file,file+'.old');await fs.writeFile(file,'replaced');await assert.rejects(files.check(pin),/maintenance_path_changed/);
 await assert.rejects(files.existing(root),/maintenance_path_invalid/);
 await assert.rejects(files.existing('relative'),/maintenance_path_invalid/);
});
test('destinations require a new path, keep parent identity and cannot write inside a source root',async t=>{
 const root=await fixture(t),parent=path.join(root,'dest');await fs.mkdir(parent);
 const pin=await files.destination(path.join(parent,'new'));await files.checkDestination(pin);
 await fs.writeFile(pin.path,'other creator');await assert.rejects(files.checkDestination(pin),/maintenance_target_exists/);
 await fs.rm(pin.path);await fs.rename(parent,parent+'.old');await fs.mkdir(parent);await assert.rejects(files.checkDestination(pin),/maintenance_path_changed/);
 assert.equal(files.outside(root,root),false);assert.equal(files.outside(root,path.join(root,'nested')),false);assert.equal(files.outside(root,root+'-other'),true);
});
test('key input is bounded UTF8 and does not create a key copy on disk',async t=>{
 const root=await fixture(t),key=path.join(root,'key');await fs.writeFile(key,'# isolated\nrecipient\n');
 assert.equal((await files.readKeys(key)).text,'# isolated\nrecipient\n');assert.deepEqual(await fs.readdir(root),['key']);
 await fs.writeFile(key,Buffer.from([0xff]));await assert.rejects(files.readKeys(key),/maintenance_key_invalid/);
 await fs.writeFile(key,Buffer.alloc(files.MAX_KEY_BYTES+1));await assert.rejects(files.readKeys(key),/maintenance_key_invalid/);
});
test('owned executable pin includes digest, not only pathname or modification time',async t=>{
 const root=await fixture(t),binary=path.join(root,'test-binary');await fs.writeFile(binary,'first');
 const pin=await files.binaryPin(binary);await files.checkBinary(pin);
 await fs.writeFile(binary,'other');await fs.utimes(binary,new Date(),new Date(pin.identity.modified));await assert.rejects(files.checkBinary(pin));
});
