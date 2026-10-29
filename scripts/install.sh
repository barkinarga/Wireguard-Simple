#!/usr/bin/env bash
# Target Linux server only. Safe to source for isolated tests.
set -euo pipefail
# Resolve the sibling module without dirname, which may not be installed yet.
wgs_script_dir=${BASH_SOURCE[0]%/*}
if [[ $wgs_script_dir == "${BASH_SOURCE[0]}" ]]; then wgs_script_dir=.; fi
source "$wgs_script_dir/dependencies.sh"

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
WGS_NETWORK=auto
WGS_NETWORK_NEW=0
WGS_QUICK_WAS_ENABLED=0
WGS_MAJOR=
WGS_FIREWALL_STATUS=

wgs_heading() {
  local color=$1 title=$2
  printf '\n'
  wgs_color "$color" "$title"
}
wgs_color() {
  local color=$1 title=$2
  if [[ -t 1 && ${TERM:-dumb} != dumb && ! -v NO_COLOR ]]; then
    printf '\033[1;%sm%s\033[0m\n' "$color" "$title"
  else
    printf '%s\n' "$title"
  fi
}
wgs_setup_token() {
  local token_file=$WGS_STATE_DIR/setup-token token
  # Print only the expected token format, never terminal control sequences.
  if [[ -f $token_file && ! -L $token_file && -r $token_file ]]; then
    token=$(< "$token_file")
    if [[ $token =~ ^[a-zA-Z0-9_-]+$ ]]; then
      wgs_color 36 "Setup token:  $token"
    fi
  fi
}
wgs_panel_links() {
  local command_line listen=127.0.0.1:8787 host port scheme=https panel_ip
  # Inspect effective service arguments; never execute or source their contents.
  command_line=$(systemctl show --property=ExecStart --value wgsimple.service 2>/dev/null) || command_line=
  if [[ -z $command_line && -r $WGS_UNIT_DIR/wgsimple.service ]]; then
    command_line=$(< "$WGS_UNIT_DIR/wgsimple.service")
  fi
  if [[ $command_line =~ --listen[=[:space:]]+([^[:space:]\;]+) ]]; then
    listen=${BASH_REMATCH[1]}
  fi
  if [[ $listen =~ ^(\[[0-9a-fA-F:]+\]|[a-zA-Z0-9._-]*):([0-9]+)$ ]]; then
    host=${BASH_REMATCH[1]}
    port=${BASH_REMATCH[2]}
  else
    wgs_message 'Panel:        See the service listen address.'
    return
  fi
  case $host in
    localhost|127.*|'[::1]')
      if [[ $command_line != *--tls-cert* ]]; then scheme=http; fi
      wgs_color 36 "Local:        $scheme://$host:$port"
      ;;
    ''|0.0.0.0|'[::]')
      panel_ip=$(ip -o -4 addr show scope global 2>/dev/null | awk '{split($4,a,"/"); print a[1]; exit}') || panel_ip=
      if [[ ! $panel_ip =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then panel_ip=SERVER_IP; fi
      wgs_color 36 "Panel:        https://$panel_ip:$port"
      wgs_color 36 "Local:        https://127.0.0.1:$port"
      ;;
    *) wgs_color 36 "Panel:        https://$host:$port" ;;
  esac
}
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
  --no-network         Leave VPN routing/firewall setup to the administrator
  --configure-network  Prepare IPv4 internet access on an existing installation
  --no-deps            Do not automatically install missing packages
  --check              Check environment and file only; make no changes
  --no-update-check    Use this embedded package without checking GitHub
  --help              Show this help

Default: install missing tools, install the panel, and start it.
Existing clients, keys, and service configuration are preserved on upgrade.
Fresh installations prepare IPv4 VPN startup, forwarding and NAT.
Supported active UFW/firewalld configurations receive scoped IPv4 permissions.
Use --no-network to leave all firewall and VPN routing changes to the administrator.
HELP
}
wgs_arguments() {
  while (($#)); do
    case $1 in
      --binary) [[ $# -ge 2 && -z $WGS_BINARY ]] || { wgs_fail '--binary requires exactly one file'; return 1; }; WGS_BINARY=$2; shift 2 ;;
      --interface) [[ $# -ge 2 ]] || { wgs_fail '--interface requires a name'; return 1; }; WGS_INTERFACE=$2; shift 2 ;;
      --no-network) WGS_NETWORK=off; shift ;;
      --configure-network) WGS_NETWORK=on; shift ;;
      --no-start) WGS_START=0; shift ;;
      --no-deps) WGS_DEPS=0; shift ;;
      --check) WGS_CHECK=1; shift ;;
      --no-update-check) shift ;;
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
$(if ((WGS_NETWORK_NEW)); then printf 'Requires=wgsimple-network.service\nAfter=wgsimple-network.service'; fi)

[Service]
Type=simple
User=root
UMask=0077
ExecStart=$WGS_BIN_DIR/wgsimple --mode live --listen 0.0.0.0:8787 --data-dir $WGS_STATE_DIR --interface $WGS_INTERFACE
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
wgs_network_unit() {
  cat > "$WGS_UNIT_DIR/wgsimple-network.service" <<UNIT
[Unit]
Description=WireGuard Simple managed IPv4 internet access
After=network-online.target systemd-sysctl.service ufw.service firewalld.service
Wants=network-online.target
Before=wgsimple.service
[Service]
Type=oneshot
UMask=0077
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
ExecStart=$WGS_BIN_DIR/wgsimple --mode live --data-dir $WGS_STATE_DIR --config-dir $WGS_CONFIG_DIR --interface $WGS_INTERFACE --prepare-network
RemainAfterExit=yes
[Install]
WantedBy=multi-user.target
UNIT
  chmod 0644 "$WGS_UNIT_DIR/wgsimple-network.service"
}
wgs_configure_firewall() {
  local command_line listen=127.0.0.1:8787 output
  command_line=$(systemctl show --property=ExecStart --value wgsimple.service 2>/dev/null) || command_line=
  if [[ -z $command_line && -r $WGS_UNIT_DIR/wgsimple.service ]]; then command_line=$(< "$WGS_UNIT_DIR/wgsimple.service"); fi
  if [[ $command_line =~ --listen[=[:space:]]+([^[:space:]\;]+) ]]; then listen=${BASH_REMATCH[1]}; fi
  if output=$("$WGS_BIN_DIR/wgsimple" --mode live --data-dir "$WGS_STATE_DIR" --config-dir "$WGS_CONFIG_DIR" --interface "$WGS_INTERFACE" --listen "$listen" --prepare-firewall 2>&1); then
    WGS_FIREWALL_STATUS=$output
  else
    WGS_FIREWALL_STATUS='Local firewall needs manual configuration.'
    wgs_message "Firewall: $output" >&2
  fi
}
wgs_ready() {
  local attempt
  for attempt in {1..20}; do
    if systemctl is-active --quiet wgsimple.service; then
      # Only a loopback health probe skips verification, never a download/login.
      if curl --noproxy '*' --insecure --fail --silent --max-time 2 https://127.0.0.1:8787/healthz >/dev/null 2>&1 || \
         curl --noproxy '*' --fail --silent --max-time 2 http://127.0.0.1:8787/healthz >/dev/null 2>&1; then return 0; fi
    fi
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
  if ((WGS_NETWORK_NEW)); then
    systemctl stop wgsimple-network.service >/dev/null 2>&1 || true
    systemctl disable wgsimple-network.service >/dev/null 2>&1 || true
    rm -f -- "$WGS_UNIT_DIR/wgsimple-network.service" || failed=1
  fi
  if ((WGS_QUICK_WAS_ENABLED)); then systemctl enable "wg-quick@$WGS_INTERFACE.service" >/dev/null 2>&1 || failed=1; fi
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
  local destination=$WGS_BIN_DIR/wgsimple unit=$WGS_UNIT_DIR/wgsimple.service marker=$WGS_STATE_DIR/network-managed
  [[ ! -L $destination && ! -L $unit && ! -L $WGS_STATE_DIR && ! -L $WGS_CONFIG_DIR && ! -L $WGS_UNIT_DIR/wgsimple-network.service && ! -L $marker ]] || { wgs_fail 'Symbolic link detected in installation targets; no automatic changes made'; return 1; }
  if systemctl is-active --quiet wgsimple.service; then WGS_WAS_ACTIVE=1; fi
  if systemctl is-enabled --quiet wgsimple.service; then WGS_WAS_ENABLED=1; fi
  if [[ -f $destination ]]; then WGS_BINARY_EXISTED=1; cp -p -- "$destination" "$WGS_TEMP/previous-binary"; fi
  if [[ -f $unit ]]; then WGS_UNIT_EXISTED=1; cp -p -- "$unit" "$WGS_TEMP/previous-unit"; fi
  local managed_retry=0
  if [[ -f $marker ]]; then
    [[ $(< "$marker") == 'WireGuard Simple managed network v1' ]] || { wgs_fail 'Unrecognized managed network marker; configure networking explicitly.'; return 1; }
    managed_retry=1
  elif [[ -e $marker ]]; then wgs_fail 'Managed network marker must be a regular file'; return 1; fi
  if [[ $WGS_NETWORK == on || ( $WGS_NETWORK == auto && ( $managed_retry == 1 || ( $WGS_UNIT_EXISTED == 0 && ! -e $WGS_STATE_DIR/control.db && ! -e $WGS_CONFIG_DIR/$WGS_INTERFACE.conf ) ) ) ]]; then
    if [[ ! -e $WGS_UNIT_DIR/wgsimple-network.service ]]; then WGS_NETWORK_NEW=1; fi
  fi
  WGS_ROLLBACK=1
  if ((WGS_WAS_ACTIVE)); then systemctl stop wgsimple.service; fi
  local suffix
  for suffix in '' -journal -wal -shm; do
    if [[ -f $WGS_STATE_DIR/control.db$suffix ]]; then cp -p -- "$WGS_STATE_DIR/control.db$suffix" "$WGS_TEMP/previous-control.db$suffix"; fi
  done
  install -d -m 0755 "$WGS_BIN_DIR" "$WGS_UNIT_DIR"
  install -d -m 0700 "$WGS_STATE_DIR" "$WGS_CONFIG_DIR"
  wgs_replace_binary "$WGS_BINARY"
  if ((WGS_UNIT_EXISTED == 0)); then wgs_write_unit; fi
  if ((WGS_NETWORK_NEW)); then wgs_network_unit; fi
  if [[ $WGS_NETWORK != off && ( $WGS_NETWORK_NEW == 1 || -f $WGS_UNIT_DIR/wgsimple-network.service ) ]]; then
    # Keep this intent across a failed first setup so a normal retry resumes it.
    printf '%s\n' 'WireGuard Simple managed network v1' > "$marker"
    chmod 0600 "$marker"
  fi
  wgs_restore_labels "$unit" "$WGS_STATE_DIR" "$WGS_CONFIG_DIR"
  systemctl daemon-reload
  if ((WGS_START)) && [[ $WGS_NETWORK != off && ( $WGS_NETWORK_NEW == 1 || -f $WGS_UNIT_DIR/wgsimple-network.service ) ]]; then
    if ! systemctl enable wgsimple-network.service || ! systemctl restart wgsimple-network.service; then
      wgs_message 'IPv4 network preparation failed. Read: journalctl -u wgsimple-network.service --no-pager' >&2
      wgs_fail 'Configure the active firewall explicitly, or use --no-network for administrator-managed routing.'; return 1
    fi
  fi
  if ((WGS_START)) && [[ $WGS_NETWORK != off && -f $WGS_UNIT_DIR/wgsimple-network.service ]]; then
    if systemctl is-enabled --quiet "wg-quick@$WGS_INTERFACE.service"; then
      WGS_QUICK_WAS_ENABLED=1
      systemctl disable "wg-quick@$WGS_INTERFACE.service"
    fi
  fi
  if ((WGS_START)) && [[ $WGS_NETWORK != off ]]; then wgs_configure_firewall; fi
  if ((WGS_START)); then WGS_START_ATTEMPTED=1; systemctl enable --now wgsimple.service; wgs_ready; fi
  WGS_ROLLBACK=0
  wgs_heading 32 '✓ WireGuard Simple ready'
  wgs_message ''
  if ((WGS_START)); then
    wgs_panel_links
  else
    wgs_message 'Start panel:  sudo systemctl enable --now wgsimple.service'
  fi
  wgs_setup_token
  wgs_message ''
  wgs_color 33 'Note: A browser certificate warning is expected.'
  wgs_message '      Verify the certificate fingerprint over SSH before accepting.'
  wgs_heading 33 'Firewall:'
  if [[ -n $WGS_FIREWALL_STATUS ]]; then wgs_message "  $WGS_FIREWALL_STATUS"; fi
  wgs_message '  Allow TCP 8787 in your server/provider firewall for panel access.'
  wgs_message '  Allow your WireGuard UDP port in the provider firewall (default: 51820).'
  wgs_message ''
}
wgs_main() {
  local result=0 arch script_dir
  wgs_arguments "$@" || result=$?
  if ((result == 2)); then return 0; elif ((result != 0)); then return "$result"; fi
  export PATH=/usr/sbin:/usr/bin:/sbin:/bin
  local -a bootstrap_options=()
  if ((WGS_CHECK)); then bootstrap_options+=(--check); fi
  if ((WGS_DEPS == 0)); then bootstrap_options+=(--no-deps); fi
  if [[ $WGS_NETWORK == off ]]; then bootstrap_options+=(--no-network); elif [[ $WGS_NETWORK == on ]]; then bootstrap_options+=(--configure-network); fi
  wgs_bootstrap_dependencies "${bootstrap_options[@]}"
  wgs_platform
  arch=$(wgs_arch)
  if [[ -z $WGS_BINARY ]]; then
    script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
    if [[ -f $script_dir/wgsimple ]]; then WGS_BINARY=$script_dir/wgsimple; else WGS_BINARY=$script_dir/../bin/wgsimple-linux-$arch; fi
  fi
  wgs_dependencies
  wgs_validate_binary "$arch"
  if ((WGS_CHECK)); then wgs_kernel; wgs_message "Check complete: $WGS_DISTRO / $arch / $WGS_INTERFACE. No changes made."; return 0; fi
  [[ $EUID -eq 0 ]] || { wgs_fail 'Run with sudo on the target Linux server'; return 1; }
  umask 077
  WGS_TEMP=$(mktemp -d /tmp/wgsimple-install.XXXXXX)
  trap wgs_finish EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
  wgs_kernel
  wgs_install
}

if [[ ${BASH_SOURCE[0]} == "$0" ]]; then wgs_main "$@"; fi
