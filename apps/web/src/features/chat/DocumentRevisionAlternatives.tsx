import type {DocumentBlock} from '../../api/client';
import {useI18n} from '../../i18n';
import {documentComparison} from '../../i18n/document-comparison';
import {wordRevisions} from '../../i18n/word-revisions';

// Source authors and timestamps are document data, not verified identities.
// Keep alternatives separate from the current-text correction editor.
export function DocumentRevisionAlternatives({block}:{block:DocumentBlock}){
 const {locale,language}=useI18n(),comparison=documentComparison(locale),copy=wordRevisions(locale);
 if(!block.revisions?.length)return null;
 return <details className="document-revision-alternatives"><summary>{copy.title} · {new Intl.NumberFormat(language).format(block.revisions.length)}</summary>
  <ol>{block.revisions.map((change,index)=><li key={index}>
   <div className="document-revision-meta"><strong>{change.kind==='deletion'||change.kind==='move_from'?comparison.before:change.kind==='insertion'||change.kind==='move_to'?comparison.after:copy.formatting}</strong>
    {change.author&&<bdi>{change.author}</bdi>}
    {change.date&&<bdi><time dateTime={change.date} title={change.date}>{revisionDate(change.date,language)}</time></bdi>}
   </div>
   {change.text&&<pre dir="auto">{change.text}</pre>}
  </li>)}</ol>
 </details>;
}

// Only format complete, zoned timestamps. Partial or unusual source dates remain
// verbatim so the reader never invents a timezone or a missing date component.
function revisionDate(value:string,language:string){
 if(!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(value))return value;
 const date=new Date(value);if(!Number.isFinite(date.getTime()))return value;
 return new Intl.DateTimeFormat(language,{dateStyle:'medium',timeStyle:'short'}).format(date);
}
