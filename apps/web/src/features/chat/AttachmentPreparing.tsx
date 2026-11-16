import {Icon} from '../../components/Icon';

/** An activity state, not an estimate of bytes transferred or extraction coverage. */
export function AttachmentPreparing({name,label,image=false}:{name:string;label:string;image?:boolean}) {
 return <div className="attachment-preparing" role="status" aria-live="polite">
  <span className="attachment-preparing-mark"><Icon name={image?'image':'file'} size={22}/></span>
  <span className="attachment-preparing-copy"><bdi title={name}>{name}</bdi><small><span className="attachment-activity" aria-hidden="true"/>{label}</small></span>
 </div>;
}
