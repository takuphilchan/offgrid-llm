import type { LocaleCode } from './index';

const copy: Record<LocaleCode, [string, string]> = {
 en: ['This screen could not load. Reload the app to try again.', 'Reload app'],
 fr: ['Cet écran n’a pas pu se charger. Rechargez l’application pour réessayer.', 'Recharger l’application'],
 de: ['Diese Seite konnte nicht geladen werden. Lade die App erneut.', 'App neu laden'],
 es: ['No se pudo cargar esta pantalla. Recarga la aplicación para volver a intentarlo.', 'Recargar aplicación'],
 ar: ['تعذر تحميل هذه الشاشة. أعد تحميل التطبيق للمحاولة مرة أخرى.', 'إعادة تحميل التطبيق'],
 sw: ['Skrini hii haikuweza kupakia. Pakia programu upya ili ujaribu tena.', 'Pakia programu upya'],
 sn: ['Peji iyi yatadza kuvhurwa. Vhurazve app kuti uedze zvakare.', 'Vhurazve app'],
 nd: ['Lelikhasi lehlulekile ukuvuleka. Vula i-app kutsha ukuze uzame futhi.', 'Vula i-app kutsha'],
 zu: ['Lesi sikrini asikwazanga ukulayisha. Layisha uhlelo kabusha ukuze uzame futhi.', 'Layisha uhlelo kabusha'],
};
export const pageLoading = (locale: LocaleCode) => ({failed: copy[locale][0], reload: copy[locale][1]});
