// Verify this recovery snapshot against hashes read from the live release.
// This is an audit command, not a deployment command or a claim of full UI recovery.
import { createHash } from 'node:crypto';
import { readFile, readdir } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root = fileURLToPath(new URL('..', import.meta.url));
const manifest = JSON.parse(await readFile(path.join(root, '../docs/history/wudi-source-recovery-20261116.json'), 'utf8'));
const expected = manifest.serverFiles;
const failures = [];
const actual = (await readdir(path.join(root, 'server'))).filter(name => name.endsWith('.mjs')).map(name => 'server/' + name).sort();
if (JSON.stringify(actual) !== JSON.stringify(Object.keys(expected).sort())) failures.push('server file inventory differs');
for (const [relative, sha256] of Object.entries(expected)) {
  if (!/^server\/[\w-]+\.mjs$/.test(relative)) throw new Error('Unexpected manifest path: ' + relative);
  const hash = createHash('sha256').update(await readFile(path.join(root, relative))).digest('hex');
  if (hash !== sha256) failures.push(relative);
}
console.log(JSON.stringify({release: manifest.productionRelease, files: actual.length, failures, originalGitObjectRecovered: false, originalFrontendFilesRecovered: false, frontendReconstructionRecord: manifest.frontendReconstructionRecord || null}, null, 2));
if (failures.length) process.exitCode = 1;
