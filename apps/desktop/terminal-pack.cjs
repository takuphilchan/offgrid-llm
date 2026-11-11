// Build-owned runtime only; a workspace cannot choose a terminal executable.
const fs=require('node:fs/promises'),path=require('node:path'),crypto=require('node:crypto'),{spawn}=require('node:child_process');
const digest=data=>crypto.createHash('sha256').update(data).digest('hex');
async function verify(root,version,platform,arch){
 const pack=JSON.parse(await fs.readFile(path.join(root,'terminal-pack.json'),'utf8'));
 if(pack.protocol!==1||pack.version!==version||pack.platform!==platform||pack.arch!==arch||pack.revision!=='owned-terminal-v1')throw Error('Terminal runtime identity mismatch');
 const supported=platform==='win32'&&arch==='x64';
 if(pack.available!==supported)throw Error('Terminal platform readiness mismatch');
 if(supported){const file=path.join(root,'offgrid-terminal-host.exe'),stat=await fs.lstat(file);if(!stat.isFile()||stat.isSymbolicLink()||digest(await fs.readFile(file))!==pack.sha256)throw Error('Terminal supervisor digest mismatch');}
 return pack;
}
async function prepare(context){
 const platform=context.electronPlatformName,arch=require('builder-util').Arch[context.arch],version=require('./build-version.cjs').buildVersion(context);
 const root=path.resolve(__dirname,'../../build/terminal-runtime',`${{win32:'win',darwin:'mac',linux:'linux'}[platform]}-${arch}`);
 await fs.mkdir(root,{recursive:true});const available=platform==='win32'&&arch==='x64';
 const pack={protocol:1,revision:'owned-terminal-v1',version,platform,arch,available};
 if(available){
  const file=path.join(root,'offgrid-terminal-host.exe');
  await new Promise((resolve,reject)=>{const child=spawn('go',['build','-trimpath','-ldflags=-H=windowsgui','-o',file,'./cmd/offgrid-terminal-host'],{cwd:path.resolve(__dirname,'../..'),stdio:'inherit',shell:false,env:{...process.env,GOOS:'windows',GOARCH:'amd64',CGO_ENABLED:'0'}});child.once('error',reject);child.once('exit',code=>code===0?resolve():reject(Error('Terminal supervisor build failed')));});
  pack.sha256=digest(await fs.readFile(file));
 }
 const notices=[];
 for(const name of ['node-pty','@xterm/xterm','@xterm/addon-fit','@xterm/addon-search']){
  const directory=path.join(__dirname,'node_modules',name),meta=JSON.parse(await fs.readFile(path.join(directory,'package.json'),'utf8'));
  if(meta.version!==require('./package.json').dependencies[name])throw Error('Terminal dependency revision mismatch');
  const files=await fs.readdir(directory),license=files.find(file=>/^LICENSE(\.txt)?$/i.test(file));if(!license)throw Error(`Missing license: ${name}`);
  notices.push(`${name} ${meta.version}\n${await fs.readFile(path.join(directory,license),'utf8')}`);
 }
 await fs.writeFile(path.join(root,'NOTICE.txt'),notices.join('\n\n'));
 await fs.writeFile(path.join(root,'terminal-pack.json'),JSON.stringify(pack,null,2));await verify(root,version,platform,arch);
}
module.exports={prepare,verify};
