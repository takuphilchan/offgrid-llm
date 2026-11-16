import {expect,test,type Page} from '@playwright/test';
import {createHash} from 'node:crypto';
import {craftFixture} from './helpers/craft-fixture';
import {documentSynthesis} from '../src/i18n/document-synthesis';
import {documentComparison} from '../src/i18n/document-comparison';
import {documentReader} from '../src/i18n/document-reader';
import {synthesisReviewCopy} from '../src/i18n/document-synthesis-review';
import {languageRegistry,type LanguageCode} from '../src/i18n/language';
import type {Synthesis,SynthesisResults,SynthesisReview} from '../src/api/document-synthesis';

test.describe.configure({mode:'parallel'});
import {fixture,open,source} from './helpers/synthesis-fixture';

for(const locale of Object.keys(languageRegistry) as LanguageCode[])test(`synthesis evidence and controls ${locale}`,async({page},info)=>{
 const state=await fixture(page,locale),copy=documentSynthesis(locale),reader=documentReader(locale),dialog=await open(page,locale);
 await dialog.getByRole('radio',{name:copy.requirements,exact:true}).check();await dialog.locator('form').getByRole('button',{name:copy.create}).click();
 await expect(dialog.locator('.synthesis-candidate-text')).toContainText(copy.review);await expect(dialog.locator('.synthesis-quote')).toContainText(source);
 await expect(dialog.locator('.comparison-export > button')).toHaveText(synthesisReviewCopy(locale).exportDraft);
 expect(state.requests[0].workflow).toBe('requirements');expect(state.requests[0].language).toBe(locale);expect(state.requests[0].sources).toEqual([{document_id:'policy',revision:'rev1'}]);
 await dialog.locator('.synthesis-quote').click();await expect(dialog.locator('.synthesis-source')).toBeFocused();await expect(dialog.locator('.synthesis-source blockquote')).toHaveText(source);
 await dialog.locator('.synthesis-source').getByRole('button',{name:reader.close,exact:true}).click();await expect(dialog.locator('.synthesis-quote')).toBeFocused();
 const review=synthesisReviewCopy(locale);await expect(dialog.getByLabel(review.checked,{exact:true})).toBeDisabled();
 await dialog.locator('.synthesis-review-choice select').selectOption('keep');await dialog.getByLabel(review.note,{exact:true}).fill('Checked the deadline and its exception.');await dialog.getByLabel(review.checked,{exact:true}).check();
 await dialog.getByRole('button',{name:review.save,exact:true}).click();await expect(dialog.getByRole('status').filter({hasText:review.saved})).toBeVisible();expect(state.review?.section_checked).toBe(true);expect(state.reviewPosts).toBe(1);
 if(['en','ar'].includes(locale)){await page.screenshot({path:info.outputPath(`synthesis-${locale}-desktop.png`)});}
 await page.setViewportSize({width:390,height:844});expect(await dialog.evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBe(true);
 if(['en','ar'].includes(locale))await page.screenshot({path:info.outputPath(`synthesis-${locale}-narrow.png`)});
 expect(state.errors).toEqual([]);expect(state.posts).toBe(1);
 await dialog.getByRole('button',{name:copy.history,exact:true}).click();await dialog.locator('.comparison-history').getByRole('button',{name:/Records policy/}).click();await expect(dialog.locator('.synthesis-candidate')).toBeVisible();expect(state.posts).toBe(1);
});



test('review preserves drafts and recovers an uncertain save without replay',async({page})=>{
 const state=await fixture(page),dialog=await open(page),copy=documentSynthesis('en'),review=synthesisReviewCopy('en');
 await dialog.locator('form').getByRole('button',{name:copy.create}).click();
 await dialog.locator('.synthesis-review-choice select').selectOption('exclude');await dialog.getByLabel(review.note,{exact:true}).fill('Claim overstates source certainty.');
 await dialog.getByRole('button',{name:copy.history,exact:true}).click();await dialog.locator('.comparison-history').getByRole('button',{name:/Records policy/}).click();
 await expect(dialog.getByLabel(review.note,{exact:true})).toHaveValue('Claim overstates source certainty.');
 state.reviewUnknown=true;await dialog.getByRole('button',{name:review.save,exact:true}).click();await expect(dialog.getByRole('alert')).toContainText(review.recover);await expect(dialog.getByRole('button',{name:review.save,exact:true})).toBeDisabled();
 await dialog.getByRole('button',{name:review.reload,exact:true}).click();await expect(dialog.locator('.synthesis-review-choice select')).toHaveValue('exclude');expect(state.reviewPosts).toBe(1);
 state.review!.revision=2;state.review!.decisions=[{decision:'keep',note:'Another window'}];
 await dialog.getByRole('button',{name:review.save,exact:true}).click();await expect(dialog.getByRole('alert')).toContainText(review.recover);expect(state.review?.decisions[0].note).toBe('Another window');
 await dialog.getByRole('button',{name:review.reload,exact:true}).click();await expect(dialog.getByLabel(review.note,{exact:true})).toHaveValue('Another window');
 await dialog.locator('.synthesis-review-history summary').click();await expect(dialog.locator('.synthesis-review-history')).toContainText('Claim overstates source certainty.');await expect(dialog.locator('.synthesis-review-history strong')).toHaveText(review.exclude);expect(state.reviewPosts).toBe(2);
});

test('synthesis clears forged or revoked source evidence',async({page})=>{
 const state=await fixture(page),dialog=await open(page);state.forged=true;await dialog.locator('form').getByRole('button',{name:documentSynthesis('en').create}).click();await expect(dialog.getByRole('alert')).toBeVisible();await expect(dialog.locator('.synthesis-candidate')).toHaveCount(0);
});
test('synthesis accepted response loss recovers through history without replay',async({page})=>{
 const state=await fixture(page),copy=documentSynthesis('en'),dialog=await open(page);state.unknown=true;await dialog.locator('form').getByRole('button',{name:copy.create}).click();await expect(dialog.getByRole('alert')).toContainText(documentComparison('en').unknown);
 await dialog.locator('.comparison-history').getByRole('button',{name:/Records policy/}).click();await expect(dialog.locator('.synthesis-candidate')).toBeVisible();expect(state.posts).toBe(1);
 state.revoked=true;await expect(dialog.locator('.synthesis-candidate')).toHaveCount(0,{timeout:6000});await expect(dialog.getByRole('alert')).toBeVisible();
});

test('synthesis exports verified drafts and withholds a forged artifact',async({page})=>{
 const state=await fixture(page),dialog=await open(page);
 await dialog.locator('form').getByRole('button',{name:documentSynthesis('en').create}).click();
 const exports=dialog.locator('.comparison-export');await exports.locator(':scope > button').click();
 for(const [label,extension] of [['JSON','json'],['CSV','csv'],['Markdown','md']]){
  const download=page.waitForEvent('download');await exports.getByRole('button',{name:label,exact:true}).click();expect((await download).suggestedFilename()).toBe(`offgrid-draft-analysis.${extension}`);
 }
 expect(state.exports).toBe(3);expect(state.posts).toBe(1);
 let unexpected=0;page.on('download',()=>unexpected++);state.forgedExport=true;
 await exports.getByRole('button',{name:'JSON',exact:true}).click();await expect(exports.getByRole('alert')).toBeVisible();expect(unexpected).toBe(0);expect(state.exports).toBe(4);
});

test('review controls fit enlarged display and support keyboard save',async({page},info)=>{
 const state=await fixture(page),copy=documentSynthesis('en'),review=synthesisReviewCopy('en');
 await page.setViewportSize({width:640,height:450});
 const cdp=await page.context().newCDPSession(page);await cdp.send('Emulation.setDeviceMetricsOverride',{width:640,height:450,deviceScaleFactor:2,mobile:false,screenWidth:1280,screenHeight:900});
 const dialog=await open(page);await dialog.locator('form').getByRole('button',{name:copy.create}).click();
 const choice=dialog.locator('.synthesis-review-choice select');await choice.focus();await page.keyboard.press('Space');await page.keyboard.press('Home');await page.keyboard.press('ArrowDown');await page.keyboard.press('Enter');await expect(choice).toHaveValue('keep');
 await page.keyboard.press('Tab');await expect(dialog.getByLabel(review.note,{exact:true})).toBeFocused();await page.keyboard.type('Verified using the source.');
 await dialog.getByLabel(review.checked,{exact:true}).focus();await page.keyboard.press('Space');await page.keyboard.press('Tab');await expect(dialog.getByRole('button',{name:review.save,exact:true})).toBeFocused();await page.keyboard.press('Enter');await expect(dialog.getByRole('status').filter({hasText:review.saved})).toBeVisible();
 expect(state.reviewPosts).toBe(1);expect(await dialog.evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBe(true);expect(await page.evaluate(()=>devicePixelRatio)).toBe(2);
 await page.screenshot({path:info.outputPath('review-zoom-200.png'),fullPage:true});await page.keyboard.press('Escape');await expect(page.getByRole('button',{name:copy.title,exact:true})).toBeFocused();
});

test('unsaved review notes survive closing and browser reload without publication',async({page})=>{
 const state=await fixture(page),copy=documentSynthesis('en'),review=synthesisReviewCopy('en');let dialog=await open(page);
 await dialog.locator('form').getByRole('button',{name:copy.create}).click();
 await dialog.locator('.synthesis-review-choice select').selectOption('exclude');await dialog.getByLabel(review.note,{exact:true}).fill('Check the exception before publication.');
 await page.keyboard.press('Escape');await page.reload();
 await page.getByRole('button',{name:copy.title,exact:true}).click();dialog=page.getByRole('region',{name:copy.title});
 await dialog.getByRole('button',{name:copy.history,exact:true}).click();await dialog.locator('.comparison-history').getByRole('button',{name:/Records policy/}).click();
 await expect(dialog.getByLabel(review.note,{exact:true})).toHaveValue('Check the exception before publication.');
 await expect(dialog.locator('.synthesis-review-choice select')).toHaveValue('exclude');expect(state.reviewPosts).toBe(0);expect(state.posts).toBe(1);
});
