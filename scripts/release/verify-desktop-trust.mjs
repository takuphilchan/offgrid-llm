// Native verification of the actual distributable. A digest is identity, not
// publisher trust. Inspection may report unsigned previews; release mode fails.
import {createHash} from 'node:crypto';
import {createReadStream} from 'node:fs';
import {lstat, mkdtemp, readdir, rm, writeFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join, resolve, basename} from 'node:path';
import {fileURLToPath} from 'node:url';
import {spawnSync} from 'node:child_process';

const nativeRun=(program,args,env)=>spawnSync(program,args,{encoding:'utf8',windowsHide:true,timeout:120000,env});
const digest=async file=>{const hash=createHash('sha256');for await(const data of createReadStream(file))hash.update(data);return hash.digest('hex');};
const succeeded=result=>!result.error&&result.status===0;

export async function verifyArtifact(file,{platform,host=process.platform,env=process.env,run=nativeRun}={}) {
  file=resolve(file);
  const info=await lstat(file);
  if(!info.isFile()||info.isSymbolicLink()||info.size===0)throw Error('Artifact must be a nonempty regular file');
  const report={file:basename(file),sha256:await digest(file),platform,trusted:false,checks:{}};
  if(platform==='windows') {
    if(host!=='win32')throw Error('Authenticode verification requires Windows');
    if(!/\.exe$/i.test(file))throw Error('Expected a Windows executable artifact');
    const result=run('powershell.exe',['-NoProfile','-NonInteractive','-Command',
      "$ErrorActionPreference='Stop'; $s=Get-AuthenticodeSignature -LiteralPath $env:OFFGRID_TRUST_ARTIFACT; [pscustomobject]@{status=$s.Status.ToString(); subject=$s.SignerCertificate.Subject; thumbprint=$s.SignerCertificate.Thumbprint} | ConvertTo-Json -Compress"],{...env,OFFGRID_TRUST_ARTIFACT:file});
    if(!succeeded(result))throw Error('Authenticode inspection failed');
    const signature=JSON.parse(result.stdout.trim().replace(/^\uFEFF/,''));
    report.signature=signature;
    report.checks.signature=signature.status==='Valid';
    const expected=(env.OFFGRID_WINDOWS_SIGNER_THUMBPRINT??'').toUpperCase();
    report.checks.publisher=/^[A-F0-9]{40}$/.test(expected)&&signature.thumbprint?.toUpperCase()===expected;
  } else if(platform==='macos') {
    if(host!=='darwin')throw Error('Apple signature and notarization verification requires macOS');
    if(!/\.zip$/i.test(file))throw Error('Expected a macOS zip artifact');
    const temporary=await mkdtemp(join(tmpdir(),'offgrid-trust-'));
    try {
      if(!succeeded(run('/usr/bin/ditto',['-x','-k',file,temporary],env)))throw Error('Could not extract macOS artifact');
      const apps=(await readdir(temporary)).filter(name=>name.endsWith('.app'));
      if(apps.length!==1)throw Error('Artifact must contain exactly one app');
      const app=join(temporary,apps[0]),appInfo=await lstat(app);
      if(!appInfo.isDirectory()||appInfo.isSymbolicLink())throw Error('Invalid application bundle');
      report.checks.signature=succeeded(run('/usr/bin/codesign',['--verify','--deep','--strict',app],env));
      const identity=run('/usr/bin/codesign',['--display','--verbose=4',app],env);
      const details=`${identity.stdout??''}\n${identity.stderr??''}`;
      const team=details.match(/^TeamIdentifier=([A-Z0-9]+)$/m)?.[1];
      report.team=team??null;
      report.checks.publisher=succeeded(identity)&&/^[A-Z0-9]{10}$/.test(env.APPLE_TEAM_ID??'')&&team===env.APPLE_TEAM_ID&&!/Signature=adhoc/.test(details);
      report.checks.gatekeeper=succeeded(run('/usr/sbin/spctl',['--assess','--type','execute','--verbose=4',app],env));
      report.checks.notarization=succeeded(run('/usr/bin/xcrun',['stapler','validate',app],env));
    } finally {await rm(temporary,{recursive:true,force:true});}
  } else throw Error('Trust verification supports Windows and macOS artifacts');
  report.trusted=Object.values(report.checks).every(Boolean);
  report.status=report.trusted?'publisher-verified':'not-release-qualified';
  return report;
}

export function artifactNames(platform,version) {
  version=version.replace(/^v/,'');
  if(!/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(version))throw Error('Invalid artifact version');
  if(platform==='windows')return ['setup','portable'].map(kind=>`OffGrid-${version}-windows-x64-${kind}.exe`);
  if(platform==='macos')return ['arm64','x64'].map(arch=>`OffGrid-${version}-macos-${arch}.zip`);
  throw Error('Unsupported artifact platform');
}

export async function main(args) {
  const options={};
  for(let i=0;i<args.length;i+=2){if(!['--file','--directory','--platform','--version','--mode','--report'].includes(args[i])||!args[i+1]||options[args[i]])throw Error('Invalid trust verifier arguments');options[args[i]]=args[i+1];}
  const mode=options['--mode']??'release';
  if(!['inspect','release'].includes(mode))throw Error('Mode must be inspect or release');
  if(Boolean(options['--file'])===Boolean(options['--directory']))throw Error('Choose --file or --directory');
  const platform=options['--platform'];
  const files=options['--file']?[options['--file']]:artifactNames(platform,options['--version']??'').map(name=>join(options['--directory'],name));
  const artifacts=[];
  for(const file of files)artifacts.push(await verifyArtifact(file,{platform}));
  const report={schema_version:1,checkedAt:new Date().toISOString(),mode,artifacts};
  const output=JSON.stringify(report,null,2)+'\n';
  if(options['--report'])await writeFile(options['--report'],output);
  console.log(output);
  if(mode==='release'&&artifacts.some(artifact=>!artifact.trusted))throw Error('Artifact trust gate failed: valid publisher identity and native trust checks are required');
  return report;
}
if(process.argv[1]&&resolve(process.argv[1])===fileURLToPath(import.meta.url))main(process.argv.slice(2)).catch(error=>{console.error(error.message);process.exitCode=1;});
