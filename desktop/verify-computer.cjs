// Verify the copied package, not only the build staging directory. In particular,
// electron-builder excludes root node_modules unless it is mapped separately.
const path=require('node:path');
const {verifyPack}=require('../computer/pack.cjs');
module.exports=async context=>{
 const root=path.join(context.packager.getResourcesDir(context.appOutDir),'computer');
 const arch=require('builder-util').Arch[context.arch];
 await verifyPack(root,context.electronPlatformName,arch);
};
