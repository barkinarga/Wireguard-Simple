// Exercise the REAL executable package header on minimal distro containers.
// The payload intentionally performs no app install/service/kernel operations.
import { createTar, installerHeader } from '../scripts/package-linux.mjs';
import { createHash } from 'node:crypto';
import { gzipSync } from 'node:zlib';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
const install = `#!/usr/bin/env bash
set -euo pipefail
source "\${BASH_SOURCE[0]%/*}/dependencies.sh"
for tool in wg wg-quick ip curl modprobe systemctl openssl nft sysctl uname mktemp rm tail sha256sum chmod cp mv install od tr dirname basename sleep cat grep awk sed; do
  command -v "$tool" >/dev/null || { printf 'Missing tool: %s\\n' "$tool"; exit 1; }
done
wgs_select_extractor
wgs_ca_available
printf 'BOOTSTRAP_FIXTURE_READY extractor=%s\\n' "$WGS_EXTRACTOR"
`;
const payload = gzipSync(createTar([
  { name: 'install.sh', data: Buffer.from(install), mode: 0o700 },
  { name: 'dependencies.sh', data: await readFile('scripts/dependencies.sh'), mode: 0o600 },
]));
const header = installerHeader(createHash('sha256').update(payload).digest('hex'));
await mkdir('.tools', { recursive: true });
await writeFile('.tools/bootstrap-fixture.run', Buffer.concat([Buffer.from(header), payload]));
