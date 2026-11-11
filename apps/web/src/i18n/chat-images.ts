import { defineMessages } from './catalog';
import type { LocaleCode } from './index';
type Copy = readonly [string,string,string,string,string,string,string,string];
const labels = defineMessages<Copy>("i18n/chat-images:labels", {
 en:['Add images','Remove image','Images','Up to 4 PNG/JPEG images, 2 MiB each. Paste or drop images here.','Choose an installed vision model with its projector. Your images are retained.','Images are not supported in agent turns yet. Use ordinary Chat.','Update the service to send images. Your draft is retained.','Some images could not be saved locally. Keep this window open.'],
 fr:['Ajouter des images','Retirer l’image','Images','Jusqu’à 4 images PNG/JPEG, 2 Mio chacune. Collez ou déposez les images ici.','Choisissez un modèle de vision installé avec son projecteur. Vos images sont conservées.','Les agents ne prennent pas encore en charge les images. Utilisez le Chat ordinaire.','Mettez le service à jour pour envoyer des images. Votre brouillon est conservé.','Certaines images n’ont pas pu être enregistrées localement. Gardez cette fenêtre ouverte.'],
 de:['Bilder hinzufügen','Bild entfernen','Bilder','Bis zu 4 PNG/JPEG-Bilder mit je 2 MiB. Hier einfügen oder ablegen.','Wählen Sie ein installiertes Bildmodell mit Projektor. Ihre Bilder bleiben erhalten.','Agenten unterstützen noch keine Bildeingaben. Verwenden Sie den normalen Chat.','Aktualisieren Sie den Dienst für Bilder. Ihr Entwurf bleibt erhalten.','Einige Bilder konnten nicht lokal gespeichert werden. Lassen Sie dieses Fenster offen.'],
 es:['Añadir imágenes','Quitar imagen','Imágenes','Hasta 4 imágenes PNG/JPEG, 2 MiB cada una. Pégalas o arrástralas aquí.','Elige un modelo de visión instalado con su proyector. Se conservan las imágenes.','Los agentes aún no admiten imágenes. Usa el Chat normal.','Actualiza el servicio para enviar imágenes. Se conserva el borrador.','No se pudieron guardar algunas imágenes localmente. Mantén esta ventana abierta.'],
 ar:['إضافة صور','إزالة الصورة','الصور','حتى 4 صور PNG/JPEG، بحجم 2 ميبيبايت لكل صورة. الصق الصور أو اسحبها هنا.','اختر نموذج رؤية مثبتاً مع ملف الإسقاط الخاص به. ستُحفظ صورك.','لا تدعم مهام الوكيل الصور بعد. استخدم الدردشة العادية.','حدّث الخدمة لإرسال الصور. ستُحفظ مسودتك.','تعذر حفظ بعض الصور محلياً. أبقِ هذه النافذة مفتوحة.'],
 sw:['Ongeza picha','Ondoa picha','Picha','Hadi picha 4 za PNG/JPEG, MiB 2 kila moja. Bandika au buruta picha hapa.','Chagua modeli ya kuona iliyosakinishwa pamoja na projekta yake. Picha zako zimehifadhiwa.','Kazi za wakala bado hazikubali picha. Tumia Chat ya kawaida.','Sasisha huduma kutuma picha. Rasimu yako imehifadhiwa.','Baadhi ya picha hazikuweza kuhifadhiwa kwenye kifaa. Weka dirisha hili wazi.'],
 sn:['Wedzera mifananidzo','Bvisa mufananidzo','Mifananidzo','Mifananidzo inosvika 4 yePNG/JPEG, 2 MiB imwe neimwe. Namira kana dhonza pano.','Sarudza modhi yekuona yakaiswa ine projekta yayo. Mifananidzo yako yachengetwa.','Mabasa eagent haasati atsigira mifananidzo. Shandisa Chat yakajairika.','Gadziridza sevhisi kutumira mifananidzo. Zvawanyora zvachengetwa.','Mimwe mifananidzo haina kukwanisa kuchengetwa pamudziyo. Siya hwindo iri rakavhurika.'],
 nd:['Faka imifanekiso','Susa umfanekiso','Imifanekiso','Imifanekiso yePNG/JPEG engedluli ku-4, 2 MiB ngamunye. Namathisela kumbe udonse lapha.','Khetha imodeli yokubona efakiweyo leprojektha yayo. Imifanekiso yakho igciniwe.','Imisebenzi ye-agent kayikasekeli imifanekiso. Sebenzisa iChat ejayelekileyo.','Vuselela isevisi ukuthumela imifanekiso. Okubhalayo kugciniwe.','Eminye imifanekiso kayigcinwanga kudivayisi. Gcina leliwindi livuliwe.'],
 zu:['Engeza izithombe','Susa isithombe','Izithombe','Izithombe zePNG/JPEG ezingafika ku-4, 2 MiB ngasinye. Namathisela noma hudulela lapha.','Khetha imodeli yokubona efakiwe neprojektha yayo. Izithombe zakho zigciniwe.','Imisebenzi ye-agent ayikasekeli izithombe. Sebenzisa iChat evamile.','Buyekeza isevisi ukuze uthumele izithombe. Okubhalayo kugciniwe.','Ezinye izithombe azikwazanga ukugcinwa kudivayisi. Gcina leli windi livuliwe.'],
});
export function chatImages(locale:LocaleCode){const [add,remove,images,limits,model,agent,service,unsaved]=labels[locale];return {add,remove,images,limits,model,agent,service,unsaved};}

export const imagePreview = defineMessages<{open:string;close:string;zoom:string}>("i18n/chat-images:imagePreview", {
 en:{open:'Preview image',close:'Close image',zoom:'Actual size'},
 fr:{open:'Aperçu de l’image',close:'Fermer l’image',zoom:'Taille réelle'},
 de:{open:'Bildvorschau',close:'Bild schließen',zoom:'Originalgröße'},
 es:{open:'Vista previa de imagen',close:'Cerrar imagen',zoom:'Tamaño real'},
 ar:{open:'معاينة الصورة',close:'إغلاق الصورة',zoom:'الحجم الفعلي'},
 sw:{open:'Hakiki picha',close:'Funga picha',zoom:'Ukubwa halisi'},
 sn:{open:'Tarisa mufananidzo',close:'Vhara mufananidzo',zoom:'Saizi chaiyo'},
 nd:{open:'Hlola umfanekiso',close:'Vala umfanekiso',zoom:'Ubukhulu bangempela'},
 zu:{open:'Buka isithombe',close:'Vala isithombe',zoom:'Usayizi wangempela'},
});
