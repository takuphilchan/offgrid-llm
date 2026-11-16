import {Icon, type IconName} from '../../components/Icon';
import { useI18n } from '../../i18n';
import {modelLibrary} from '../../i18n/model-library';

export type LibraryView = 'installed' | 'discover';
export function LibraryNavigation({ view, source, change }: { view: LibraryView; source:'curated'|'search'; change: (view: LibraryView,source:'curated'|'search') => void }) {
  const { messages: text,locale } = useI18n();
  const choices=[{id:'installed',label:text.models.installed,view:'installed',source:'curated'}, {id:'discover',label:text.models.discover,view:'discover',source:'curated'}, {id:'search',label:modelLibrary(locale).search,view:'discover',source:'search'}] as const;
  const icons:Record<string,IconName>={installed:'models',discover:'plus',search:'search'};
  return <nav className="model-library-navigation" aria-label={text.models.title}>
    {choices.map(choice=><button key={choice.id} id={`model-view-${choice.id}`} className="text-button" aria-current={view===choice.view&&(view==='installed'||source===choice.source)?'page':undefined} onClick={()=>change(choice.view,choice.source)}><Icon name={icons[choice.id]} size={16}/>{choice.label}</button>)}
  </nav>;
}
