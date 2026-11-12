import {spawnSync} from 'node:child_process';
import {resolve} from 'node:path';
const root=resolve(import.meta.dirname,'../..'),reportOnly=process.argv.includes('--report-only');
const args=process.argv.slice(2).filter(value=>value!=='--report-only');
const result=spawnSync(process.execPath,[resolve(root,'apps/web/node_modules/@playwright/test/cli.js'),'test','localization-rendered-audit.spec.ts',...args],{
 cwd:resolve(root,'apps/web'),stdio:'inherit',env:{...process.env,OFFGRID_I18N_STRICT:reportOnly?'0':'1'}
});
if(result.error)throw result.error;process.exitCode=result.status??1;
