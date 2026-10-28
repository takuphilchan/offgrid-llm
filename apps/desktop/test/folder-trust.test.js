const {test}=require('node:test');const assert=require('node:assert/strict');const fs=require('node:fs/promises');const os=require('node:os');const path=require('node:path');const crypto=require('node:crypto');
const {FolderTrust}=require('../folder-trust');
const {folderTrustCopy}=require('../folder-trust-copy');

test('native trust controls have complete labels in all nine locales',()=>{
  const expected=Object.keys(folderTrustCopy('en')).sort();
  for(const locale of ['en','fr','de','es','ar','sw','sn','nd','zu']){
    const copy=folderTrustCopy(locale);assert.deepEqual(Object.keys(copy).sort(),expected);
    for(const [key,value] of Object.entries(copy)){if(key==='permissions'){assert.equal(value.length,3);assert.ok(value.every(label=>typeof label==='string'&&label.length>0));}else assert.ok(typeof value==='string'&&value.length>0);}
  }
});

// Isolated authenticated cipher double; no real OS keychain or user state.
function storage(){const key=crypto.randomBytes(32);return {isEncryptionAvailable:()=>true,getSelectedStorageBackend:()=> 'fixture',encryptString:value=>{const iv=crypto.randomBytes(12),cipher=crypto.createCipheriv('aes-256-gcm',key,iv);const data=Buffer.concat([cipher.update(value,'utf8'),cipher.final()]);return Buffer.concat([iv,cipher.getAuthTag(),data]);},decryptString:data=>{const cipher=crypto.createDecipheriv('aes-256-gcm',key,data.subarray(0,12));cipher.setAuthTag(data.subarray(12,28));return Buffer.concat([cipher.update(data.subarray(28)),cipher.final()]).toString();}};}

test('remembered desktop trust is exact, encrypted, revocable and root-replacement aware',async t=>{
  const base=await fs.mkdtemp(path.join(os.tmpdir(),'offgrid-trust-'));t.after(()=>fs.rm(base,{recursive:true,force:true}));const folder=path.join(base,'workspace');await fs.mkdir(folder);
  const trust=new FolderTrust(path.join(base,'private'),storage()),approval={folder,permissions:{read:true,write:false,processes:false},service:'http://localhost:11611',workspace:'installation',actor:'alice'};
  const pin=await trust.pin(approval);assert.equal(await trust.matches(pin),false);await trust.remember(pin);assert.equal(await trust.matches(await trust.pin(approval)),true);
  for(const change of [{actor:'bob'},{workspace:'other'},{service:'http://localhost:11612'},{permissions:{read:true,write:true,processes:false}}])assert.equal(await trust.matches(await trust.pin({...approval,...change})),false);
  await fs.rename(folder,folder+'-old');await fs.mkdir(folder);assert.equal(await trust.matches(await trust.pin(approval)),false);await assert.rejects(trust.remember(pin),/folder_trust_changed/);
  assert.equal((await trust.list()).length,1);await trust.revoke(pin.id);assert.equal((await trust.list()).length,0);
  await assert.rejects(trust.revoke('../file'));
});
test('plaintext keyring fallback, corrupt records and workspace-owned trust fail closed',async t=>{
  const base=await fs.mkdtemp(path.join(os.tmpdir(),'offgrid-trust-'));t.after(()=>fs.rm(base,{recursive:true,force:true}));const folder=path.join(base,'workspace');await fs.mkdir(folder);const approval={folder,permissions:{read:true},service:'http://localhost',workspace:'installation',actor:'alice'};
  const insecure=new FolderTrust(path.join(base,'private'),{...storage(),getSelectedStorageBackend:()=> 'basic_text'});await assert.rejects(insecure.pin(approval));
  const inside=new FolderTrust(path.join(folder,'private'),storage());await assert.rejects(inside.pin(approval));
  const trust=new FolderTrust(path.join(base,'private'),storage()),pin=await trust.pin(approval);await trust.remember(pin);await fs.writeFile(trust.file(pin.id),Buffer.from('corrupt'));await assert.rejects(trust.matches(pin));
});

test('workspace-owned trust through an alias of its parent fails closed',async t=>{
  const base=await fs.mkdtemp(path.join(os.tmpdir(),'offgrid-trust-alias-'));
  const alias=base+'-alias';
  t.after(async()=>{await fs.rm(alias,{force:true});await fs.rm(base,{recursive:true,force:true});});
  try{await fs.symlink(base,alias,'dir');}
  catch(error){if(error.code==='EPERM'||error.code==='EACCES'){t.skip('directory symlinks unavailable');return;}throw error;}
  const folder=path.join(base,'workspace');await fs.mkdir(folder);
  const trust=new FolderTrust(path.join(alias,'workspace','private'),storage());
  await assert.rejects(trust.pin({folder,permissions:{read:true},service:'http://localhost',workspace:'installation',actor:'alice'}));
});
