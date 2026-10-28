import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { gunzipSync } from 'node:zlib';
import { spawnSync } from 'node:child_process';
import { createTar, installerHeader } from '../scripts/package-linux.mjs';

test('archive rejects paths outside its directory', () => {
  assert.throws(() => createTar([{ name: '../outside', data: Buffer.alloc(0) }]));
  assert.throws(() => createTar([{ name: '/absolute', data: Buffer.alloc(0) }]));
});
test('no-argument bootstrap handles unset positional arguments', () => {
  assert.match(installerHeader('a'.repeat(64)), /\$\{1:-\}/);
});
test('produced installer has a verified payload with both correct Linux binaries', async () => {
  const bytes = await readFile(new URL('../release/wgsimple-install.run', import.meta.url));
  const marker = Buffer.from('\n__WGS_PAYLOAD__\n');
  const start = bytes.indexOf(marker) + marker.length;
  assert.ok(start > marker.length);
  const header = bytes.subarray(0, start).toString('utf8');
  const payload = bytes.subarray(start);
  const expected = header.match(/'([0-9a-f]{64})'/)?.[1];
  assert.equal(createHash('sha256').update(payload).digest('hex'), expected);
  const changed = Buffer.from(payload); changed[changed.length - 10] ^= 1;
  assert.notEqual(createHash('sha256').update(changed).digest('hex'), expected);
  const tar = gunzipSync(payload);
  const listed = spawnSync('tar', ['-tzf', '-'], { input: payload, maxBuffer: 1024 * 1024 });
  assert.equal(listed.status, 0, listed.stderr?.toString() || listed.error?.message);
  assert.deepEqual(new Set(listed.stdout.toString().trim().split(/\r?\n/)), new Set(['install.sh', 'wgsimple-amd64', 'wgsimple-arm64', 'KURULUM.md', 'THIRD_PARTY_NOTICES.md', 'LICENSE']));
  const entries = new Map();
  for (let offset = 0; offset < tar.length && tar[offset] !== 0;) {
    const name = tar.subarray(offset, offset + 100).toString().replace(/\0.*$/s, '');
    const size = parseInt(tar.subarray(offset + 124, offset + 136).toString().replace(/\0.*$/s, ''), 8);
    assert.ok(!name.includes('/') && !name.includes('..'));
    entries.set(name, tar.subarray(offset + 512, offset + 512 + size));
    offset += 512 + Math.ceil(size / 512) * 512;
  }
  for (const [arch, machine] of [['amd64', 62], ['arm64', 183]]) {
    const packed = entries.get(`wgsimple-${arch}`);
    assert.equal(packed.readUInt16LE(18), machine);
    assert.deepEqual(packed, await readFile(new URL(`../bin/wgsimple-linux-${arch}`, import.meta.url)));
  }
  assert.deepEqual(entries.get('install.sh'), await readFile(new URL('../scripts/install.sh', import.meta.url)));
  assert.ok(entries.has('KURULUM.md') && entries.has('THIRD_PARTY_NOTICES.md'));
});
