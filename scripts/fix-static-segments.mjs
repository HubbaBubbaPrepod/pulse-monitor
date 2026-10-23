import { readdir, copyFile, access } from 'node:fs/promises';
import { join, relative, sep, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

// Next's exporter passes path.relative() to a URL-path encoder. On Windows
// nested segment paths retain backslashes and become subdirectories rather
// than the flat __next.<segment>.txt names requested by the browser.
const root = resolve(fileURLToPath(new URL('..', import.meta.url)));
const source = join(root, '.next/server/app');
const output = join(root, 'out');
let copied = 0;
async function visit(directory, base, destination) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) {
      await visit(path, base, destination);
      continue;
    }
    if (!entry.isFile() || !entry.name.endsWith('.segment.rsc')) continue;
    const segment = relative(base, path).split(sep).join('/').slice(0, -'.segment.rsc'.length);
    const filename = '__next.' + segment.replaceAll('/', '.') + '.txt';
    await copyFile(path, join(destination, filename));
    copied++;
  }
}
for (const entry of await readdir(source, { withFileTypes: true })) {
  if (!entry.isDirectory() || !entry.name.endsWith('.segments')) continue;
  const route = entry.name.slice(0, -'.segments'.length);
  const destination = route === 'index' ? output : join(output, route);
  try {
    await access(join(destination, 'index.html'));
  } catch {
    continue;
  }
  const directory = join(source, entry.name);
  await visit(directory, directory, destination);
}
console.log(`Verified portable static export: ${copied} prefetch segments`);
