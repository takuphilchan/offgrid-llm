import {test} from 'node:test';
import assert from 'node:assert/strict';
import {validateLanguageScript} from './language-script-check.mjs';
test('rejects wrong-language and mixed-script drafts without treating script checks as review',()=>{
 assert.ok(validateLanguageScript('Select document','문서를 선택하세요','ta').length);
 assert.ok(validateLanguageScript('Select document','ஆவணம் 문서','ta').length);
 assert.ok(validateLanguageScript('Select document','Select document','zh-Hans').length);
 assert.deepEqual(validateLanguageScript('Select document','ஆவணத்தைத் தேர்ந்தெடுக்கவும்','ta'),[]);
 assert.deepEqual(validateLanguageScript('Select document','文書を選択','ja'),[]);
 assert.ok(validateLanguageScript('Settings','设置','zh-Hant').length);
 assert.ok(validateLanguageScript('Settings','設定','zh-Hans').length);
 assert.deepEqual(validateLanguageScript('Settings','設定','zh-Hant'),[]);
 assert.deepEqual(validateLanguageScript('Settings','设置','zh-Hans'),[]);
});
test('preserves identifiers, placeholders and scripts explicitly present in source',()=>{
 for(const source of ['OffGrid CLI','Hugging Face','CPU','API','tokens/s','{count}'])assert.deepEqual(validateLanguageScript(source,source,'ta'),[]);
 assert.deepEqual(validateLanguageScript('Open 日本語','日本語 திறக்கவும்','ta'),[]);
});
