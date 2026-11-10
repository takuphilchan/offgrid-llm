(function(root, factory) {
  if (typeof module === 'object' && module.exports) module.exports = factory();
  else root.terminalCommandContext = factory();
})(typeof window === 'object' ? window : globalThis, function() {
  'use strict';
  const MAX_OUTPUT = 32768, MAX_COMMANDS = 100;
  const clean = (value, limit) => typeof value === 'string' && value.length <= limit && !/[\x00-\x08\x0b-\x1f\x7f]/.test(value);
  function decode(data) {
    if (!data.startsWith('OffGrid;') || data.length > 20000) return null;
    try {
      const bytes = Uint8Array.from(atob(data.slice(8)), c => c.charCodeAt(0));
      const value = JSON.parse(new TextDecoder('utf-8', {fatal:true}).decode(bytes));
      if (!value || value.v !== 1) return null;
      if (value.event === 'begin' && clean(value.command,4096) && clean(value.cwd,2048) && typeof value.truncated === 'boolean') return value;
      if (value.event === 'end' && ((Number.isInteger(value.exit) && Math.abs(value.exit) <= 2147483648) || (value.exit === null && value.failed === true))) return value;
      if (value.event === 'prompt' && clean(value.cwd,2048)) return value;
      if (value.event === 'unavailable') return value;
    } catch {}
    return null;
  }
  function create(term, {changed=()=>{},now=()=>performance.now()}={}) {
    let active=null, cwd='', available=false, nextID=0;
    const commands=[];
    function finish(exit, failed=false) {
      if (!active) return;
      active.exit=exit; active.failed=failed; active.duration=Math.max(0,now()-active.started); active.finished=true;
      active.end=term.registerMarker(0);
      active.endColumn=term.buffer.normal.cursorX;
      // Snapshot before the next prompt or resize changes the final output row.
      active.output=capture(active);
      active=null;
    }
    const handler=term.parser.registerOscHandler(633, data=>{
      const value=decode(data);
      if (!value) return data.startsWith('OffGrid;');
      // Alternate-screen programs cannot describe normal-buffer command blocks.
      if (term.buffer.active.type !== 'normal') return true;
      if (value.event==='begin') {
        finish(null); // Missing end or forged overlap is unknown, never success.
        available=true;cwd=value.cwd;
        active={id:++nextID,command:value.command,cwd,commandTruncated:value.truncated,started:now(),start:term.registerMarker(0),startColumn:term.buffer.normal.cursorX,finished:false,exit:null};
        commands.push(active);
        if(commands.length>MAX_COMMANDS){const old=commands.shift();old.start?.dispose();old.end?.dispose();}
      } else if(value.event==='end') finish(value.exit,value.failed===true);
      else if(value.event==='prompt') {cwd=value.cwd;}
      else {available=false;finish(null);}
      changed();return true;
    });
    function capture(command) {
      if(!command || !commands.includes(command) || !command.start || command.start.isDisposed || !command.finished || !command.end || command.end.isDisposed) return {text:'',truncated:true,available:false};
      const buffer=term.buffer.normal, end=command.end.line;
      let text='',truncated=!!command.commandTruncated;
      for(let line=command.start.line;line<=end;line++) {
        const row=buffer.getLine(line);
        if(!row)return {text:'',truncated:true,available:false};
        const next=row.translateToString(true,line===command.start.line?command.startColumn:0,line===end?command.endColumn:undefined)+(line===end||buffer.getLine(line+1)?.isWrapped?'':'\n');
        if(text.length+next.length>MAX_OUTPUT){text+=(next.slice(0,MAX_OUTPUT-text.length));truncated=true;break;}
        text+=next;
      }
      // Never keep a dangling surrogate at the output bound.
      if(/[\uD800-\uDBFF]$/.test(text))text=text.slice(0,-1);
      return {text,truncated,available:true};
    }
    return {commands,output:command=>commands.includes(command)&&command.output?{...command.output}:{text:'',truncated:true,available:false},get cwd(){return cwd;},get available(){return available;},get active(){return active;},
      interrupt(){finish(null);changed();},
      dispose(){handler.dispose();for(const c of commands){c.start?.dispose();c.end?.dispose();}commands.length=0;}
    };
  }
  return {create,decode,MAX_OUTPUT,MAX_COMMANDS};
});
