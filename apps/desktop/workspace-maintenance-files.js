'use strict';
const fs=require('node:fs/promises'),{constants}=require('node:fs'),path=require('node:path'),crypto=require('node:crypto');
const MAX_KEY_BYTES=1024*1024;
function identity(info){return {device:String(info.dev),inode:String(info.ino),directory:info.isDirectory(),size:info.size,modified:info.mtimeMs};}
function same(before,after){return before.device===after.device&&before.inode===after.inode&&before.directory===after.directory&&(before.directory||before.size===after.size&&before.modified===after.modified);}
async function existing(filename,directory=false){
  if(typeof filename!=='string'||!path.isAbsolute(filename)||Buffer.byteLength(filename)>4096||/[\x00\r\n]/.test(filename))throw Error('maintenance_path_invalid');
  const canonical=await fs.realpath(filename),info=await fs.lstat(canonical);
  if(info.isSymbolicLink()||(directory?!info.isDirectory():!info.isFile()))throw Error('maintenance_path_invalid');
  return {path:canonical,identity:identity(info)};
}
async function check(pin){
  const info=await fs.lstat(pin.path);
  if(info.isSymbolicLink()||!same(pin.identity,identity(info)))throw Error('maintenance_path_changed');
}
async function destination(filename){
  if(typeof filename!=='string'||!path.isAbsolute(filename)||Buffer.byteLength(filename)>4096||/[\x00\r\n]/.test(filename))throw Error('maintenance_path_invalid');
  const parent=await existing(path.dirname(filename),true),target=path.join(parent.path,path.basename(filename));
  await absent(target);return {path:target,parent};
}
async function absent(target){try{await fs.lstat(target);}catch(error){if(error.code==='ENOENT')return;throw error;}throw Error('maintenance_target_exists');}
async function checkDestination(pin){await check(pin.parent);await absent(pin.path);}
function outside(root,target){const relative=path.relative(root,target);return relative!==''&&(relative==='..'||relative.startsWith('..'+path.sep)||path.isAbsolute(relative));}
async function readKeys(filename){
  const pin=await existing(filename);
  if(pin.identity.size>MAX_KEY_BYTES)throw Error('maintenance_key_invalid');
  const flags=constants.O_RDONLY|(process.platform==='win32'?0:constants.O_NONBLOCK|constants.O_NOFOLLOW);
  const file=await fs.open(pin.path,flags);let content=Buffer.alloc(MAX_KEY_BYTES+1);
  try{
    if(!same(pin.identity,identity(await file.stat())))throw Error('maintenance_path_changed');
    let size=0;
    while(size<content.length){const read=await file.read(content,size,content.length-size,null);if(!read.bytesRead)break;size+=read.bytesRead;}
    if(size===0||size>MAX_KEY_BYTES)throw Error('maintenance_key_invalid');
    await check(pin);if(!same(pin.identity,identity(await file.stat())))throw Error('maintenance_path_changed');
    const text=new TextDecoder('utf-8',{fatal:true}).decode(content.subarray(0,size));
    return {pin,text};
  }catch{throw Error('maintenance_key_invalid');}finally{content.fill(0);await file.close();}
}
async function openBinary(pin){
  if(pin.identity.size<1||pin.identity.size>512*1024*1024)throw Error('maintenance_binary_changed');
  const flags=constants.O_RDONLY|(process.platform==='win32'?0:constants.O_NONBLOCK|constants.O_NOFOLLOW);
  const file=await fs.open(pin.path,flags);
  try{const current=await file.stat();if(!current.isFile()||!same(pin.identity,identity(current)))throw Error('maintenance_binary_changed');return file;}catch(error){await file.close();throw error;}
}
async function binaryPin(filename){
  const pin=await existing(filename),file=await openBinary(pin);
  try{
    if(!same(pin.identity,identity(await file.stat())))throw Error('maintenance_binary_changed');
    const hash=crypto.createHash('sha256');
    let size=0;
    for await(const chunk of file.createReadStream({autoClose:false,signal:AbortSignal.timeout(30000)})){size+=chunk.length;if(size>pin.identity.size)throw Error('maintenance_binary_changed');hash.update(chunk);}
    if(size!==pin.identity.size||!same(pin.identity,identity(await file.stat())))throw Error('maintenance_binary_changed');
    await check(pin);return {...pin,digest:hash.digest('hex')};
  }finally{await file.close();}
}
async function checkBinary(pin){const current=await binaryPin(pin.path);if(!same(pin.identity,current.identity)||current.digest!==pin.digest)throw Error('maintenance_binary_changed');}
module.exports={existing,check,destination,checkDestination,outside,readKeys,binaryPin,checkBinary,openBinary,MAX_KEY_BYTES};
