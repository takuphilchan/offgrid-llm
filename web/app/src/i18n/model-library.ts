import type { LocaleCode } from './index';
type Labels = readonly [string, string, string, string, string, string, string, string, string, string];
const labels: Record<LocaleCode, Labels> = {
  en: ['Installed size', 'Repository size (all files)', 'Unknown', 'Runtime unchecked', 'Loaded', 'No catalog models in this category. Search Hugging Face to discover more.', 'Filter installed models', 'Curated suggestions', 'Search Hugging Face', 'Model details'],
  fr: ['Taille installée', 'Taille du dépôt (tous les fichiers)', 'Inconnu', 'Moteur non vérifié', 'Chargé', 'Aucun modèle du catalogue dans cette catégorie. Recherchez sur Hugging Face.', 'Filtrer les modèles installés', 'Suggestions sélectionnées', 'Rechercher sur Hugging Face', 'Détails du modèle'],
  es: ['Tamaño instalado', 'Tamaño del repositorio (todos los archivos)', 'Desconocido', 'Motor sin comprobar', 'Cargado', 'No hay modelos del catálogo en esta categoría. Busca en Hugging Face.', 'Filtrar modelos instalados', 'Sugerencias seleccionadas', 'Buscar en Hugging Face', 'Detalles del modelo'],
  de: ['Installierte Größe', 'Repository-Größe (alle Dateien)', 'Unbekannt', 'Laufzeit ungeprüft', 'Geladen', 'Keine Katalogmodelle in dieser Kategorie. Auf Hugging Face suchen.', 'Installierte Modelle filtern', 'Kuratierte Vorschläge', 'Auf Hugging Face suchen', 'Modelldetails'],
  ar: ['الحجم المثبت', 'حجم المستودع (كل الملفات)', 'غير معروف', 'لم يُفحص المحرك', 'محمّل', 'لا توجد نماذج في الكتالوج لهذه الفئة. ابحث في Hugging Face.', 'تصفية النماذج المثبتة', 'اقتراحات منتقاة', 'البحث في Hugging Face', 'تفاصيل النموذج'],
  sw: ['Ukubwa uliosakinishwa', 'Ukubwa wa hazina (faili zote)', 'Haijulikani', 'Injini haijakaguliwa', 'Imepakiwa', 'Hakuna modeli za katalogi katika aina hii. Tafuta Hugging Face.', 'Chuja modeli zilizosakinishwa', 'Mapendekezo yaliyochaguliwa', 'Tafuta Hugging Face', 'Maelezo ya modeli'],
  sn: ['Ukuru hwakaiswa', 'Ukuru hwerepository (mafaira ose)', 'Hazvizivikanwi', 'Injini haisati yaongororwa', 'Yakaiswa mundangariro', 'Hapana mamodheru mukatalogi yerudzi urwu. Tsvaga paHugging Face.', 'Sefa mamodheru akaiswa', 'Zvakakurudzirwa zvakasarudzwa', 'Tsvaga paHugging Face', 'Tsananguro yemodheru'],
  nd: ['Ubukhulu obufakiweyo', 'Ubukhulu bendawo yamafayela (wonke)', 'Akwaziwa', 'Injini ayikahlolwa', 'Ilayishiwe', 'Akulamamodeli ekhathalogi kulolu hlobo. Dinga kuHugging Face.', 'Hlunga amamodeli afakiweyo', 'Iziphakamiso ezikhethiweyo', 'Dinga kuHugging Face', 'Imininingwane yemodeli'],
  zu: ['Usayizi ofakiwe', 'Usayizi wenqolobane (wonke amafayela)', 'Akwaziwa', 'Injini ayikahlolwa', 'Ilayishiwe', 'Awekho amamodeli ekhathalogi kulolu hlobo. Sesha kuHugging Face.', 'Hlunga amamodeli afakiwe', 'Iziphakamiso ezikhethiwe', 'Sesha kuHugging Face', 'Imininingwane yemodeli'],
};
const legacy: Record<LocaleCode, readonly [string, string]> = {
  en: ['Unpinned revision', 'This legacy download service does not provide a disk-space preflight or immutable source revision. Check available space and model/runtime compatibility before downloading.'],
  fr: ['Révision non figée', 'Ce service historique ne fournit ni contrôle préalable de l’espace disque ni révision source immuable. Vérifiez l’espace libre et la compatibilité du modèle et du moteur avant de télécharger.'],
  es: ['Revisión no fijada', 'Este servicio antiguo no ofrece una comprobación previa de espacio ni una revisión inmutable. Comprueba el espacio y la compatibilidad del modelo y motor antes de descargar.'],
  de: ['Nicht festgelegte Revision', 'Dieser ältere Downloaddienst prüft den Speicherplatz nicht vorab und bietet keine unveränderliche Quellrevision. Freien Speicher und Modell-/Laufzeitkompatibilität vor dem Download prüfen.'],
  ar: ['مراجعة غير مثبتة', 'لا توفر خدمة التنزيل القديمة فحصًا مسبقًا لمساحة القرص أو مراجعة مصدر ثابتة. تحقق من المساحة المتاحة وتوافق النموذج والمحرك قبل التنزيل.'],
  sw: ['Toleo halijafungwa', 'Huduma hii ya zamani haikagui nafasi kabla wala kutoa toleo lisilobadilika. Kagua nafasi na utangamano wa modeli na injini kabla ya kupakua.'],
  sn: ['Vhezheni haina kusungirirwa', 'Sevhisi yekare iyi haiongorori nzvimbo isati yatanga kana kupa vhezheni isingachinji. Ongorora nzvimbo uye kuenderana kwemodheru neinjini usati wadhaunirodha.'],
  nd: ['Inguqulo ayibotshwanga', 'Le nsizakalo endala ayihloli isikhala kusengaphambili njalo ayiniki inguqulo engaguqukiyo. Hlola isikhala lokuhambelana kwemodeli lenjini ungakalandi.'],
  zu: ['Inguqulo ayiboshiwe', 'Le sevisi endala ayihloli isikhala ngaphambi kwesikhathi futhi ayinikezi inguqulo engaguquki. Hlola isikhala nokuhambisana kwemodeli nenjini ngaphambi kokulanda.'],
};
export function modelLibrary(locale: LocaleCode) {
  const [installedSize, repositorySize, unknown, unchecked, loaded, emptyCatalog, filterInstalled, curated, search, details] = labels[locale];
  const noMatches: Record<LocaleCode, string> = { en: 'No models match this filter.', fr: 'Aucun modèle ne correspond à ce filtre.', es: 'Ningún modelo coincide con este filtro.', de: 'Keine Modelle entsprechen diesem Filter.', ar: 'لا توجد نماذج تطابق هذا المرشح.', sw: 'Hakuna modeli zinazolingana na kichujio hiki.', sn: 'Hapana mamodheru anoenderana nesefa iyi.', nd: 'Akulamamodeli ahambelana lalesi sihlungi.', zu: 'Awekho amamodeli afana nalesi sihlungi.' };
  const [unpinned, legacyPreflight] = legacy[locale];
  return { installedSize, repositorySize, unknown, unchecked, loaded, emptyCatalog, filterInstalled, curated, search, details, noMatches: noMatches[locale], unpinned, legacyPreflight };
}
