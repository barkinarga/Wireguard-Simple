import { readFile, writeFile, mkdir, readdir } from 'node:fs/promises';
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { gzipSync } from 'node:zlib';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root = fileURLToPath(new URL('../', import.meta.url));
const sha256 = bytes => createHash('sha256').update(bytes).digest('hex');
const dependencyHelper = readFileSync(new URL('./dependencies.sh', import.meta.url), 'utf8').replace(/\r\n/g, '\n');
const panelOutputHelper = readFileSync(new URL('./panel-output.sh', import.meta.url), 'utf8').replace(/\r\n/g, '\n');
const updateHelper = readFileSync(new URL('./update-check.sh', import.meta.url), 'utf8').replace(/\r\n/g, '\n');
const packageVersion = JSON.parse(readFileSync(new URL('../package.json', import.meta.url), 'utf8')).version;

export function createTar(entries) {
  const chunks = [];
  for (const { name, data, mode = 0o644 } of entries) {
    if (!/^[a-zA-Z0-9._-]+$/.test(name) || name.length > 99) throw new Error('Unsafe archive name');
    const header = Buffer.alloc(512);
    header.write(name, 0, 100, 'ascii');
    for (const [offset, length, value] of [[100, 8, mode], [108, 8, 0], [116, 8, 0], [124, 12, data.length], [136, 12, 0]]) {
      header.write(value.toString(8).padStart(length - 1, '0') + '\0', offset, length, 'ascii');
    }
    header.fill(32, 148, 156);
    header.write('0', 156);
    header.write('ustar\0', 257, 6, 'ascii');
    header.write('00', 263, 2, 'ascii');
    const checksum = header.reduce((sum, value) => sum + value, 0);
    header.write(checksum.toString(8).padStart(6, '0') + '\0 ', 148, 8, 'ascii');
    chunks.push(header, data, Buffer.alloc((512 - data.length % 512) % 512));
  }
  return Buffer.concat([...chunks, Buffer.alloc(1024)]);
}

export function installerHeader(payloadHash) {
  return String.raw`#!/usr/bin/env bash
# WireGuard Simple: self-extracting installer for a target Linux server.
set -euo pipefail
if [[ $1 == --help || $1 == -h ]]; then
  printf '%s\n' 'WireGuard Simple — install or update' 'Debian / Ubuntu / RHEL / CentOS Stream / AlmaLinux / Rocky Linux / Fedora' 'Usage: sudo bash wgsimple-install.run [--check] [--no-start] [--no-deps] [--interface wg0] [--no-network] [--configure-network] [--no-update-check]' 'Checks GitHub for the latest stable release by default. Does not install on Windows or WSL.'
  exit 0
fi
export PATH=/usr/sbin:/usr/bin:/sbin:/bin
@DEPENDENCY_HELPER@
wgs_bootstrap_dependencies "$@"
case $(uname -m) in x86_64|amd64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; *) printf '%s\n' 'Unsupported architecture.' >&2; exit 1 ;; esac
if ((EUID != 0 && WGS_CHECK == 0)); then printf '%s\n' 'Install using sudo bash wgsimple-install.run.' >&2; exit 1; fi
umask 077
temporary=$(mktemp -d /tmp/wgsimple-package.XXXXXX)
cleanup() {
  if [[ -d $temporary && ! -L $temporary ]]; then
    case $(cd "$temporary" && pwd -P) in /tmp/wgsimple-package.*) rm -rf -- "$temporary" ;; esac
  fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
@PANEL_OUTPUT_HELPER@
@UPDATE_HELPER@
update_result=0
wgs_check_release "$temporary" '@PACKAGE_VERSION@' "$@" || update_result=$?
case $update_result in 0) ;; 10|11) exit 0 ;; *) exit "$update_result" ;; esac
payload_line=$(awk '$0 == "__WGS_PAYLOAD__" { print NR + 1; exit }' "$0")
[[ $payload_line =~ ^[0-9]+$ ]] || { printf '%s\n' 'Invalid package header.' >&2; exit 1; }
tail -n +"$payload_line" "$0" > "$temporary/payload.tar.gz"
printf '%s  %s\n' '@PAYLOAD_SHA256@' "$temporary/payload.tar.gz" | sha256sum --check --status - || { printf '%s\n' 'Package integrity check failed; nothing installed.' >&2; exit 1; }
wgs_extract_payload "$temporary/payload.tar.gz" "$temporary"
bash "$temporary/install.sh" --binary "$temporary/wgsimple-$arch" "$@"
exit 0
__WGS_PAYLOAD__
`.replace('@DEPENDENCY_HELPER@', () => dependencyHelper).replace('@PAYLOAD_SHA256@', payloadHash).replace('@PANEL_OUTPUT_HELPER@', () => panelOutputHelper).replace('@UPDATE_HELPER@', () => updateHelper).replace('@PACKAGE_VERSION@', packageVersion).replace('[[ $1 == --help || $1 == -h ]]', '[[ ${1:-} == --help || ${1:-} == -h ]]');
}

export async function buildInstaller() {
  const files = [
    ['install.sh', 'scripts/install.sh', 0o700],
    ['dependencies.sh', 'scripts/dependencies.sh', 0o600],
    ['panel-output.sh', 'scripts/panel-output.sh', 0o600],
    ['wgsimple-amd64', 'bin/wgsimple-linux-amd64', 0o700],
    ['wgsimple-arm64', 'bin/wgsimple-linux-arm64', 0o700],
    ['INSTALL.md', 'INSTALL.md', 0o600],
    ['THIRD_PARTY_NOTICES.md', 'THIRD_PARTY_NOTICES.md', 0o600],
    ['LICENSE', 'LICENSE', 0o600],
  ];
  const entries = await Promise.all(files.map(async ([name, source, mode]) => ({ name, data: await readFile(path.join(root, source)), mode })));
  for (const entry of entries.filter(entry => entry.name.endsWith('.sh'))) {
    if (entry.data.includes(13)) throw new Error(`Linux script must use LF line endings: ${entry.name}`);
  }
  for (const [name, machine] of [['wgsimple-amd64', 62], ['wgsimple-arm64', 183]]) {
    const bytes = entries.find(entry => entry.name === name).data;
    if (bytes.length < 20 || !bytes.subarray(0, 6).equals(Buffer.from([127, 69, 76, 70, 2, 1])) || bytes.readUInt16LE(18) !== machine) throw new Error(`Wrong Linux architecture: ${name}`);
  }
  const payload = gzipSync(createTar(entries), { level: 9 });
  const header = Buffer.from(installerHeader(sha256(payload)), 'utf8');
  await mkdir(path.join(root, 'release'), { recursive: true });
  const output = path.join(root, 'release/wgsimple-install.run');
  await writeFile(output, Buffer.concat([header, payload]));
  const sums = [];
  for (const name of (await readdir(path.join(root, 'release'))).filter(name => /\.(zip|run)$/.test(name)).sort()) {
    sums.push(`${sha256(await readFile(path.join(root, 'release', name)))}  ${name}`);
  }
  await writeFile(path.join(root, 'release/SHA256SUMS'), sums.join('\n') + '\n');
  return { output, bytes: header.length + payload.length, architectures: ['amd64', 'arm64'] };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) console.log(await buildInstaller());
