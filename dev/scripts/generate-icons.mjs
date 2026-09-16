import { copyFile, mkdir, mkdtemp, readdir, rm, stat } from 'node:fs/promises';
import { createRequire } from 'node:module';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..');
const desktopRoot = join(repositoryRoot, 'desktop');
const requireFromDesktop = createRequire(join(desktopRoot, 'package.json'));
const appBuilderEntry = requireFromDesktop.resolve('app-builder-lib');
const { runIconsTool } = requireFromDesktop(join(dirname(appBuilderEntry), 'toolsets', 'icons.js'));
const source = join(desktopRoot, 'assets', 'icon-192.svg');
const temporaryRoot = await mkdtemp(join(tmpdir(), 'offgrid-icons-'));

try {
  const setDirectory = join(temporaryRoot, 'set');
  const icoDirectory = join(temporaryRoot, 'ico');
  const icnsDirectory = join(temporaryRoot, 'icns');
  await Promise.all([setDirectory, icoDirectory, icnsDirectory].map(path => mkdir(path)));

  await runIconsTool({ inputFile: source, outputFormat: 'set', outDir: setDirectory });
  await runIconsTool({ inputFile: source, outputFormat: 'ico', outDir: icoDirectory });
  await runIconsTool({ inputFile: source, outputFormat: 'icns', outDir: icnsDirectory });

  const pngs = (await readdir(setDirectory))
    .map(name => ({ name, size: Number.parseInt(name, 10) }))
    .filter(file => file.name.endsWith('.png') && Number.isFinite(file.size))
    .sort((left, right) => right.size - left.size);
  if (pngs.length === 0 || pngs[0].size < 512) {
    throw new Error('Icon generation did not produce a PNG of at least 512 pixels.');
  }

  const outputs = [
    [join(setDirectory, pngs[0].name), join(desktopRoot, 'assets', 'icon.png')],
    [join(icoDirectory, 'icon.ico'), join(desktopRoot, 'assets', 'icon.ico')],
    [join(icnsDirectory, 'icon.icns'), join(desktopRoot, 'assets', 'icon.icns')]
  ];
  for (const [input, output] of outputs) {
    const details = await stat(input);
    if (details.size === 0) throw new Error(`Generated icon is empty: ${input}`);
    await copyFile(input, output);
  }

  console.log(`Generated desktop icons from ${source}`);
} finally {
  await rm(temporaryRoot, { recursive: true, force: true });
}
