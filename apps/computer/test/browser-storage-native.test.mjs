import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,realpath,mkdir,copyFile,writeFile,readFile,rm,readdir} from 'node:fs/promises';
import {tmpdir,homedir} from 'node:os';
import {join,isAbsolute,dirname,basename} from 'node:path';
import {fileURLToPath} from 'node:url';
import {createHash} from 'node:crypto';
import {createServer} from 'node:http';
import {spawn} from 'node:child_process';
import {createManifest,runtimeFiles} from '../pack.cjs';
import {BrowserStorageCheck} from '../browser-storage-check.mjs';
import {BrowserWorkspace} from '../browser-workspace.mjs';

test('verified native storage-only mode protects actual upload state before browser assignment',{
 skip:process.env.OFFGRID_BROWSER_STORAGE_NATIVE_TEST!=='1',timeout:60000,
},async()=>{
 const binary=process.env.OFFGRID_BROWSER_STORAGE_NATIVE_BINARY;assert.ok(isAbsolute(binary??''),'explicit isolated compiled binary required');
 // A private leaf cannot compensate for a shared writable ancestor. On
 // Windows the general temp directory can have broader rename authority;
 // use a new disposable home leaf, never an existing application profile.
 const requestedParent=process.env.OFFGRID_BROWSER_STORAGE_TEST_PARENT??(process.platform==='win32'?homedir():tmpdir());
 assert.ok(isAbsolute(requestedParent),'explicit absolute fixture parent required');
 const parent=await realpath(requestedParent);
 const root=await mkdtemp(join(parent,'offgrid-storage-native-')),pack=join(root,'pack'),directory=join(root,'private'),workspaceRoot=join(root,'workspace');
 let checker,workspace,server;
 try{
  await mkdir(pack);await mkdir(join(pack,'native'));await mkdir(join(pack,'browsers'));await mkdir(workspaceRoot);
  const account=join(root,'account');await mkdir(account);
  if(process.platform==='darwin')await mkdir(join(account,'Library','Application Support'),{recursive:true});
  const spawnWorker=(file,args,options)=>spawn(file,args,{...options,env:{...options.env,APPDATA:account,XDG_CONFIG_HOME:account,...(process.platform==='darwin'?{HOME:account}:{})}});
  for(const file of runtimeFiles)await copyFile(fileURLToPath(new URL('../'+file,import.meta.url)),join(pack,file));
  await writeFile(join(pack,'browsers','fixture-marker'),'Storage checker fixture; no installed browser payload is represented.');
  await copyFile(binary,join(pack,'native',process.platform==='win32'?'offgrid-computer.exe':'offgrid-computer'));
  await createManifest(pack,process.platform,process.arch,'1.62.1');
  checker=new BrowserStorageCheck({root:pack,directory,workspaceRoots:[workspaceRoot],spawnWorker});
  assert.equal(await checker.prepare(),true);assert.equal(checker.child,null);
  const content=Buffer.from('native protected snapshot and actual browser selection'),source=join(workspaceRoot,'approved.txt');
  await writeFile(source,content);
  const grant={id:'c'.repeat(32),path:source,name:'approved.txt',size:content.length,sha256:createHash('sha256').update(content).digest('hex')};
  server=createServer((_req,res)=>{res.setHeader('content-type','text/html');res.end('<input type="file" aria-label="Approved file">');});
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
  workspace=await BrowserWorkspace.open(`http://127.0.0.1:${server.address().port}`,{headless:true,testLoopback:true,upload:grant});
  const binding={actorId:'local-admin',workspaceId:'workspace',runId:'run-'+'a'.repeat(32),inputId:'input-initial',sessionId:'b'.repeat(32)};
  let rechecks=0;
  await workspace.bindFileTask(directory,binding,async()=>{assert.equal(await checker.prepare(),true);rechecks++;});
  const view=await workspace.execute('browser_observe',{}),element=view.elements.find(row=>row.label==='Approved file');assert.ok(element);
  const result=await workspace.execute('browser_upload',{observation_id:view.observation_id,element:element.id},{actionId:binding.runId+':upload',binding});
  assert.equal(result.file.sha256,grant.sha256);assert.equal(result.verification_scope,'file_input_selection');assert.equal(rechecks,2);
  const store=workspace.root.uploadStore,row=store.retained().records[0];assert.equal(row.state,'in_use');assert.deepEqual(row.binding,binding);
  assert.deepEqual(await readFile(store.records.get(grant.id).snapshot.path),content);
  const overlap=new BrowserStorageCheck({root:pack,directory,workspaceRoots:[root],spawnWorker});
  try{await assert.rejects(overlap.prepare(),/computer_storage_unavailable/);assert.equal(overlap.child,null);}finally{await overlap.stop();}
  await workspace.close();assert.equal(store.closed,true);assert.deepEqual(await readdir(join(directory,'uploads')),[]);
 }finally{
  await workspace?.close();await checker?.stop();if(server){server.closeAllConnections();await new Promise(resolve=>server.close(resolve));}
  assert.equal(dirname(root),parent);assert.ok(basename(root).startsWith('offgrid-storage-native-'));
  await rm(root,{recursive:true});
 }
});
