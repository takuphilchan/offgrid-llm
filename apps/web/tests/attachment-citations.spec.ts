import {test, expect} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import {resolveAttachmentCitation, type Selection} from '../src/features/chat/attachment-citations';

const id='file-'+ 'a'.repeat(32), revision='b'.repeat(64), sha='c'.repeat(64);
const passage='Verified café 日本語 passage.';
const end=new TextEncoder().encode(passage).length;
const selection:Selection={id,revision,name:'paper.pdf',mode:'excerpts',total_bytes:200,ranges:[[0,end],[100,110]]};

test('citation binding rejects ambiguous, forged and out-of-selection references',()=>{
 expect(resolveAttachmentCitation('[S1.1]',[selection])?.start).toBe(0);
 expect(resolveAttachmentCitation(`[paper.pdf: 0-${end}]`,[selection])?.end).toBe(end);
 for(const token of ['[S1]','[S0.1]','[S2.1]','[S1.3]','[paper.pdf: 1-10]','[../secret: 0-10]']) expect(resolveAttachmentCitation(token,[selection])).toBeUndefined();
 expect(resolveAttachmentCitation(`[paper.pdf: 0-${end}]`,[selection,{...selection,id:'file-'+ 'd'.repeat(32)}])).toBeUndefined();
 expect(resolveAttachmentCitation('[S1.1]',[{...selection,revision:'changed'}])).toBeUndefined();
 expect(resolveAttachmentCitation('[S1.1]',[{...selection,total_bytes:NaN}])).toBeUndefined();
 for(const value of [null, {}, {...selection,ranges:null}, {...selection,ranges:[null]}, {...selection,ranges:{0:[0,end]}}, {...selection,ranges:[{0:0,1:end,length:2}]}]) {
  for(const token of ['[S1.1]',`[paper.pdf: 0-${end}]`]) expect(resolveAttachmentCitation(token,[value as Selection])).toBeUndefined();
 }
});

for(const format of ['compact','legacy','long','ambiguous','revoked']) test(`saved attachment citation opens exact retained passage: ${format}`,async({page})=>{
 await page.setViewportSize({width:1440,height:1000});
 const state=await craftFixture(page,'en','dark');
 const token=format==='legacy'?`[paper.pdf: 0-${end}]`:'[S1.1]';
 const content=(format==='long'?'Introductory text. '.repeat(260)+'\n\n':'')+`Supported claim **${token}**. Unbound [S9.1].\n\n\`[S1.1]\`\n\n[ordinary link](https://example.com)`;
 const session={name:'Demo briefing',model_id:'Local-Research-7B-Instruct-Q4_K_M-long-model-name',messages:[{role:'user',content:'Explain this source'},{role:'assistant',content,attachment_selections:[selection]}]};
 await page.route('**/v1/sessions/Demo%20briefing',r=>r.fulfill({json:session}));
 await page.route('**/v1/sessions',r=>r.fulfill({json:{sessions:[session]}}));
 const requests:string[]=[];
 await page.route('**/attachments/*/passage?*',r=>{requests.push(r.request().url());return r.fulfill(format==='revoked'?{status:422,json:{error:{message:'Source no longer available'}}}:{json:{id,revision,name:'paper.pdf',sha256:sha,start_byte:0,end_byte:end,text:passage}});});
 await page.route('**/attachments/*/preview*',r=>r.fulfill({json:{name:'paper.pdf',sha256:sha,format:'pdf',pages:2,state:'ready',units:[{kind:'page',index:1,text:format==='ambiguous'?passage:'First page',state:'ready'},{kind:'page',index:2,text:`Context ${passage} End`,state:'ready'}]}}));
 await page.goto('/ui/#/chat');
 const citation=page.getByRole('button',{name:'Open document: paper.pdf · 1',exact:true});
 await expect(citation).toHaveCount(1);await citation.click();
 expect(requests).toHaveLength(1);expect(new URL(requests[0]).searchParams.get('revision')).toBe(revision);
 if(format==='revoked'){await expect(page.getByRole('alert')).toContainText('Source no longer available');await expect(page.locator('.document-reader')).toHaveCount(0);}
 else if(format==='ambiguous'){await expect(page.locator('.document-verified-passage')).toContainText(passage);await expect(page.getByRole('spinbutton',{name:'Page',exact:true})).toHaveValue('1');}
 else {await expect(page.getByRole('spinbutton',{name:'Page',exact:true})).toHaveValue('2');await expect(page.locator('.document-reader mark')).toHaveText(passage);}
 expect(state.writes).toEqual([]);
 if(format==='compact') await page.screenshot({path:test.info().outputPath('readable-source-citation.png')});
 if(format!=='revoked'){await page.keyboard.press('Escape');await expect(citation).toBeFocused();}
});
