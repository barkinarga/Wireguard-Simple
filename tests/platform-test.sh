#!/usr/bin/env bash
# Pure distribution decisions and mocked package/kernel/SELinux operations.
set -euo pipefail
ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
source "$ROOT/scripts/install.sh"
wgs_ca_available() { return 0; }
wgs_is_root() { return 0; }
assert_equal() { [[ $1 == "$2" ]] || { printf 'Expected [%s], got [%s]\n' "$2" "$1" >&2; exit 1; }; }

for id in debian ubuntu rhel centos almalinux rocky fedora; do
  (
    version=9; name=$id
    case $id in debian) version=13 ;; ubuntu) version=24.04 ;; centos) name='CentOS Stream' ;; fedora) version=43 ;; esac
    wgs_distribution "$id" "$version" "$name"
    if [[ $id == debian || $id == ubuntu ]]; then
      assert_equal "$WGS_FAMILY:$WGS_MANAGER:$WGS_IP_PACKAGE" 'debian:apt-get:iproute2'
    else
      assert_equal "$WGS_MANAGER:$WGS_IP_PACKAGE" 'dnf:iproute'
    fi
  )
done
for id in rhel almalinux rocky; do wgs_distribution "$id" 8 "$id"; done
wgs_distribution centos 10 'CentOS Stream'
for scenario in 'centos|7|CentOS Linux' 'centos|8|CentOS Linux' 'centos|8|CentOS Stream' 'rhel|7|Red Hat' 'arch|1|Arch' 'rhel|bad|Red Hat'; do
  IFS='|' read -r id version name <<< "$scenario"
  if (wgs_distribution "$id" "$version" "$name") >/dev/null 2>&1; then printf 'Unsupported platform accepted: %s\n' "$scenario" >&2; exit 1; fi
done
printf '%s\n' 'PASS all seven distributions, EL8 conditional support, legacy and unknown rejection'

for id in debian ubuntu rhel centos almalinux rocky fedora; do
  (
    wgs_distribution "$id" 9 'CentOS Stream'
    calls=(); tools_ready=0; WGS_CHECK=0; WGS_DEPS=1
    apt-get() { calls+=("apt-get $*"); [[ $1 != install ]] || tools_ready=1; }
    dnf() { calls+=("dnf $*"); tools_ready=1; }
    yum() { calls+=("yum $*"); tools_ready=1; }
    command() { if [[ ${1:-} == -v ]]; then ((tools_ready)); return; fi; builtin command "$@"; }
    WGS_CHECK=1; wgs_dependencies >/dev/null; assert_equal "${#calls[@]}" 0
    WGS_CHECK=0; WGS_DEPS=0; if wgs_dependencies >/dev/null 2>&1; then exit 1; fi; assert_equal "${#calls[@]}" 0
    WGS_DEPS=1; wgs_dependencies >/dev/null
    if [[ $WGS_FAMILY == debian ]]; then
      assert_equal "${calls[0]}" 'apt-get update'
      assert_equal "${calls[1]}" 'apt-get install -y --no-install-recommends wireguard-tools iproute2 curl kmod systemd openssl nftables procps coreutils grep gawk sed tar gzip ca-certificates'
    else
      assert_equal "${calls[0]}" 'dnf install -y --setopt=install_weak_deps=False wireguard-tools iproute curl kmod systemd openssl nftables procps-ng coreutils grep gawk sed tar gzip ca-certificates'
    fi
  )
done
printf '%s\n' 'PASS apt/dnf package plans, check-only and no-deps for every distribution'

(
  wgs_distribution rhel 9 RHEL
  command() { if [[ ${1:-} == -v ]]; then [[ $2 == yum ]]; return; fi; builtin command "$@"; }
  wgs_package_manager
  assert_equal "$WGS_MANAGER" yum
  wgs_distribution fedora 43 Fedora
  if wgs_package_manager >/dev/null 2>&1; then exit 1; fi
)
(
  wgs_distribution rocky 9 Rocky
  calls=(); WGS_CHECK=0; WGS_DEPS=1
  command() { if [[ ${1:-} == -v ]]; then [[ $2 == curl || $2 == modprobe ]]; return; fi; builtin command "$@"; }
  dnf() { calls+=("$*"); return 1; }
  if wgs_dependencies >/dev/null 2>&1; then exit 1; fi
  assert_equal "${#calls[@]}" 3
  assert_equal "${calls[0]}" 'install -y --setopt=install_weak_deps=False wireguard-tools iproute systemd openssl nftables procps-ng coreutils grep gawk sed tar gzip ca-certificates'
)
printf '%s\n' 'PASS RHEL yum fallback, repository failure stops without adding repos, existing curl preserved'

(
  wgs_kernel_loaded() { return 1; }
  modprobe() { assert_equal "$*" '--dry-run --quiet wireguard'; }
  wgs_kernel
  modprobe() { return 1; }
  if wgs_kernel >/dev/null 2>&1; then exit 1; fi
  wgs_kernel_loaded() { return 0; }
  wgs_kernel
)
(
  labels=()
  restorecon() { labels=("$@"); }
  wgs_restore_labels /example/binary /example/unit
  assert_equal "${labels[*]}" '-- /example/binary /example/unit'
)
printf '%s\n' 'PASS read-only WireGuard kernel preflight and exact-path SELinux label restore'
