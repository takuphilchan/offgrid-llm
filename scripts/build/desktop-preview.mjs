// Builds an identifiable local Windows preview; never installs or publishes.
import {createRequire} from 'node:module';
import {execFileSync} from 'node:child_process';
import {mkdir,writeFile,readFile} from 'node:fs/promises';
import {resolve,join} from 'node:path';
import {fileURLToPath} from 'node:url';
import {main as inspectTrust} from '../release/verify-desktop-trust.mjs';

const root=resolve(import.meta.dirname,'../..');
export function previewVersion(base,revision,date=new Date()) {
 const match=/^(\d+)\.(\d+)\.(\d+)$/.exec(base);
 if(!match||!/^[a-f0-9]{8,40}$/.test(revision))throw Error('Preview requires a stable base version and Git revision');
 return `${match[1]}.${match[2]}.${Number(match[3])+1}-preview.${date.toISOString().replace(/[-:TZ.]/g,'')}.g${revision.slice(0,8)}`;
}
export async function buildPreview() {
 if(process.platform!=='win32')throw Error('This preview builder currently qualifies Windows x64 only');
 const desktop=join(root,'apps/desktop'),require=createRequire(join(desktop,'package.json'));
 const {build,Platform,Arch}=require('electron-builder');
 const revision=execFileSync('git',['rev-parse','HEAD'],{cwd:root,encoding:'utf8'}).trim();
 const dirty=!!execFileSync('git',['status','--porcelain'],{cwd:root,encoding:'utf8'}).trim();
 const version=previewVersion(require('./package.json').version,revision);
 const output=join(root,'build',`desktop-${version}`);
 await mkdir(output,{recursive:false});
 const run=(program,args,cwd=root)=>execFileSync(program,args,{cwd,stdio:'inherit',windowsHide:true});
 // Exact local tool versions, no implicit npx installation or shared source-version edits.
 run(process.execPath,[join(root,'apps/web/node_modules/typescript/bin/tsc'),'--noEmit'],join(root,'apps/web'));
 run(process.execPath,[join(root,'apps/web/node_modules/vite/bin/vite.js'),'build'],join(root,'apps/web'));
 await mkdir(join(root,'build/windows'),{recursive:true});
 // Retain Go's linker metadata for the cross-platform service-version gate.
 run('go',['build',`-ldflags=-s -w -X main.Version=${version} -X github.com/takuphilchan/offgrid-llm/internal/server.BuildRevision=${revision}${dirty?'-dirty':''}`,'-o','build/windows/offgrid.exe','./cmd/offgrid']);
 const receipt={schema_version:1,version,revision,dirty,channel:'local-preview',releaseQualified:false,createdAt:new Date().toISOString()};
 await writeFile(join(output,'preview.json'),JSON.stringify(receipt,null,2)+'\n');
 // electron-builder resolves relative package hooks from the current directory.
 const previousDirectory=process.cwd();
 process.chdir(desktop);
 try {await build({projectDir:desktop,targets:Platform.WINDOWS.createTarget(['nsis','portable'],Arch.x64),publish:'never',config:{extraMetadata:{version},directories:{output}}});}
 finally {process.chdir(previousDirectory);}
 await inspectTrust(['--directory',output,'--platform','windows','--version',version,'--mode','inspect','--report',join(output,'artifact-trust.json')]);
 // Keep the UI identity alongside the build identity and native signature report.
 receipt.ui=JSON.parse(await readFile(join(output,'win-unpacked/resources/ui/desktop-compatibility.json'),'utf8'));
 await writeFile(join(output,'preview.json'),JSON.stringify(receipt,null,2)+'\n');
 console.log(`Preview built: ${output}`);
 return output;
}
if(process.argv[1]&&resolve(process.argv[1])===fileURLToPath(import.meta.url))buildPreview().catch(error=>{console.error(error);process.exit(1);});
