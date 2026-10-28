const fs=require('node:fs/promises');
const path=require('node:path');
const os=require('node:os');
const crypto=require('node:crypto');

async function resolvedDestination(destination){
  const suffix=[];let cursor=path.resolve(destination);
  for(;;){
    try{return path.join(await fs.realpath(cursor),...suffix.reverse());}
    catch(error){
      if(error.code!=='ENOENT')throw error;
      const parent=path.dirname(cursor);if(parent===cursor)throw error;
      suffix.push(path.basename(cursor));cursor=parent;
    }
  }
}

// Native-only preferences. The renderer never supplies paths or approval
// records. OS-keyring encryption is required; basic-text fallback is disabled.
class FolderTrust {
  constructor(directory,storage){this.directory=directory;this.storage=storage;}
  available(){return this.storage.isEncryptionAvailable()&&this.storage.getSelectedStorageBackend?.()!=='basic_text';}
  async pin({folder,permissions,service,workspace,actor}){
    if(!this.available())throw Error('folder_trust_unavailable');
    const root=await fs.realpath(folder),stat=await fs.stat(root,{bigint:true});
    const relative=path.relative(root,await resolvedDestination(this.directory));
    if(!stat.isDirectory()||stat.ino===0n||stat.birthtimeNs<=0n||(!relative.startsWith('..'+path.sep)&&relative!=='..'&&!path.isAbsolute(relative)))throw Error('folder_trust_unavailable');
    if(![service,workspace,actor].every(v=>typeof v==='string'&&v.length>0&&v.length<1024))throw Error('folder_trust_unavailable');
    const scope={version:1,root,host:os.hostname(),service,workspace,actor,permissions:{read:permissions.read===true,write:permissions.write===true,processes:permissions.processes===true},identity:`${stat.dev}:${stat.ino}:${stat.birthtimeNs}`};
    return {...scope,id:crypto.createHash('sha256').update(JSON.stringify(scope)).digest('hex')};
  }
  async ready(){
    if(!this.available())throw Error('folder_trust_unavailable');
    await fs.mkdir(this.directory,{recursive:true,mode:0o700});
    const stat=await fs.lstat(this.directory);if(!stat.isDirectory()||stat.isSymbolicLink())throw Error('folder_trust_invalid');
  }
  file(id){if(!/^[a-f0-9]{64}$/.test(id))throw Error('folder_trust_invalid');return path.join(this.directory,id+'.bin');}
  async read(id){
    const file=this.file(id),stat=await fs.lstat(file);if(!stat.isFile()||stat.isSymbolicLink()||stat.size>16384)throw Error('folder_trust_invalid');
    const record=JSON.parse(this.storage.decryptString(await fs.readFile(file)));
    if(record.version!==1||record.id!==id||typeof record.created!=='string')throw Error('folder_trust_invalid');
    return record;
  }
  async matches(pin){
    await this.ready();
    try{const record=await this.read(pin.id);delete record.created;return JSON.stringify(record)===JSON.stringify(pin);}
    catch(error){if(error.code==='ENOENT')return false;throw error;}
  }
  async remember(pin){
    await this.ready();
    const fresh=await this.pin({folder:pin.root,...pin});if(fresh.id!==pin.id)throw Error('folder_trust_changed');
    const data=this.storage.encryptString(JSON.stringify({...pin,created:new Date().toISOString()}));
    try{await fs.writeFile(this.file(pin.id),data,{flag:'wx',mode:0o600});}
    catch(error){if(error.code==='EEXIST'&&await this.matches(pin))return;throw error;}
  }
  async list(){await this.ready();const files=await fs.readdir(this.directory);if(files.length>1024)throw Error('folder_trust_invalid');return Promise.all(files.filter(f=>f.endsWith('.bin')).map(f=>this.read(f.slice(0,-4))));}
  async revoke(id){await this.ready();await this.read(id);await fs.unlink(this.file(id));}
}
module.exports={FolderTrust};
