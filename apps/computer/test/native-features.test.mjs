import test from 'node:test';
import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
const {nativeFeatures,requestedNativeFeatures,verifyNativeFeatures}=createRequire(import.meta.url)('../native-features.cjs');

test('document negotiation remains empty for either legacy endpoint',()=>{
  const service={capabilities:['native-document-negotiation-v1']};
  assert.deepEqual(requestedNativeFeatures(service,{}),{});
  assert.deepEqual(requestedNativeFeatures({capabilities:[]},{features:{native_documents:1}}),{});
  assert.deepEqual(requestedNativeFeatures(service,{features:{native_documents:1}}),{native_documents:1});
  assert.deepEqual(verifyNativeFeatures({},undefined),{});
  assert.deepEqual(verifyNativeFeatures({native_documents:1},{native_documents:1}),{native_documents:1});
  assert.throws(()=>verifyNativeFeatures({}, {native_documents:1}),/protocol_invalid/);
  assert.throws(()=>verifyNativeFeatures({native_documents:1},{}),/protocol_invalid/);
});

test('document feature names and revisions are closed',()=>{
  for(const value of [null,[],true,{native_documents:0},{native_documents:2},{native_documents:'1'},{native_documents:1,shell:1},{vision:1}])assert.throws(()=>nativeFeatures(value),/protocol_invalid/);
});
