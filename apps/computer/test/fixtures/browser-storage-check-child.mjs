// Disposable subprocess protocol fixture. It never protects or accesses storage.
if(process.env.OFFGRID_STORAGE_CHECK_FIXTURE!=='1')throw Error('fixture only');
const chunks=[];for await(const chunk of process.stdin)chunks.push(chunk);
const input=Buffer.concat(chunks),request=JSON.parse(input.subarray(4));
if(input.readUInt32BE()!==input.length-4||request.protocol!==1||request.kind!=='prepare_browser_storage'||!Array.isArray(request.workspace_roots)||Object.keys(request).length!==3)process.exit(9);
const mode=process.argv[2],value={protocol:1,id:'browser-storage',result:{ready:true,browser_storage:1}};
let body=Buffer.from(JSON.stringify(value));
if(mode==='duplicate')body=Buffer.from('{"protocol":1,"protocol":1,"id":"browser-storage","result":{"ready":true,"browser_storage":1}}');
if(mode==='extra')body=Buffer.from(JSON.stringify({...value,path:'must-not-escape'}));
if(mode==='version')body=Buffer.from(JSON.stringify({...value,protocol:2}));
if(mode==='utf8')body=Buffer.from([0xc0,0xaf]);
const header=Buffer.alloc(4);header.writeUInt32BE(mode==='oversize'?0xffffffff:body.length);
const frame=Buffer.concat([header,body]);
if(mode==='hang'){setInterval(()=>{},1000);}
else{
 process.stdout.write(frame.subarray(0,3));process.stdout.write(frame.subarray(3));
 if(mode==='double')process.stdout.write(frame);
 if(mode==='hold')setInterval(()=>{},1000);
 if(mode==='exit-error')process.exitCode=3;
}
