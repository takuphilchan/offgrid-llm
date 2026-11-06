// Read build metadata rather than running a foreign-architecture binary or
// trusting --version from a working directory that may contain Electron's version.
const path = require('node:path');
const { execFileSync } = require('node:child_process');

function verifyServiceVersion(binary, expected, inspect = file => execFileSync('go', ['version', '-m', file], { encoding: 'utf8', windowsHide: true })) {
  const metadata = inspect(binary);
  const flags = metadata.split(/\r?\n/).find(line => /\s-ldflags=/.test(line)) || '';
  const versions = [...flags.matchAll(/(?:^|[\s"'])main\.Version=([^\s"'\\]+)/g)].map(match => match[1].replace(/^v(?=\d)/, ''));
  if (versions.length !== 1 || versions[0] !== expected) {
    throw Error(`Bundled service must embed OffGrid ${expected} with -X main.Version=${expected}: ${binary}`);
  }
}

function verifyBundledServices(resources, platform, version) {
  const names = platform === 'darwin' ? ['offgrid-amd64', 'offgrid-arm64'] : [platform === 'win32' ? 'offgrid.exe' : 'offgrid'];
  for (const name of names) verifyServiceVersion(path.join(resources, 'bin', name), version);
}

module.exports = { verifyServiceVersion, verifyBundledServices };
