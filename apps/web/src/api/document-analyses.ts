import type {components,operations} from './schema.generated';
type Schema=components['schemas'];
export type Analysis=Schema['DocumentAnalysis'];
export type AnalysisRef=Schema['AnalysisSourceRef'];
export type AnalysisResults=Schema['DocumentAnalysisResults'];
export type AnalysisHistory=Schema['DocumentAnalysisHistory'];
export type AnalysisSource=operations['getDocumentAnalysisSource']['responses'][200]['content']['application/json'];
type Request=<T>(path:string,init?:RequestInit)=>Promise<T>;
const states=['queued','running','interrupted','cancelled','failed','complete','incomplete'];
const integer=(v:number)=>Number.isSafeInteger(v)&&v>=0;
const nonempty=(v:unknown):v is string=>typeof v==='string'&&v.length>0;
const digest=(v:unknown):v is string=>typeof v==='string'&&/^[a-f0-9]{64}$/.test(v);
const invalid=()=>{throw Error('document_analysis_invalid');}; // UI resolves codes, never server prose.
const sha256=async(text:string)=>Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',new TextEncoder().encode(text))),v=>v.toString(16).padStart(2,'0')).join('');
const sameRef=(a:AnalysisRef,b:AnalysisRef)=>a.document_id===b.document_id&&a.revision===b.revision&&(a.original_sha256||'')===(b.original_sha256||'');
export function validateAnalysis(r:Analysis,id=''):Analysis {
 if(!r||!nonempty(r.id)||(id&&r.id!==id)||!states.includes(r.state)||!integer(r.attempt)||!integer(r.completed_steps)||!integer(r.completed_units)||typeof r.review_required!=='boolean')invalid();
 const p=r.plan;
 if(!p||p.version!=='document-analysis-v1'||p.workflow!=='comparison'||!nonempty(p.id)||!Array.isArray(p.sources)||p.sources.length!==2||p.sources.some(s=>!s||!nonempty(s.document_id)||!nonempty(s.revision)||(s.original_sha256!==undefined&&!digest(s.original_sha256)))||sameRef(p.sources[0],p.sources[1]))invalid();
 if(!Array.isArray(p.units)||p.units.length>400000||!Array.isArray(p.gaps)||!Array.isArray(r.pairs)||r.pairs.length>400000||r.completed_steps>r.pairs.length||r.completed_units>p.units.length)invalid();
 const units=new Set<string>();
 p.units.forEach((u,i)=>{if(!u||u.ordinal!==i||![0,1].includes(u.source)||!nonempty(u.unit_id)||!u.span||!integer(u.span.start_byte)||!integer(u.span.end_byte)||u.span.end_byte<u.span.start_byte||!integer(u.blocks)||units.has(`${u.source}:${u.unit_id}`))invalid();units.add(`${u.source}:${u.unit_id}`);});
 const paired=new Set<string>();
 for(const pair of r.pairs){if(!pair||!['sheet_name','position','unmatched'].includes(pair.alignment)||(!pair.before_unit&&!pair.after_unit))invalid();for(const [i,u] of [pair.before_unit,pair.after_unit].entries()){if(u){const key=`${i}:${u}`;if(!units.has(key)||paired.has(key))invalid();paired.add(key);}}}
 if(paired.size!==units.size||p.gaps.some(g=>!g||![0,1].includes(g.source)||!nonempty(g.code)||(g.unit_id&&!units.has(`${g.source}:${g.unit_id}`))))invalid();
 const completed=r.pairs.slice(0,r.completed_steps).reduce((n,p)=>n+Number(!!p.before_unit)+Number(!!p.after_unit),0);
 if(completed!==r.completed_units||(['complete','incomplete'].includes(r.state)&&(r.completed_steps!==r.pairs.length||r.completed_units!==p.units.length))||(r.state==='complete'&&(r.review_required||p.gaps.length)))invalid();
 return r;
}
function citation(c:Schema['AnalysisCitation'],job:Analysis,step:number,side:'before'|'after') {
 const source=side==='before'?0:1,unit=job.pairs[step][side==='before'?'before_unit':'after_unit'];
 const bounds=job.plan.units.find(u=>u.source===source&&u.unit_id===unit)?.span;
 if(!c||c.source!==source||c.revision!==job.plan.sources[source].revision||c.unit_id!==unit||!bounds||!c.span||!integer(c.span.start_byte)||!integer(c.span.end_byte)||c.span.start_byte<bounds.start_byte||c.span.end_byte>bounds.end_byte||c.span.end_byte<c.span.start_byte||!digest(c.text_sha256))invalid();
}
export function validateAnalysisResults(p:AnalysisResults,job:Analysis,step:number,offset:number,limit:number) {
 if(!p||p.analysis_id!==job.id||p.step!==step||p.offset!==offset||!integer(p.total_changes)||p.total_changes>400000||!p.result||step>=job.completed_steps||!p.result.pair||p.result.pair.before_unit!==job.pairs[step].before_unit||p.result.pair.after_unit!==job.pairs[step].after_unit||p.result.pair.alignment!==job.pairs[step].alignment||!Array.isArray(p.result.changes)||p.result.changes.length>limit)invalid();
 const end=offset+p.result.changes.length;if(end>p.total_changes||(end<p.total_changes?(!p.result.changes.length||p.next_offset!==end):p.next_offset!==null))invalid();
 for(const change of p.result.changes){if(!change||!['text_removed','text_added','cell_modified','cell_removed','cell_added'].includes(change.kind))invalid();const before=['text_removed','cell_removed','cell_modified'].includes(change.kind),after=['text_added','cell_added','cell_modified'].includes(change.kind);if(!!change.before!==before||!!change.after!==after)invalid();if(change.before)citation(change.before,job,step,'before');if(change.after)citation(change.after,job,step,'after');}
 for(const count of [p.result.unchanged_lines,p.result.before_lines,p.result.after_lines,p.result.compared_cells,p.result.ambiguous_cells])if(!integer(count))invalid();
 return p;
}
export function analysisAPI(request:Request){return {
 export:async(id:string,format:'json'|'csv'|'markdown',signal?:AbortSignal)=>{
  const r=await request<{analysis_id:string;format:string;content:string;sha256:string}>(`/v1/document-analyses/export?${new URLSearchParams({id,format})}`,{signal});
  if(!r||r.analysis_id!==id||r.format!==format||typeof r.content!=='string'||!r.content||new TextEncoder().encode(r.content).length>8*1024*1024||await sha256(r.content)!==r.sha256)invalid();return r;
 },
 history:async(offset=0,signal?:AbortSignal)=>{const p=await request<AnalysisHistory>(`/v1/document-analyses?offset=${offset}&limit=20`,{signal});if(!p||!Array.isArray(p.analyses)||p.analyses.length>20||(p.next_offset!==null&&p.next_offset!==offset+20)||p.analyses.some(r=>!r||!nonempty(r.id)||!states.includes(r.state)||!Array.isArray(r.documents)||r.documents.length!==2||r.documents.some(d=>!nonempty(d))||!Number.isFinite(Date.parse(r.created_at)))||new Set(p.analyses.map(r=>r.id)).size!==p.analyses.length)invalid();return p;},
 get:async(id:string,signal?:AbortSignal)=>validateAnalysis(await request<Analysis>(`/v1/document-analyses?${new URLSearchParams({id})}`,{signal}),id),
 compare:async(key:string,sources:AnalysisRef[],signal?:AbortSignal)=>{if(!key||sources.length!==2)invalid();const r=validateAnalysis(await request<Analysis>('/v1/document-analyses',{method:'POST',body:JSON.stringify({request_key:key,workflow:'comparison',sources}),signal}));if(r.plan.sources.some((s,i)=>!sameRef(s,sources[i])))invalid();return r;},
 control:async(id:string,action:'cancel'|'retry',signal?:AbortSignal)=>{const r=await request<{accepted:boolean}>(`/v1/document-analyses/${action}`,{method:'POST',body:JSON.stringify({id}),signal});if(r?.accepted!==true)invalid();},
 results:async(job:Analysis,step:number,offset=0,signal?:AbortSignal)=>{
  const p=validateAnalysisResults(await request<AnalysisResults>(`/v1/document-analyses/results?${new URLSearchParams({id:job.id,step:String(step),offset:String(offset),limit:'20'})}`,{signal}),job,step,offset,20);
  if(p.previews!==undefined){
   if(!Array.isArray(p.previews)||p.previews.length!==p.result.changes.length)invalid();
   await Promise.all(p.previews.map(async(preview,i)=>{if(!preview)invalid();for(const side of ['before','after'] as const){const value=preview[side],c=p.result.changes[i][side];if(!!value!==!!c)invalid();if(!value||!c)continue;
    if(typeof value.text!=='string'||typeof value.truncated!=='boolean'||!value.span||!integer(value.span.start_byte)||!integer(value.span.end_byte)||value.span.start_byte!==c.span.start_byte||value.span.end_byte<value.span.start_byte||value.span.end_byte>c.span.end_byte||value.truncated!==(value.span.end_byte<c.span.end_byte))invalid();
    const size=new TextEncoder().encode(value.text).length;if(size>512||size!==value.span.end_byte-value.span.start_byte||!digest(value.text_sha256)||await sha256(value.text)!==value.text_sha256||(!value.truncated&&value.text_sha256!==c.text_sha256))invalid();
   }}));
  }return p;
 },
 source:async(job:Analysis,step:number,change:number,side:'before'|'after',expected:Schema['AnalysisCitation'],signal?:AbortSignal)=>{
  const r=await request<AnalysisSource>(`/v1/document-analyses/source?${new URLSearchParams({id:job.id,step:String(step),change:String(change),side})}`,{signal});
  if(!r||r.analysis_id!==job.id||r.document_id!==job.plan.sources[side==='before'?0:1].document_id||typeof r.text!=='string'||typeof r.truncated!=='boolean')invalid();citation(r.citation,job,step,side);
  const c=r.citation;if(c.source!==expected.source||c.revision!==expected.revision||c.unit_id!==expected.unit_id||c.block_id!==expected.block_id||c.span.start_byte!==expected.span.start_byte||c.span.end_byte!==expected.span.end_byte||c.text_sha256!==expected.text_sha256)invalid();
  const bytes=new TextEncoder().encode(r.text),length=c.span.end_byte-c.span.start_byte;if(bytes.length>262144||(r.truncated?bytes.length>=length:bytes.length!==length))invalid();
  if(!r.truncated&&await sha256(r.text)!==c.text_sha256)invalid();
  return r;
 }
};}
