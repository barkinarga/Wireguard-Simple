import test from 'node:test';
import assert from 'node:assert/strict';
import { resolveVersion } from '../scripts/check-release-version.mjs';

const input = { packageVersion: '0.4.15', lockVersion: '0.4.15',
  defaultVersion: '0.4.15-dev', notesTitle: '# WireGuard Simple v0.4.15' };
test('development builds use the project version without publishing', () => {
  assert.deepEqual(resolveVersion(input), { tag: '', version: '0.4.15-dev' });
});
test('release triggers must agree with the package, lockfile, binary default and notes', () => {
  assert.deepEqual(resolveVersion({ ...input, requestedVersion: 'v0.4.15' }),
    { tag: 'v0.4.15', version: '0.4.15' });
  assert.equal(resolveVersion({ ...input, headline: 'release: v0.4.15' }).tag, 'v0.4.15');
  for (const change of [{ requestedVersion: 'v0.4.16' }, { lockVersion: '0.4.14' },
    { defaultVersion: '0.4.14-dev' }, { requestedVersion: 'v0.4.15', notesTitle: '# wrong' },
    { requestedVersion: 'v0.4.15;bad' }]) {
    assert.throws(() => resolveVersion({ ...input, ...change }));
  }
});
