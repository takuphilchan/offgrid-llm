// electron-builder extraMetadata is authoritative for local preview builds.
function buildVersion(context) {
  const version=context?.packager?.appInfo?.version;
  if(typeof version!=='string'||!/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(version))throw Error('Missing or invalid desktop build version');
  return version;
}
module.exports={buildVersion};
