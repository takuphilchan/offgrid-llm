import type {CollectionPolicy} from '../../api/collections';
import {useI18n} from '../../i18n';
import {collectionDestination} from '../../i18n/collection-destination';

export function CollectionAudience({policy}:{policy:CollectionPolicy}){
 const {locale}=useI18n(),copy=collectionDestination(locale);
 return policy.id==='workspace'?<p>{copy.shared}</p>:<dl>{(['owner','editors','readers'] as const).map(role=><div key={role}><dt>{copy[role]}</dt><dd>{role==='owner'?<bdi>{policy.owner}</bdi>:policy[role].length?<ul>{policy[role].map(member=><li key={member}><bdi>{member}</bdi></li>)}</ul>:copy.none}</dd></div>)}</dl>;
}
