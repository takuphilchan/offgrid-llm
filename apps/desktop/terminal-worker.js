'use strict';
// Runs inside the supervisor's kill-on-close OS job, never in a web renderer.
const readline = require('node:readline');
const pty = require('node-pty');
const { createTerminalOutput } = require('./terminal-output');
const send = value => process.stdout.write(JSON.stringify(value) + '\n');
let terminal, output, exit;
const finish = () => { if (exit && output.bufferedBytes === 0) { send({type:'exit',...exit}); exit = null; } };
readline.createInterface({input:process.stdin}).on('line', line => {
  try {
    if (Buffer.byteLength(line) > 128 * 1024) throw Error('terminal_frame_too_large');
    const request = JSON.parse(line);
    if (request.action === 'start') {
      if (terminal) throw Error('terminal_already_started');
      terminal = pty.spawn(request.file, request.args, {name:'xterm-256color',cols:request.cols,rows:request.rows,cwd:request.cwd,env:request.env,useConpty:true,useConptyDll:true});
      output = createTerminalOutput({send:frame=>send({type:'data',...frame}),pause:()=>terminal.pause(),resume:()=>terminal.resume(),overflow:()=>{send({type:'error',message:'Terminal output exceeded its buffer limit.'});process.exit(1);}});
      terminal.onData(data=>output.push(data));
      terminal.onExit(value=>{exit=value;finish();});
      send({type:'ready',pid:terminal.pid});
    } else if (!terminal) throw Error('terminal_not_started');
    else if (request.action === 'input') terminal.write(request.text);
    else if (request.action === 'resize') terminal.resize(request.cols,request.rows);
    else if (request.action === 'ack') {output.ack(request.sequence);finish();}
    else throw Error('terminal_invalid_action');
  } catch (error) {send({type:'error',message:error.message});process.exit(1);}
});
// The parent supervises EOF, crashes and process-tree termination. This worker
// never enumerates bare PIDs or claims that shell exit means descendants died.
