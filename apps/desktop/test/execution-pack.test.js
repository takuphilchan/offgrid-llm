const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs/promises');
const path=require('node:path');
const os=require('node:os');
const crypto=require('node:crypto');
const {target,verify}=require('../execution-pack.cjs');

test('matching executor assets are required and altered assets fail closed',async t=>{
  const root=await fs.mkdtemp(path.join(os.tmpdir(),'offgrid-executor-pack-'));
  t.after(()=>fs.rm(root,{recursive:true,force:true}));
  const bytes=Buffer.from('fixture executable, never run');
  const pack={schema_version:1,protocol:1,revision:'offgrid-executor-v1',version:'0.0.0-test',platform:'linux',architecture:'amd64',file:'offgrid-executor',sha256:crypto.createHash('sha256').update(bytes).digest('hex')};
  await fs.writeFile(path.join(root,pack.file),bytes);await fs.writeFile(path.join(root,'execution-pack.json'),JSON.stringify(pack));
  await verify(root,'0.0.0-test','linux','x64');
  await assert.rejects(()=>verify(root,'0.0.1','linux','x64'));
  await assert.rejects(()=>verify(root,'0.0.0-test','darwin','x64'));
  await fs.writeFile(path.join(root,pack.file),'modified');await assert.rejects(()=>verify(root,'0.0.0-test','linux','x64'));
  pack.file='../foreign';await fs.writeFile(path.join(root,'execution-pack.json'),JSON.stringify(pack));await assert.rejects(()=>verify(root,'0.0.0-test','linux','x64'));
  assert.throws(()=>target('unknown','x64'));assert.throws(()=>target('linux','unknown'));
});
