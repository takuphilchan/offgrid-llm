import type {components,operations} from './schema.generated';
type Schema=components['schemas'];
export type Synthesis=Schema['DocumentSynthesis'];
export type SynthesisResults=Schema['DocumentSynthesisResults'];
export type SynthesisHistory=Schema['DocumentSynthesisHistory'];
export type SynthesisQuote=Schema['SynthesisQuote'];
export type SynthesisReview=Schema['SynthesisReview'];
export type SynthesisReviewRequest=Schema['SynthesisReviewRequest'];
export function validateSynthesisReview(r:SynthesisReview,id:string,batch:number,receipt:SynthesisResults['receipts'][number]){
 if(!r||r.analysis_id!==id||r.batch!==batch||r.batch_id!==receipt.batch_id||r.response_sha256!==receipt.response_sha256||!integer(r.revision)||r.revision>256||typeof r.section_checked!=='boolean'||!Array.isArray(r.decisions)||r.decisions.length!==receipt.result.candidates.length||r.decisions.some(d=>!d||!['keep','exclude','unreviewed'].includes(d.decision)||typeof d.note!=='string'||new TextEncoder().encode(d.note).length>2048||(r.section_checked&&d.decision==='unreviewed'))||typeof r.author!=='string'||typeof r.updated_at!=='string'||(r.revision>0&&(!r.author||!Number.isFinite(Date.parse(r.updated_at))))||(r.revision===0&&(r.author!==''||r.updated_at!==''||r.section_checked||r.decisions.some(d=>d.decision!=='unreviewed'||d.note!==''))))invalid();
 return r;
}
export type SynthesisRequest=operations['createDocumentSynthesis']['requestBody']['content']['application/json'];
type Request=<T>(path:string,init?:RequestInit)=>Promise<T>;
const states=['queued','running','interrupted','cancelled','failed','awaiting_review'];
const integer=(v:number)=>Number.isSafeInteger(v)&&v>=0;
const text=(v:unknown):v is string=>typeof v==='string'&&v.length>0;
const digest=(v:unknown)=>typeof v==='string'&&/^[a-f0-9]{64}$/.test(v);
const invalid=()=>{throw Error('document_analysis_invalid');};
async function modelIdentity(m:Synthesis['model']){
 const runtime={name:m.runtime.name,revision:m.runtime.revision,...(m.runtime.sha256?{sha256:m.runtime.sha256}:{})};
 // Match the versioned Go identity serialization, including HTML-safe escaping.
 const json=JSON.stringify({model_id:m.model_id,model_sha256:m.model_sha256,runtime,language:m.language,context_tokens:m.context_tokens,response_tokens:m.response_tokens,protocol:m.protocol}).replace(/[<>&\u2028\u2029]/g,c=>'\\u'+c.charCodeAt(0).toString(16).padStart(4,'0'));
 const hash=await crypto.subtle.digest('SHA-256',new TextEncoder().encode(json));
 return 'synthesis-model-'+Array.from(new Uint8Array(hash),v=>v.toString(16).padStart(2,'0')).join('');
}
export function validateSynthesis(r:Synthesis,id?:string){
 const p=r?.plan,m=r?.model;
 if(!r||!text(r.id)||(id&&r.id!==id)||!p||!text(p.id)||p.version!=='document-analysis-v1'||!['summary','requirements'].includes(p.workflow)||!Array.isArray(p.sources)||p.sources.length<1||p.sources.length>16||p.sources.some(s=>!text(s.document_id)||!text(s.revision))||!Array.isArray(p.units)||!p.units.length||!Array.isArray(p.gaps)||!states.includes(r.state)||!integer(r.attempt)||!integer(r.completed_batches)||!integer(r.unit_cursor)||r.unit_cursor>p.units.length||r.completed_units!==r.unit_cursor||!integer(r.next_byte)||!m||!text(m.model_id)||!digest(m.model_sha256)||m.protocol!=='document-synthesis-candidates-v1'||!text(m.language)||!m.runtime||!text(m.runtime.name)||!text(m.runtime.revision)||!integer(m.context_tokens)||m.context_tokens<1024||!integer(m.response_tokens)||m.response_tokens<256||m.response_tokens>=m.context_tokens||typeof r.review_required!=='boolean')invalid();
 p.units.forEach((u,i)=>{if(!u||u.ordinal!==i||!integer(u.source)||u.source>=p.sources.length||!text(u.unit_id)||!u.span||!integer(u.span.start_byte)||!integer(u.span.end_byte)||u.span.end_byte<u.span.start_byte)invalid();});
 if(r.unit_cursor<p.units.length){const span=p.units[r.unit_cursor].span;if(r.next_byte<span.start_byte||r.next_byte>span.end_byte)invalid();}else if(r.next_byte!==0)invalid();
 if((r.state==='awaiting_review')!==(r.unit_cursor===p.units.length))invalid();
 if(p.gaps.some(g=>!g||!integer(g.source)||g.source>=p.sources.length||!text(g.code)))invalid();
 return r;
}
export async function validateQuote(q:SynthesisQuote,job:Synthesis){
 const c=q?.citation;if(!q||!text(q.text)||!c||!integer(c.source)||c.source>=job.plan.sources.length||c.revision!==job.plan.sources[c.source].revision||!c.span||!integer(c.span.start_byte)||!integer(c.span.end_byte)||c.span.end_byte<=c.span.start_byte||!digest(c.text_sha256))invalid();
 const u=job.plan.units.find(u=>u.source===c.source&&u.unit_id===c.unit_id);if(!u||c.span.start_byte<u.span.start_byte||c.span.end_byte>u.span.end_byte)invalid();
 const bytes=new TextEncoder().encode(q.text);if(bytes.length>8192||bytes.length!==c.span.end_byte-c.span.start_byte)invalid();
 const hash=Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',bytes)),v=>v.toString(16).padStart(2,'0')).join('');if(hash!==c.text_sha256)invalid();
}
export async function validateSynthesisResults(p:SynthesisResults,job:Synthesis,offset:number){
 if(!p||p.analysis_id!==job.id||p.offset!==offset||!integer(p.total_batches)||p.total_batches>100000||!Array.isArray(p.receipts)||p.receipts.length>2)invalid();
 const end=offset+p.receipts.length;if(end>p.total_batches||(end<p.total_batches?(!p.receipts.length||p.next_offset!==end):p.next_offset!==null))invalid();
 const identity=await modelIdentity(job.model);
 for(const receipt of p.receipts){
  if(receipt.validation!==undefined&&receipt.validation!=='source-values-v2')invalid();
  const u=job.plan.units[receipt.unit_ordinal];
  if(!u||!integer(receipt.unit_ordinal)||!text(receipt.batch_id)||receipt.model_identity!==identity||!digest(receipt.response_sha256)||!['model','empty_source'].includes(receipt.method)||!receipt.primary||!integer(receipt.primary.start_byte)||!integer(receipt.primary.end_byte)||receipt.primary.start_byte<u.span.start_byte||receipt.primary.end_byte>u.span.end_byte||receipt.primary.end_byte<receipt.primary.start_byte||receipt.end_of_unit!==(receipt.primary.end_byte===u.span.end_byte)||!receipt.result||receipt.result.batch_id!==receipt.batch_id||!Array.isArray(receipt.result.candidates)||receipt.result.candidates.length>32||!Array.isArray(receipt.result.gaps)||(receipt.method==='model'&&!receipt.result.semantic_review_required))invalid();
  if(!integer(receipt.prompt_tokens)||(receipt.method==='model'&&(receipt.prompt_tokens<1||receipt.prompt_tokens>job.model.context_tokens-job.model.response_tokens-256))||(receipt.method==='empty_source'&&(receipt.prompt_tokens!==0||receipt.result.candidates.length>0||receipt.result.no_findings_reported)))invalid();
  for(const candidate of receipt.result.candidates){if(!text(candidate.text)||candidate.verification!=='source_identity_only'||!Array.isArray(candidate.evidence)||candidate.evidence.length<1)invalid();for(const group of [candidate.evidence,candidate.conditions,candidate.exceptions]){if(!Array.isArray(group)||group.length>8)invalid();await Promise.all(group.map(q=>validateQuote(q,job)));}}
 }
 return p;
}
export function synthesisAPI(request:Request){return {
 review:async(id:string,batch:number,receipt:SynthesisResults['receipts'][number],signal?:AbortSignal,revision=0)=>{
  const r=validateSynthesisReview(await request<SynthesisReview>(`/v1/document-analyses/synthesis/review?${new URLSearchParams({id,batch:String(batch),revision:String(revision)})}`,{signal}),id,batch,receipt);
  if(revision>0&&r.revision!==revision)invalid();return r;
 },
 saveReview:async(body:SynthesisReviewRequest,receipt:SynthesisResults['receipts'][number],signal?:AbortSignal)=>{
  const r=validateSynthesisReview(await request<SynthesisReview>('/v1/document-analyses/synthesis/review',{method:'POST',body:JSON.stringify(body),signal}),body.id,body.batch,receipt);
  if(r.revision!==body.expected_revision+1||r.section_checked!==body.section_checked||JSON.stringify(r.decisions)!==JSON.stringify(body.decisions))invalid();return r;
 },
 export:async(id:string,format:'json'|'csv'|'markdown',signal?:AbortSignal)=>{
  const r=await request<{analysis_id:string;format:string;content:string;sha256:string}>(`/v1/document-analyses/synthesis/export?${new URLSearchParams({id,format})}`,{signal});
  if(!r||r.analysis_id!==id||r.format!==format||typeof r.content!=='string'||!r.content)invalid();
  const bytes=new TextEncoder().encode(r.content);if(bytes.length>8*1024*1024)invalid();
  const hash=Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',bytes)),v=>v.toString(16).padStart(2,'0')).join('');if(hash!==r.sha256)invalid();return r;
 },
 get:async(id:string,signal?:AbortSignal)=>validateSynthesis(await request<Synthesis>(`/v1/document-analyses/synthesis?${new URLSearchParams({id})}`,{signal}),id),
 create:async(body:SynthesisRequest,signal?:AbortSignal)=>{const job=validateSynthesis(await request<Synthesis>('/v1/document-analyses/synthesis',{method:'POST',body:JSON.stringify(body),signal}));if(job.model.model_id!==body.model||job.model.language!==body.language||job.plan.workflow!==body.workflow||job.plan.sources.length!==body.sources.length||job.plan.sources.some((s,i)=>s.document_id!==body.sources[i].document_id||s.revision!==body.sources[i].revision||(s.original_sha256??'')!==(body.sources[i].original_sha256??'')))invalid();return job;},
 history:async(offset=0,signal?:AbortSignal)=>{const page=await request<SynthesisHistory>(`/v1/document-analyses/synthesis?offset=${offset}`,{signal});if(!page||!Array.isArray(page.analyses)||page.analyses.length>20||(page.next_offset!==null&&page.next_offset!==offset+20)||page.analyses.some(r=>!text(r.id)||!states.includes(r.state)||!Array.isArray(r.documents)||!r.documents.length||r.documents.length>16||r.documents.some(d=>!text(d))))invalid();return page;},
 results:async(job:Synthesis,offset=0,signal?:AbortSignal)=>validateSynthesisResults(await request<SynthesisResults>(`/v1/document-analyses/synthesis/results?${new URLSearchParams({id:job.id,offset:String(offset)})}`,{signal}),job,offset),
 control:async(id:string,action:'cancel'|'retry',signal?:AbortSignal)=>{const path=action==='cancel'?'/v1/document-analyses/cancel':'/v1/document-analyses/synthesis/retry';const response=await request<{accepted:boolean}>(path,{method:'POST',body:JSON.stringify({id}),signal});if(response?.accepted!==true)invalid();}
};}
