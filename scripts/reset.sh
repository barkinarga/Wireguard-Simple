#!/usr/bin/env bash
# Reset the default installation for a fresh install, with a private backup.
# Safe to source for isolated tests. Never run automatically during an update.
set -euo pipefail

reset_fail() { printf 'Error: %s\n' "$*" >&2; return 1; }
reset_paths() {
  RESET_STATE=/var/lib/wgsimple
  RESET_CONFIG=/etc/wireguard/wg0.conf
  RESET_BINARY=/usr/local/bin/wgsimple
  RESET_UNIT=/etc/systemd/system/wgsimple.service
  RESET_NETWORK_UNIT=/etc/systemd/system/wgsimple-network.service
  RESET_SYSCTL=/etc/sysctl.d/90-wgsimple-internet.conf
  RESET_BACKUP_ROOT=/root
}
reset_platform() {
  [[ $(uname -s) == Linux && $EUID -eq 0 ]] || { reset_fail 'Run with sudo on the target Linux server.'; return 1; }
  local tool
  for tool in python3 systemctl ip wg nft flock readlink; do
    command -v "$tool" >/dev/null || { reset_fail "Required tool is missing: $tool"; return 1; }
  done
}
reset_preflight() {
  local path identity live configured unit
  for path in "$RESET_STATE" "$RESET_CONFIG" "$RESET_BINARY" "$RESET_UNIT" "$RESET_NETWORK_UNIT" "$RESET_SYSCTL" "$RESET_STATE/app.lock" "$RESET_STATE/control.db"; do
    [[ ! -L $path ]] || { reset_fail "Refusing a symbolic link: $path"; return 1; }
    [[ $(readlink -m -- "$path") == "$path" ]] || { reset_fail "Unexpected resolved path: $path"; return 1; }
  done
  for unit in "$RESET_UNIT" "$RESET_NETWORK_UNIT"; do
    [[ ! -d $unit.d ]] || { reset_fail 'Custom systemd drop-ins detected; reset this installation manually.'; return 1; }
    if [[ -e $unit ]]; then
      grep -q '^Description=WireGuard Simple ' "$unit" &&
        grep -Eq -- '--data-dir(=|[[:space:]])/var/lib/wgsimple([[:space:]]|$)' "$unit" || {
          reset_fail 'Custom or unrecognized service configuration; reset manually.'; return 1;
        }
      if grep -q -- '--interface' "$unit"; then
        grep -Eq -- '--interface(=|[[:space:]])wg0([[:space:]]|$)' "$unit" || { reset_fail 'Custom VPN interface; reset manually.'; return 1; }
      fi
      if grep -q -- '--config-dir' "$unit"; then
        grep -Eq -- '--config-dir(=|[[:space:]])/etc/wireguard([[:space:]]|$)' "$unit" || { reset_fail 'Custom VPN configuration directory; reset manually.'; return 1; }
      fi
    fi
  done
  RESET_OWN_INTERFACE=0
  RESET_PUBLIC_KEY=
  if [[ -f $RESET_STATE/control.db ]]; then
    # Read only the interface and public key; never print private keys.
    identity=$(python3 - "$RESET_STATE/control.db" <<'PY'
import json, sqlite3, sys
from pathlib import Path
db = sqlite3.connect(Path(sys.argv[1]).resolve().as_uri() + '?mode=ro', uri=True)
server = json.loads(db.execute("SELECT value FROM config WHERE key='server'").fetchone()[0])
if server['interface'] != 'wg0':
    raise SystemExit('Custom VPN interface; reset manually.')
print(server['publicKey'])
PY
    ) || { reset_fail 'Could not verify saved VPN ownership.'; return 1; }
    RESET_PUBLIC_KEY=$identity
    RESET_OWN_INTERFACE=1
  fi
  if ip link show dev wg0 >/dev/null 2>&1; then
    live=$(wg show wg0 public-key) || { reset_fail 'wg0 is not a recognized WireGuard interface.'; return 1; }
    [[ $RESET_OWN_INTERFACE == 1 && $live == "$RESET_PUBLIC_KEY" ]] || { reset_fail 'wg0 does not match the saved application identity; leaving it untouched.'; return 1; }
  fi
  if [[ -f $RESET_CONFIG ]]; then
    configured=$(awk -F= '{ key=$1; gsub(/[[:space:]]/,"",key); if(key=="PrivateKey"){value=substr($0,index($0,"=")+1);gsub(/[[:space:]]/,"",value);print value;exit} }' "$RESET_CONFIG" | wg pubkey) || {
      reset_fail 'Could not verify the saved WireGuard configuration.'; return 1;
    }
    [[ $RESET_OWN_INTERFACE == 1 && $configured == "$RESET_PUBLIC_KEY" ]] || { reset_fail 'wg0.conf is not owned by this application; leaving it untouched.'; return 1; }
  fi
  RESET_OWN_NAT=0
  local tables table has_table
  tables=$(nft -j list tables) || { reset_fail 'Could not inspect firewall tables.'; return 1; }
  has_table=$(python3 -c 'import json,sys;print("yes" if any(x.get("table",{}).get("family")=="ip" and x.get("table",{}).get("name")=="wgsimple_internet" for x in json.load(sys.stdin)["nftables"]) else "no")' <<< "$tables") || { reset_fail 'Invalid firewall table response.'; return 1; }
  if [[ $has_table == yes ]]; then
    table=$(nft -j list table ip wgsimple_internet) || return 1
    python3 -c 'import json,sys;sys.exit(0 if any(x.get("table",{}).get("comment")=="WireGuard Simple managed IPv4 egress" for x in json.load(sys.stdin)["nftables"]) else 1)' <<< "$table" || {
      reset_fail 'The reserved NAT table is not owned by WireGuard Simple.'; return 1;
    }
    RESET_OWN_NAT=1
  fi
}
reset_apply() {
  local unit path backup lock_fd
  for unit in wgsimple.service wgsimple-network.service; do
    systemctl stop "$unit" 2>/dev/null || {
      if systemctl is-active --quiet "$unit"; then reset_fail "Could not stop $unit"; return 1; fi
    }
    systemctl disable "$unit" >/dev/null 2>&1 || true
  done
  if [[ -d $RESET_STATE ]]; then
    exec {lock_fd}> "$RESET_STATE/app.lock"
    flock -n "$lock_fd" || { reset_fail 'Application data is still in use by another process.'; return 1; }
  fi
  umask 077
  backup=$(mktemp -d "$RESET_BACKUP_ROOT/wgsimple-backup.XXXXXXXX") || return 1
  for path in "$RESET_STATE" "$RESET_CONFIG" "$RESET_UNIT" "$RESET_NETWORK_UNIT" "$RESET_SYSCTL"; do
    if [[ -e $path ]]; then
      cp -a -- "$path" "$backup/$(basename -- "$path")" || { reset_fail 'Backup failed; application files have not been removed.'; return 1; }
    fi
  done
  printf 'Private backup: %s\n' "$backup"
  if ((RESET_OWN_INTERFACE)); then
    # Stop a legacy boot unit before removing only the verified app interface.
    systemctl stop wg-quick@wg0.service 2>/dev/null || true
    systemctl disable wg-quick@wg0.service >/dev/null 2>&1 || true
    if ip link show dev wg0 >/dev/null 2>&1; then ip link delete dev wg0; fi
  fi
  if ((RESET_OWN_NAT)); then nft delete table ip wgsimple_internet; fi
  if [[ -f $RESET_SYSCTL ]] && grep -q '^# WireGuard Simple managed' "$RESET_SYSCTL"; then rm -f -- "$RESET_SYSCTL"; fi
  rm -f -- "$RESET_BINARY" "$RESET_UNIT" "$RESET_NETWORK_UNIT"
  if ((RESET_OWN_INTERFACE)); then rm -f -- "$RESET_CONFIG"; fi
  rm -rf -- "$RESET_STATE"
  systemctl daemon-reload
  systemctl reset-failed wgsimple.service wgsimple-network.service >/dev/null 2>&1 || true
  printf '\nWireGuard Simple reset. You can run the normal installer again.\n'
  printf 'SSH, other VPNs, firewall permissions and installed packages were preserved.\n'
  printf 'Global IPv4 forwarding was not disabled because other services may use it.\n'
}
reset_main() {
  case ${1:-} in
    --help|-h) printf '%s\n' 'Reset the DEFAULT WireGuard Simple installation for a fresh install.' 'Stops this VPN, backs up app data/configuration privately under /root, then removes the app, wg0 configuration and owned NAT table.' 'Saved clients, accounts and TLS identity will not be used by the next installation.' 'Firewall permissions/packages are preserved. Custom paths/interfaces require manual removal.' 'Usage: sudo bash wgsimple-reset.sh [--yes]'; return 0 ;;
    --yes|'') ;;
    *) reset_fail 'Unknown option. Use --help.'; return 1 ;;
  esac
  (($# <= 1)) || { reset_fail 'Unexpected arguments.'; return 1; }
  reset_paths
  reset_platform
  reset_preflight
  if [[ ${1:-} != --yes ]]; then
    local answer
    printf 'This stops your VPN and resets all clients/accounts. A private backup will be saved.\n'
    read -r -p 'Type RESET to continue: ' answer
    [[ $answer == RESET ]] || { printf 'Cancelled. No changes made.\n'; return 1; }
  fi
  reset_apply
}

if [[ ${BASH_SOURCE[0]} == "$0" ]]; then reset_main "$@"; fi
