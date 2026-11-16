import {expect,test}from'@playwright/test';
import {craftFixture}from'./helpers/craft-fixture';
import {languageRegistry,type LanguageCode}from'../src/i18n/language';
import {linkedEvidenceNotice}from'../src/i18n/linked-evidence';

for(const locale of Object.keys(languageRegistry) as LanguageCode[])test(`saved missing-link receipt remains visible in ${locale}`,async({page})=>{
 await craftFixture(page,locale,'light');await page.setViewportSize({width:390,height:844});
 const session={name:'Demo briefing',messages:[{role:'assistant',content:'17.25 [1]',knowledge_sources:[{document_id:'doc',chunk_id:'value',name:'Policy',content:'17.25',content_hash:'a'.repeat(32),evidence_receipt:{extraction_revision:'extraction-old',group_id:'value',required_chunks:['value','header'],links_complete:false}}]}]};
 await page.route('**/v1/sessions',r=>r.fulfill({json:{sessions:[session]}}));
 await page.route('**/v1/sessions/Demo%20briefing',r=>r.fulfill({json:session}));
 await page.goto('/ui/#/chat');
 await page.locator('.knowledge-passages > summary').click();
 await expect(page.locator('[data-linked-evidence=incomplete]')).toHaveText(linkedEvidenceNotice(locale));
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 if(locale!=='en')expect(linkedEvidenceNotice(locale)).not.toEqual(linkedEvidenceNotice('en'));
 session.messages[0].knowledge_sources[0].evidence_receipt.links_complete=true;
 await page.reload();await page.locator('.knowledge-passages > summary').click();
 await expect(page.locator('[data-linked-evidence=incomplete]')).toHaveCount(0);
});

for(const locale of Object.keys(languageRegistry) as LanguageCode[])test(`private attachment missing-link receipt remains visible in ${locale}`,async({page})=>{
 await craftFixture(page,locale,'dark');await page.setViewportSize({width:390,height:844});
 const session={name:'Demo briefing',messages:[{role:'assistant',content:'17.25 [S1.1]',attachment_selections:[{id:'file-'+ 'a'.repeat(32),revision:'b'.repeat(64),name:'Policy.docx',mode:'excerpts',total_bytes:9000,ranges:[[0,5]],linked_evidence:{extraction_revision:'extraction-old',links_complete:false,omitted_blocks:['exception']}}]}]};
 await page.route('**/v1/sessions',r=>r.fulfill({json:{sessions:[session]}}));
 await page.route('**/v1/sessions/Demo%20briefing',r=>r.fulfill({json:session}));
 await page.goto('/ui/#/chat');
 await expect(page.locator('[data-linked-evidence=incomplete]')).toHaveText(linkedEvidenceNotice(locale));
 await expect(page.locator('.citation-chip')).toHaveCount(1);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 session.messages[0].attachment_selections[0].linked_evidence.links_complete=true;
 await page.reload();await expect(page.locator('.citation-chip')).toHaveCount(1);
 await expect(page.locator('[data-linked-evidence=incomplete]')).toHaveCount(0);
});
