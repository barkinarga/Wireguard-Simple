import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export function resolveVersion({ packageVersion, lockVersion, defaultVersion,
  notesTitle, requestedVersion = '', headline = '' }) {
  const semver = /^\d+\.\d+\.\d+(?:-[a-zA-Z0-9.-]+)?$/;
  assert.match(packageVersion, semver, 'Invalid project version');
  assert.equal(lockVersion, packageVersion, 'Lockfile and package versions differ');
  assert.equal(defaultVersion, packageVersion + '-dev', 'Binary default and package versions differ');
  const tag = requestedVersion || (headline.startsWith('release: v') ? headline.slice(9) : '');
  if (tag) {
    assert.equal(tag, 'v' + packageVersion, 'Release tag and project version differ');
    assert.equal(notesTitle, '# WireGuard Simple ' + tag, 'Release notes and tag differ');
  }
  return { tag, version: tag ? packageVersion : packageVersion + '-dev' };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const pkg = JSON.parse(fs.readFileSync('package.json', 'utf8'));
  const lock = JSON.parse(fs.readFileSync('package-lock.json', 'utf8'));
  assert.equal(lock.packages[''].version, pkg.version, 'Root lockfile version differs');
  const result = resolveVersion({ packageVersion: pkg.version, lockVersion: lock.version,
    defaultVersion: fs.readFileSync('cmd/wgsimple/main.go', 'utf8').match(/var version = "([^"]+)"/)?.[1],
    notesTitle: fs.readFileSync('docs/release-notes.md', 'utf8').split(/\r?\n/)[0],
    requestedVersion: process.env.REQUESTED_VERSION || '',
    headline: (process.env.COMMIT_MESSAGE || '').split('\n')[0] });
  if (process.env.GITHUB_OUTPUT) {
    fs.appendFileSync(process.env.GITHUB_OUTPUT, `tag=${result.tag}\nversion=${result.version}\n`);
  }
  console.log(`Verified project version: ${result.version}${result.tag ? ' (release)' : ''}`);
}
