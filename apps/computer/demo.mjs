import { createServer } from 'node:http';

// Only this companion-created page is permitted. No user-supplied loopback URL,
// filesystem paths, proxying, uploads or outbound requests are implemented.
export const DEMO_ORIGIN = 'offgrid-demo://research';
export async function startDemo() {
  const server = createServer((request, response) => {
    if (request.method !== 'GET' || request.url !== '/') {
      response.writeHead(404).end(); return;
    }
    response.writeHead(200, {
      'Content-Type': 'text/html; charset=utf-8',
      'Cache-Control': 'no-store',
      'Content-Security-Policy': "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; form-action 'none'; frame-ancestors 'none'; base-uri 'none'"
    });
    response.end(`<!doctype html><html lang="en"><meta charset="utf-8"><title>OffGrid local browser demo</title>
      <style>body{font:18px system-ui;max-width:650px;margin:60px auto;padding:24px;color:#161616}input,button,select{font:inherit;padding:12px;margin:12px 0}label,input{display:block}input[type=checkbox]{display:inline}button{background:#161616;color:white;border:0;border-radius:8px}</style>
      <h1>Research notes</h1><p>Isolated demo. Changes stay in this page and disappear when it closes. No external submission or file is created.</p>
      <label for="report">Report title</label><input id="report" maxlength="200">
      <label for="format">Report format</label><select id="format"><option>Narrative</option><option>Table</option></select>
      <label><input id="sources" type="checkbox">Include sources</label><button id="save">Save draft</button>
      <p id="status" role="status">No draft saved</p>
      <p id="preferences" role="status"></p>
      <script>document.getElementById('save').addEventListener('click',()=>{document.getElementById('status').textContent='Draft saved: '+document.getElementById('report').value;document.getElementById('preferences').textContent='Format: '+document.getElementById('format').value+'; Sources: '+(document.getElementById('sources').checked?'included':'not included');});</script></html>`);
  });
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
  return {
    origin: `http://127.0.0.1:${server.address().port}`,
    close: () => new Promise((resolve, reject) => { server.close(error => error ? reject(error) : resolve()); server.closeAllConnections(); })
  };
}
