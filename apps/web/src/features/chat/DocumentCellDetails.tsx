import type { DocumentBlock } from '../../api/client';
import { useI18n } from '../../i18n';
import { documentComparison } from '../../i18n/document-comparison';
import { documentNavigation } from '../../i18n/document-navigation';
import { documentCell } from '../../i18n/document-cell';
import { presentation } from '../../i18n/presentation';

/** Source values are inert evidence. Inspection never evaluates a formula. */
export function DocumentCellDetails({ cell }: { cell: DocumentBlock['cell'] }) {
 const { locale } = useI18n(), copy = documentComparison(locale), metadata = documentCell(locale);
 const number = (value: number) => new Intl.NumberFormat(locale).format(value);
 if (!cell) return null;
 // A shared formula can retain only its anchor/type, with no local formula text.
 const hasFormula = !!(cell.formula || cell.formula_type || cell.formula_anchor);
 const rows = [
  { label: copy.value, value: cell.value },
  { label: metadata.display, value: cell.display },
  { label: metadata.displayState, value: cell.display_state },
  { label: metadata.type, value: cell.value_type },
  { label: metadata.format, value: cell.number_format },
  { label: metadata.formatID, value: cell.number_format_id?.toString() },
  { label: metadata.dateSystem, value: cell.date_system },
  { label: copy.formula, value: cell.formula },
  { label: metadata.formulaType, value: cell.formula_type },
  { label: metadata.formulaRange, value: cell.formula_reference },
  { label: metadata.formulaAnchor, value: cell.formula_anchor },
  { label: metadata.sharedFormulaID, value: cell.shared_formula_index?.toString() },
  { label: copy.cached, value: cell.cached_value ?? (hasFormula ? presentation[locale].unknown : undefined) },
  { label: metadata.resultState, value: cell.result_state },
  { label: metadata.mergedRange, value: cell.merged_range },
  { label: metadata.rows, value: cell.row_span === undefined ? undefined : number(cell.row_span) },
  { label: metadata.columns, value: cell.column_span === undefined ? undefined : number(cell.column_span) },
 ].filter(row => row.value !== undefined);
 const hidden = cell.hidden_reasons ?? [];
 const flags = [
  cell.horizontal_merge && metadata.horizontal,
  cell.vertical_merge && metadata.vertical,
  hidden.includes('sheet') && metadata.hiddenSheet,
  hidden.includes('row') && metadata.hiddenRow,
  hidden.includes('column') && metadata.hiddenColumn,
  cell.hidden && !hidden.length && documentNavigation(locale).hidden,
 ].filter(Boolean);
 if (!rows.length && !flags.length) return null;
 return <details className="document-cell-details">
  <summary>{copy.metadata}</summary>
  <p className="workspace-secondary">{metadata.note}</p>
  {!!flags.length && <p>{flags.join(' · ')}</p>}
  {!!rows.length && <dl>{rows.map(row => <div key={row.label}><dt>{row.label}</dt><dd><code dir="auto">{row.value === '' ? '""' : row.value}</code></dd></div>)}</dl>}
 </details>;
}
