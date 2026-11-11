const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');

const input=()=>({registry:{en:{fallback:'en',direction:'ltr'},fr:{fallback:'fr',direction:'ltr'},ja:{fallback:'en',direction:'ltr'}},
 tables:[{id:'core',file:'fixture.ts',values:{en:{title:'Title',count:'{count} files'},fr:{title:'Titre',count:'{count} fichiers'}}},
 {id:'feature',file:'feature.ts',values:{en:{body:'Body',cancel:'Cancel'},fr:{body:'Contenu',cancel:'Annuler'}}}],
 packs:{ja:{messages:{title:'タイトル'}}},overrides:{ja:{feature:{cancel:'キャンセル'}}},reviews:{}});

test('all-language audit counts feature fallbacks and separates completeness from review',async()=>{
 const {coverageReport,entriesFor}=await import('../../../scripts/quality/translation-review.mjs');
 const fixture=input(),report=coverageReport(fixture);
 assert.equal(report.ja.authored,2);assert.equal(report.ja.missing,2);assert.equal(report.ja.ready,false);
 assert.equal(report.fr.missing,0);assert.equal(report.fr.ready,false);assert.equal(report.en.ready,true);
 fixture.overrides.fr={feature:{body:'Texte révisé'}};
 assert.equal(entriesFor('fr',fixture).find(e=>e.key==='body').translation,'Texte révisé');
});

test('review import rejects stale sources, changed translations, broken tokens and lost entries',async()=>{
 const {reviewPack,validateReviewPack,importReview}=await import('../../../scripts/quality/translation-review.mjs');
 const fixture=input(),pack=reviewPack('ja',fixture);
 assert.deepEqual(validateReviewPack(pack,fixture),[]);
 const bad=mutate=>{const copy=structuredClone(pack);mutate(copy);return validateReviewPack(copy,fixture).join('\n');};
 assert.match(bad(p=>p.entries.pop()),/missing review entry/);
 assert.match(bad(p=>p.entries.push(p.entries[0])),/duplicate/);
 assert.match(bad(p=>p.entries[0].source='changed'),/changed source/);
 assert.match(bad(p=>p.entries[0].currentText='old'),/changed since export/);
 assert.match(bad(p=>p.entries[1].translation='ファイル'),/placeholders/);
 assert.match(bad(p=>p.entries[0].translation=''),/cannot erase/);
 assert.match(bad(p=>p.entries[0].review.status='approved'),/reviewer and evidence/);
 const translated=structuredClone(pack);translated.entries[1].translation='{count} 件のファイル';
 assert.equal(importReview(translated,fixture).overrides.ja.core.count,'{count} 件のファイル');
 assert.equal(fixture.overrides.ja.core,undefined,'validation/import preparation must not mutate live catalogs');
});

test('desktop and terminal resolve selected-language overrides without translating user values',()=>{
 const {localizedCopy,resolveMessages}=require('../locale-messages');
 const catalogs={en:{ready:'Ready',retry:'Retry'},fr:{ready:'Prêt',retry:'Réessayer'}};
 assert.deepEqual(localizedCopy(catalogs,'ja','desktop/presentation',{ja:{'desktop/presentation':{ready:'準備完了'}}}),{ready:'準備完了',retry:'Retry'});
 assert.equal(localizedCopy(catalogs,'fr','desktop/presentation').ready,'Prêt');
 assert.deepEqual(resolveMessages({permissions:['Read','Write']},{'permissions.0':'読む'}),{permissions:['読む','Write']});
 const sandbox={};
 for(const file of ['terminal-accessibility.js','terminal-translations.js'])vm.runInNewContext(fs.readFileSync(path.join(__dirname,'../'+file),'utf8'),sandbox);
 sandbox.terminalTranslations.configure('ja',{'Close {title}':'{title} を閉じる'});
 assert.equal(sandbox.terminalTranslations.text('ja','Close {title}',{title:'User <input> $&.txt'}),'User <input> $&.txt を閉じる');
 assert.equal(sandbox.terminalTranslations.text('ja','Done'),'Done');
});

test('every selectable language is exported with the full contextual message inventory',async()=>{
 const {inputs,reviewPack,coverageReport}=await import('../../../scripts/quality/translation-review.mjs');
 const actual=inputs(),report=coverageReport(actual),total=report.en.total;
 assert.ok(total>1300);assert.equal(Object.keys(report).length,29);
 for(const language of Object.keys(actual.registry)){
  const pack=reviewPack(language,actual);
  assert.equal(pack.entries.length,total,language);
  assert.ok(pack.entries.some(e=>e.namespace==='desktop/terminal'));
  assert.ok(pack.entries.some(e=>e.namespace==='desktop/folder-trust'));
  assert.ok(pack.entries.some(e=>e.namespace==='i18n/computer-experience:copy'));
 }
});

test('failed import replacement rolls back sources and preserves pre-existing staging files',async()=>{
 const {applyImport}=await import('../../../scripts/quality/translation-review.mjs');
 const dir=fs.mkdtempSync(path.join(require('node:os').tmpdir(),'offgrid-translation-import-'));
 const file=path.join(dir,'feature-packs.json');fs.writeFileSync(file,'original');
 let count=0;
 const io={...fs,renameSync:(from,to)=>{if(++count===2)throw Error('simulated write failure');fs.renameSync(from,to);}};
 assert.throws(()=>applyImport({overrides:{ja:{}},sources:{ja:{}},reviews:{}},dir,path.join(dir,'backup'),io),/simulated/);
 assert.equal(fs.readFileSync(file,'utf8'),'original');
 assert.equal(fs.existsSync(path.join(dir,'translation-sources.json')),false);
 fs.writeFileSync(file+'.pending','another import');
 assert.throws(()=>applyImport({overrides:{},sources:{},reviews:{}},dir,path.join(dir,'backup')),/EEXIST/);
 assert.equal(fs.readFileSync(file+'.pending','utf8'),'another import');
});

test('review approval expires when text changes and consequential copy needs a second reviewer',async()=>{
 const {reviewPack,validateReviewPack,coverageReport,digest}=await import('../../../scripts/quality/translation-review.mjs');
 const fixture=input();fixture.tables[1].values.en.body='Allow access';
 const pack=reviewPack('ja',fixture),entry=pack.entries.find(e=>e.key==='body');
 entry.translation='アクセスを許可';entry.review={status:'approved',reviewer:'fixture-reviewer',evidence:'test-only record'};
 assert.match(validateReviewPack(pack,fixture).join('\n'),/second reviewer/);
 entry.review.safetyReviewer='fixture-second-reviewer';assert.deepEqual(validateReviewPack(pack,fixture),[]);
 fixture.overrides.ja.feature.body=entry.translation;
 fixture.reviews.ja={feature:{body:{...entry.review,sourceHash:entry.sourceHash,translationHash:digest(entry.translation)}}};
 assert.equal(coverageReport(fixture).ja.reviewed,1);
 fixture.overrides.ja.feature.body='changed';assert.equal(coverageReport(fixture).ja.reviewed,0);
});
