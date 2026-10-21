const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const os = require('node:os');
const { attachmentStore } = require('../attachment');

test('ownership persists across stores and remains endpoint/profile scoped', async t => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(),'offgrid-attachment-'));
  t.after(()=>fs.rm(root,{recursive:true,force:true}));
  const store = attachmentStore(root); const url = 'http://127.0.0.1:11611';
  assert.equal(await store.load(url),false); await store.save(url); await store.save(url);
  assert.equal(await attachmentStore(root).load(url),true);
  assert.equal(await store.load('http://127.0.0.1:11612'),false);
  assert.equal(await attachmentStore(path.join(root,'another-profile')).load(url),false);
  const [name] = await fs.readdir(path.join(root,'desktop-backends'));
  const file = path.join(root,'desktop-backends',name);
  assert.deepEqual(JSON.parse(await fs.readFile(file,'utf8')),{version:1,url,external:true});
  for (const body of ['{', 'null', JSON.stringify({version:2,url,external:true}), JSON.stringify({version:1,url:'http://127.0.0.1:11612',external:true}), ' '.repeat(4097)]) {
    await fs.writeFile(file,body);
    await assert.rejects(store.load(url));
    await assert.rejects(store.save(url));
    assert.equal(await fs.readFile(file,'utf8'),body,'corrupt evidence must not be overwritten');
  }
});
