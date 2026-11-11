import { defineMessages } from './catalog';
import type {LocaleCode} from './index';
type Copy={title:string;browse:string;search:string;empty:string;hint:string};
export const themeChooser=defineMessages<Copy>("i18n/theme-chooser:themeChooser", {
 en:{title:'Choose a theme',browse:'Browse themes',search:'Search themes',empty:'No matching themes.',hint:'Preview a theme, then apply it. Light and dark choices are saved separately.'},
 fr:{title:'Choisir un thème',browse:'Parcourir les thèmes',search:'Rechercher un thème',empty:'Aucun thème correspondant.',hint:'Prévisualisez un thème, puis appliquez-le. Les choix clair et sombre sont enregistrés séparément.'},
 es:{title:'Elegir un tema',browse:'Explorar temas',search:'Buscar temas',empty:'No hay temas coincidentes.',hint:'Previsualiza un tema y aplícalo. Las opciones clara y oscura se guardan por separado.'},
 de:{title:'Design auswählen',browse:'Designs ansehen',search:'Designs suchen',empty:'Keine passenden Designs.',hint:'Sehen Sie sich ein Design an und wenden Sie es an. Helle und dunkle Designs werden getrennt gespeichert.'},
 ar:{title:'اختيار سمة',browse:'تصفح السمات',search:'البحث عن سمات',empty:'لا توجد سمات مطابقة.',hint:'عاين السمة ثم طبّقها. يُحفظ اختيارا الوضع الفاتح والداكن بشكل منفصل.'},
 sw:{title:'Chagua mandhari',browse:'Vinjari mandhari',search:'Tafuta mandhari',empty:'Hakuna mandhari yanayolingana.',hint:'Hakiki mandhari, kisha yatumie. Chaguo za mwonekano mwepesi na mweusi huhifadhiwa tofauti.'},
 sn:{title:'Sarudza dingindira',browse:'Ona madingindira',search:'Tsvaga madingindira',empty:'Hapana dingindira rinoenderana.',hint:'Tarisa dingindira usati warishandisa. Sarudzo dzakajeka nedzakasviba dzinochengetwa dzakaparadzana.'},
 nd:{title:'Khetha itimu',browse:'Bona amatimu',search:'Dinga amatimu',empty:'Akulatimu ehambelanayo.',hint:'Bona itimu kuqala, ubusuyisebenzisa. Izinketho ezikhanyayo lezimnyama zigcinwa ngokwehlukana.'},
 zu:{title:'Khetha itimu',browse:'Buka amatimu',search:'Sesha amatimu',empty:'Ayikho itimu ehambisanayo.',hint:'Buka itimu kuqala, bese uyisebenzisa. Izinketho ezikhanyayo nezimnyama zigcinwa ngokwehlukana.'}
});
