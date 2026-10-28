#!/usr/bin/env bash
# Target Linux server only. Safe to source for isolated tests.
set -euo pipefail

WGS_BIN_DIR=/usr/local/bin
WGS_STATE_DIR=/var/lib/wgsimple
WGS_CONFIG_DIR=/etc/wireguard
WGS_UNIT_DIR=/etc/systemd/system
WGS_INTERFACE=wg0
WGS_BINARY=
WGS_START=1
WGS_DEPS=1
WGS_CHECK=0
WGS_TEMP=
WGS_ROLLBACK=0
WGS_WAS_ACTIVE=0
WGS_WAS_ENABLED=0
WGS_UNIT_EXISTED=0
WGS_BINARY_EXISTED=0
WGS_START_ATTEMPTED=0
WGS_DISTRO=
WGS_FAMILY=
WGS_MANAGER=
WGS_IP_PACKAGE=
WGS_MAJOR=

wgs_message() { printf '%s\n' "$*"; }
wgs_fail() { printf 'Error: %s\n' "$*" >&2; return 1; }
wgs_usage() {
  cat <<'HELP'
WireGuard Simple — Linux installation
Debian / Ubuntu / RHEL / CentOS Stream / AlmaLinux / Rocky Linux / Fedora

Usage:
  sudo bash install.sh [APPLICATION_FILE] [options]

Options:
  --binary FILE        Local Linux application to install
  --interface NAME     WireGuard interface for a fresh install (default: wg0)
  --no-start           Install files without starting/enabling the service
  --no-deps            Do not automatically install missing packages
  --check              Check environment and file only; make no changes
  --help              Show this help

Default: install missing tools, install the panel, and start it.
Existing clients, keys, and service configuration are preserved on upgrade.
VPN interfaces, NAT, forwarding, DNS, and firewall rules are not changed.
HELP
}
wgs_arguments() {
  while (($#)); do
    case $1 in
      --binary) [[ $# -ge 2 && -z $WGS_BINARY ]] || { wgs_fail '--binary requires exactly one file'; return 1; }; WGS_BINARY=$2; shift 2 ;;
      --interface) [[ $# -ge 2 ]] || { wgs_fail '--interface requires a name'; return 1; }; WGS_INTERFACE=$2; shift 2 ;;
      --no-start) WGS_START=0; shift ;;
      --no-deps) WGS_DEPS=0; shift ;;
      --check) WGS_CHECK=1; shift ;;
      --help|-h) wgs_usage; return 2 ;;
      --*) wgs_fail "Unknown option: $1"; return 1 ;;
      *) [[ -z $WGS_BINARY ]] || { wgs_fail 'Specify only one application file'; return 1; }; WGS_BINARY=$1; shift ;;
    esac
  done
  [[ $WGS_INTERFACE =~ ^[a-zA-Z0-9][a-zA-Z0-9_-]{0,14}$ ]] || { wgs_fail 'Invalid interface name'; return 1; }
}
wgs_arch() {
  case $(uname -m) in x86_64|amd64) printf amd64 ;; aarch64|arm64) printf arm64 ;; *) wgs_fail 'Supported architectures: amd64 and arm64'; return 1 ;; esac
}
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
wgs_platform() {
  [[ $(uname -s) == Linux ]] || { wgs_fail 'This installer is for Linux servers only; it does not run on Windows'; return 1; }
  if [[ -r /proc/sys/kernel/osrelease ]] && grep -qi microsoft /proc/sys/kernel/osrelease; then
    wgs_fail 'Do not run this installer on WSL; use the target Linux server'; return 1
  fi
  [[ -f /etc/os-release ]] || { wgs_fail 'Could not determine the Linux distribution'; return 1; }
  local ID= VERSION_ID= NAME=
  source /etc/os-release
  wgs_distribution "$ID" "$VERSION_ID" "$NAME" || return 1
  if [[ -e /run/ostree-booted ]]; then wgs_fail 'Immutable/ostree systems require a separate deployment method'; return 1; fi
  wgs_package_manager || return 1
  command -v systemctl >/dev/null 2>&1 || { wgs_fail 'systemd/systemctl is required'; return 1; }
  [[ -d /run/systemd/system ]] || { wgs_fail 'A Linux server running systemd is required'; return 1; }
  wgs_arch >/dev/null
}
wgs_validate_binary() {
  local arch=$1 magic machine
  [[ -f $WGS_BINARY && -r $WGS_BINARY ]] || { wgs_fail 'Application file not found'; return 1; }
  magic=$(od -An -tx1 -N6 "$WGS_BINARY" | tr -d ' \n')
  [[ $magic == 7f454c460201 ]] || { wgs_fail 'File is not a 64-bit Linux ELF application'; return 1; }
  machine=$(od -An -tx1 -j18 -N2 "$WGS_BINARY" | tr -d ' \n')
  case "$arch:$machine" in amd64:3e00|arm64:b700) ;; *) wgs_fail 'Application architecture does not match the server'; return 1 ;; esac
}
wgs_dependencies() {
  local -a packages=()
  if ! command -v wg >/dev/null 2>&1 || ! command -v wg-quick >/dev/null 2>&1; then packages+=(wireguard-tools); fi
  command -v ip >/dev/null 2>&1 || packages+=("$WGS_IP_PACKAGE")
  command -v curl >/dev/null 2>&1 || packages+=(curl)
  command -v modprobe >/dev/null 2>&1 || packages+=(kmod)
  if ((${#packages[@]} == 0)); then return 0; fi
  packages+=(ca-certificates)
  if ((WGS_CHECK)); then wgs_message "Missing packages via $WGS_MANAGER: ${packages[*]} (check only)"; return 0; fi
  ((WGS_DEPS)) || { wgs_fail "Missing packages: ${packages[*]}. Install them yourself or omit --no-deps"; return 1; }
  wgs_message "Installing required packages via $WGS_MANAGER: ${packages[*]}"
  if [[ $WGS_FAMILY == debian ]]; then
    apt-get update || return 1
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends "${packages[@]}" || return 1
  elif ! "$WGS_MANAGER" install -y --setopt=install_weak_deps=False "${packages[@]}"; then
    wgs_fail 'Package installation failed. Check enabled distribution repositories and RHEL subscription access. Extra repositories are never enabled automatically.'; return 1
  fi
  local tool
  for tool in wg wg-quick ip curl modprobe; do
    command -v "$tool" >/dev/null 2>&1 || { wgs_fail "Required tool is still unavailable after installation: $tool"; return 1; }
  done
}
wgs_kernel_loaded() { [[ -d /sys/module/wireguard ]]; }
wgs_kernel() {
  wgs_kernel_loaded && return 0
  if ! command -v modprobe >/dev/null 2>&1; then
    if ((WGS_CHECK)); then wgs_message 'WireGuard kernel check pending: kmod must be installed first.'; return 0; fi
    wgs_fail 'modprobe is required to check WireGuard kernel support'; return 1
  fi
  # Dry run only: do not create an interface, load modules, or alter routes.
  if modprobe --dry-run --quiet wireguard; then return 0; fi
  wgs_fail 'The running kernel has no usable WireGuard module. Prepare a WireGuard-capable kernel/module (especially on EL8) and retry. No external repository or kernel replacement is performed.'
}
wgs_write_unit() {
  cat > "$WGS_UNIT_DIR/wgsimple.service" <<UNIT
[Unit]
Description=WireGuard Simple management console
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
UMask=0077
ExecStart=$WGS_BIN_DIR/wgsimple --mode live --listen 0.0.0.0:8787 --allow-http --data-dir $WGS_STATE_DIR --interface $WGS_INTERFACE
Restart=on-failure
RestartSec=5
WorkingDirectory=$WGS_STATE_DIR
Environment=GOMEMLIMIT=128MiB
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
PrivateDevices=true
ProtectKernelModules=true
ProtectKernelLogs=true
ProtectControlGroups=true
RestrictSUIDSGID=true
LockPersonality=true
RestrictRealtime=true
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK
CapabilityBoundingSet=CAP_NET_ADMIN
ReadWritePaths=$WGS_STATE_DIR $WGS_CONFIG_DIR
MemoryHigh=128M
MemoryMax=192M
TasksMax=64
StandardOutput=null
StandardError=null

[Install]
WantedBy=multi-user.target
UNIT
  chmod 0644 "$WGS_UNIT_DIR/wgsimple.service"
}
wgs_ready() {
  local attempt
  for attempt in {1..20}; do
    if systemctl is-active --quiet wgsimple.service && curl --noproxy '*' --fail --silent --max-time 2 http://127.0.0.1:8787/healthz >/dev/null 2>&1; then return 0; fi
    sleep 1
  done
  wgs_fail 'Panel health check failed; restoring the previous installation. Use --no-start for custom service settings'
}
wgs_replace_binary() {
  local source=$1 staged
  staged=$(mktemp "$WGS_BIN_DIR/.wgsimple.XXXXXX")
  if ! install -m 0755 "$source" "$staged"; then rm -f -- "$staged"; return 1; fi
  if ! mv -f -- "$staged" "$WGS_BIN_DIR/wgsimple"; then rm -f -- "$staged"; return 1; fi
  wgs_restore_labels "$WGS_BIN_DIR/wgsimple"
}
wgs_restore_labels() {
  if command -v restorecon >/dev/null 2>&1; then restorecon -- "$@" || return 1; fi
  return 0
}
wgs_rollback() {
  set +e
  local failed=0
  wgs_message 'Installation failed; restoring the previous application and service state.' >&2
  if ((WGS_WAS_ACTIVE || WGS_START_ATTEMPTED)); then
    if ! systemctl stop wgsimple.service >/dev/null 2>&1; then wgs_message "Could not stop the service. Backups preserved: $WGS_TEMP" >&2; return 1; fi
  fi
  if ((WGS_BINARY_EXISTED)); then wgs_replace_binary "$WGS_TEMP/previous-binary" || failed=1; else rm -f -- "$WGS_BIN_DIR/wgsimple" || failed=1; fi
  if ((WGS_UNIT_EXISTED)); then cp -p -- "$WGS_TEMP/previous-unit" "$WGS_UNIT_DIR/wgsimple.service" || failed=1; else systemctl disable wgsimple.service >/dev/null 2>&1 || true; rm -f -- "$WGS_UNIT_DIR/wgsimple.service" || failed=1; fi
  if [[ -f $WGS_TEMP/previous-control.db ]]; then
    rm -f -- "$WGS_STATE_DIR/control.db-journal" "$WGS_STATE_DIR/control.db-wal" "$WGS_STATE_DIR/control.db-shm" || return 1
    local suffix
    for suffix in '' -journal -wal -shm; do
      if [[ -f $WGS_TEMP/previous-control.db$suffix ]]; then install -m 0600 "$WGS_TEMP/previous-control.db$suffix" "$WGS_STATE_DIR/control.db$suffix" || failed=1; fi
    done
  fi
  systemctl daemon-reload || failed=1
  if ((WGS_WAS_ENABLED)); then systemctl enable wgsimple.service >/dev/null 2>&1 || failed=1; elif ((WGS_UNIT_EXISTED)); then systemctl disable wgsimple.service >/dev/null 2>&1 || failed=1; fi
  if ((WGS_WAS_ACTIVE)); then systemctl start wgsimple.service || failed=1; fi
  if ((failed)); then wgs_message "Rollback could not complete. Backups preserved: $WGS_TEMP" >&2; return 1; fi
  return 0
}
wgs_finish() {
  local status=$?
  trap - EXIT
  if ((status != 0 && WGS_ROLLBACK)); then if ! wgs_rollback; then exit "$status"; fi; fi
  if [[ -n $WGS_TEMP && -d $WGS_TEMP && ! -L $WGS_TEMP ]]; then
    case $(cd "$WGS_TEMP" && pwd -P) in /tmp/wgsimple-install.*) rm -rf -- "$WGS_TEMP" ;; *) printf '%s\n' 'Temporary directory was not automatically cleaned up.' >&2 ;; esac
  fi
  exit "$status"
}
wgs_install() {
  local destination=$WGS_BIN_DIR/wgsimple unit=$WGS_UNIT_DIR/wgsimple.service
  [[ ! -L $destination && ! -L $unit && ! -L $WGS_STATE_DIR && ! -L $WGS_CONFIG_DIR ]] || { wgs_fail 'Symbolic link detected in installation targets; no automatic changes made'; return 1; }
  if systemctl is-active --quiet wgsimple.service; then WGS_WAS_ACTIVE=1; fi
  if systemctl is-enabled --quiet wgsimple.service; then WGS_WAS_ENABLED=1; fi
  if [[ -f $destination ]]; then WGS_BINARY_EXISTED=1; cp -p -- "$destination" "$WGS_TEMP/previous-binary"; fi
  if [[ -f $unit ]]; then WGS_UNIT_EXISTED=1; cp -p -- "$unit" "$WGS_TEMP/previous-unit"; fi
  WGS_ROLLBACK=1
  if ((WGS_WAS_ACTIVE)); then systemctl stop wgsimple.service; fi
  local suffix
  for suffix in '' -journal -wal -shm; do
    if [[ -f $WGS_STATE_DIR/control.db$suffix ]]; then cp -p -- "$WGS_STATE_DIR/control.db$suffix" "$WGS_TEMP/previous-control.db$suffix"; fi
  done
  install -d -m 0755 "$WGS_BIN_DIR" "$WGS_UNIT_DIR"
  install -d -m 0700 "$WGS_STATE_DIR" "$WGS_CONFIG_DIR"
  wgs_replace_binary "$WGS_BINARY"
  if ((WGS_UNIT_EXISTED == 0)); then wgs_write_unit; else wgs_message 'Existing service configuration preserved.'; fi
  wgs_restore_labels "$unit" "$WGS_STATE_DIR" "$WGS_CONFIG_DIR"
  systemctl daemon-reload
  if ((WGS_START)); then WGS_START_ATTEMPTED=1; systemctl enable --now wgsimple.service; wgs_ready; fi
  WGS_ROLLBACK=0
  wgs_message ''
  wgs_message 'WireGuard Simple installed.'
  if ((WGS_START)); then
    wgs_message 'Panel ready: http://127.0.0.1:8787'
    if ((WGS_UNIT_EXISTED == 0)); then
      local panel_ip
      panel_ip=$(ip -o -4 addr show scope global 2>/dev/null | awk '{split($4,a,"/"); print a[1]; exit}') || panel_ip=
      if [[ $panel_ip =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        wgs_message "Panel ready: http://$panel_ip:8787"
      else
        wgs_message 'Server IP could not be detected. Use http://SERVER_IP:8787 or the SSH tunnel.'
      fi
      wgs_message 'Public HTTP is unencrypted. Use HTTPS on untrusted networks. Restrict access in Settings > Panel access.'
      wgs_message 'External access requires TCP 8787 in your server/provider firewall. Behind NAT, use the mapped public IP; local address detection cannot discover it.'
    else
      wgs_message 'Existing listener and panel access policy preserved; check the service configuration for external access.'
    fi
  else wgs_message 'To start the panel: sudo systemctl enable --now wgsimple.service'; fi
  wgs_message 'From your own computer: ssh -L 8787:127.0.0.1:8787 USER@SERVER_IP'
  wgs_message "If a setup token exists, read it privately on the server: sudo cat $WGS_STATE_DIR/setup-token"
  wgs_message 'Client data is preserved; traffic history and log preferences are unchanged.'
  wgs_message 'NAT, DNS, forwarding, firewall rules, and VPN interfaces were not changed.'
}
wgs_main() {
  local result=0 arch script_dir
  wgs_arguments "$@" || result=$?
  if ((result == 2)); then return 0; elif ((result != 0)); then return "$result"; fi
  export PATH=/usr/sbin:/usr/bin:/sbin:/bin
  wgs_platform
  arch=$(wgs_arch)
  if [[ -z $WGS_BINARY ]]; then
    script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
    if [[ -f $script_dir/wgsimple ]]; then WGS_BINARY=$script_dir/wgsimple; else WGS_BINARY=$script_dir/../bin/wgsimple-linux-$arch; fi
  fi
  wgs_validate_binary "$arch"
  if ((WGS_CHECK)); then wgs_dependencies; wgs_kernel; wgs_message "Check complete: $WGS_DISTRO / $arch / $WGS_INTERFACE. No changes made."; return 0; fi
  [[ $EUID -eq 0 ]] || { wgs_fail 'Run with sudo on the target Linux server'; return 1; }
  umask 077
  WGS_TEMP=$(mktemp -d /tmp/wgsimple-install.XXXXXX)
  trap wgs_finish EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
  wgs_dependencies
  wgs_kernel
  wgs_install
}

if [[ ${BASH_SOURCE[0]} == "$0" ]]; then wgs_main "$@"; fi
