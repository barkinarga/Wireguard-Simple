import { build } from 'esbuild';
import { mkdir, copyFile, stat } from 'node:fs/promises';
await mkdir('internal/web/dist', { recursive: true });
await build({ entryPoints: ['ui/app.ts'], outfile: 'internal/web/dist/app.js', bundle: true, minify: true, target: 'es2022', legalComments: 'none' });
for (const file of ['index.html', 'style.css', 'favicon.svg']) await copyFile(`ui/${file}`, `internal/web/dist/${file}`);
const files = await Promise.all(['app.js', 'style.css', 'index.html'].map(async name => ({ name, bytes: (await stat(`internal/web/dist/${name}`)).size })));
console.log('Embedded UI built:', files);
