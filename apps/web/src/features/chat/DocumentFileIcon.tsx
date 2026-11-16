import { Icon } from '../../components/Icon';

/** Format marks identify the retained file, never its extraction capabilities. */
export function DocumentFileIcon({ name }: { name: string }) {
 const extension = name.split('.').pop()?.toUpperCase() ?? '';
 const format = ['PDF','DOCX','XLSX','PPTX','TXT','MD','CSV','PNG','JPG','JPEG','TIFF','TIF'].includes(extension) ? extension : '';
 return <span className="document-format-icon" data-format={format.toLowerCase()} aria-hidden="true"><Icon name="file" size={20}/>{format && <small>{format}</small>}</span>;
}
