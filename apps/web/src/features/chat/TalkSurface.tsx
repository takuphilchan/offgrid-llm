import {useCallback,useEffect,useRef,useState,type ReactNode} from 'react';
import {createPortal} from 'react-dom';
import {Icon} from '../../components/Icon';
import {useI18n} from '../../i18n';
import {talkText,talkInterruptionText} from '../../i18n/talk';
import {operationFeedback} from '../../i18n/operation-feedback';
import {voiceText} from '../../i18n/voice';
import {voiceSettingsText} from '../../i18n/voice-settings';
import type {VoiceCaptureCapability} from '../../lib/voice-capture';
import type {TalkState} from './useTalkMode';
import './talk-surface.css';

type CapturePhase = 'idle'|'checking'|'permission'|'recording'|'transcribing'|'monitoring';
export function TalkSurface({state,error,end,retry,canRetry,interrupt,canInterrupt=false,renderCapture}:{state:TalkState;error:string;end:()=>void;retry:()=>void;canRetry:boolean;interrupt:()=>void;canInterrupt?:boolean;renderCapture:(onLevel:(level:number)=>void,onPhase:(phase:CapturePhase)=>void,onCapability:(capability:VoiceCaptureCapability)=>void)=>ReactNode}) {
 const {locale}=useI18n(), copy=talkText(locale), feedback=operationFeedback(locale), voice=voiceText(locale);
 const [capturePhase,setCapturePhase]=useState<CapturePhase>('checking');
 const [capability,setCapability]=useState<VoiceCaptureCapability>();
 const surface=useRef<HTMLElement>(null), orb=useRef<HTMLDivElement>(null);
 const level=useCallback((value:number)=>orb.current?.style.setProperty('--voice-energy',String(Math.max(0,Math.min(1,value)))),[]);
 useEffect(()=>{
  const target=surface.current!,previous=document.activeElement as HTMLElement|null;
  const shell=document.querySelector<HTMLElement>('.app-shell');
  const composer=shell?.querySelector<HTMLTextAreaElement>('.composer textarea');
  const wasInert=shell?.inert??false;
  // Text and voice are two presentations of the same work. Keep text mounted
  // to preserve drafts and scroll position, but remove it from sight and input.
  if(shell){shell.inert=true;shell.setAttribute('data-voice-hidden','');}
  target.focus({preventScroll:true});
  return()=>{
   if(shell){shell.inert=wasInert;shell.removeAttribute('data-voice-hidden');}
   // Exiting voice returns to typing, regardless of whether the removed Talk
   // trigger or body was active when this effect ran. Retain the originating
   // composer reference instead of querying for another one during cleanup.
   if(composer?.isConnected)composer.focus({preventScroll:true});else if(previous?.isConnected)previous.focus({preventScroll:true});
  };
 },[]);
 const status=state==='checking'?copy.checking:state==='thinking'?copy.thinking:state==='speaking'?copy.speaking:state==='needs_attention'?copy.attention:state==='transcribing'?copy.transcribing:capturePhase==='permission'?feedback.permissionPending:capturePhase==='checking'?copy.checking:capturePhase==='idle'?voice.record:copy.listening;
 const visualState=state==='listening'&&capturePhase!=='recording'&&capturePhase!=='monitoring'?'checking':state;
 return createPortal(<section ref={surface} className="talk-surface" tabIndex={-1} aria-label={copy.start} onKeyDown={event=>{if(event.key==='Escape'){event.preventDefault();event.stopPropagation();end();}}}>
  <div className="talk-surface-content">
  <div className="talk-visual" ref={orb} data-state={visualState} aria-hidden="true"><i className="voice-halo"/><i className="voice-core"/><i className="voice-light"/></div>
  <div className="talk-surface-copy"><p role="status">{status}</p>{state==='speaking'&&capability?.automatic===false&&<p className="voice-interruption-hint">{talkInterruptionText(locale)}</p>}{error&&<p className="talk-surface-error" role="alert">{error}</p>}{state==='needs_attention'&&<a href="#/settings/voice" onClick={end}>{voiceSettingsText(locale).title}</a>}</div>
  <div className="talk-surface-controls">
   {renderCapture(level,setCapturePhase,setCapability)}
   {(state==='speaking'||state==='thinking'&&canInterrupt)&&<button type="button" className="text-button" onClick={interrupt}>{copy.again}</button>}
   {state==='needs_attention'&&canRetry&&<button className="text-button" onClick={retry}>{copy.again}</button>}
   <button type="button" className="talk-end" aria-label={copy.stop} title={copy.stop} onClick={end}><Icon name="close" size={22}/></button>
  </div>
  </div>
 </section>,document.body);
}
