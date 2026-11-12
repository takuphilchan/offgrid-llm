import {test} from 'node:test';
import assert from 'node:assert/strict';
import {inputs,entriesFor} from './translation-review.mjs';
import {terminologyReport} from './translation-consistency.mjs';
const input=inputs();
test('all 29 languages share the Chat destination name and have complete terminal copy',()=>{
 assert.equal(Object.keys(input.registry).length,29);
 for(const [language,row]of Object.entries(terminologyReport(input))){
  assert.deepEqual(row.failures,[],language);
  const terminal=entriesFor(language,input).filter(entry=>entry.namespace==='desktop/terminal');
  assert.ok(terminal.length>=90);
  assert.deepEqual(terminal.filter(entry=>entry.coverage==='missing'),[],language);
 }
});
test('conflicting imported navigation terminology is reported for an existing language',()=>{
 const changed=structuredClone(input);
 changed.overrides.fr['desktop/terminal'].Chat='Inconsistent destination';
 assert.equal(terminologyReport(changed).fr.failures.length,1);
});
test('Voice preferences, runtime status, model-choice copy and terminal settings are authored in all languages',()=>{
 const namespaces=['i18n/voice-settings:copy','i18n/voice:runtimeCopy','i18n/model-choice:modelChoiceText'];
 for(const language of Object.keys(input.registry)){
  const entries=entriesFor(language,input).filter(entry=>namespaces.includes(entry.namespace)||entry.namespace==='i18n/settings-navigation:settingsNavigation'&&['descriptions.2','descriptions.3','browser'].includes(entry.key));
  assert.equal(entries.length,23,language);
  assert.deepEqual(entries.filter(entry=>entry.coverage==='missing'),[],language);
 }
});
