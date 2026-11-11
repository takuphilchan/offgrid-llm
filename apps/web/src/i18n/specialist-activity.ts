import { defineMessages } from './catalog';
import type { LocaleCode } from './index';
const labels = defineMessages<readonly [string, string]>("i18n/specialist-activity:labels", {
  en: ['Depends on', 'Open this subtask to inspect its current state.'],
  fr: ['Dépend de', 'Ouvrez cette sous-tâche pour consulter son état actuel.'],
  es: ['Depende de', 'Abre esta subtarea para consultar su estado actual.'],
  de: ['Abhängig von', 'Öffne diese Teilaufgabe, um ihren aktuellen Zustand zu prüfen.'],
  ar: ['يعتمد على', 'افتح هذه المهمة الفرعية لفحص حالتها الحالية.'],
  sw: ['Inategemea', 'Fungua kazi hii ndogo ili ukague hali yake ya sasa.'],
  sn: ['Zvinoenderana ne', 'Vhura basa diki iri kuti uongorore mamiriro aro azvino.'],
  nd: ['Kuncike ku', 'Vula umsebenzi omncane lo ukuze uhlole isimo sawo khathesi.'],
  zu: ['Kuncike ku', 'Vula lo msebenzi omncane ukuze uhlole isimo sawo samanje.'],
});
export function specialistActivityText(locale: LocaleCode) {
  const [depends, inspect] = labels[locale]; return { depends, inspect };
}
