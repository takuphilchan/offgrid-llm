import {useMemo} from 'react';
import type {SynthesisReview} from '../../api/document-synthesis';
import {useDraftScope} from '../../lib/draft-scope';
import {draftKey,readDraft,writeDraft} from '../../lib/drafts';

class ReviewDraftStore extends Map<string,SynthesisReview> {
 constructor(private key:string) {
  super();
  try {for(const [id,value] of JSON.parse(readDraft(key))){
   if(typeof id==='string'&&value&&Array.isArray(value.decisions)&&value.decisions.every((d: {decision?:unknown;note?:unknown})=>d&&['keep','exclude','unreviewed'].includes(String(d.decision))&&typeof d.note==='string'))super.set(id,value);
  }}catch{/* Invalid local recovery data cannot replace service state. */}
 }
 private persist(){writeDraft(this.key,JSON.stringify([...this]));}
 override set(id:string,value:SynthesisReview){super.set(id,value);this.persist();return this;}
 override delete(id:string){const removed=super.delete(id);this.persist();return removed;}
 override clear(){super.clear();this.persist();}
}
export function useReviewDrafts(){const scope=useDraftScope();return useMemo(()=>new ReviewDraftStore(draftKey(scope,'document-review')), [scope]);}
