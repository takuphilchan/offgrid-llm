// Real UI ingestion and scoped retrieval in the caller's disposable workspace.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {join} from 'node:path';

export async function runKnowledgeJourney({page,api,evidence,expect}) {
  const sourceName='Mariner launch notes.txt';
  const source='Project Mariner launch code is ORCHID-742. Its launch date is 19 November 2031. The coordinator is Nia. These are disposable qualification notes.';
  const otherName='Atlas launch notes.txt';
  const other='Project Atlas launch code is COBALT-119. Its launch date is 4 March 2032. The coordinator is Leo. These are unrelated disposable qualification notes.';
  await page.getByRole('link',{name:'Knowledge',exact:true}).click();
  const initiallyEnabled=(await api('/v1/rag/status')).enabled===true;
  if(!initiallyEnabled)await page.getByRole('button',{name:'Enable knowledge',exact:true}).click();
  await expect(page.getByRole('button',{name:'Add document',exact:true})).toBeEnabled({timeout:90000});
  for(const [name,content] of [[sourceName,source],[otherName,other]]) {
    await page.locator('.knowledge-workspace input[type=file]').setInputFiles({name,mimeType:'text/plain',buffer:Buffer.from(content)});
    const row=page.locator('.knowledge-documents article').filter({has:page.getByRole('heading',{name,exact:true})});
    await expect(row.locator('.document-readiness')).toHaveText('Indexed',{timeout:90000});
  }
  const documents=(await api('/v1/documents')).documents;
  assert.equal(documents.length,2);
  const selected=documents.find(doc=>doc.name===sourceName),unselected=documents.find(doc=>doc.name===otherName);
  assert.ok(selected&&unselected);assert.notEqual(selected.id,unselected.id);
  await page.getByRole('checkbox',{name:'Select source: '+sourceName,exact:true}).check();
  await page.getByRole('button',{name:/Ask about selected sources/}).click();
  await expect(page.locator('.composer-source-scope')).toContainText(sourceName);
  await expect(page.locator('.composer-source-scope')).not.toContainText(otherName);
  const prompt="What is Project Mariner's launch code and launch date? Use only the selected source and mention its name.";
  await page.locator('.composer textarea').fill(prompt);
  const started=Date.now();await page.getByRole('button',{name:'Send',exact:true}).click();
  await expect.poll(async()=>{
    const sessions=(await api('/v1/sessions')).sessions??[];
    for(const summary of sessions){const session=await api('/v1/sessions/'+encodeURIComponent(summary.name));
      if(session.messages.some(message=>message.role==='user'&&message.content===prompt)&&session.messages.at(-1)?.role==='assistant')return summary.name;
    }
    return '';
  },{timeout:120000}).not.toBe('');
  let saved;
  for(const summary of (await api('/v1/sessions')).sessions??[]){const session=await api('/v1/sessions/'+encodeURIComponent(summary.name));if(session.messages.some(message=>message.role==='user'&&message.content===prompt))saved={name:summary.name,session};}
  assert.ok(saved);const answer=saved.session.messages.at(-1);
  assert.match(answer.content,/ORCHID-742/);assert.match(answer.content,/19 November 2031/);assert.doesNotMatch(answer.content,/COBALT-119/);
  assert.ok(answer.knowledge_sources?.length>0,'A plausible answer without saved retrieval evidence does not pass');
  for(const passage of answer.knowledge_sources){
    assert.equal(passage.document_id,selected.id);assert.equal(passage.name,sourceName);
    assert.ok(source.includes(passage.content));
    assert.equal(passage.content_hash,createHash('sha256').update(passage.content).digest('hex').slice(0,32));
  }
  await expect(page.locator('.message.assistant:not(.streaming-response) .message-body').last()).toContainText('ORCHID-742');
  const passages=page.locator('.knowledge-passages').last();await passages.locator('summary').first().click();
  await expect(passages).toContainText(sourceName);await expect(passages).not.toContainText(otherName);
  await page.screenshot({path:join(evidence,'knowledge-reply.png'),fullPage:true});
  return {...saved,answer:answer.content.trim(),elapsedMs:Date.now()-started,initiallyEnabled,selectedDocument:selected.id,excludedDocument:unselected.id,passages:answer.knowledge_sources};
}
