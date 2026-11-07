const test=require('node:test'),assert=require('node:assert/strict');
const {terminalShortcut}=require('../terminal-shortcuts');
const control={type:'keydown',ctrlKey:true,code:'KeyC'};
test('copying selected output never sends Ctrl+C to a running command',()=>{
  assert.equal(terminalShortcut(control,'selected command'),'copy');
  assert.equal(terminalShortcut(control,''),'');
  assert.equal(terminalShortcut({...control,shiftKey:true},''),'copy');
  assert.equal(terminalShortcut({...control,ctrlKey:false,metaKey:true},''),'copy');
});
test('paste, key release, AltGr and ordinary typing remain native',()=>{
  for(const event of [{...control,code:'KeyV'},{...control,type:'keyup'},
    {...control,altKey:true},{...control,ctrlKey:false}]) {
    assert.equal(terminalShortcut(event,'selected output'),'');
  }
});
