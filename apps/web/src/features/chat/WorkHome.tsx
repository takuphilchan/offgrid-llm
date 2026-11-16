import type {ChatSession} from '../../api/client';
import {BrandMark} from '../../components/BrandMark';
import {Icon} from '../../components/Icon';
import {useI18n} from '../../i18n';
import {workSurface} from '../../i18n/work-surface';
import './work-home.css';

export function WorkHome({sessions,open}:{sessions:ChatSession[];open:(session:ChatSession)=>void;addFile:()=>void}) {
 const {locale,messages,language}=useI18n(),copy=workSurface(locale);
 return <section className="work-home" aria-label={copy.home}>
  <div className="work-home-intro"><span className="work-home-brand" aria-hidden="true"><BrandMark /></span><h2>{copy.welcome}</h2></div>
  {sessions.length>0&&<div className="work-home-recent"><h3>{messages.chat.conversations}</h3><ul>{sessions.slice(0,5).map(session=><li key={session.name}><button onClick={()=>open(session)}><Icon name="chat" size={17}/><span><bdi className="work-home-session-name">{session.name}</bdi></span>{Number.isFinite(Date.parse(session.updated_at))&&<time>{new Date(session.updated_at).toLocaleDateString(language,{month:'short',day:'numeric'})}</time>}<Icon name="chevron-right" size={16}/></button></li>)}</ul></div>}
 </section>;
}
