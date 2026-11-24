// This fixed selector is invoked alone against an already retained ElementHandle.
// It runs in Playwright's isolated content-script world, never the page world.
// It returns only that exact scope node: no DOM writes, selector retargeting,
// detached carrier, arbitrary expression or file contents cross into the page.
export function fileReadbackEngine() {
  const records=new Map(), chunkLimit=1024*1024;
  const getter=(prototype,name)=>Object.getOwnPropertyDescriptor(prototype,name).get;
  const files=getter(HTMLInputElement.prototype,'files'),type=getter(HTMLInputElement.prototype,'type');
  const connected=getter(Node.prototype,'isConnected'),owner=getter(Node.prototype,'ownerDocument');
  const length=getter(FileList.prototype,'length'),item=FileList.prototype.item;
  const size=getter(Blob.prototype,'size'),name=getter(File.prototype,'name');
  const slice=Blob.prototype.slice,read=FileReader.prototype.readAsArrayBuffer,abort=FileReader.prototype.abort;
  const result=getter(FileReader.prototype,'result'),digest=crypto.subtle.digest.bind(crypto.subtle);
  const release=record=>{
    if(!record)return;
    records.delete(record.token);record.retired=true;clearTimeout(record.timer);
    if(record.reader){abort.call(record.reader);record.reader=null;}
    record.file=null;record.root=null;
  };
  const current=(root,record)=>{
    if(!connected.call(root)||owner.call(root)!==document||type.call(root)!=='file')throw Error('File readback target changed.');
    const list=files.call(root);
    if(length.call(list)!==1)throw Error('File readback selection changed.');
    const file=item.call(list,0);
    if(record&&(record.root!==root||record.file!==file))throw Error('File readback selection changed.');
    return file;
  };
  const query=(root,selector)=>{
    const command=JSON.parse(selector);
    if(!command||! /^[a-f0-9]{32}$/.test(command.token??''))throw Error('Invalid file readback token.');
    let record=records.get(command.token);
    if(command.op==='release'){if(record&&record.root!==root)throw Error('File readback target changed.');release(record);return root;}
    if(command.op==='start'){
      if(record||records.size>=8)throw Error('File readback is already active.');
      const file=current(root);
      if(name.call(file)!==command.name||size.call(file)!==command.size)throw Error('File readback metadata differs.');
      record={token:command.token,root,file,size:command.size,offset:0,done:false,retired:false};
      record.timer=setTimeout(()=>release(record),120000);records.set(command.token,record);return root;
    }
    if(!record)throw Error('File readback expired.');
    current(root,record);
    if(record.error)throw Error('File readback content differs or could not be read.');
    if(command.op==='chunk'){
      if(record.reader||record.pending||command.offset!==record.offset||!Number.isSafeInteger(command.length)||command.length<1||command.length>chunkLimit||command.length>record.size-record.offset||! /^[a-f0-9]{64}$/.test(command.sha256??''))throw Error('Invalid file readback chunk.');
      record.done=false;record.pending=true;
      const reader=new FileReader();record.reader=reader;
      reader.onerror=()=>{if(!record.retired){record.error=true;record.pending=false;record.reader=null;}};
      reader.onabort=reader.onerror;
      reader.onload=async()=>{
        let bytes;
        try{
          bytes=new Uint8Array(result.call(reader));
          const hash=Array.from(new Uint8Array(await digest('SHA-256',bytes)),byte=>byte.toString(16).padStart(2,'0')).join('');
          if(record.retired)return;
          current(root,record);
          if(bytes.byteLength!==command.length||hash!==command.sha256)throw Error('File readback differs.');
          record.offset+=bytes.byteLength;record.done=true;
        }catch{if(!record.retired)record.error=true;}
        finally{bytes?.fill(0);record.pending=false;record.reader=null;}
      };
      read.call(reader,slice.call(record.file,record.offset,record.offset+command.length));return root;
    }
    if(command.op==='poll')return record.done?root:null;
    if(command.op==='finish'){
      if(record.pending||record.offset!==record.size)throw Error('File readback incomplete.');
      release(record);return root;
    }
    throw Error('Unknown file readback operation.');
  };
  return {query,queryAll:(root,selector)=>{const match=query(root,selector);return match?[match]:[];}};
}
