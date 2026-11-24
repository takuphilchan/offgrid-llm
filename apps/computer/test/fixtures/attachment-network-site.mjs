import {createServer} from 'node:https';
import {createHash} from 'node:crypto';
import {readFileSync} from 'node:fs';

// All listeners and WebSocket frames belong to this isolated diagnostic server.
export async function startAttachmentNetworkSite() {
  const sockets=new Set(), counts={requests:[],messages:[],upgrades:[]};
  const server=createServer({
    key:readFileSync(new URL('./localhost-test-key.pem',import.meta.url)),
    cert:readFileSync(new URL('./localhost-test-cert.pem',import.meta.url)),
  }, (request,response) => {
    const url=new URL(request.url,`https://${request.headers.host}`);
    if(counts.requests.length<128)counts.requests.push({host:url.hostname,path:url.pathname,method:request.method,remote:request.socket.localAddress});
    response.setHeader('Access-Control-Allow-Origin','*');
    response.setHeader('Cache-Control','no-store');
    if(url.pathname==='/selected') {
      response.setHeader('Content-Type','text/html');
      response.end(`<!doctype html><title>Attachment selected fixture</title>
        <button id="send">Send existing socket</button><button id="private">Private destination</button>
        <script>
        const port=location.port;
        const socket=new WebSocket('wss://sink.offgrid-fixture.example:'+port+'/socket');
        window.socketReady=new Promise((resolve,reject)=>{socket.onopen=resolve;socket.onerror=reject});
        socket.onmessage=event=>{window.lastMessage=event.data};
        document.querySelector('#send').onclick=()=>socket.send('effect-after-install');
        document.querySelector('#private').onclick=()=>{
          window.privateDone=fetch('https://private.offgrid-fixture.example:'+port+'/private-effect',{method:'POST',body:'synthetic-effect'})
            .then(response=>response.text()).catch(error=>'failed:'+error.name);
        };
        </script>`);
      return;
    }
    if(url.pathname==='/sentinel') { response.end('<!doctype html><title>Unrelated sentinel</title><p>keep this tab</p>');return; }
    response.end('fixture-response');
  });
  server.on('connection',socket=>{sockets.add(socket);socket.once('close',()=>sockets.delete(socket));});
  server.on('upgrade',(request,socket)=>{
    counts.upgrades.push({host:request.headers.host,path:request.url});
    const accept=createHash('sha1').update(request.headers['sec-websocket-key']+'258EAFA5-E914-47DA-95CA-C5AB0DC85B11').digest('base64');
    socket.write(`HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: ${accept}\r\n\r\n`);
    let pending=Buffer.alloc(0);
    socket.on('data',chunk=>{
      pending=Buffer.concat([pending,chunk]);
      if(pending.length>4096){socket.destroy();return;}
      while(pending.length>=2) {
        const opcode=pending[0]&15, masked=!!(pending[1]&128), length=pending[1]&127;
        if(length>125||!masked){socket.destroy();return;}
        if(pending.length<6+length)return;
        const mask=pending.subarray(2,6), payload=Buffer.from(pending.subarray(6,6+length));
        for(let i=0;i<length;i++)payload[i]^=mask[i%4];
        pending=pending.subarray(6+length);
        if(opcode===8){socket.end(Buffer.from([0x88,0]));return;}
        if(opcode!==1){socket.destroy();return;}
        if(counts.messages.length<128)counts.messages.push(payload.toString('utf8'));
        socket.write(Buffer.concat([Buffer.from([0x81,payload.length]),payload]));
      }
    });
  });
  await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve);});
  return {counts,port:server.address().port,async close(){
    for(const socket of sockets)socket.destroy();
    await new Promise(resolve=>server.close(resolve));
  }};
}
