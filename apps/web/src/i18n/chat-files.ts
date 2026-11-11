import { defineMessages } from './catalog';
import type { LocaleCode } from './index';
type Labels = readonly [string,string,string,string,string,string,string,string,string];
const labels = defineMessages<Labels>("i18n/chat-files:labels", {
 en:['Add files','Files','Remove from draft','Preparing file…','Ready','Retry preparation','Download original','Files need preparation before sending. Your draft is retained.','Files are supported in ordinary Chat. Switch off the agent to use these attachments.'],
 fr:['Ajouter des fichiers','Fichiers','Retirer du brouillon','Préparation du fichier…','Prêt','Réessayer la préparation','Télécharger l’original','Préparez les fichiers avant l’envoi. Votre brouillon est conservé.','Les fichiers sont disponibles dans le Chat ordinaire. Désactivez l’agent pour utiliser ces pièces jointes.'],
 de:['Dateien hinzufügen','Dateien','Aus Entwurf entfernen','Datei wird vorbereitet…','Bereit','Vorbereitung wiederholen','Original herunterladen','Bereiten Sie die Dateien vor dem Senden vor. Ihr Entwurf bleibt erhalten.','Dateien werden im normalen Chat unterstützt. Deaktivieren Sie den Agenten für diese Anhänge.'],
 es:['Añadir archivos','Archivos','Quitar del borrador','Preparando archivo…','Listo','Reintentar preparación','Descargar original','Prepara los archivos antes de enviar. Se conserva el borrador.','Los archivos se admiten en el Chat normal. Desactiva el agente para usar estos adjuntos.'],
 ar:['إضافة ملفات','الملفات','إزالة من المسودة','جارٍ تجهيز الملف…','جاهز','إعادة محاولة التجهيز','تنزيل الأصل','يجب تجهيز الملفات قبل الإرسال. تم الاحتفاظ بالمسودة.','تُدعم الملفات في الدردشة العادية. عطّل الوكيل لاستخدام هذه المرفقات.'],
 sw:['Ongeza faili','Faili','Ondoa kwenye rasimu','Inaandaa faili…','Tayari','Jaribu kuandaa tena','Pakua faili asili','Faili zinahitaji kuandaliwa kabla ya kutuma. Rasimu imehifadhiwa.','Faili zinatumika katika Chat ya kawaida. Zima wakala kutumia viambatisho hivi.'],
 sn:['Wedzera mafaera','Mafaera','Bvisa pane zvawanyora','Kugadzirira faera…','Ragadzirira','Edza kugadzirira zvakare','Dhawunirodha rekutanga','Mafaera anofanira kugadzirirwa asati atumirwa. Zvawanyora zvachengetwa.','Mafaera anoshanda muChat yakajairika. Dzima agent kuti ushandise mafaera aya.'],
 nd:['Faka amafayela','Amafayela','Susa kokubhaliweyo','Kulungiswa ifayela…','Selilungile','Zama ukulungisa futhi','Landa elokuqala','Amafayela kumele alungiswe engakathunyelwa. Okubhalileyo kugciniwe.','Amafayela asebenza kuChat ejayelekileyo. Vala i-agent ukusebenzisa lezi zinamathiselo.'],
 zu:['Engeza amafayela','Amafayela','Susa kokubhaliwe','Kulungiswa ifayela…','Lilungile','Zama ukulungisa futhi','Landa elokuqala','Amafayela adinga ukulungiswa ngaphambi kokuthumela. Okubhaliwe kugciniwe.','Amafayela asebenza kuChat evamile. Vala i-agent ukuze usebenzise lezi zinamathiselo.'],
});
export function chatFiles(locale:LocaleCode){const [add,files,remove,preparing,ready,retry,download,blocked,agent]=labels[locale];return {add,files,remove,preparing,ready,retry,download,blocked,agent};}

const recovery=defineMessages<readonly[string,string,string,string,string]>("i18n/chat-files:recovery", {
 en:['Saved files','No saved files in this conversation.','Use in next message','Selected','This file selection could not be saved on this device. Keep this window open.'],
 fr:['Fichiers conservés','Aucun fichier dans cette conversation.','Utiliser au prochain message','Sélectionné','Cette sélection ne peut pas être enregistrée sur cet appareil. Gardez cette fenêtre ouverte.'],
 de:['Gespeicherte Dateien','Keine Dateien in diesem Gespräch.','In nächster Nachricht verwenden','Ausgewählt','Diese Auswahl konnte nicht auf diesem Gerät gespeichert werden. Lassen Sie das Fenster geöffnet.'],
 es:['Archivos guardados','No hay archivos en esta conversación.','Usar en el próximo mensaje','Seleccionado','No se pudo guardar la selección en este dispositivo. Mantén esta ventana abierta.'],
 ar:['الملفات المحفوظة','لا توجد ملفات محفوظة في هذه المحادثة.','استخدم في الرسالة التالية','محدد','تعذر حفظ هذا الاختيار على هذا الجهاز. أبقِ هذه النافذة مفتوحة.'],
 sw:['Faili zilizohifadhiwa','Hakuna faili zilizohifadhiwa katika mazungumzo haya.','Tumia katika ujumbe unaofuata','Imechaguliwa','Uteuzi haukuhifadhiwa kwenye kifaa hiki. Acha dirisha hili wazi.'],
 sn:['Mafaera akachengetwa','Hapana mafaera akachengetwa muhurukuro iyi.','Shandisa mumeseji inotevera','Rasarudzwa','Sarudzo iyi haina kuchengetwa pamudziyo uyu. Siya hwindo iri rakavhurika.'],
 nd:['Amafayela agciniweyo','Akulafayela eligciniweyo kule ingxoxo.','Sebenzisa emlayezweni olandelayo','Likhethiwe','Ukukhetha lokhu akugcinwanga kulo umtshina. Tshiya iwindi livuliwe.'],
 zu:['Amafayela agciniwe','Awekho amafayela agciniwe kule ngxoxo.','Sebenzisa emyalezweni olandelayo','Likhethiwe','Lokhu okukhethiwe akugcinwanga kule divayisi. Gcina leli windi livuliwe.'],
});
export function fileRecovery(locale:LocaleCode){const [saved,empty,use,selected,unsaved]=recovery[locale];return {saved,empty,use,selected,unsaved};}
