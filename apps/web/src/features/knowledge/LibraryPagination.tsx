import {useI18n} from '../../i18n';
import {documentReader} from '../../i18n/document-reader';
import {knowledgeUnitLabels} from '../../i18n/knowledge-locators';

export function LibraryPagination({page,pages,onChange}:{page:number;pages:number;onChange:(page:number)=>void}){
 const {locale,language}=useI18n(),copy=documentReader(locale),number=new Intl.NumberFormat(language);
 if(pages<=1)return null;
 return <nav className="collection-library-controls library-pagination" aria-label={knowledgeUnitLabels[locale].page}>
  <button type="button" className="secondary-button" disabled={page<=1} onClick={()=>onChange(page-1)}>{copy.previous}</button>
  <span role="status" aria-live="polite">{knowledgeUnitLabels[locale].page} {number.format(page)} / {number.format(pages)}</span>
  <button type="button" className="secondary-button" disabled={page>=pages} onClick={()=>onChange(page+1)}>{copy.next}</button>
 </nav>;
}
