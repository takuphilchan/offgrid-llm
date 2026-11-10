'use strict';
const {randomUUID}=require('node:crypto');

// A bounded, acknowledged handoff. Keep the terminal's editable preview if the
// workspace is unavailable, already reviewing context, or reloads during delivery.
function createContextDelivery({send,timeout=5000}) {
  let pending=null;
  return {
    deliver(context) {
      if(pending)return Promise.reject(Error('A context preview is already opening. Try again after reviewing it.'));
      return new Promise((resolve,reject)=>{
        const id=randomUUID();
        const finish=(accepted)=>{
          if(pending?.id!==id)return;
          clearTimeout(pending.timer);pending=null;
          if(accepted)resolve({opened:true});
          else reject(Error('Chat could not accept this preview. Keep it here and try again when the workspace is ready.'));
        };
        pending={id,finish,timer:setTimeout(()=>finish(false),timeout)};
        try {send({id,context});} catch {finish(false);}
      });
    },
    acknowledge(id,accepted) {
      if(typeof id!=='string'||typeof accepted!=='boolean'||pending?.id!==id)return false;
      pending.finish(accepted);return true;
    },
    cancel(){pending?.finish(false);}
  };
}
module.exports={createContextDelivery};
