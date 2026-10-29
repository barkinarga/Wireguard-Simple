#!/usr/bin/env bash
# Shared by the executable package header and the unpacked installer.
# Dependency discovery uses Bash builtins only; missing core tools are expected.
wgs_message() { printf '%s\n' "$*"; }
wgs_fail() { printf 'Error: %s\n' "$*" >&2; return 1; }
wgs_distribution() {
  local id=$1 version=$2 name=$3
  WGS_DISTRO=$id
  WGS_MAJOR=${version%%.*}
  case $id in
    debian|ubuntu) WGS_FAMILY=debian; WGS_MANAGER=apt-get; WGS_IP_PACKAGE=iproute2 ;;
    rhel|rocky|almalinux|centos)
      [[ $WGS_MAJOR =~ ^[0-9]+$ ]] && ((WGS_MAJOR >= 8)) || { wgs_fail 'Enterprise Linux 8 or newer is required'; return 1; }
      if [[ $id == centos ]] && { [[ $name != *Stream* ]] || ((WGS_MAJOR < 9)); }; then
        wgs_fail 'Use CentOS Stream 9 or newer; CentOS Linux 7/8 and Stream 8 are end-of-life'; return 1
      fi
      WGS_FAMILY=rhel; WGS_MANAGER=dnf; WGS_IP_PACKAGE=iproute ;;
    fedora) WGS_FAMILY=fedora; WGS_MANAGER=dnf; WGS_IP_PACKAGE=iproute ;;
    *) wgs_fail 'Supported distributions: Debian, Ubuntu, RHEL, CentOS Stream, AlmaLinux, Rocky Linux, Fedora'; return 1 ;;
  esac
}
wgs_package_manager() {
  if command -v "$WGS_MANAGER" >/dev/null 2>&1; then return 0; fi
  if [[ $WGS_FAMILY == rhel ]] && command -v yum >/dev/null 2>&1; then WGS_MANAGER=yum; return 0; fi
  wgs_fail "Package manager $WGS_MANAGER is unavailable; immutable/ostree images are not supported"
}

