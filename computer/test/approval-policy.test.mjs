import test from 'node:test';
import assert from 'node:assert/strict';
import {classifyBrowserAction,policyAllows} from '../approval-policy.mjs';

test('browser approval modes preserve non-bypassable blocks',()=>{
  assert.equal(classifyBrowserAction('browser_click',{label:'Save draft',tag:'button'}),'reversible');
  assert.equal(classifyBrowserAction('browser_click',{label:'Submit report',tag:'button'}),'consequential');
  assert.equal(classifyBrowserAction('browser_click',{label:'Pay now',tag:'button'}),'forbidden');
  assert.equal(policyAllows('ask_every_time','reversible'),false);
  assert.equal(policyAllows('scoped_changes','reversible'),true);
  assert.equal(policyAllows('scoped_changes','consequential'),false);
  assert.equal(policyAllows('full_task','consequential'),true);
  for(const mode of ['ask_every_time','scoped_changes','full_task'])assert.equal(policyAllows(mode,'forbidden'),false);
});
