'use strict';
const {spawn}=require('node:child_process'),path=require('node:path'),{createInterface}=require('node:readline');

// Explicit Windows-only, disposable-root OS fault fixture. The handle owner
// reports readiness and the actual path, then waits for its private stdin EOF.
exports.holdRestoreStage=async root=>{
 const child=spawn('powershell.exe',['-NoProfile','-NonInteractive','-File',path.join(__dirname,'recovery-stage-lock.ps1'),'-Root',root],{stdio:['pipe','pipe','pipe'],windowsHide:true});
 let stderr='',resolveReady,rejectReady,resolveLocked,rejectLocked;
 const ready=new Promise((resolve,reject)=>{resolveReady=resolve;rejectReady=reject;});
 const locked=new Promise((resolve,reject)=>{resolveLocked=resolve;rejectLocked=reject;});
 // A failed admission may never await locked; still preserve that failure for
 // callers without creating an unhandled rejection during cleanup.
 locked.catch(()=>{});
 child.stderr.on('data',chunk=>{stderr=(stderr+chunk).slice(0,4096);});
 createInterface({input:child.stdout}).on('line',line=>{
  if(line==='ready')resolveReady();else try{resolveLocked(JSON.parse(line));}catch{rejectLocked(Error('Invalid isolated stage-lock response'));}
 });
 const closed=new Promise((resolve,reject)=>{
  child.once('error',error=>{rejectReady(error);rejectLocked(error);reject(error);});
  child.once('close',code=>{const error=Error(`Isolated stage lock exited ${code}: ${stderr}`);rejectReady(error);rejectLocked(error);code===0?resolve():reject(error);});
 });
 closed.catch(()=>{});
 await ready;
 return {locked,release:async()=>{child.stdin.end();await closed;}};
};
