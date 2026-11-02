'use strict';
const { randomUUID } = require('node:crypto');

const PROTOCOL = 1;
const MAX_INPUT_BYTES = 16 * 1024;
const sessionID = value => typeof value === 'string' && /^[a-zA-Z0-9-]{1,80}$/.test(value);
function keys(value, allowed) {
  if (!value || typeof value !== 'object' || Array.isArray(value) || Object.keys(value).some(key => !allowed.includes(key))) throw Error('terminal_invalid_request');
}
function dimensions(value) {
  if (!Number.isInteger(value.cols) || value.cols < 2 || value.cols > 500 || !Number.isInteger(value.rows) || value.rows < 1 || value.rows > 200) throw Error('terminal_invalid_size');
}

// This bridge belongs ONLY to the bundled terminal renderer. The workspace
// preload never receives it, even when the workspace service is on localhost.
// A fresh handshake fences stale requests from a renderer that has reloaded.
function createTerminalProtocol({ trustedURL, contents, operations, available }) {
  let generation = '';
  const owned = new Set();
  function trusted(event) {
    const current = contents();
    if (!current || current.isDestroyed() || event.sender !== current || event.senderFrame !== current.mainFrame || event.senderFrame.url !== trustedURL || current.getURL() !== trustedURL) throw Error('terminal_untrusted_sender');
  }
  function ready() {
    if (!available()) throw Error('terminal_runtime_unavailable');
  }
  return {
    invalidate() { generation = ''; },
    releaseStopped() { owned.clear(); },
    async dispatch(event, request) {
      trusted(event);
      keys(request, ['protocol','generation','action','id','text','cols','rows','sequence','kind','height']);
      if (request.protocol !== PROTOCOL) throw Error('terminal_protocol_mismatch');
      if (request.action === 'handshake') {
        keys(request,['protocol','action']);
        generation = randomUUID();
        return { protocol: PROTOCOL, generation, available: !!available(), ...(operations.presentation ? {presentation:operations.presentation()} : {}) };
      }
      if (!generation || request.generation !== generation) throw Error('terminal_stale_renderer');
      const base = ['protocol','generation','action'];
      if(request.action==='hide'){keys(request,base);return operations.hide();}
      if(request.action==='layout'){keys(request,[...base,'height']);if(!Number.isInteger(request.height)||request.height<120||request.height>2000)throw Error('terminal_invalid_height');return operations.layout(request.height);}
      if(request.action==='copy'){keys(request,[...base,'text']);if(typeof request.text!=='string'||Buffer.byteLength(request.text)>1024*1024)throw Error('terminal_invalid_copy');return operations.copy(request.text);}
      ready();
      if (request.action === 'create') {
        keys(request,[...base,'kind','cols','rows']); dimensions(request);
        if (!['shell','offgrid'].includes(request.kind)) throw Error('terminal_invalid_kind');
        if (owned.size >= 4) throw Error('terminal_session_limit');
        // Reserve before the folder dialog/spawn await to prevent oversubscription.
        const reservation = randomUUID(); owned.add(reservation);
        try {
          const session = await operations.create({kind:request.kind,cols:request.cols,rows:request.rows});
          if (!session) return null; // Folder selection cancelled; no process.
          if (!sessionID(session.id)) throw Error('terminal_invalid_session');
          owned.add(session.id);
          if (request.generation !== generation) {
            await operations.close(session.id);
            owned.delete(session.id);
            throw Error('terminal_stale_renderer');
          }
          return session;
        } finally { owned.delete(reservation); }
      }
      if (!sessionID(request.id) || !owned.has(request.id)) throw Error('terminal_unknown_session');
      switch(request.action) {
        case 'attach':
          keys(request,[...base,'id']);
          return operations.attach(request.id);
        case 'input':
          keys(request,[...base,'id','text']);
          if (typeof request.text !== 'string' || !request.text.length || Buffer.byteLength(request.text,'utf8') > MAX_INPUT_BYTES) throw Error('terminal_invalid_input');
          return operations.input(request.id,request.text);
        case 'resize':
          keys(request,[...base,'id','cols','rows']); dimensions(request);
          return operations.resize(request.id,request.cols,request.rows);
        case 'ack':
          keys(request,[...base,'id','sequence']);
          if (!Number.isSafeInteger(request.sequence) || request.sequence < 1) throw Error('terminal_invalid_ack');
          return operations.ack(request.id,request.sequence);
        case 'close':
          keys(request,[...base,'id']);
          await operations.close(request.id); owned.delete(request.id); return {stopped:true};
        default: throw Error('terminal_invalid_action');
      }
    },
  };
}

module.exports = { createTerminalProtocol, PROTOCOL, MAX_INPUT_BYTES };
