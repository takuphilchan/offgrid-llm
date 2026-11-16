import {defineMessages} from './catalog';
import type {LocaleCode} from './index';
const copy=defineMessages('i18n/response-display:copy',{
 en:{label:'Response display',live:'As it is generated',complete:'When complete',hint:'Live text is a draft. Document checks and runtimes without streaming may delay display.'},
 fr:{label:'Affichage des réponses',live:'Au fil de la génération',complete:'Une fois terminées',hint:'Le texte en direct est un brouillon. Les vérifications des documents et les moteurs sans diffusion progressive peuvent retarder son affichage.'},
 de:{label:'Antwortanzeige',live:'Während der Erstellung',complete:'Nach Abschluss',hint:'Live-Text ist ein Entwurf. Dokumentprüfungen und Laufzeitumgebungen ohne Streaming können die Anzeige verzögern.'},
 es:{label:'Visualización de respuestas',live:'Durante la generación',complete:'Al finalizar',hint:'El texto en directo es un borrador. Las comprobaciones de documentos y los motores sin transmisión progresiva pueden retrasar su visualización.'},
 ar:{label:'عرض الردود',live:'أثناء الإنشاء',complete:'بعد الاكتمال',hint:'النص المباشر مسودة. قد تؤخر فحوصات المستندات وبيئات التشغيل التي لا تدعم البث عرض الرد.'},
 sw:{label:'Uonyeshaji wa majibu',live:'Jibu linapoandikwa',complete:'Jibu likikamilika',hint:'Maandishi yanayoonyeshwa papo hapo ni rasimu. Ukaguzi wa hati na mifumo isiyosaidia utiririshaji inaweza kuchelewesha kuonyeshwa.'},
 sn:{label:'Kuratidzwa kwemhinduro',live:'Painenge ichinyorwa',complete:'Kana yapera',hint:'Mashoko ari kuratidzwa achinyorwa irasimu. Kuongororwa kwemagwaro nemasisitimu asingatsigiri kuratidza mashoko achinyorwa zvinogona kunonotsa kuratidzwa.'},
 nd:{label:'Ukuboniswa kwezimpendulo',live:'Ngesikhathi kubhalwa',complete:'Nxa sekuphelile',hint:'Umbhalo oboniswa ngesikhathi ubhalwa usalungiswa. Ukuhlolwa kwamadokhumenti lezinhlelo ezingasekeli lokhu kungaphuzisa ukuboniswa.'},
 zu:{label:'Ukuboniswa kwezimpendulo',live:'Ngesikhathi kubhalwa',complete:'Uma sekuqediwe',hint:'Umbhalo oboniswa ngesikhathi ubhalwa usalungiswa. Ukuhlolwa kwamadokhumenti nezinhlelo ezingakusekeli lokhu kungabambezela ukuboniswa.'}
});
export const responseDisplay=(locale:LocaleCode)=>copy[locale];
