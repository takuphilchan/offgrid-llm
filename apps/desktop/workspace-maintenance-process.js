'use strict';
const {EventEmitter}=require('node:events');
const {spawn}=require('node:child_process');

const MAX_REQUEST=2*1024*1024,MAX_RESULT=32768;
const operationID=value=>typeof value==='string'&&/^[a-f0-9]{64}$/.test(value);
function frame(value,limit=MAX_REQUEST){
  const body=Buffer.from(JSON.stringify(value));
  if(body.length<1||body.length>limit)throw Error('maintenance_request_invalid');
  const result=Buffer.alloc(body.length+4);result.writeUInt32BE(body.length);body.copy(result,4);body.fill(0);return result;
}
function validateMessage(value,id){
  if(!value||value.protocol!==1||value.id!==id||!['working','finished'].includes(value.phase))throw Error('maintenance_protocol_invalid');
  const allowed=new Set(['protocol','id','phase','outcome','code','summary','retained']);
  if(Object.keys(value).some(key=>!allowed.has(key)))throw Error('maintenance_protocol_invalid');
  if(value.phase==='working'){
    if(Object.keys(value).length!==3)throw Error('maintenance_protocol_invalid');
    return value;
  }
  if(!['succeeded','cancelled','failed'].includes(value.outcome)||!['','operation_failed','cancelled','invalid_request','invalid_control'].includes(value.code??''))throw Error('maintenance_protocol_invalid');
  if(value.summary!==undefined){
    const s=value.summary,keys=['files','bytes','application_version','encryption','authenticated','published','database_checked'];
    if(!s||Object.keys(s).length!==keys.length||Object.keys(s).some(key=>!keys.includes(key))||
       !Number.isSafeInteger(s.files)||s.files<0||s.files>100000||!Number.isSafeInteger(s.bytes)||s.bytes<0||s.bytes>100*1024**3||
       typeof s.application_version!=='string'||Buffer.byteLength(s.application_version)>256||/\p{Cc}/u.test(s.application_version)||
       !['none','age-v1'].includes(s.encryption)||s.authenticated!==(s.encryption==='age-v1')||
       typeof s.published!=='boolean'||typeof s.database_checked!=='boolean')throw Error('maintenance_protocol_invalid');
  }
  if(value.outcome==='succeeded'&&!value.summary)throw Error('maintenance_protocol_invalid');
  if(value.retained!==undefined&&(!Array.isArray(value.retained)||value.retained.length>4||value.retained.some(item=>
    !item||Object.keys(item).some(key=>!['path','directory','contains_plaintext'].includes(key))||
    typeof item.path!=='string'||!item.path||Buffer.byteLength(item.path)>4096||/[\x00\r\n]/.test(item.path)||
    typeof item.directory!=='boolean'||typeof item.contains_plaintext!=='boolean')))throw Error('maintenance_protocol_invalid');
  return value;
}

// One fixed helper command, private bounded framing and no raw diagnostic/log
// forwarding. The caller owns reviewed paths and ephemeral credentials.
class MaintenanceProcess extends EventEmitter{
  constructor(options={}){super();this.options={spawn,stopTimeoutMs:45000,...options};this.child=null;this.result=null;this.uncertain=false;}
  run(binary,request,env){
    if(this.child||!operationID(request.id))throw Error('maintenance_request_invalid');
    const encoded=frame(request);
    let child;
    try{child=this.options.spawn(binary,['workspace','--desktop-stdio'],{shell:false,windowsHide:true,stdio:['pipe','pipe','pipe'],env});}catch(error){encoded.fill(0);throw error;}
    this.child=child;this.id=request.id;
    let pending=Buffer.alloc(0),finished=false,working=false,protocolFailure=false;
    this.done=new Promise(resolve=>{
      const fail=()=>{protocolFailure=true;void this.cancel();};
      child.stdout.on('data',chunk=>{
        if(protocolFailure)return;
        if(pending.length+chunk.length>2*(MAX_RESULT+4)){fail();return;}
        pending=Buffer.concat([pending,chunk]);
        while(pending.length>=4){
          const length=pending.readUInt32BE();if(length<1||length>MAX_RESULT){fail();return;}
          if(pending.length<length+4)return;
          let value;try{value=validateMessage(JSON.parse(pending.subarray(4,length+4).toString('utf8')),request.id);}catch{fail();return;}
          pending=pending.subarray(length+4);
          if(finished||value.phase==='working'&&working){fail();return;}
          if(value.phase==='working'){working=true;this.emit('progress',value);}
          else {finished=true;this.result=value;}
        }
      });
      child.stderr.resume();
      const settle=(code,signal)=>{
        if(this.child!==child)return;
        clearTimeout(this.stopTimer);this.child=null;
        const expected=this.result?.outcome==='succeeded'?0:this.result?.outcome==='cancelled'?130:this.result?.outcome==='failed'?1:undefined;
        if(protocolFailure||pending.length||!finished||signal||code!==expected){this.uncertain=true;resolve({phase:'uncertain',code:'maintenance_unconfirmed',result:this.result});}
        else resolve({phase:this.result.outcome,result:this.result});
      };
      child.once('error',()=>settle(undefined));child.once('close',settle);
      child.stdin.on('error',()=>{if(this.child===child&&!finished)protocolFailure=true;});
      child.stdin.write(encoded,()=>encoded.fill(0));
    });
    return this.done;
  }
  cancel(){
    const child=this.child;if(!child)return this.done??Promise.resolve();
    if(this.cancelled)return this.cancelled;
    this.cancelled=new Promise(resolve=>{
      const uncertain=()=>{if(this.child!==child)return;this.uncertain=true;this.emit('uncertain');resolve({phase:'uncertain',code:'maintenance_unconfirmed',result:this.result});};
      this.stopTimer=setTimeout(uncertain,this.options.stopTimeoutMs);
      this.done?.then(resolve);
      try{child.stdin.end(frame({protocol:1,kind:'cancel',id:this.id},4096));}catch{uncertain();}
    });
    return this.cancelled;
  }
}
module.exports={MaintenanceProcess,frame,validateMessage,MAX_REQUEST,MAX_RESULT};
