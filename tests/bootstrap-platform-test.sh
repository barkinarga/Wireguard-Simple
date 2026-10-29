#!/usr/bin/env bash
# Run inside a disposable distro container: package installation is intentional.
set -euo pipefail
source /etc/os-release
fixture=/workspace/.tools/bootstrap-fixture.run
if [[ $ID == almalinux ]]; then
  # CI only: bound slow mirror requests without changing repositories, package
  # verification or the shipped installer. Its normal retry still applies.
  dnf() { command dnf --setopt=timeout=20 --setopt=minrate=1000 --setopt=retries=2 "$@"; }
  export -f dnf
fi
if [[ $ID == centos && ${VERSION_ID%%.*} == 10 ]]; then
  # Explicit regression for the reported minimal CentOS Stream 10 failure.
  if command -v tar >/dev/null 2>&1; then dnf remove -y --setopt=clean_requirements_on_remove=False tar; fi
  ! command -v tar >/dev/null 2>&1
  if bash "$fixture" --no-deps --no-update-check >/tmp/no-deps.log 2>&1; then exit 1; fi
  no_deps_output=$(</tmp/no-deps.log)
  [[ $no_deps_output == *'Missing packages:'* ]]
  if bash "$fixture" --check >/tmp/check.log 2>&1; then exit 1; fi
  ! command -v tar >/dev/null 2>&1
  printf '%s\n' 'PASS CentOS Stream 10 starts without tar; no-deps/check leave it uninstalled'
fi
printf 'Starting executable-package bootstrap on %s\n' "$PRETTY_NAME"
# Stream package-manager progress; a slow mirror must not look like a frozen job.
bash "$fixture" --no-update-check 2>&1 | tee /tmp/bootstrap.log
grep -q 'BOOTSTRAP_FIXTURE_READY' /tmp/bootstrap.log
[[ ! -e /usr/local/bin/wgsimple && ! -e /var/lib/wgsimple/control.db && ! -e /etc/systemd/system/wgsimple.service ]]
printf 'PASS %s: real header installs prerequisites, verifies and extracts its payload\n' "$PRETTY_NAME"
if [[ $ID == debian ]]; then
  apt-get update
  apt-get install -y --no-install-recommends libarchive-tools
  # Hide GNU tar in this disposable container; bsdtar must really unpack gzip.
  mv "$(command -v tar)" /tmp/gnu-tar-test-only
  bash "$fixture" --no-deps --no-update-check > /tmp/bsdtar.log 2>&1 || { cat /tmp/bsdtar.log; exit 1; }
  grep -q 'BOOTSTRAP_FIXTURE_READY extractor=bsdtar' /tmp/bsdtar.log
  printf '%s\n' 'PASS real bsdtar gzip extraction without GNU tar'
fi
