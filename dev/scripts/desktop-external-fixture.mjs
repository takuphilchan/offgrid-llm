// Owned fixture for the isolated Windows installer test. No real workspace,
// inference, host control, authentication credentials, or externally bound port.
import { createServer } from 'node:http';
import { readFile, writeFile } from 'node:fs/promises';
import { resolve, join } from 'node:path';
import { createHash } from 'node:crypto';
const directory = resolve(process.argv[2]);
const root = resolve(import.meta.dirname, '../..');
const legacy = JSON.parse(await readFile(join(root, 'desktop/legacy-compatibility.json'), 'utf8')).entries[0];
const sentinel = join(directory, 'external-workspace.txt');
await writeFile(sentinel, 'External fixture workspace; installer must preserve this.');
let uiRequests = 0;
const server = createServer(async (request, response) => {
  if (request.method !== 'GET') { response.writeHead(405); response.end(); return; }
  response.setHeader('Cache-Control', 'no-store');
  if (request.url === '/api/v2/system') {
    response.setHeader('Content-Type','application/json');
    response.end(JSON.stringify({product:'offgrid',version:'0.4.14',api_version:2,workspace_id:'installer-external-fixture',ui_build_id:legacy.ui_build_id,capabilities:['sessions-v1','chat-streaming-v1','durable-agent-runs-v1']}));
  } else if (request.url === '/fixture-status') {
    response.setHeader('Content-Type','application/json');
    response.end(JSON.stringify({uiRequests,digest:createHash('sha256').update(await readFile(sentinel)).digest('hex')}));
  } else if (request.url.startsWith('/ui/')) {
    uiRequests++;
    response.setHeader('Content-Type','text/html');
    response.end('<!doctype html><title>Installer fixture</title><h1>Existing fixture workspace</h1>');
  } else {response.writeHead(404);response.end();}
});
server.listen(0,'127.0.0.1',async()=>{
  await writeFile(join(directory,'ready.json'),JSON.stringify({port:server.address().port}));
});
