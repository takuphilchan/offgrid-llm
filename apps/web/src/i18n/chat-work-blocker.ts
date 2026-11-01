import type { LocaleCode } from './index';

export const chatUncertainHint: Record<LocaleCode, string> = {
  en: 'Review the unconfirmed action before continuing. Your draft stays here.',
  fr: 'Examinez l’action non confirmée avant de continuer. Votre brouillon reste ici.',
  es: 'Revisa la acción no confirmada antes de continuar. Tu borrador permanece aquí.',
  de: 'Prüfe die unbestätigte Aktion vor dem Fortsetzen. Dein Entwurf bleibt hier.',
  ar: 'راجع الإجراء غير المؤكد قبل المتابعة. تبقى مسودتك هنا.',
  sw: 'Kagua kitendo kisichothibitishwa kabla ya kuendelea. Rasimu yako inabaki hapa.',
  sn: 'Ongorora chiito chisina kusimbiswa usati waenderera. Rasimu yako inosara pano.',
  nd: 'Hlola isenzo esingaqinisekiswanga ungakaqhubeki. Uhlaka lwakho luhlala lapha.',
  zu: 'Hlola isenzo esingaqinisekisiwe ngaphambi kokuqhubeka. Uhlaka lwakho luhlala lapha.',
};

// Loading, access, approval, children, and other non-steerable active work.
export const chatWorkBlockerText: Record<LocaleCode, readonly [string, string, string, string, string]> = {
  en: ['Checking the saved task. Your draft is kept here.', 'This task needs access before it can continue. Inspect work to review the request.', 'This task needs your approval. Inspect work to review the exact action.', 'Specialists are still working. Wait for their results or start a separate conversation.', 'This task cannot accept another instruction yet. Inspect its progress or start a separate conversation.'],
  fr: ['Vérification de la tâche enregistrée. Votre brouillon reste ici.', 'Cette tâche attend un accès. Consultez le travail pour examiner la demande.', 'Cette tâche attend votre approbation. Consultez le travail pour examiner l’action exacte.', 'Les spécialistes travaillent encore. Attendez leurs résultats ou ouvrez une autre conversation.', 'Cette tâche ne peut pas encore recevoir de consigne. Consultez sa progression ou ouvrez une autre conversation.'],
  es: ['Comprobando la tarea guardada. Tu borrador se conserva aquí.', 'Esta tarea necesita acceso. Inspecciona el trabajo para revisar la solicitud.', 'Esta tarea necesita tu aprobación. Inspecciona el trabajo para revisar la acción exacta.', 'Los especialistas siguen trabajando. Espera los resultados o inicia otra conversación.', 'Esta tarea aún no puede recibir instrucciones. Revisa su progreso o inicia otra conversación.'],
  de: ['Gespeicherte Aufgabe wird geprüft. Dein Entwurf bleibt hier.', 'Diese Aufgabe benötigt Zugriff. Prüfe die Arbeit und die Zugriffsanfrage.', 'Diese Aufgabe benötigt deine Freigabe. Prüfe die Arbeit und die genaue Aktion.', 'Die Spezialisten arbeiten noch. Warte auf Ergebnisse oder starte ein separates Gespräch.', 'Diese Aufgabe kann noch keine Anweisung empfangen. Prüfe den Fortschritt oder starte ein separates Gespräch.'],
  ar: ['جارٍ فحص المهمة المحفوظة. تبقى مسودتك هنا.', 'تحتاج هذه المهمة إلى إذن وصول. افحص العمل لمراجعة الطلب.', 'تحتاج هذه المهمة إلى موافقتك. افحص العمل لمراجعة الإجراء المحدد.', 'ما زال المختصون يعملون. انتظر النتائج أو ابدأ محادثة منفصلة.', 'لا تستطيع هذه المهمة قبول تعليمات أخرى بعد. افحص تقدمها أو ابدأ محادثة منفصلة.'],
  sw: ['Inakagua kazi iliyohifadhiwa. Rasimu yako inabaki hapa.', 'Kazi hii inahitaji ufikiaji. Kagua kazi ili kuona ombi.', 'Kazi hii inahitaji idhini yako. Kagua kazi ili kuona kitendo halisi.', 'Wataalamu bado wanafanya kazi. Subiri matokeo au anza mazungumzo tofauti.', 'Kazi hii haiwezi kupokea maagizo mengine bado. Kagua maendeleo au anza mazungumzo tofauti.'],
  sn: ['Kuongorora basa rakachengetwa. Rasimu yako inosara pano.', 'Basa iri rinoda mvumo yekuwana. Ongorora basa kuti uone chikumbiro.', 'Basa iri rinoda kubvumidzwa newe. Ongorora basa kuti uone chiito chaicho.', 'Nyanzvi dzichiri kushanda. Mirira mhinduro kana tanga imwe hurukuro.', 'Basa iri harisati ragamuchira mumwe murayiro. Ongorora kufambira mberi kana tanga imwe hurukuro.'],
  nd: ['Kuhlolwa umsebenzi ogciniweyo. Uhlaka lwakho luhlala lapha.', 'Umsebenzi lo udinga ukufinyelela. Hlola umsebenzi ukuze ubone isicelo.', 'Umsebenzi lo udinga imvumo yakho. Hlola umsebenzi ukuze ubone isenzo esiqondileyo.', 'Izingcitshi zisasebenza. Linda imiphumela kumbe uqale enye ingxoxo.', 'Umsebenzi lo kawukwazi ukwamukela omunye umlayo okwamanje. Hlola ukuqhubeka kumbe uqale enye ingxoxo.'],
  zu: ['Kuhlolwa umsebenzi olondoloziwe. Uhlaka lwakho luhlala lapha.', 'Lo msebenzi udinga ukufinyelela. Hlola umsebenzi ukuze ubone isicelo.', 'Lo msebenzi udinga imvume yakho. Hlola umsebenzi ukuze ubone isenzo esiqondile.', 'Ochwepheshe basasebenza. Linda imiphumela noma uqale enye ingxoxo.', 'Lo msebenzi awukwazi ukwamukela omunye umyalelo okwamanje. Hlola ukuqhubeka noma uqale enye ingxoxo.'],
};
