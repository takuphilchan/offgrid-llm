const test=require('node:test'),assert=require('node:assert/strict');
const {create,decode,MAX_OUTPUT}=require('../terminal-command-context');
const {createContextDelivery}=require('../terminal-context-delivery');
const encode=value=>'OffGrid;'+Buffer.from(JSON.stringify({v:1,...value})).toString('base64');
function fixture(){
 let handler;const rows=[''],markers=[];
 const buffer={type:'normal',cursorX:0,getLine:i=>rows[i]===undefined?null:{translateToString:(_trim,start=0,end)=>rows[i].slice(start,end),isWrapped:false}};
 const term={buffer:{normal:buffer,active:buffer},parser:{registerOscHandler:(_code,fn)=>{handler=fn;return {dispose(){}};}},registerMarker(){const m={line:rows.length-1,isDisposed:false,dispose(){this.isDisposed=true;}};markers.push(m);return m;}};
 const tracker=create(term,{now:()=>10});
 return {tracker,markers,buffer,rows,send:value=>handler(encode(value)),line:text=>{rows[rows.length-1]=text;buffer.cursorX=text.length;}};
}
test('command output captures final unterminated line before next prompt and remains bounded',()=>{
 const f=fixture();f.send({event:'begin',command:'Write-Host café',cwd:'C:\\test',truncated:false});
 f.line('café 日本語');f.send({event:'end',exit:0});f.line('café 日本語PS next>');
 assert.deepEqual(f.tracker.output(f.tracker.commands[0]),{text:'café 日本語',truncated:false,available:true});
 f.rows.push('');f.buffer.cursorX=0;f.send({event:'begin',command:'large',cwd:'C:\\test',truncated:false});
 f.line('x'.repeat(MAX_OUTPUT+1));f.send({event:'end',exit:1});
 const output=f.tracker.output(f.tracker.commands[1]);assert.equal(output.text.length,MAX_OUTPUT);assert.equal(output.truncated,true);
});
test('malformed markers, alternate buffers, missing ends and trimmed starts cannot claim success',()=>{
 for(const value of ['invalid','OffGrid;!!!',encode({event:'begin',command:'x',cwd:'\x1b',truncated:false}),encode({event:'end',exit:'0'}),encode({event:'begin',command:'x'.repeat(4097),cwd:'',truncated:false})])assert.equal(decode(value),null);
 const f=fixture();f.buffer.type='alternate';f.send({event:'begin',command:'x',cwd:'',truncated:false});assert.equal(f.tracker.commands.length,0);
 f.buffer.type='normal';f.send({event:'begin',command:'first',cwd:'',truncated:false});f.markers[0].dispose();
 f.send({event:'begin',command:'second',cwd:'',truncated:false});
 assert.equal(f.tracker.commands[0].exit,null);assert.equal(f.tracker.output(f.tracker.commands[0]).available,false);
 f.tracker.interrupt();assert.equal(f.tracker.commands[1].exit,null);
 for(let i=0;i<110;i++)f.send({event:'begin',command:String(i),cwd:'',truncated:false});
 assert.equal(f.tracker.commands.length,100);f.tracker.dispose();assert.equal(f.tracker.commands.length,0);
});
test('context delivery requires current acknowledgment, rejects overlap and preserves retry on failure',async()=>{
 let sent;const delivery=createContextDelivery({send:value=>{sent=value;},timeout:20});
 const promise=delivery.deliver({output:'selected'});
 await assert.rejects(delivery.deliver({output:'other'}),/already opening/);
 assert.equal(delivery.acknowledge('stale',true),false);
 assert.equal(delivery.acknowledge(sent.id,true),true);assert.deepEqual(await promise,{opened:true});
 await assert.rejects(delivery.deliver({output:'selected'}),/could not accept/);
 const next=delivery.deliver({output:'selected'});delivery.cancel();await assert.rejects(next,/could not accept/);
});

test('shell failure without an exact code stays distinct from an interrupted command',()=>{
 const f=fixture();f.send({event:'begin',command:'failed cmdlet',cwd:'',truncated:false});
 f.send({event:'end',exit:null,failed:true});
 assert.equal(f.tracker.commands[0].failed,true);assert.equal(f.tracker.commands[0].exit,null);
 f.send({event:'begin',command:'interrupted',cwd:'',truncated:false});f.tracker.interrupt();
 assert.equal(f.tracker.commands[1].failed,false);assert.equal(f.tracker.commands[1].exit,null);
 assert.equal(decode(encode({event:'end',exit:null})),null);
});
