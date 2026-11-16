import {useI18n} from '../i18n';
import {documentEvidenceNotice} from '../i18n/document-evidence';
import {documentFormatLimits} from '../i18n/document-format-limits';
import type {components} from '../api/schema.generated';

export function DocumentEvidenceNotice({evidence}: {evidence?:components['schemas']['DocumentEvidence']}) {
 const {locale}=useI18n();
 if(!evidence || evidence.status==='complete')return null;
 const detail=documentFormatLimits(locale,evidence.format);
 return <div className="document-coverage"><p role="status" data-extraction-coverage={evidence.status}>{documentEvidenceNotice(locale)}</p>{detail&&<p>{detail}</p>}</div>;
}
