// Explicit build-time operation only. No downloads, builds or installed paths.
import {readFile,lstat,realpath} from 'node:fs/promises';
import {resolve,dirname,join,relative,isAbsolute} from 'node:path';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';

const here=dirname(fileURLToPath(import.meta.url));
const sha256=bytes=>createHash('sha256').update(bytes).digest('hex');
const git=(source,...args)=>execFileSync('git',['-C',source,...args],{
  encoding:'utf8',stdio:['ignore','pipe','pipe'],timeout:30000,
  env:{...process.env,GIT_OPTIONAL_LOCKS:'0'},
}).trim();

export async function verifyFiles(source,changes,side='before') {
  const root=await realpath(source);
  for(const change of changes) {
    const parts=change.path.split('/');
    if(!parts.length||parts.some(part=>!part||part==='.'||part==='..'||part.includes('\\')||part.includes(':')))
      throw Error('Invalid locked source path.');
    let target=root,missing=false;
    for(let index=0;index<parts.length;index++) {
      target=join(target,parts[index]);
      let info;
      try{info=await lstat(target);}catch(error){if(error.code==='ENOENT'){missing=true;break;}throw error;}
      if(info.isSymbolicLink()||(index<parts.length-1&&!info.isDirectory()))
        throw Error(`Unsafe source ancestry: ${change.path}`);
      if(index===parts.length-1&&!info.isFile())throw Error(`Not a regular source: ${change.path}`);
    }
    const expected=change[side];
    if(expected===null) {
      if(!missing)throw Error(`New test already exists: ${change.path}`);
    } else {
      if(missing||sha256(await readFile(target))!==expected)
        throw Error(`Source digest mismatch: ${change.path}`);
    }
  }
}

export async function run(args) {
  if(args.length!==2||!['--check','--apply'].includes(args[0]))
    throw Error('Usage: node apply-patch.mjs --check|--apply PINNED_CHROMIUM_CHECKOUT');
  const source=await realpath(resolve(args[1]));
  const lock=JSON.parse(await readFile(join(here,'source.lock.json'),'utf8'));
  const patch=join(here,lock.patch);
  if(lock.schema!==1||lock.patch!=='metadata-only-fetch.patch'||
     sha256(await readFile(patch))!==lock.patchSha256)throw Error('Patch identity mismatch.');
  const checkout=await realpath(git(source,'rev-parse','--show-toplevel'));
  if(relative(source,checkout)!==''||isAbsolute(relative(source,checkout)))
    throw Error('Pass the Chromium checkout root, not a nested directory.');
  if(git(source,'rev-parse','HEAD')!==lock.revision)
    throw Error(`Patch requires Chromium ${lock.revision}.`);
  await verifyFiles(source,lock.changes);
  git(source,'diff','--exit-code','HEAD','--',...lock.changes.filter(row=>row.before!==null).map(row=>row.path));
  git(source,'-c','core.autocrlf=false','apply','--check','--whitespace=error',patch);
  if(args[0]==='--apply') {
    // Caller must keep this dedicated checkout quiescent through application.
    git(source,'-c','core.autocrlf=false','apply','--whitespace=error',patch);
    await verifyFiles(source,lock.changes,'after');
  }
  return `${args[0]==='--apply'?'Applied':'Checked'} source patch only; native build and qualification remain unrun.`;
}

if(process.argv[1]&&import.meta.url===pathToFileURL(resolve(process.argv[1])).href) {
  try{console.log(await run(process.argv.slice(2)));}
  catch(error){console.error(error.stderr?.toString().trim()||error.message);process.exitCode=1;}
}
