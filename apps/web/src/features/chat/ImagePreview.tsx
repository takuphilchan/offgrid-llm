import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import type { SessionImage } from '../../api/client';
import { Icon } from '../../components/Icon';
import { useI18n } from '../../i18n';
import { imagePreview } from '../../i18n/chat-images';
import { restoreFocus } from '../../lib/restore-focus';

/** Inspect the exact attachment; never re-encode it or submit a model request. */
export function ImagePreview({ image, close }: { image: SessionImage; close: () => void }) {
  const { locale } = useI18n(), copy = imagePreview[locale];
  const dialog = useRef<HTMLDialogElement>(null);
  const [zoomed, setZoomed] = useState(false);
  useEffect(() => {
    const node = dialog.current!, previous = document.activeElement as HTMLElement | null;
    node.showModal();
    return () => { node.close(); restoreFocus(previous); };
  }, []);
  return createPortal(<dialog ref={dialog} className="image-preview" aria-label={`${copy.open}: ${image.name}`}
    onCancel={event => { event.preventDefault(); close(); }}>
    <header><h2 dir="auto" title={image.name}>{image.name}</h2><div>
      <button type="button" className="utility-button" aria-pressed={zoomed} onClick={() => setZoomed(value => !value)}>{copy.zoom}</button>
      <button type="button" autoFocus className="icon-button" aria-label={copy.close} onClick={close}><Icon name="close" size={20}/></button>
    </div></header>
    <div className="image-preview-surface" data-zoomed={zoomed} tabIndex={0}>
      <img src={/^data:image\/(png|jpeg);base64,[A-Za-z0-9+/=]+$/.test(image.data_url)?image.data_url:undefined} alt={image.name}/>
    </div>
  </dialog>, document.body);
}
