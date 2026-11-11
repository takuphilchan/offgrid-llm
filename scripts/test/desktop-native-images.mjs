import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {join} from 'node:path';

// A real packaged renderer/service with a text-only model must retain input,
// explain missing vision and refuse inference. This is not a vision model test.
export async function stageImageDraft({page,origin,name,api,expect,evidence}){
 const dataURL=await page.evaluate(()=>{const c=document.createElement('canvas');c.width=4;c.height=4;const ctx=c.getContext('2d');ctx.fillStyle='red';ctx.fillRect(0,0,4,4);return c.toDataURL('image/png');});
 const image={name:'qualification-red.png',data_url:dataURL};
 await page.locator('.composer input[type=file][accept^="image/"]').setInputFiles({name:image.name,mimeType:'image/png',buffer:Buffer.from(dataURL.split(',')[1],'base64')});
 await expect(page.locator('.composer .image-attachments img')).toHaveAttribute('src',dataURL);
 await expect(page.locator('.composer')).toContainText('Choose an installed vision model');
 await expect(page.getByRole('button',{name:'Send',exact:true})).toBeDisabled();
 const before=await api('/v1/sessions/'+encodeURIComponent(name));
 const response=await fetch(origin+'/v1/sessions/'+encodeURIComponent(name)+'/generate',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({content:'Describe the image',images:[image],stream:true,durable:true,request_id:'image-unavailable-qualification'}),signal:AbortSignal.timeout(10000)});
 assert.equal(response.status,422);assert.match(await response.text(),/vision/i);
 assert.deepEqual(await api('/v1/sessions/'+encodeURIComponent(name)),before,'Rejected images must not create or replace a turn');
 await page.screenshot({path:join(evidence,'image-draft.png')});
 return image;
}

export async function verifyImageDraftRestart({page,image,expect}){
 await expect(page.locator('.composer .image-attachments img')).toHaveAttribute('src',image.data_url);
 await expect(page.getByRole('button',{name:'Send',exact:true})).toBeDisabled();
 await page.getByRole('button',{name:'Remove image: '+image.name,exact:true}).click();
 await expect(page.locator('.composer .image-attachments')).toHaveCount(0);
 await expect(page.getByRole('button',{name:'Add files or images',exact:true})).toBeFocused();
 return {draftRestart:true,unsupportedModelBlocked:true,noTurnCreated:true,visionInferenceQualified:false,sha256:createHash('sha256').update(Buffer.from(image.data_url.split(',')[1],'base64')).digest('hex')};
}
