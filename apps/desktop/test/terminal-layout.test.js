const test=require('node:test'),assert=require('node:assert/strict');
const {validateWorkspaceBounds,terminalBounds}=require('../terminal-layout');
test('terminal occupies the main workspace, preserving sidebar and zoom coordinates',()=>{
  const rect=validateWorkspaceBounds({x:248,y:0,width:952,height:800});
  assert.deepEqual(terminalBounds(rect,1200,800),{x:248,y:0,width:952,height:800});
  assert.deepEqual(terminalBounds({x:124,y:0,width:476,height:400},1200,800,2),{x:248,y:0,width:952,height:800});
  assert.deepEqual(terminalBounds({x:0,y:0,width:500,height:700},500,700),{x:0,y:0,width:500,height:700});
  const bounded=terminalBounds(rect,300,200);
  assert.ok(bounded.x+bounded.width<=300&&bounded.y+bounded.height<=200);
});
test('geometry messages cannot carry commands or invalid sizes',()=>{
  for(const value of [null,[],{}, {x:-1,y:0,width:100,height:100},{x:0,y:0,width:0,height:100},{x:0,y:0,width:NaN,height:100},{x:0,y:0,width:100,height:100,command:'shell'}])assert.throws(()=>validateWorkspaceBounds(value),/invalid_bounds/);
});

test('terminal never covers native caption controls, including before layout and at zoom',()=>{
  assert.equal(terminalBounds(undefined,1200,800,1,36).y,36);
  assert.deepEqual(terminalBounds({x:124,y:18,width:476,height:382},1200,800,2,36),{x:248,y:36,width:952,height:764});
  assert.equal(terminalBounds({x:0,y:0,width:1200,height:800},1200,800,1,36).y,36);
});
