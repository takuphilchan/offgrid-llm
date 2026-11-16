import type {DocumentStructurePage} from '../../api/client';
import {useI18n} from '../../i18n';
import {documentNavigation} from '../../i18n/document-navigation';
import {documentReader} from '../../i18n/document-reader';
import {knowledgeUnitLabels} from '../../i18n/knowledge-locators';

export function DocumentStructureNavigation({page,unit,disabled,select,paginate,expanded,toggle}:{page:DocumentStructurePage;unit:string;disabled:boolean;select:(unit:string,offset:number,block?:string)=>void;paginate:(offset:number)=>void;expanded:{coverage:boolean;outline:boolean};toggle:(kind:'coverage'|'outline',value:boolean)=>void}){
 const {locale,language}=useI18n(),copy=documentNavigation(locale),reader=documentReader(locale),number=new Intl.NumberFormat(language);
 const label=(value:DocumentStructurePage['units'][number])=>value.name||`${knowledgeUnitLabels[locale][value.kind]} ${number.format(value.index)}`;
 return <div className="document-structure-navigation">
  <details open={expanded.coverage} onToggle={event=>toggle('coverage',event.currentTarget.open)}><summary>{copy.coverage}</summary><ul className="document-unit-coverage">{page.units.map(item=><li key={item.id}>
   <button type="button" className="text-button" disabled={disabled} aria-current={unit===item.id?'location':undefined} onClick={()=>select(item.id,0)}><bdi>{label(item)}</bdi></button>
   {item.block_count!==undefined&&<span>{copy.blocks}: {number.format(item.block_count)}</span>}
   {((item.review_blocks??0)>0||!['ready','empty'].includes(item.state)||page.coverage?.review_units?.includes(item.index))&&<span data-review-needed>{copy.needsReview}{(item.review_blocks??0)>0?`: ${number.format(item.review_blocks!)}`:''}</span>}
   {!!item.figure_blocks&&<span>{copy.figures}: {number.format(item.figure_blocks)}</span>}{item.hidden&&<span>{copy.hidden}</span>}
  </li>)}</ul></details>
  <details open={expanded.outline} onToggle={event=>toggle('outline',event.currentTarget.open)}><summary>{copy.outline}{page.total_navigation!==undefined?` · ${number.format(page.total_navigation)}`:''}</summary>
   {page.navigation===undefined?<p role="status">{copy.unavailable}</p>:!page.navigation.length?<p>{copy.empty}</p>:<ol className="document-outline" start={(page.navigation_offset??0)+1}>{page.navigation.map(entry=><li key={entry.block_id} data-heading-level={entry.heading_level??0}>
    <button className="text-button" disabled={disabled} onClick={()=>select(entry.unit_id,entry.block_offset,entry.block_id)}><bdi>{entry.title||`${entry.kind==='figure'?copy.figures:copy.blocks} ${number.format(entry.block_offset+1)}`}</bdi></button>{entry.needs_review&&<small>{copy.needsReview}</small>}
   </li>)}</ol>}
   {page.navigation!==undefined&&<nav aria-label={copy.outline}><button className="text-button" disabled={disabled||!page.navigation_offset} onClick={()=>paginate(Math.max(0,(page.navigation_offset??0)-100))}>{reader.previous}</button><bdi dir="ltr">{number.format((page.navigation_offset??0)+(page.navigation.length?1:0))}–{number.format((page.navigation_offset??0)+page.navigation.length)} / {number.format(page.total_navigation??0)}</bdi><button className="text-button" disabled={disabled||page.next_navigation_offset==null} onClick={()=>paginate(page.next_navigation_offset!)}>{reader.next}</button></nav>}
  </details>
 </div>;
}
