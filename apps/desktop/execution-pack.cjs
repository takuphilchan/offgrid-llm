// Build/packaging only. No model, renderer or service can select these inputs.
const fs=require('node:fs/promises');
const path=require('node:path');
const crypto=require('node:crypto');
const {spawn}=require('node:child_process');

function target(platform,arch){
  const goos={win32:'windows',darwin:'darwin',linux:'linux'}[platform];
  const goarch={x64:'amd64',arm64:'arm64'}[arch];
  if(!goos||!goarch)throw Error('Unsupported executor pack target');
  return {goos,goarch,file:goos==='windows'?'offgrid-executor.exe':'offgrid-executor'};
}

async function verify(root,version,platform,arch){
  const expected=target(platform,arch),manifest=path.join(root,'execution-pack.json');
  const stat=await fs.lstat(manifest);
  if(!stat.isFile()||stat.isSymbolicLink()||stat.size>65536)throw Error('Invalid executor pack manifest');
  const pack=JSON.parse(await fs.readFile(manifest,'utf8'));
  const keys=['schema_version','protocol','revision','version','platform','architecture','file','sha256'];
  if(Object.keys(pack).some(key=>!keys.includes(key))||keys.some(key=>!(key in pack))||pack.schema_version!==1||pack.protocol!==1||pack.revision!=='offgrid-executor-v1'||pack.version!==version.replace(/^v/,'')||pack.platform!==expected.goos||pack.architecture!==expected.goarch||pack.file!==expected.file||!/^[a-f0-9]{64}$/.test(pack.sha256))throw Error('Executor pack identity does not match desktop');
  const binary=path.join(root,expected.file),info=await fs.lstat(binary);
  if(!info.isFile()||info.isSymbolicLink()||info.size>256*1024*1024||info.size<1)throw Error('Invalid executor binary');
  const digest=crypto.createHash('sha256').update(await fs.readFile(binary)).digest('hex');
  if(digest!==pack.sha256)throw Error('Executor binary digest mismatch');
  return pack;
}

async function prepare(context){
  const platform=context.electronPlatformName,arch=require('builder-util').Arch[context.arch];
  const {goos,goarch}=target(platform,arch);
  const version=require('./package.json').version;
  const directory=path.resolve(__dirname,'../../build/execution-runtime',`${{win32:'win',darwin:'mac',linux:'linux'}[platform]}-${arch}`);
  const env={...process.env};delete env.GOOS;delete env.GOARCH;
  await new Promise((resolve,reject)=>{
    const child=spawn('go',['run','./tools/executor-pack','--output',directory,'--version',version,'--goos',goos,'--goarch',goarch],{cwd:path.resolve(__dirname,'../..'),env,stdio:'inherit',shell:false});
    child.once('error',reject);child.once('exit',code=>code===0?resolve():reject(Error('Matching executor build failed')));
  });
  await verify(directory,version,platform,arch);
}

module.exports={target,verify,prepare};
