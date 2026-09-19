import type { LocaleCode } from './index';

export const presentation = {
  en: { unknown: 'Status unavailable', details: 'Technical details', filterModels: 'Filter suggested models' },
  de: { unknown: 'Status nicht verfügbar', details: 'Technische Details', filterModels: 'Modellvorschläge filtern' },
  fr: { unknown: 'État indisponible', details: 'Détails techniques', filterModels: 'Filtrer les modèles suggérés' },
  es: { unknown: 'Estado no disponible', details: 'Detalles técnicos', filterModels: 'Filtrar modelos sugeridos' },
  ar: { unknown: 'الحالة غير متاحة', details: 'التفاصيل التقنية', filterModels: 'تصفية النماذج المقترحة' },
  sw: { unknown: 'Hali haipatikani', details: 'Maelezo ya kiufundi', filterModels: 'Chuja modeli zilizopendekezwa' },
  sn: { unknown: 'Mamiriro haawanikwi', details: 'Ruzivo rwehunyanzvi', filterModels: 'Sefa mamodhi akakurudzirwa' },
  nd: { unknown: 'Isimo asitholakali', details: 'Imininingwane yobuchwepheshe', filterModels: 'Hlunga amamodeli anconyiweyo' },
  zu: { unknown: 'Isimo asitholakali', details: 'Imininingwane yobuchwepheshe', filterModels: 'Hlunga amamodeli anconyiwe' }
} satisfies Record<LocaleCode, { unknown: string; details: string; filterModels: string }>;
