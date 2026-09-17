// Deterministic, dependency-free NSIS bitmaps derived from OffGrid's layer mark.
// Keep generated artwork monochrome; Windows owns controls and accessibility.
import { writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

function bitmap(width, height, dark, scale) {
  const stride = (width * 3 + 3) & ~3;
  const file = Buffer.alloc(54 + stride * height, dark ? 17 : 255);
  file.fill(0, 0, 54);
  file.write('BM'); file.writeUInt32LE(file.length, 2); file.writeUInt32LE(54, 10);
  file.writeUInt32LE(40, 14); file.writeInt32LE(width, 18); file.writeInt32LE(height, 22);
  file.writeUInt16LE(1, 26); file.writeUInt16LE(24, 28); file.writeUInt32LE(stride * height, 34);
  const ox = (width - 24 * scale) / 2, oy = dark ? 62 : (height - 24 * scale) / 2;
  const paths = [[[12,2],[2,7],[12,12],[22,7],[12,2]], [[2,12],[12,17],[22,12]], [[2,17],[12,22],[22,17]]];
  const segments = paths.flatMap(points => points.slice(1).map((point, i) => [points[i], point]));
  for (let y = 0; y < height; y++) for (let x = 0; x < width; x++) {
    const px = (x - ox) / scale, py = (y - oy) / scale;
    const distance = Math.min(...segments.map(([a,b]) => {
      const dx = b[0] - a[0], dy = b[1] - a[1];
      const t = Math.max(0, Math.min(1, ((px-a[0])*dx+(py-a[1])*dy)/(dx*dx+dy*dy)));
      return Math.hypot(px-a[0]-t*dx, py-a[1]-t*dy);
    }));
    const coverage = Math.max(0, Math.min(1, (.75 - distance) * scale + .5));
    const color = Math.round(dark ? 17 + coverage * 223 : 255 - coverage * 230);
    const offset = 54 + (height - 1 - y) * stride + x * 3;
    file.fill(color, offset, offset + 3);
  }
  return file;
}
for (const [name, width, height, dark, scale] of [
  ['installer-sidebar.bmp', 164, 314, true, 3],
  ['installer-header.bmp', 150, 57, false, 1.7]
]) {
  await writeFile(fileURLToPath(new URL(`../../desktop/assets/${name}`, import.meta.url)), bitmap(width, height, dark, scale));
}
console.log('Generated monochrome NSIS header and sidebar.');
