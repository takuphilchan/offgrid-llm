import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,writeFile,readFile,mkdir,rm,readdir,symlink} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {createHash} from 'node:crypto';
import {verifyFiles,run} from '../../runtimes/chromium/apply-patch.mjs';

const digest=bytes=>createHash('sha256').update(bytes).digest('hex');
async function fixture(t) {
  const root=await mkdtemp(join(tmpdir(),'offgrid-chromium-patch-'));
  t.after(()=>rm(root,{recursive:true,force:true}));
  await mkdir(join(root,'source'));
  await writeFile(join(root,'source','known.cc'),'exact pinned source\n');
  return {root,changes:[{path:'source/known.cc',before:digest('exact pinned source\n')},
    {path:'source/new-test.js',before:null}]};
}

test('exact preimages and absent native tests are read-only admission',async t=>{
  const {root,changes}=await fixture(t);
  await verifyFiles(root,changes);
  assert.deepEqual(await readdir(join(root,'source')),['known.cc']);
  assert.equal(await readFile(join(root,'source','known.cc'),'utf8'),'exact pinned source\n');
});
test('changed preimage and occupied new test fail without overwrite',async t=>{
  const {root,changes}=await fixture(t);
  await writeFile(join(root,'source','known.cc'),'user changes');
  await assert.rejects(verifyFiles(root,changes),/digest mismatch/);
  assert.equal(await readFile(join(root,'source','known.cc'),'utf8'),'user changes');
  await writeFile(join(root,'source','known.cc'),'exact pinned source\n');
  await writeFile(join(root,'source','new-test.js'),'existing work');
  await assert.rejects(verifyFiles(root,changes),/already exists/);
  assert.equal(await readFile(join(root,'source','new-test.js'),'utf8'),'existing work');
});
test('escaped or non-regular targets fail before writes',async t=>{
  const {root}=await fixture(t);
  for(const path of ['../outside','/absolute','source/../outside','C:/outside','source\\outside'])
    await assert.rejects(verifyFiles(root,[{path,before:null}]),/Invalid/);
  await assert.rejects(verifyFiles(root,[{path:'source',before:null}]),/regular/);
});
test('symlinked ancestor cannot redirect patch admission',async t=>{
  const {root,changes}=await fixture(t);
  try{await symlink(join(root,'source'),join(root,'alias'),process.platform==='win32'?'junction':'dir');}
  catch(error){if(error.code==='EPERM'){t.skip('directory link privilege unavailable');return;}throw error;}
  await assert.rejects(verifyFiles(root,[{...changes[0],path:'alias/known.cc'}]),/Unsafe/);
});
test('CLI requires an explicit check or apply operation',async()=>{
  for(const args of [[],['source'],['--download','source'],['--apply','source','extra']])
    await assert.rejects(run(args),/Usage/);
});
