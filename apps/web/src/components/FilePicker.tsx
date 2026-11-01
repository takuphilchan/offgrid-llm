import { useId, useRef, useState } from 'react';
import { Icon } from './Icon';
import './file-picker.css';

export function FilePicker({ label, accept, multiple = false, directory = false, disabled = false, onFiles }: {
  label: string; accept?: string; multiple?: boolean; directory?: boolean; disabled?: boolean;
  onFiles: (files: File[]) => void | Promise<void>;
}) {
  const input = useRef<HTMLInputElement>(null), id = useId();
  const [selection, setSelection] = useState('');
  return <div className="file-picker">
    <input ref={input} id={id} aria-label={label} hidden type="file" accept={accept} multiple={multiple || directory} disabled={disabled}
      {...(directory ? { webkitdirectory: '' } : {})}
      onChange={event => {
        const files = Array.from(event.currentTarget.files ?? []);
        event.currentTarget.value = '';
        if (!files.length) return;
        setSelection(directory ? files[0].webkitRelativePath.split('/')[0] || files[0].name : files.map(file => file.name).join(', '));
        void onFiles(files);
      }} />
    <button type="button" className="secondary-button file-picker-trigger" disabled={disabled} aria-controls={id} onClick={() => input.current?.click()}>
      <Icon name="upload" size={16}/><span>{label}</span>
    </button>
    {selection && <span className="file-picker-selection" role="status" title={selection}>{selection}</span>}
  </div>;
}
