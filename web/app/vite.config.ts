import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import computerCopy from './src/i18n/computer-experience.json' with { type: 'json' };

export default defineConfig({
  base: '/ui/',
  plugins: [react(), {name:'shared-computer-copy', generateBundle() {
    this.emitFile({type:'asset',fileName:'computer-experience.json',source:JSON.stringify(computerCopy)});
  }}],
  build: {
    outDir: '../dist',
    emptyOutDir: true,
    sourcemap: true,
    target: 'es2022'
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://127.0.0.1:11611',
      '/v1': 'http://127.0.0.1:11611',
      '/health': 'http://127.0.0.1:11611',
      '/mcp': 'http://127.0.0.1:11611'
    }
  }
});
