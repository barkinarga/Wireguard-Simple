#!/usr/bin/env bash
# No root paths or real services/interfaces/firewalls are changed.
set -euo pipefail
ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
source "$ROOT/scripts/reset.sh"
mkdir -p "$ROOT/.tools"
TEST_ROOT=$(mktemp -d "$ROOT/.tools/reset-tests.XXXXXXXX")
trap 'case $TEST_ROOT in "$ROOT"/.tools/reset-tests.*) rm -rf -- "$TEST_ROOT" ;; esac' EXIT
python3() { command python "$@"; }
setup_case() {
  CASE_DIR=$TEST_ROOT/$1
  mkdir -p "$CASE_DIR/state" "$CASE_DIR/etc" "$CASE_DIR/backups" "$CASE_DIR/bin"
  RESET_STATE=$CASE_DIR/state
  RESET_CONFIG=$CASE_DIR/etc/wg0.conf
  RESET_BINARY=$CASE_DIR/bin/wgsimple
  RESET_UNIT=$CASE_DIR/etc/wgsimple.service
  RESET_NETWORK_UNIT=$CASE_DIR/etc/wgsimple-network.service
  RESET_SYSCTL=$CASE_DIR/etc/90-wgsimple-internet.conf
  RESET_BACKUP_ROOT=$CASE_DIR/backups
  printf 'binary' > "$RESET_BINARY"
  printf 'PrivateKey = test-key\n' > "$RESET_CONFIG"
  printf 'Description=WireGuard Simple management console\nExecStart=/usr/local/bin/wgsimple --data-dir /var/lib/wgsimple --interface wg0\n' > "$RESET_UNIT"
  printf 'Description=WireGuard Simple managed IPv4 internet access\nExecStart=/usr/local/bin/wgsimple --data-dir /var/lib/wgsimple --interface wg0 --prepare-network\n' > "$RESET_NETWORK_UNIT"
  printf '# WireGuard Simple managed IPv4 internet forwarding\nnet.ipv4.ip_forward = 1\n' > "$RESET_SYSCTL"
  python3 - "$RESET_STATE/control.db" <<'PY'
import sqlite3,json,sys
db=sqlite3.connect(sys.argv[1])
db.execute('create table config (key text, value text)')
db.execute('insert into config values (?,?)',('server',json.dumps(dict(interface='wg0',publicKey='test-public'))))
db.commit()
PY
  printf 'private-test-state' > "$RESET_STATE/setup-token"
  printf 'another-vpn' > "$CASE_DIR/etc/wg-other.conf"
  touch "$CASE_DIR/live"
  ip() {
    printf 'ip %s\n' "$*" >> "$CASE_DIR/calls"
    case $* in 'link show dev wg0') [[ -f $CASE_DIR/live ]] ;; 'link delete dev wg0') rm -f "$CASE_DIR/live" ;; *) return 1 ;; esac
  }
  wg() {
    if [[ $1 == pubkey ]]; then cat >/dev/null; fi
    printf 'test-public\n'
  }
  nft() {
    printf 'nft %s\n' "$*" >> "$CASE_DIR/calls"
    case $* in
      '-j list tables') printf '%s\n' '{"nftables":[{"table":{"family":"ip","name":"wgsimple_internet"}}]}' ;;
      '-j list table ip wgsimple_internet') printf '%s\n' '{"nftables":[{"table":{"family":"ip","name":"wgsimple_internet","comment":"WireGuard Simple managed IPv4 egress"}}]}' ;;
      'delete table ip wgsimple_internet') : ;;
      *) printf 'Unsafe nft call\n' >&2; return 1 ;;
    esac
  }
  systemctl() { printf 'systemctl %s\n' "$*" >> "$CASE_DIR/calls"; if [[ $1 == is-active ]]; then return 1; fi; }
  flock() { return 0; }
}
(
  setup_case success
  reset_preflight
  reset_apply > "$CASE_DIR/output"
  [[ ! -e $RESET_STATE && ! -e $RESET_BINARY && ! -e $RESET_CONFIG && ! -e $RESET_UNIT && ! -e $RESET_NETWORK_UNIT && ! -e $RESET_SYSCTL ]]
  [[ $(cat "$CASE_DIR/etc/wg-other.conf") == another-vpn ]]
  backups=("$RESET_BACKUP_ROOT"/wgsimple-backup.*)
  [[ ${#backups[@]} == 1 && -f ${backups[0]}/state/control.db ]]
  [[ $(cat "${backups[0]}/state/setup-token") == private-test-state ]]
  if [[ $(uname -s) == Linux ]]; then [[ $(stat -c %a "${backups[0]}") == 700 ]]; fi
  ! grep -q 'sysctl\|ufw\|firewall-cmd\|ssh\|flush' "$CASE_DIR/calls"
  ! grep -q private-test-state "$CASE_DIR/output"
)
printf '%s\n' 'PASS private backup, reset, preserved other VPN and no global firewall changes'
(
  setup_case backup-failure
  reset_preflight
  cp() { return 1; }
  if reset_apply >/dev/null 2>&1; then printf 'Backup failure ignored\n' >&2; exit 1; fi
  [[ -f $RESET_STATE/control.db && -f $RESET_CONFIG && -f $RESET_BINARY ]]
  ! grep -q 'delete' "$CASE_DIR/calls"
)
printf '%s\n' 'PASS failed backup prevents deletion'
for refusal in different-live-key foreign-config custom-service custom-dropin unknown-nat malformed-nft; do
  (
    setup_case "$refusal"
    case $refusal in
      different-live-key) wg() { printf 'another-public\n'; } ;;
      foreign-config) wg() { if [[ $1 == pubkey ]]; then cat >/dev/null; printf 'another-public\n'; else printf 'test-public\n'; fi; } ;;
      custom-service) printf 'Description=Some other service\n' > "$RESET_UNIT" ;;
      custom-dropin) mkdir "$RESET_UNIT.d" ;;
      unknown-nat) nft() { if [[ $* == '-j list tables' ]]; then printf '%s\n' '{"nftables":[{"table":{"family":"ip","name":"wgsimple_internet"}}]}'; else printf '%s\n' '{"nftables":[{"table":{"family":"ip","name":"wgsimple_internet","comment":"someone else"}}]}'; fi; } ;;
      malformed-nft) nft() { printf '{invalid'; } ;;
    esac
    if reset_preflight >/dev/null 2>&1; then printf 'Unsafe reset accepted: %s\n' "$refusal" >&2; exit 1; fi
    [[ -f $RESET_STATE/control.db && -f $RESET_CONFIG && -f $RESET_BINARY ]]
    if [[ -f $CASE_DIR/calls ]]; then ! grep -q 'systemctl\|delete' "$CASE_DIR/calls"; fi
  )
done
printf '%s\n' 'PASS foreign identity/configuration, custom layouts and unowned/malformed firewall refusal'
