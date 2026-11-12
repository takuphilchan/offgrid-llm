import {test} from 'node:test';
import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {resolve} from 'node:path';
import {isUiLiteral} from './translation-inventory.mjs';
import {validateUiLiterals} from './ui-literal-check.mjs';
const ts=createRequire(resolve(import.meta.dirname,'../../apps/web/package.json'))('typescript');
test('literal audit catches visible text and accessible copy, including lowercase labels',()=>{
 const ast=ts.createSourceFile('fixture.tsx',`const x=<div><button aria-label="open menu">Open</button><input placeholder="search models" data-testid="model-id"/>{'Save'}{busy?'Waiting':'Continue'}{copy.title}</div>`,ts.ScriptTarget.Latest,true);
 const found=[];function visit(node){if(isUiLiteral(node))found.push(node.text.trim());ts.forEachChild(node,visit);}visit(ast);
 assert.deepEqual(found,['open menu','Open','search models','Save','Waiting','Continue']);
});
test('literal allowances are exact, scoped and must have an explanation',()=>{
 const allowed=[{file:'view.tsx',text:'OffGrid',reason:'Brand'}];
 assert.deepEqual(validateUiLiterals([{file:'view.tsx',text:'OffGrid',line:1}],allowed),[]);
 assert.ok(validateUiLiterals([{file:'other.tsx',text:'OffGrid',line:1}],allowed).length);
 assert.ok(validateUiLiterals([{file:'view.tsx',text:'Open settings',line:1}],allowed).length);
 assert.ok(validateUiLiterals([],allowed).length);
});

test('literal audit includes short labels, other scripts and JSX template fragments',()=>{
 const ast=ts.createSourceFile('fixture.tsx','const x=<div><button>No</button><span>设置</span>{`Found ${count} files`}<input aria-label={`Open ${name}`}/></div>',ts.ScriptTarget.Latest,true);
 const found=[];function visit(node){if(isUiLiteral(node))found.push(node.text.trim());ts.forEachChild(node,visit);}visit(ast);
 assert.deepEqual(found,['No','设置','Found','files','Open']);
});
