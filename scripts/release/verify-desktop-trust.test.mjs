import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,writeFile,mkdir,rm} from 'node:fs/promises';
import {mkdirSync,readFileSync} from 'node:fs';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {verifyArtifact,artifactNames,main} from './verify-desktop-trust.mjs';

const thumbprint='A'.repeat(40),team='ABCD123456';
async function fixture(t,suffix){const root=await mkdtemp(join(tmpdir(),'offgrid-trust-test-'));t.after(()=>rm(root,{recursive:true,force:true}));const file=join(root,`OffGrid space.${suffix}`);await writeFile(file,'fixture artifact, not a signed executable');return file;}
test('Windows trust requires a valid signature and the configured publisher',async t=>{
 const file=await fixture(t,'exe');
 for(const [status,signer,expected,trusted] of [['Valid',thumbprint,thumbprint,true],['NotSigned',null,thumbprint,false],['HashMismatch',thumbprint,thumbprint,false],['Valid','B'.repeat(40),thumbprint,false],['Valid',thumbprint,'',false]]){
  const result=await verifyArtifact(file,{platform:'windows',host:'win32',env:{OFFGRID_WINDOWS_SIGNER_THUMBPRINT:expected},run:(program,args,env)=>{
   assert.equal(program,'powershell.exe');assert.ok(args.includes('-NonInteractive'));assert.equal(env.OFFGRID_TRUST_ARTIFACT,file);
   assert.ok(!args.join(' ').includes(file),'Artifact path is passed as data, not PowerShell code');
   return {status:0,stdout:JSON.stringify({status,thumbprint:signer})};
  }});
  assert.equal(result.trusted,trusted);assert.match(result.sha256,/^[a-f0-9]{64}$/);
 }
 await assert.rejects(verifyArtifact(file,{platform:'windows',host:'linux'}),/requires Windows/);
 await assert.rejects(verifyArtifact(file,{platform:'windows',host:'win32',run:()=>({status:1})}),/inspection failed/);
});
test('macOS trust requires the expected team, strict signature, Gatekeeper and stapled ticket',async t=>{
 const file=await fixture(t,'zip');
 for(const fail of ['', 'signature','publisher','gatekeeper','notarization','adhoc']){
  const calls=[];
  const result=await verifyArtifact(file,{platform:'macos',host:'darwin',env:{APPLE_TEAM_ID:team},run:(program,args)=>{
   calls.push([program,args]);
   if(program.endsWith('ditto'))mkdirSync(join(args.at(-1),'OffGrid.app'));
   if(args.includes('--display'))return {status:0,stderr:`TeamIdentifier=${fail==='publisher'?'OTHER12345':team}\nSignature=${fail==='adhoc'?'adhoc':'Developer ID'}\n`};
   const failed=(fail==='signature'&&args.includes('--verify'))||(fail==='gatekeeper'&&program.endsWith('spctl'))||(fail==='notarization'&&args.includes('stapler'));
   return {status:failed?1:0,stdout:''};
  }});
  assert.equal(result.trusted,fail==='');assert.ok(calls.some(([,args])=>args.includes('--strict')));
 }
});
test('release inputs require every advertised architecture and installer variant',()=>{
 assert.deepEqual(artifactNames('windows','v0.4.28'),['OffGrid-0.4.28-windows-x64-setup.exe','OffGrid-0.4.28-windows-x64-portable.exe']);
 assert.equal(artifactNames('macos','0.4.29-preview.20261111').length,2);
 assert.throws(()=>artifactNames('macos','../0.4.28'),/Invalid/);
});
test('release mode cannot be bypassed with an unknown mode or empty inputs',async()=>{
 await assert.rejects(main(['--mode','trust-me','--file','missing.exe']),/Mode/);
 await assert.rejects(main(['--directory','missing','--platform','windows']),/version/);
 await assert.rejects(main([]),/Choose/);
});
test('release workflow verifies downloaded draft bytes before finalization can publish',()=>{
 const workflow=readFileSync(new URL('../../.github/workflows/release-unified.yml',import.meta.url),'utf8');
 assert.match(workflow,/Verify published desktop artifact trust/);
 const step=workflow.split('- name: Verify published desktop artifact trust')[1].split('\n  #')[0];
 assert.match(step,/application\/octet-stream/);assert.match(step,/sha256/);assert.match(step,/verify-desktop-trust\.mjs/);
 assert.match(workflow,/needs: \[release-context, build-bundles, build-desktop, publish-containers\]/);
});
