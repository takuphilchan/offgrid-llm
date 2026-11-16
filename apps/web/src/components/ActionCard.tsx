import {useId} from 'react';
import {Icon, type IconName} from './Icon';
import './action-card.css';

type Props = {title:string; description:string; icon:IconName} & ({href:string; onClick?:()=>void} | {href?:never; onClick:()=>void});

/** A destination or launch choice, not a container for nested controls. */
export function ActionCard({title,description,icon,href,onClick}:Props) {
  const id=useId();
  const content=<><span className="action-card-icon"><Icon name={icon} size={22}/></span><span className="action-card-copy"><strong id={id}>{title}</strong><span id={`${id}-description`}>{description}</span></span><span className="action-card-arrow"><Icon name="chevron-right" size={16}/></span></>;
  const shared={className:'action-card','aria-labelledby':id,'aria-describedby':`${id}-description`};
  return href ? <a {...shared} href={href} onClick={onClick}>{content}</a> : <button {...shared} type="button" onClick={onClick}>{content}</button>;
}
