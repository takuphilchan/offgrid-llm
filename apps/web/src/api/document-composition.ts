import type {components} from './schema.generated';
import {validateQuote,type Synthesis} from './document-synthesis';
type Schema=components['schemas'];
export type Composition=Schema['DocumentComposition'];
export type CompositionReport=Schema['DocumentCompositionReport'];
export type CompositionEvidence=Schema['DocumentCompositionEvidence'];
export type CompositionHistory=Schema['DocumentCompositionHistory'];
type Request=<T>(path:string,init?:RequestInit)=>Promise<T>;
const base='/v1/document-analyses/synthesis/compositions';
const integer=(v:unknown):v is number=>typeof v==='number'&&Number.isSafeInteger(v)&&v>=0;
const text=(v:unknown):v is string=>typeof v==='string'&&v.length>0;
const fail=()=>{throw Error('document_analysis_invalid');};
async function candidateDigest(c:Schema['SynthesisCandidate']){
 const quotes=(items:Schema['SynthesisQuote'][])=>items.map(q=>{const v=q.citation;return {text:q.text,citation:{source:v.source,revision:v.revision,unit_id:v.unit_id,...(v.block_id?{block_id:v.block_id}:{}),span:{start_byte:v.span.start_byte,end_byte:v.span.end_byte},text_sha256:v.text_sha256}};});
 const raw=JSON.stringify({text:c.text,evidence:quotes(c.evidence),conditions:quotes(c.conditions),exceptions:quotes(c.exceptions),verification:c.verification}).replace(/[<>&\u2028\u2029]/g,c=>'\\u'+c.charCodeAt(0).toString(16).padStart(4,'0'));
 return Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',new TextEncoder().encode(raw))),v=>v.toString(16).padStart(2,'0')).join('');
}
export function validateComposition(r:Composition,parent:Synthesis,id?:string){
 if(!r||!text(r.id)||(id&&r.id!==id)||r.analysis_id!==parent.id||r.source_plan_id!==parent.plan.id||!text(r.plan_id)||!['queued','running','interrupted','cancelled','failed','awaiting_review'].includes(r.state)||!integer(r.attempt)||!integer(r.completed_steps)||!integer(r.total_steps)||r.total_steps>65536||r.completed_steps>r.total_steps||(r.state==='awaiting_review'&&r.completed_steps!==r.total_steps)||typeof r.reviews_current!=='boolean'||!r.model||!text(r.model.model_id)||!text(r.model.language)||r.model.protocol!=='document-synthesis-candidates-v1'||!Number.isFinite(Date.parse(r.created_at))||!Number.isFinite(Date.parse(r.updated_at)))fail();
 return r;
}
export function compositionAPI(request:Request){return {
 export:async(id:string,format:'json'|'csv'|'markdown',signal?:AbortSignal)=>{
  const r=await request<{analysis_id:string;format:string;content:string;sha256:string}>(`${base}/export?${new URLSearchParams({id,format})}`,{signal});
  if(!r||r.analysis_id!==id||r.format!==format||!text(r.content))fail();const bytes=new TextEncoder().encode(r.content);if(bytes.length>8*1024*1024)fail();
  const hash=Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',bytes)),v=>v.toString(16).padStart(2,'0')).join('');if(hash!==r.sha256)fail();return r;
 },
 history:async(parent:Synthesis,offset=0,signal?:AbortSignal)=>{
  const r=await request<CompositionHistory>(`${base}?${new URLSearchParams({analysis_id:parent.id,offset:String(offset)})}`,{signal});
  if(!r||r.analysis_id!==parent.id||!Array.isArray(r.items)||r.items.length>20||(r.next_offset!==null&&r.next_offset!==offset+20))fail();r.items.forEach(v=>validateComposition(v,parent));return r;
 },
 get:async(parent:Synthesis,id:string,signal?:AbortSignal)=>validateComposition(await request<Composition>(`${base}?${new URLSearchParams({id})}`,{signal}),parent,id),
 create:async(parent:Synthesis,requestKey:string,signal?:AbortSignal)=>{
  const body={request_key:requestKey,analysis_id:parent.id,model:parent.model.model_id,language:parent.model.language};
  const r=validateComposition(await request<Composition>(base,{method:'POST',body:JSON.stringify(body),signal}),parent);
  if(r.model.model_id!==body.model||r.model.language!==body.language)fail();return r;
 },
 control:async(id:string,action:'cancel'|'retry',signal?:AbortSignal)=>{const r=await request<{accepted:boolean}>(`${base}/${action}`,{method:'POST',body:JSON.stringify({id}),signal});if(r?.accepted!==true)fail();},
 report:async(parent:Synthesis,id:string,signal?:AbortSignal)=>{
  const r=await request<CompositionReport>(`${base}/report?${new URLSearchParams({id})}`,{signal});
  if(!r)fail();validateComposition(r.composition,parent,id);
  if(r.composition.state!=='awaiting_review'||r.semantic_review_required!==true||r.source_units!==parent.plan.units.length||!Array.isArray(r.sections)||r.sections.length>4||!integer(r.unrepresented_batches)||r.unrepresented_batches>parent.completed_batches)fail();
  for(const s of r.sections){if(!text(s.text)||new TextEncoder().encode(s.text).length>2048||!integer(s.contributors)||!s.contributors||s.contributors>65536||s.verification!=='source_identity_only')fail();}
  for(const gaps of [r.gaps,r.extraction_gaps,r.candidate_gaps])if(!Array.isArray(gaps)||gaps.length>256||gaps.some(g=>!g||!text(g.code)||!integer(g.count)||!g.count))fail();return r;
 },
 evidence:async(parent:Synthesis,report:CompositionReport,section:number,offset=0,signal?:AbortSignal)=>{
  const r=await request<CompositionEvidence>(`${base}/evidence?${new URLSearchParams({id:report.composition.id,section:String(section),offset:String(offset)})}`,{signal});
  if(!r||r.composition_id!==report.composition.id||r.plan_id!==report.composition.plan_id||r.section!==section||r.offset!==offset||r.total!==report.sections[section]?.contributors||!Array.isArray(r.items)||r.items.length>8)fail();
  const end=offset+r.items.length;if(end>r.total||(end<r.total?(!r.items.length||r.next_offset!==end):r.next_offset!==null))fail();
  for(const item of r.items){const i=item?.input,c=item?.candidate;
   if(!i||!text(i.id)||!integer(i.batch)||i.batch>=parent.completed_batches||!text(i.batch_id)||!integer(i.candidate_index)||i.candidate_index>31||!/^[a-f0-9]{64}$/.test(i.candidate_sha256)||!integer(i.review_revision)||i.review_revision>256||!['keep','unreviewed'].includes(i.decision)||typeof i.section_checked!=='boolean'||(i.section_checked&&i.decision==='unreviewed')||!c||!text(c.text)||c.verification!=='source_identity_only'||!Array.isArray(c.evidence)||!c.evidence.length)fail();
   for(const group of [c.evidence,c.conditions,c.exceptions]){if(!Array.isArray(group)||group.length>8)fail();await Promise.all(group.map(q=>validateQuote(q,parent)));}
   if(await candidateDigest(c)!==i.candidate_sha256)fail();
  }
  return r;
 }
};}
