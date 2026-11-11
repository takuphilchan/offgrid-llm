import {test} from 'node:test';
import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {previewVersion} from './desktop-preview.mjs';
const require=createRequire(import.meta.url);
const {buildVersion}=require('../../apps/desktop/build-version.cjs');
test('preview identity is distinguishable, monotonic by time, and propagated from builder metadata',()=>{
 const first=previewVersion('0.4.28','2054693b',new Date('2026-11-11T16:00:00Z'));
 const next=previewVersion('0.4.28','2054693b',new Date('2026-11-11T16:00:01Z'));
 assert.match(first,/^0\.4\.29-preview\./);assert.notEqual(first,next);
 assert.equal(buildVersion({packager:{appInfo:{version:first}}}),first);
 assert.throws(()=>buildVersion({}),/Missing/);
 assert.throws(()=>previewVersion('dev','2054693b'),/stable base/);
});
