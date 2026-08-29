import type { Model } from '../api/client';
import { useI18n } from '../i18n';

export function ModelSelect({ models, value, onChange }: { models: Model[]; value: string; onChange: (value: string) => void }) {
  const { messages: text } = useI18n();
  return <label className="model-select"><span>{text.chat.model}</span><select value={value} onChange={event => onChange(event.target.value)}><option value="">{text.chat.selectModel}</option>{models.filter(item => item.type !== 'embedding').map(item => <option value={item.id} key={item.id}>{item.id}</option>)}</select></label>;
}
