import { defineConfig } from 'vite';

import react from '@vitejs/plugin-react';
import computerCopy from './src/i18n/computer-experience.json' with { type: 'json' };
import { readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { createRequire } from 'node:module';
import compatibility from '../desktop/compatibility.cjs';
import { desktopCommandMessages } from './src/i18n/desktop-command';

export default defineConfig({
  base: '/ui/',
  // The package's browser export requires document at module initialization.
  // Its data-only decoder has identical entity semantics and also runs inside
  // the Markdown worker, where no DOM exists.
  resolve: { alias: { 'decode-named-character-reference': createRequire(import.meta.url).resolve('decode-named-character-reference') } },
  plugins: [react(), {name:'shared-startup-surface',
    configureServer(server) {
      server.middlewares.use((req,res,next)=>{
        const name=(req.url||'').split('?')[0].replace(/^\/ui\//,'/');
        if(!['/startup-surface.css','/startup-welcome.js','/workspace-startup.js'].includes(name))return next();
        res.setHeader('Content-Type',name.endsWith('.css')?'text/css':'application/javascript');
        res.end(readFileSync(resolve(import.meta.dirname,'../desktop'+name)));
      });
    },
    generateBundle() {
      for(const name of ['startup-surface.css','startup-welcome.js','workspace-startup.js'])this.emitFile({type:'asset',fileName:name,source:readFileSync(resolve(import.meta.dirname,'../desktop',name),'utf8')});
    }
  }, {name:'shared-computer-copy', generateBundle() {
    this.emitFile({type:'asset',fileName:'desktop-command.json',source:JSON.stringify(desktopCommandMessages)});
    this.emitFile({type:'asset',fileName:'theme-NOTICE.txt',source:readFileSync(resolve(import.meta.dirname,'../desktop/theme-NOTICE.txt'),'utf8')});
    this.emitFile({type:'asset',fileName:'computer-experience.json',source:JSON.stringify(computerCopy)});
  }}, {name:'desktop-compatibility', writeBundle(options) {
    const root = resolve(options.dir!);
    writeFileSync(resolve(root, 'desktop-compatibility.json'), JSON.stringify(compatibility.createManifest(readFileSync(resolve(root, 'index.html')))) + '\n');
    compatibility.verifyUI(root);
  }}],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    sourcemap: true,
    target: 'es2022'
  },
  server: {
    port: 5173,
    // Playwright writes and removes locked traces on Windows. These generated
    // artifacts are never application modules and must not restart the server.
    watch: { ignored: ['**/test-results*/**', '**/playwright-report/**'] },
    proxy: process.env.OFFGRID_FIXTURE_TEST === '1' ? undefined : {
      '/api': 'http://127.0.0.1:11611',
      '/v1': 'http://127.0.0.1:11611',
      '/health': 'http://127.0.0.1:11611',
      '/mcp': 'http://127.0.0.1:11611'
    }
  }
});
