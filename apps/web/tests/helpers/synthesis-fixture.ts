import {expect,test,type Page} from '@playwright/test';
import {createHash} from 'node:crypto';
import {craftFixture} from './craft-fixture';
import {documentSynthesis} from '../../src/i18n/document-synthesis';
import {documentComparison} from '../../src/i18n/document-comparison';
import {documentReader} from '../../src/i18n/document-reader';
import {synthesisReviewCopy} from '../../src/i18n/document-synthesis-review';
import {languageRegistry,type LanguageCode} from '../../src/i18n/language';
import type {Synthesis,SynthesisResults,SynthesisReview} from '../../src/api/document-synthesis';


export const source='Submit the report by 31 July. Exception: approved extensions remain valid. 中文';
export const hash=(s:string)=>createHash('sha256').update(s).digest('hex');
const span={start_byte:0,end_byte:Buffer.byteLength(source)};
export function job(locale:LanguageCode):Synthesis{return {id:'synthesis-1',model:{model_id:'fixture',model_sha256:'a'.repeat(64),runtime:{name:'fixture',revision:'1'},language:locale,context_tokens:8192,response_tokens:2048,protocol:'document-synthesis-candidates-v1'},primary_bytes:1024,state:'awaiting_review',attempt:1,completed_batches:1,completed_units:1,unit_cursor:1,next_byte:0,review_required:true,created_at:'2026-11-13T01:00:00Z',updated_at:'2026-11-13T01:00:00Z',plan:{id:'plan',version:'document-analysis-v1',workflow:'summary',sources:[{document_id:'policy',revision:'rev1'}],units:[{ordinal:0,source:0,unit_id:'section-1',span,blocks:1}],gaps:[]}};}
export function result():SynthesisResults{return {analysis_id:'synthesis-1',offset:0,next_offset:null,total_batches:1,receipts:[{method:'model',batch_id:'batch1',model_identity:'identity',unit_ordinal:0,primary:span,end_of_unit:true,prompt_tokens:1000,response_sha256:'b'.repeat(64),result:{batch_id:'batch1',candidates:[{text:'The report is due by 31 July, subject to approved extensions.',verification:'source_identity_only',evidence:[{text:source,citation:{source:0,revision:'rev1',unit_id:'section-1',span,text_sha256:hash(source)}}],conditions:[],exceptions:[]}],gaps:[],no_findings_reported:false,semantic_review_required:true}}]};}
export async function fixture(page:Page,locale:LanguageCode='en'){
 await craftFixture(page,locale,locale==='ar'?'dark':'light');
 const state={created:false,unknown:false,revoked:false,forged:false,forgedExport:false,old:false,posts:0,exports:0,record:job(locale),errors:[] as string[],requests:[] as any[],reviewPosts:0,reviewUnknown:false,review:undefined as SynthesisReview|undefined,reviewHistory:new Map<number,SynthesisReview>()};
 page.on('pageerror',e=>state.errors.push(e.message));
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',version:'fixture',workspace_id:'craft',api_version:2,capabilities:state.old?[]:['document-synthesis-candidates-v1','document-synthesis-export-v1','document-synthesis-review-v1']}}));
 await page.route('**/v1/documents',r=>r.fulfill({json:{documents:state.revoked?[]:[{id:'policy',name:'Records policy.txt',size:200,index_status:'ready',source_retained:true,extraction_revision:'rev1'}]}}));
 await page.route('**/v1/documents/structure?*',r=>r.fulfill({json:{document_id:'policy',revision:'rev1',format:'txt',method:'native',units:[],blocks:[],offset:0,total_blocks:0,next_offset:null,relationships:[],relationship_offset:0,total_relationships:0,next_relationship_offset:null,can_correct:false}}));
 await page.route('**/v1/document-analyses**',r=>{
  const request=r.request(),url=new URL(request.url()),path=url.pathname,q=url.searchParams;
  if(state.revoked)return r.fulfill({status:404,json:{error:'unavailable'}});
  if(path.endsWith('/review')){
   state.review??={analysis_id:state.record.id,batch:0,batch_id:'batch1',response_sha256:'b'.repeat(64),revision:0,section_checked:false,decisions:[{decision:'unreviewed',note:''}],author:'',updated_at:''};
   if(request.method()==='POST'){state.reviewPosts++;const body=request.postDataJSON();if(body.expected_revision!==state.review.revision)return r.fulfill({status:409,json:{error:'conflict'}});state.review={...state.review,revision:state.review.revision+1,section_checked:body.section_checked,decisions:body.decisions,author:'reviewer',updated_at:'2026-11-13T12:00:00Z'};state.reviewHistory.set(state.review.revision,structuredClone(state.review));if(state.reviewUnknown)return r.abort('failed');}
   if(Number(q.get('revision'))>0)return r.fulfill({json:state.reviewHistory.get(Number(q.get('revision')))});
   return r.fulfill({json:state.review});
  }
  if(path.endsWith('/export')){state.exports++;const content='Draft analysis — source_identity_only — '+source;return r.fulfill({json:{analysis_id:state.record.id,format:q.get('format'),content,sha256:state.forgedExport?'f'.repeat(64):hash(content)}});}
  if(path.endsWith('/results')){const p=result(),m=state.record.model;p.receipts[0].model_identity='synthesis-model-'+hash(JSON.stringify({model_id:m.model_id,model_sha256:m.model_sha256,runtime:m.runtime,language:m.language,context_tokens:m.context_tokens,response_tokens:m.response_tokens,protocol:m.protocol}));if(state.forged)p.receipts[0].result.candidates[0].evidence[0].text='forged';return r.fulfill({json:p});}
  if(path.endsWith('/retry')||path.endsWith('/cancel')){state.record.state=path.endsWith('/cancel')?'cancelled':'running';state.record.unit_cursor=0;state.record.completed_units=0;state.record.next_byte=0;return r.fulfill({json:{accepted:true}});}
  if(request.method()==='POST'){state.posts++;state.created=true;const body=request.postDataJSON();state.requests.push(body);state.record.model.model_id=body.model;state.record.model.language=body.language;state.record.plan.workflow=body.workflow;return state.unknown?r.abort('failed'):r.fulfill({status:202,json:state.record});}
  if(q.has('id'))return r.fulfill({json:state.record});
  return r.fulfill({json:{analyses:state.created?[{id:state.record.id,state:state.record.state,documents:['policy'],created_at:state.record.created_at,updated_at:state.record.updated_at}]:[],next_offset:null}});
 });
 await page.goto('/ui/#/knowledge');return state;
}
export async function open(page:Page,locale:LanguageCode='en'){
 const copy=documentSynthesis(locale);await page.getByRole('button',{name:copy.title,exact:true}).click();const dialog=page.getByRole('region',{name:copy.title});await dialog.locator('.synthesis-model select').selectOption({index:1});await dialog.getByRole('checkbox',{name:'Records policy.txt'}).check();return dialog;
}
