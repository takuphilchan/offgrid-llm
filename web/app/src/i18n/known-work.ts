import type { LocaleCode } from './index';
export const unavailableTurnText: Record<LocaleCode, string> = {
  en: 'This service cannot recover the previous turn. Saved messages and your draft are retained. Check status before starting new work; nothing was resent.',
  fr: 'Ce service ne peut pas récupérer le tour précédent. Messages et brouillon conservés. Vérifiez l’état avant un nouveau travail ; rien n’a été renvoyé.',
  es: 'Este servicio no puede recuperar el turno anterior. Se conservan los mensajes y el borrador. Comprueba el estado antes de iniciar otro trabajo; no se reenvió nada.',
  de: 'Dieser Dienst kann den vorherigen Durchgang nicht wiederherstellen. Nachrichten und Entwurf bleiben erhalten. Status vor neuer Arbeit prüfen; nichts erneut gesendet.',
  ar: 'لا تستطيع هذه الخدمة استعادة الجولة السابقة. الرسائل والمسودة محفوظة. تحقق من الحالة قبل بدء عمل جديد؛ لم يُعد إرسال شيء.',
  sw: 'Huduma haiwezi kurejesha zamu iliyopita. Ujumbe na rasimu zimehifadhiwa. Kagua hali kabla ya kazi mpya; hakuna kilichotumwa tena.',
  sn: 'Sevhisi haigoni kudzosa mhinduro yapfuura. Mameseji nezvawanyora zvakachengetwa. Tarisa mamiriro usati watanga basa idzva; hapana chatumirwazve.',
  nd: 'Isevisi ayikwazi ukubuyisela impendulo eyedlulileyo. Imilayezo lokubhalileyo kugciniwe. Hlola isimo ungakaqali umsebenzi omutsha; akukho okuthunyelwe futhi.',
  zu: 'Isevisi ayikwazi ukubuyisela impendulo edlule. Imilayezo nokubhalile kugciniwe. Hlola isimo ngaphambi komsebenzi omusha; akukho okuthunyelwe futhi.',
};
export const knownWorkText: Record<LocaleCode, { title: string; scope: string; stale: string }> = {
  en: { title: 'Known work', scope: 'Recent conversations opened here and active tasks. This is not a count of all workspace activity.', stale: 'Status unavailable — open to check' },
  fr: { title: 'Travail connu', scope: 'Conversations ouvertes ici et tâches actives, pas toute l’activité de l’espace.', stale: 'État indisponible — ouvrir pour vérifier' },
  es: { title: 'Trabajo conocido', scope: 'Conversaciones abiertas aquí y tareas activas, no toda la actividad del espacio.', stale: 'Estado no disponible — abrir para comprobar' },
  de: { title: 'Bekannte Arbeit', scope: 'Hier geöffnete Unterhaltungen und aktive Aufgaben, nicht die gesamte Arbeitsbereichsaktivität.', stale: 'Status nicht verfügbar — zum Prüfen öffnen' },
  ar: { title: 'العمل المعروف', scope: 'المحادثات المفتوحة هنا والمهام النشطة، وليس كل نشاط مساحة العمل.', stale: 'الحالة غير متاحة — افتح للتحقق' },
  sw: { title: 'Kazi zinazojulikana', scope: 'Mazungumzo yaliyofunguliwa hapa na kazi hai, si shughuli zote za eneo la kazi.', stale: 'Hali haipatikani — fungua ukague' },
  sn: { title: 'Basa rinozivikanwa', scope: 'Hurukuro dzakavhurwa pano nemabasa ari kushanda, kwete zvose zviri munzvimbo yebasa.', stale: 'Mamiriro haawanikwi — vhura utarise' },
  nd: { title: 'Umsebenzi owaziwayo', scope: 'Izingxoxo ezivulwe lapha lemisebenzi esebenzayo, hatshi konke okwenzakala endaweni yomsebenzi.', stale: 'Isimo asitholakali — vula uhlole' },
  zu: { title: 'Umsebenzi owaziwayo', scope: 'Izingxoxo ezivulwe lapha nemisebenzi esebenzayo, hhayi konke okwenzeka endaweni yomsebenzi.', stale: 'Isimo asitholakali — vula uhlole' },
};
