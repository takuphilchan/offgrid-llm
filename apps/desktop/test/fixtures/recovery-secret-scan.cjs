'use strict';
const assert=require('node:assert/strict'),fs=require('node:fs'),fsp=require('node:fs/promises'),path=require('node:path');

// Scan only the generated fixture after every child has drained. This covers
// service databases, recovered files and Chromium persistence without recording
// their contents. No secret-dialog screenshots or process memory are collected.
exports.assertNoPersistedSecret=async(root,secret)=>{
 const needles=[Buffer.from(secret),Buffer.from(secret,'utf16le'),Buffer.from(secret,'utf16le').swap16()];
 const overlap=Math.max(...needles.map(value=>value.length))-1;let files=0,bytes=0;
 async function visit(directory){
  for(const entry of await fsp.readdir(directory,{withFileTypes:true})){
   const name=path.join(directory,entry.name);assert.equal(entry.isSymbolicLink(),false);
   if(entry.isDirectory()){await visit(name);continue;}
   assert.equal(entry.isFile(),true);assert.ok(++files<=10000);
   let tail=Buffer.alloc(0);
   for await(const chunk of fs.createReadStream(name,{highWaterMark:1024*1024})){
    bytes+=chunk.length;assert.ok(bytes<=1024*1024*1024);
    const joined=Buffer.concat([tail,chunk]);assert.equal(needles.some(needle=>joined.includes(needle)),false,`Secret persisted in isolated ${path.relative(root,name)}`);
    tail=joined.subarray(Math.max(0,joined.length-overlap));
   }
  }
 }
 await visit(root);return {files,bytes};
};
