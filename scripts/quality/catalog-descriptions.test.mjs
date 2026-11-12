import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync,readdirSync} from 'node:fs';
import {resolve} from 'node:path';
const root=resolve(import.meta.dirname,'../..'),read=file=>readFileSync(resolve(root,file),'utf8');
test('every built-in service description is tracked with its exact source',()=>{
 const expected={};
 for(const match of read('internal/models/catalog_current.go').matchAll(/entry\("([^"]+)", "[^"]+", ("(?:[^"\\]|\\.)*")/g))expected[match[1]]=JSON.parse(match[2]);
 for(const match of read('internal/models/catalog.go').matchAll(/ID:\s*"([^"]+)",[\s\S]*?Description:\s*("(?:[^"\\]|\\.)*")/g))expected[match[1]]=JSON.parse(match[2]);
 const speech=read('internal/models/package_catalog.go').match(/Description:\s*("(?:[^"\\]|\\.)*")/);assert.ok(speech);
 expected.speechPackage=JSON.parse(speech[1]);assert.ok(Object.keys(expected).length>=24);
 assert.deepEqual(JSON.parse(read('apps/web/src/i18n/catalog-descriptions.json')),expected);
 const resolver=read('apps/web/src/i18n/catalog-descriptions.ts');
 for(const file of readdirSync(resolve(root,'internal/models/catalog-packages')).filter(f=>f.endsWith('.json'))){
  const id=JSON.parse(read('internal/models/catalog-packages/'+file)).manifest.id;
  assert.ok(resolver.includes("'"+id+"'"),'Unmapped curated package '+id);
 }
});
