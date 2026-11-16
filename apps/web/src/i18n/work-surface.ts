import {defineMessages} from './catalog';
import type {LocaleCode} from './index';
// Draft copy; independent linguistic review remains pending.
const labels=defineMessages<string[]>('i18n/work-surface:labels',{
 "en": [
  "Home",
  "Library",
  "Ask about this page",
  "What would you like to work on?",
  "Ask about this selection",
  "This selection is unavailable or too large. Select less text or check source access. Your draft is retained."
 ],
 "fr": [
  "Accueil",
  "Bibliothèque",
  "Poser une question sur cette page",
  "Sur quoi souhaitez-vous travailler ?",
  "Poser une question sur cette sélection",
  "Cette sélection est indisponible ou trop volumineuse. Sélectionnez moins de texte ou vérifiez l’accès à la source. Votre brouillon est conservé."
 ],
 "es": [
  "Inicio",
  "Biblioteca",
  "Preguntar sobre esta página",
  "¿En qué quieres trabajar?",
  "Preguntar sobre esta selección",
  "La selección no está disponible o es demasiado grande. Selecciona menos texto o comprueba el acceso a la fuente. Tu borrador se conserva."
 ],
 "de": [
  "Start",
  "Bibliothek",
  "Zu dieser Seite fragen",
  "Woran möchtest du arbeiten?",
  "Zu dieser Auswahl fragen",
  "Diese Auswahl ist nicht verfügbar oder zu groß. Wähle weniger Text oder prüfe den Zugriff auf die Quelle. Dein Entwurf bleibt erhalten."
 ],
 "ar": [
  "الرئيسية",
  "المكتبة",
  "اسأل عن هذه الصفحة",
  "على ماذا تريد العمل؟",
  "اسأل عن هذا التحديد",
  "هذا التحديد غير متاح أو كبير جدًا. حدد نصًا أقل أو تحقق من صلاحية الوصول إلى المصدر. تم الاحتفاظ بمسودتك."
 ],
 "sw": [
  "Mwanzo",
  "Maktaba",
  "Uliza kuhusu ukurasa huu",
  "Ungependa kufanyia kazi nini?",
  "Uliza kuhusu sehemu iliyochaguliwa",
  "Sehemu hii haipatikani au ni kubwa mno. Chagua maandishi machache au angalia ruhusa ya kufikia chanzo. Rasimu yako imehifadhiwa."
 ],
 "sn": [
  "Kutanga",
  "Raibhurari",
  "Bvunza nezvepeji rino",
  "Unoda kushanda pachii?",
  "Bvunza nezvechikamu chakasarudzwa",
  "Chikamu ichi hachiwanikwi kana kuti chakakura zvikuru. Sarudza mashoko mashoma kana kuongorora mvumo yekuwana tsime. Gwaro rako risati ratumirwa rachengetwa."
 ],
 "nd": [
  "Ekuqaleni",
  "Umtapo wolwazi",
  "Buza ngaleli khasi",
  "Ufuna ukusebenza ngani?",
  "Buza ngengxenye ekhethiweyo",
  "Ingxenye le ayitholakali kumbe inkulu kakhulu. Khetha umbhalo omncane kumbe uhlole imvumo yokufinyelela umthombo. Umbhalo wakho ongakathunyelwa ugciniwe."
 ],
 "zu": [
  "Ekhaya",
  "Umtapo wolwazi",
  "Buza ngaleli khasi",
  "Ufuna ukusebenza ngani?",
  "Buza ngengxenye ekhethiwe",
  "Le ngxenye ayitholakali noma inkulu kakhulu. Khetha umbhalo omncane noma uhlole imvume yokufinyelela emthonjeni. Okusalungiswa kwakho kugciniwe."
 ]
});
export function workSurface(locale:LocaleCode) {const [home,library,askPage,welcome,askSelection,scopeUnavailable]=labels[locale];return {home,library,askPage,welcome,askSelection,scopeUnavailable};}