wgs_is_root() { [[ $EUID -eq 0 ]]; }
wgs_ca_available() {
  [[ -s /etc/ssl/certs/ca-certificates.crt || -s /etc/pki/tls/certs/ca-bundle.crt ]]
}
wgs_select_extractor() {
  if command -v tar >/dev/null 2>&1 && command -v gzip >/dev/null 2>&1; then
    WGS_EXTRACTOR=tar
  elif command -v bsdtar >/dev/null 2>&1; then
    WGS_EXTRACTOR=bsdtar
  else
    WGS_EXTRACTOR=
    return 1
  fi
}
wgs_extract_payload() {
  wgs_select_extractor || { wgs_fail 'No gzip-capable archive extractor. Install tar and gzip, or libarchive/bsdtar.'; return 1; }
  if [[ $WGS_EXTRACTOR == tar ]]; then
    if tar --extract --gzip --file "$1" --directory "$2" --no-same-owner --no-same-permissions; then return 0; fi
    command -v bsdtar >/dev/null 2>&1 || { wgs_fail 'Verified payload could not be extracted. Check available disk space and tar/gzip.'; return 1; }
    wgs_message 'GNU tar extraction failed; trying the installed bsdtar.' >&2
  fi
  bsdtar --extract --file "$1" --directory "$2" --no-same-owner --no-same-permissions || {
    wgs_fail 'Verified payload could not be extracted; application files and services were not changed.'; return 1;
  }
}
wgs_package_install() {
  local attempt
  for attempt in 1 2; do
    if [[ $WGS_FAMILY == debian ]]; then
      apt-get update || wgs_message 'Repository refresh failed; trying the existing signed package metadata.' >&2
      if DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends "$@"; then return 0; fi
    elif ((attempt == 1)); then
      if "$WGS_MANAGER" install -y --setopt=install_weak_deps=False "$@"; then return 0; fi
    else
      # Refresh stale repository metadata without enabling repos or weakening trust.
      "$WGS_MANAGER" makecache || wgs_message 'Repository refresh failed; retrying with existing package metadata.' >&2
      if "$WGS_MANAGER" install -y --setopt=install_weak_deps=False "$@"; then return 0; fi
    fi
    if ((attempt == 1)); then wgs_message 'Package setup failed; refreshing repository metadata and retrying once.' >&2; fi
  done
  wgs_fail "Could not install required packages: $*. Check enabled repositories/network/RHEL subscription and rerun. Application services and data have not been changed."
}
wgs_dependencies() {
  local entry tool package
  local -a packages=() tools=(wg wg-quick ip curl modprobe systemctl openssl)
  local -a entries=(wg:wireguard-tools wg-quick:wireguard-tools "ip:$WGS_IP_PACKAGE" curl:curl modprobe:kmod systemctl:systemd openssl:openssl)
  local -A seen=()
  if [[ $WGS_NETWORK != off ]]; then
    tools+=(nft sysctl)
    entries+=(nft:nftables)
    if [[ $WGS_FAMILY == debian ]]; then entries+=(sysctl:procps); else entries+=(sysctl:procps-ng); fi
  fi
  if [[ $WGS_FAMILY == rhel || $WGS_FAMILY == fedora ]] && [[ -e /sys/fs/selinux/enforce ]]; then
    tools+=(restorecon); entries+=(restorecon:policycoreutils)
  fi
  # Check every command the header/installer invokes, not just VPN executables.
  for tool in uname mktemp rm tail sha256sum chmod cp mv install od tr dirname basename sleep cat; do entries+=("$tool:coreutils"); tools+=("$tool"); done
  entries+=(grep:grep awk:gawk sed:sed)
  tools+=(grep awk sed)
  if ! wgs_select_extractor; then entries+=(tar:tar gzip:gzip); fi
  for entry in "${entries[@]}"; do
    tool=${entry%%:*}; package=${entry#*:}
    if ! command -v "$tool" >/dev/null 2>&1 && [[ ! -v seen[$package] ]]; then
      packages+=("$package"); seen[$package]=1
    fi
  done
  if ((${#packages[@]})) || ! wgs_ca_available; then packages+=(ca-certificates); fi
  if ((${#packages[@]})); then
    if ((WGS_CHECK)); then wgs_message "Missing packages via $WGS_MANAGER: ${packages[*]} (check only)"; return 0; fi
    ((WGS_DEPS)) || { wgs_fail "Missing packages: ${packages[*]}. Install them manually or omit --no-deps"; return 1; }
    wgs_is_root || { wgs_fail 'Run with sudo to install missing dependencies.'; return 1; }
    wgs_message "Installing required packages via $WGS_MANAGER: ${packages[*]}"
    wgs_package_install "${packages[@]}" || return 1
    hash -r
  fi
  for tool in "${tools[@]}"; do
    command -v "$tool" >/dev/null 2>&1 || { wgs_fail "Required tool is still unavailable: $tool"; return 1; }
  done
  wgs_select_extractor || { wgs_fail 'Archive extraction is still unavailable after package setup.'; return 1; }
  wgs_ca_available || { wgs_fail 'The system CA certificate bundle is missing after package setup.'; return 1; }
}
wgs_bootstrap_dependencies() {
  local arg kernel= ID= VERSION_ID= NAME=
  WGS_CHECK=0; WGS_DEPS=1; WGS_NETWORK=auto
  while (($#)); do
    arg=$1; shift
    case $arg in
      --check) WGS_CHECK=1 ;;
      --no-deps) WGS_DEPS=0 ;;
      --no-network) WGS_NETWORK=off ;;
      --configure-network) WGS_NETWORK=on ;;
      --no-start|--no-update-check) ;;
      --interface) [[ $# -ge 1 && $1 =~ ^[a-zA-Z0-9][a-zA-Z0-9_-]{0,14}$ ]] || { wgs_fail 'Invalid or missing interface name'; return 1; }; shift ;;
      *) wgs_fail "Unknown package option: $arg"; return 1 ;;
    esac
  done
  [[ $OSTYPE == linux* ]] || { wgs_fail 'Run this installer on the target Linux server only.'; return 1; }
  if [[ -r /proc/sys/kernel/osrelease ]]; then IFS= read -r kernel < /proc/sys/kernel/osrelease || true; fi
  [[ ${kernel,,} != *microsoft* ]] || { wgs_fail 'Use the target Linux server instead of WSL.'; return 1; }
  [[ -f /etc/os-release ]] || { wgs_fail 'Could not determine the Linux distribution.'; return 1; }
  source /etc/os-release
  wgs_distribution "$ID" "$VERSION_ID" "$NAME" || return 1
  [[ ! -e /run/ostree-booted ]] || { wgs_fail 'Immutable/ostree systems require a separate deployment method.'; return 1; }
  wgs_package_manager || return 1
  wgs_dependencies || return 1
  # --check never installs packages; do not try unpacking with missing tools.
  for arg in uname mktemp rm tail sha256sum chmod cp mv install od tr dirname basename sleep cat grep awk sed; do
    command -v "$arg" >/dev/null 2>&1 || { wgs_fail "Cannot unpack: missing $arg. Run without --check to install dependencies."; return 1; }
  done
  wgs_select_extractor || { wgs_fail 'Cannot unpack: install tar/gzip or bsdtar, or run without --check.'; return 1; }
}
