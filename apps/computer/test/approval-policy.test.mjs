import test from 'node:test';
import assert from 'node:assert/strict';
import {classifyBrowserAction,policyAllows} from '../approval-policy.mjs';

test('browser approval modes preserve non-bypassable blocks',()=>{
  for (const label of ['Save draft', 'Save', 'Apply', 'Add', 'Update draft']) {
    const classification = classifyBrowserAction('browser_click',{label,tag:'button'});
    assert.equal(classification,'consequential', 'page labels cannot establish reversibility');
    assert.equal(policyAllows('scoped_changes',classification),false);
  }
  assert.equal(classifyBrowserAction('browser_click',{label:'Submit report',tag:'button'}),'consequential');
  assert.equal(classifyBrowserAction('browser_click',{label:'Pay now',tag:'button'}),'forbidden');
  assert.equal(policyAllows('ask_every_time','reversible'),false);
  assert.equal(policyAllows('scoped_changes','reversible'),true);
  assert.equal(policyAllows('scoped_changes','consequential'),false);
  assert.equal(policyAllows('full_task','consequential'),true);
  for(const mode of ['ask_every_time','scoped_changes','full_task'])assert.equal(policyAllows(mode,'forbidden'),false);
});
