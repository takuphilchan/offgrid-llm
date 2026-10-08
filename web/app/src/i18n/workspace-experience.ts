import type { LocaleCode } from './index';

type ExperienceCopy = { context: string; knowledgeOn: string; responseDetails: string; manageHistory: string; returnDraft: string };
export const workspaceExperience = {
  en: { context: 'Context & response', knowledgeOn: 'Knowledge on', responseDetails: 'Response details', manageHistory: 'Manage history', returnDraft: 'Return to your draft' },
  fr: { context: 'Contexte et réponse', knowledgeOn: 'Connaissances activées', responseDetails: 'Détails de la réponse', manageHistory: 'Gérer l’historique', returnDraft: 'Revenir au brouillon' },
  es: { context: 'Contexto y respuesta', knowledgeOn: 'Conocimiento activado', responseDetails: 'Detalles de la respuesta', manageHistory: 'Gestionar historial', returnDraft: 'Volver al borrador' },
  de: { context: 'Kontext und Antwort', knowledgeOn: 'Wissen aktiv', responseDetails: 'Antwortdetails', manageHistory: 'Verlauf verwalten', returnDraft: 'Zurück zum Entwurf' },
  ar: { context: 'السياق والرد', knowledgeOn: 'المعرفة مفعّلة', responseDetails: 'تفاصيل الرد', manageHistory: 'إدارة السجل', returnDraft: 'العودة إلى المسودة' },
  sw: { context: 'Muktadha na jibu', knowledgeOn: 'Maarifa yamewashwa', responseDetails: 'Maelezo ya jibu', manageHistory: 'Dhibiti historia', returnDraft: 'Rudi kwenye rasimu yako' },
  sn: { context: 'Zvinoenderana nemhinduro', knowledgeOn: 'Ruzivo rwabatidzwa', responseDetails: 'Tsananguro yemhinduro', manageHistory: 'Ronga nhoroondo', returnDraft: 'Dzokera kune zvawanga uchinyora' },
  nd: { context: 'Umongo lempendulo', knowledgeOn: 'Ulwazi luvuliwe', responseDetails: 'Imininingwane yempendulo', manageHistory: 'Phatha imbali', returnDraft: 'Buyela kokubhalileyo' },
  zu: { context: 'Umongo nempendulo', knowledgeOn: 'Ulwazi luvuliwe', responseDetails: 'Imininingwane yempendulo', manageHistory: 'Phatha umlando', returnDraft: 'Buyela kokubhalile' },
} satisfies Record<LocaleCode, ExperienceCopy>;
