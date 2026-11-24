'use strict';
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs/promises'),path=require('node:path'),os=require('node:os');
const enabled=process.env.OFFGRID_RECOVERY_ELECTRON_TEST==='1'&&!!process.env.OFFGRID_RECOVERY_TEST_BINARY;
test('hidden real Electron recovery renderer, host secret dialog and production Go preserve stop/publication boundaries',{skip:!enabled,timeout:180000},async t=>{
 const {_electron}=require(require.resolve('playwright-core',{paths:[path.resolve(__dirname,'../../web')]}));
 const root=await fs.mkdtemp(path.join(os.tmpdir(),'offgrid-recovery-electron-'));
 assert.equal(path.dirname(root),os.tmpdir());assert.ok(path.basename(root).startsWith('offgrid-recovery-electron-'));
 const env={...process.env,OFFGRID_RECOVERY_FIXTURE_ROOT:root,HOME:root,USERPROFILE:root,APPDATA:root,LOCALAPPDATA:root};delete env.ELECTRON_RUN_AS_NODE;
 const electron=await _electron.launch({executablePath:require('electron'),args:[path.join(__dirname,'fixtures/recovery-host.cjs')],env,timeout:30000});
 const secret='isolated-electron-secret-never-persisted';
 let stageLock;
 t.after(async()=>{
  let lockError;try{await stageLock?.release();}catch(error){lockError=error;}
  // A failed OS-fault helper must not prevent draining the independent service
  // owner. Keep fixture state if either cleanup cannot be confirmed.
  await electron.close();assert.equal(await fs.readFile(path.join(root,'cleanup-complete'),'utf8'),'owned children drained');if(lockError)throw lockError;
  const scanned=await require('./fixtures/recovery-secret-scan.cjs').assertNoPersistedSecret(root,secret);t.diagnostic(`Secret exclusion checked across ${scanned.files} isolated files (${scanned.bytes} bytes)`);
  await fs.rm(root,{recursive:true,force:true});
 });
 const page=await electron.firstWindow();await page.waitForURL(/\/ui\//);
 await page.evaluate(()=>{localStorage.setItem('offgrid.locale','en');localStorage.setItem('offgrid.onboarding.complete','true');});await page.reload();
 const area=page.locator('.workspace-recovery');
 try{await area.waitFor();}catch(error){
  t.diagnostic(JSON.stringify(await area.evaluate(element=>{const ancestors=[];for(let node=element;node;node=node.parentElement){const style=getComputedStyle(node),rect=node.getBoundingClientRect();ancestors.push({tag:node.tagName,class:node.className,hidden:node.hidden,display:style.display,visibility:style.visibility,width:rect.width,height:rect.height});}return {hash:location.hash,visibility:document.visibilityState,ancestors};})));
  throw error;
 }
 const agentState=require('./fixtures/recovery-agent-state.cjs'),serviceURL=await electron.evaluate(()=>global.recoveryFixture.runtime.url);
 await agentState.checkService(serviceURL);await agentState.checkSentinel(path.join(root,'data'));
 const archive=path.join(root,'backup.age');
 async function confirmAndClose(dialogPage){
  const closed=dialogPage.waitForEvent('close');
  try{await dialogPage.getByRole('button',{name:'Continue',exact:true}).click();}
  catch(error){if(!dialogPage.isClosed()||page.isClosed())throw error;}
  await closed;
 }
 async function select(operation,destination,source=archive){
  await electron.evaluate((_electron,value)=>Object.assign(global.recoveryFixture.selection,value),{source,destination});
  await area.getByRole('combobox',{name:'Operation',exact:true}).selectOption(operation);
 }
 async function admit(operation,passphrase=secret,{cancel=false,collision=false,afterConfirmation}={}){
  const label={backup:'Create encrypted backup',verify:'Verify backup',restore:'Restore to a new folder'}[operation];
  const prompt=electron.waitForEvent('window');await area.getByRole('button',{name:label,exact:true}).click();const secretPage=await prompt;
  await secretPage.locator('#secret').fill(passphrase);if(operation==='backup')await secretPage.locator('#secret-confirm').fill(passphrase);
  assert.equal(await page.locator('input[type=password]').count(),0);
  const confirm=electron.waitForEvent('window');await confirmAndClose(secretPage);const confirmation=await confirm;
  await confirmation.getByText('Stop this workspace for recovery?',{exact:true}).waitFor();
  if(collision)await fs.writeFile(path.join(root,'collision'),'other creator wins');
  await confirmAndClose(confirmation);
  if(afterConfirmation)await afterConfirmation();
  if(cancel){
   await area.getByRole('status').filter({hasText:'Checking archive and workspace data'}).waitFor();
   // Hidden Electron throttles animation frames, which can keep Playwright's
   // two-frame stability wait pending until fast real maintenance has finished.
   // Still send a real mouse click to the enabled, visible renderer control.
   const button=area.getByRole('button',{name:'Cancel',exact:true});assert.equal(await button.isEnabled(),true);await button.click({force:true});
  }
  await page.waitForFunction(()=>{const state=document.querySelector('.workspace-recovery [role=status]')?.textContent;return state&&['Recovery operation completed','Operation cancelled','Recovery could not finish. Review the selected archive, key, permissions and available space before retrying.'].includes(state);},{},{timeout:60000});
  return electron.evaluate(()=>global.recoveryFixture.recovery.snapshot());
 }
 async function resume(){await area.getByRole('button',{name:'Resume original workspace',exact:true}).click();await area.getByRole('button',{name:/^(Create encrypted backup|Verify backup|Restore to a new folder)$/,exact:true}).waitFor({state:'visible'});await page.waitForFunction(()=>!document.querySelector('.workspace-recovery .primary-button')?.disabled);await agentState.checkService(serviceURL);await agentState.checkSentinel(path.join(root,'data'));}
 await select('backup',archive);let state=await admit('backup');assert.equal(state.phase,'succeeded');assert.equal(state.result.published,true);assert.equal(state.serviceStopped,true);assert.equal((await fs.readFile(archive)).subarray(0,22).toString(),'age-encryption.org/v1\n');await resume();
 await select('verify',archive);state=await admit('verify','wrong-isolated-passphrase');assert.equal(state.phase,'failed');assert.equal(state.result,undefined);await resume();
 await select('restore',path.join(root,'collision'));state=await admit('restore',secret,{collision:true});assert.equal(state.phase,'failed');assert.equal(state.serviceStopped,false);assert.equal(await fs.readFile(path.join(root,'collision'),'utf8'),'other creator wins');
 await select('restore',path.join(root,'restored'));state=await admit('restore');assert.equal(state.phase,'succeeded');assert.equal(state.result.database_checked,true);assert.equal(await fs.readFile(path.join(root,'restored/fixture.txt'),'utf8'),'isolated Electron recovery content');await agentState.checkRestored(path.join(root,'restored'));await resume();
 await select('restore',path.join(root,'cancelled'));state=await admit('restore',secret,{cancel:true});assert.equal(state.phase,'cancelled');await assert.rejects(fs.stat(path.join(root,'cancelled')),{code:'ENOENT'});await resume();
 if(process.platform==='win32'){
  stageLock=await require('./fixtures/recovery-stage-lock.cjs').holdRestoreStage(root);
  let locked;
  await select('restore',path.join(root,'retained-failure'));
  state=await admit('restore',secret,{afterConfirmation:async()=>{locked=await stageLock.locked;}});
  assert.equal(state.phase,'failed');assert.equal(state.result,undefined);
  assert.deepEqual(state.retainedPaths,[{path:locked.stage,directory:true,contains_plaintext:true}]);
  assert.equal(path.dirname(locked.stage),root);assert.ok(path.basename(locked.stage).startsWith('.offgrid-restore-'));
  assert.equal(await area.locator('.recovery-retained code').innerText(),locked.stage);
  assert.ok((await fs.stat(locked.file)).isFile());
  await assert.rejects(fs.stat(path.join(root,'retained-failure')),{code:'ENOENT'});
  await stageLock.release();stageLock=undefined;
  // Explicit fixture cleanup after the real failure has been inspected; the
  // product does not retry deletion or activate this retained plaintext stage.
  await fs.rm(locked.stage,{recursive:true});await resume();
 }
 const corrupt=path.join(root,'corrupt.age'),bytes=await fs.readFile(archive);bytes[Math.floor(bytes.length/2)]^=1;await fs.writeFile(corrupt,bytes);await select('verify',archive,corrupt);state=await admit('verify');assert.equal(state.phase,'failed');await resume();
 assert.equal(await electron.evaluate(()=>global.recoveryFixture.runtime.workspace.data),path.join(root,'data'));
 assert.equal(await page.evaluate(secret=>JSON.stringify({...localStorage}).includes(secret),secret),false);
 assert.equal(await page.locator('body').innerText().then(text=>text.includes(secret)),false);
 // No screenshot/trace of secret dialogs or clipboard collection is performed.
});
