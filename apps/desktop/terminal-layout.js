'use strict';
function validateWorkspaceBounds(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value) || Object.keys(value).some(key=>!['x','y','width','height'].includes(key)) ||
      !['x','y','width','height'].every(key=>Number.isInteger(value[key])&&value[key]>=0&&value[key]<=20000) || !value.width || !value.height) throw Error('terminal_invalid_bounds');
  return {...value};
}
function terminalBounds(rect, width, height, zoom=1, captionHeight=0) {
  const x=Math.max(0,Math.min(width-1,Math.round((rect?.x||0)*zoom)));
  const y=Math.min(height-1,Math.max(captionHeight,Math.round((rect?.y||0)*zoom)));
  return {x,y,width:Math.max(1,Math.min(width-x,Math.round((rect?.width||width/zoom)*zoom))),height:Math.max(1,Math.min(height-y,Math.round((rect?.height||height/zoom)*zoom)))};
}
module.exports={validateWorkspaceBounds,terminalBounds};
