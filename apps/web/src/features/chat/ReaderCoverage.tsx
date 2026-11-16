import type {DocumentPreview} from '../../api/client';
import {useI18n} from '../../i18n';
import {documentReader} from '../../i18n/document-reader';
import {documentNavigation} from '../../i18n/document-navigation';
import {knowledgeUnitLabels} from '../../i18n/knowledge-locators';
import {DocumentEvidenceNotice} from '../../components/DocumentEvidenceNotice';
import {Icon} from '../../components/Icon';
import {UtilityPopover} from '../../components/UtilityPopover';

export function ReaderCoverage({preview,page,select,original}:{preview:DocumentPreview;page:number;select:(page:number)=>void;original:boolean}) {
 const {locale}=useI18n(),copy=documentReader(locale);
 const unread=preview.units.filter(unit=>!['ready','empty'].includes(unit.state));
 const limited=preview.state==='partial'||preview.evidence&&preview.evidence.status!=='complete';
 if(!limited&&original)return null;
 return <div className="document-coverage-summary">
  <UtilityPopover label={documentNavigation(locale).coverage} className="document-coverage-disclosure" triggerClassName="document-coverage-trigger" trigger={<><Icon name="info" size={14}/><span>{limited?documentNavigation(locale).needsReview:copy.extracted}</span>{unread.length>0&&<span className="document-review-count">{unread.length}</span>}</>}>
  {preview.state==='partial'?<><p role="status">{copy.partial}</p>
   <nav className="document-coverage-pages" aria-label={copy.partial}>{unread.map(unit=><button key={`${unit.kind}:${unit.index}`} type="button" className="text-button" aria-current={preview.units[page-1]===unit?'page':undefined} onClick={()=>select(preview.units.indexOf(unit)+1)}>{knowledgeUnitLabels[locale][unit.kind as keyof typeof knowledgeUnitLabels[typeof locale]]??knowledgeUnitLabels[locale].page} {unit.index}</button>)}</nav>
  </>:<DocumentEvidenceNotice evidence={preview.evidence}/>}
  {!original&&<p className="document-coverage">{copy.structured}</p>}
  </UtilityPopover>
 </div>;
}
