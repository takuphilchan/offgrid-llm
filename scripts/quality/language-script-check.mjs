// Script checks detect wrong-language drafts, not linguistic quality. Technical
// identifiers and scripts already present in the source remain untouched.
const expected={
 'zh-Hans':['Han'],'zh-Hant':['Han'],ja:['Hiragana','Katakana','Han'],ko:['Hangul'],
 ru:['Cyrillic'],uk:['Cyrillic'],ar:['Arabic'],ur:['Arabic'],fa:['Arabic'],he:['Hebrew'],
 hi:['Devanagari'],bn:['Bengali'],ta:['Tamil'],th:['Thai']
};
const scripts=['Arabic','Han','Hiragana','Katakana','Hangul','Cyrillic','Devanagari','Bengali','Tamil','Thai','Hebrew','Telugu','Kannada','Malayalam','Sinhala','Greek','Armenian','Georgian','Ethiopic','Lao','Myanmar'];
const expressions=Object.fromEntries(scripts.map(script=>[script,new RegExp('\\p{Script='+script+'}','u')]));
const identifiers=new Set(['OffGrid','OffGrid CLI','Hugging Face','PowerShell','Cmd','Bash','Git Bash','Win32','AppKit','UI Automation','macOS','Windows','Linux','SHA-256','Enter','Shift+Enter','Ctrl+C','Ctrl+V','tokens/s']);
export function validateLanguageScript(source,translation,language){
 const allowed=expected[language];if(!allowed)return [];
 const errors=[],literal=source.replace(/\{[A-Za-z][\w]*\}/g,'');
 if(!identifiers.has(source)&&/[a-z]{3}/i.test(literal)&&!/^[-A-Z0-9_+./:%\s]*$/.test(literal)&&!allowed.some(script=>expressions[script].test(translation)))errors.push('expected '+language+' script is absent');
 for(const script of scripts)if(!allowed.includes(script)&&expressions[script].test(translation)&&!expressions[script].test(source))errors.push('unexpected '+script+' script in '+language+' translation');
 const wrongChineseForm=language==='zh-Hant'?/[这为与发云体国话启关设显则仅创从删请页边过进连传态时线选认说写读错档录]/gu:language==='zh-Hans'?/[這為與發雲體國話啟關設顯則僅創從刪請頁邊過進連傳態時線選認說寫讀錯檔錄]/gu:null;
 if(wrongChineseForm&&[...translation.matchAll(wrongChineseForm)].some(match=>!source.includes(match[0])))errors.push('Chinese text uses characters from the other script variant');
 return errors;
}
