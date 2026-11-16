import {clientMessage} from '../i18n/client-errors';
import type { Document, DocumentStructurePage, DocumentPreview } from './client';

export function validatePreview(value:DocumentPreview):DocumentPreview {
 if(!value||typeof value.sha256!=='string'||!value.sha256||typeof value.format!=='string'||!Array.isArray(value.units)||!value.units.length||value.units.length>1000)throw Error(clientMessage('response'));
 const ids=new Set<string>();
 for(const unit of value.units){if(!unit)throw Error(clientMessage('response'));const id=`${unit.kind}:${unit.index}`;if(!['page','slide','sheet','section'].includes(unit.kind)||!Number.isSafeInteger(unit.index)||unit.index<1||typeof unit.text!=='string'||typeof unit.state!=='string'||ids.has(id))throw Error(clientMessage('response'));ids.add(id);}
 for(const unit of value.units){const g=unit.image_geometry;if(g&&(![g.width,g.height,g.orientation].every(Number.isSafeInteger)||g.width<1||g.height<1||g.width>8192||g.height>8192||g.width*g.height>16000000||g.orientation<1||g.orientation>8))throw Error(clientMessage('response'));}
 return value;
}

// An older service can return an unrelated JSON projection. Never let that
// become an editable block or a confirmed correction receipt.
export function validateStructure(value: DocumentStructurePage, id: string, revision: string, offset: number, readOnly = false, navigationOffset = 0): DocumentStructurePage {
 if (!value || value.document_id !== id || !value.revision || (revision && value.revision !== revision) || value.offset !== offset
  || !Array.isArray(value.blocks) || value.blocks.length > 100 || !Array.isArray(value.units) || value.units.length > 1000
  || !Number.isSafeInteger(value.total_blocks) || value.total_blocks < offset + value.blocks.length || value.total_blocks > 200000
  || (value.next_offset !== null && (!Number.isSafeInteger(value.next_offset) || value.next_offset !== offset + value.blocks.length || value.next_offset <= offset || value.next_offset >= value.total_blocks))
  || typeof value.can_correct !== 'boolean' || (readOnly && value.can_correct)) throw Error('Invalid document structure receipt');
 const ids = new Set<string>();
 if(value.navigation!==undefined){
  const units=new Map(value.units.map(unit=>[unit.id,unit]));
  if(!Array.isArray(value.navigation)||value.navigation.length>100||value.navigation_offset!==navigationOffset||!Number.isSafeInteger(value.total_navigation)||value.total_navigation!<navigationOffset+value.navigation.length||value.total_navigation!>200000
   ||(value.next_navigation_offset!==null&&(!Number.isSafeInteger(value.next_navigation_offset)||value.next_navigation_offset!==navigationOffset+value.navigation.length||value.next_navigation_offset<=navigationOffset||value.next_navigation_offset>=value.total_navigation!)))throw Error(clientMessage('response'));
  const entries=new Set<string>();
  for(const entry of value.navigation){const unit=units.get(entry.unit_id);
   if(!entry.block_id||entries.has(entry.block_id)||!unit||!Number.isSafeInteger(entry.block_offset)||entry.block_offset<0||(unit.block_count!==undefined&&entry.block_offset>=unit.block_count)||typeof entry.title!=='string'||new TextEncoder().encode(entry.title).length>256||typeof entry.needs_review!=='boolean')throw Error(clientMessage('response'));
   entries.add(entry.block_id);
  }
 }else if(navigationOffset)throw Error(clientMessage('response'));
 for (const block of value.blocks) {
  if (!block || typeof block.id !== 'string' || !block.id || ids.has(block.id) || typeof block.text !== 'string' || !/^[a-f0-9]{64}$/.test(block.text_sha256)
   || typeof block.editable !== 'boolean' || typeof block.truncated !== 'boolean' || typeof block.metadata_omitted !== 'boolean'
   || (block.editable && (!value.can_correct || block.truncated || block.metadata_omitted))) throw Error('Invalid document block receipt');
  ids.add(block.id);
 }
 return value;
}

export function validateCorrection(value: {success: boolean; document: Document}, id: string, revision: string) {
 if (!value?.success || value.document?.id !== id || !value.document.extraction_revision || value.document.extraction_revision === revision) throw Error('Unconfirmed document correction');
 return value;
}
