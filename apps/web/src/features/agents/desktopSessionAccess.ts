// Readiness is host-owned. Neither a saved radio value nor localStorage grants it.
export async function ensureDesktopSessionAccess(workspace:string){
 const host=window.electron;
 if(!host?.enableDesktopAccess||!host.getDesktopAccess)return;
 const current=await host.getDesktopAccess();
 if(current.enabled&&current.workspace===workspace)return;
 const result=await host.enableDesktopAccess({workspace});
 if(!result.enabled||result.workspace!==workspace)throw Error('computer_scope_violation');
}
export async function revokeDesktopSessionAccess(){
 const host=window.electron;
 if(!host?.getDesktopAccess||!host.stopComputerBrowser)return;
 if((await host.getDesktopAccess()).enabled){
   const state=await host.stopComputerBrowser();
   if(state.state!=='stopped')throw Error(state.code??'computer_stop_unconfirmed');
 }
}
