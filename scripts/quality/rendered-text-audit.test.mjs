import {test} from 'node:test';
import assert from 'node:assert/strict';
import {classifyRendered} from './rendered-text-audit.mjs';
test('rendered audit detects missing and stale body copy, placeholders and attributes',()=>{
 const entries=[{namespace:'voice',key:'body',source:'Choose a model.',currentText:'Choose a model.',coverage:'missing'},
 {namespace:'voice',key:'label',source:'Voice',currentText:'语音',coverage:'translated'},
 {namespace:'terminal',key:'count',source:'{count} lines',currentText:'{count} 行',coverage:'translated'}];
 const rows=['Choose a model.','Voice','语音','1,000 行','Server-authored description','Qwen3.5'].map(text=>({kind:'title',text,element:'#control'}));
 assert.deepEqual(classifyRendered(rows,entries,'zh-Hans',[{text:'Qwen3.5',reason:'Model identifier'}]).map(row=>row.status),['untranslated','untranslated','catalogued','catalogued','unclassified','exempt']);
});
test('same English spelling is not automatically a defect; exclusions are element scoped',()=>{
 const entries=[{namespace:'nav',key:'chat',source:'Chat',currentText:'Chat',coverage:'same-as-source'}];
 const rows=[{text:'Chat',element:'#chat'},{text:'Fixture',element:'#name'},{text:'Fixture',element:'#help'}];
 assert.deepEqual(classifyRendered(rows,entries,'es',[{text:'Fixture',element:'#name',reason:'User title'}]).map(row=>row.status),['catalogued','exempt','unclassified']);
});
