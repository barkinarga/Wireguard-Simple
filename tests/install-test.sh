#!/usr/bin/env bash
# Isolated installer tests: all writes stay in the workspace; services are mocked.
set -euo pipefail
ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
source "$ROOT/scripts/install.sh"

mock_service() {
  local operation=$1; shift
  printf '%s %s\n' "$operation" "$*" >> "$CASE_DIR/service-calls"
  case $operation in
    is-active) [[ -f $CASE_DIR/active ]] ;;
    is-enabled) [[ -f $CASE_DIR/enabled ]] ;;
    stop) rm -f -- "$CASE_DIR/active" ;;
    start) touch "$CASE_DIR/active" ;;
    enable) touch "$CASE_DIR/enabled"; if [[ $* == *--now* ]]; then touch "$CASE_DIR/active"; fi ;;
    disable) rm -f -- "$CASE_DIR/enabled" ;;
    daemon-reload) : ;;
    *) printf 'Unexpected service operation: %s\n' "$operation" >&2; return 1 ;;
  esac
}
prepare_case() {
  CASE_DIR=$1
  case $(cd "$CASE_DIR" && pwd -P) in "$ROOT"/.tools/install-tests.*/*) ;; *) printf '%s\n' 'Unsafe test directory' >&2; exit 1 ;; esac
  WGS_BIN_DIR=$CASE_DIR/bin
  WGS_STATE_DIR=$CASE_DIR/state
  WGS_CONFIG_DIR=$CASE_DIR/wireguard
  WGS_UNIT_DIR=$CASE_DIR/systemd
  WGS_TEMP=$CASE_DIR/backup
  WGS_BINARY=$CASE_DIR/input-binary
  WGS_START=1
  mkdir -p "$WGS_BIN_DIR" "$WGS_STATE_DIR" "$WGS_CONFIG_DIR" "$WGS_UNIT_DIR" "$WGS_TEMP"
  printf new-binary > "$WGS_BINARY"
  systemctl() { mock_service "$@"; }
  wgs_ready() { [[ -f $CASE_DIR/active && -f $WGS_BIN_DIR/wgsimple && -f $WGS_UNIT_DIR/wgsimple.service ]]; }
  if [[ $(uname -s) != Linux ]]; then
    # NTFS permission semantics differ from Linux. Test file/service behavior
    # here without changing Windows ACLs; real Linux modes remain unverified.
    install() {
      printf 'install %s\n' "$*" >> "$CASE_DIR/filesystem-calls"
      local directory=0
      local -a operands=()
      while (($#)); do
        case $1 in -d) directory=1; shift ;; -m|-o|-g) shift 2 ;; --) shift ;; *) operands+=("$1"); shift ;; esac
      done
      if ((directory)); then command mkdir -p -- "${operands[@]}"; else command cp -- "${operands[0]}" "${operands[1]}"; fi
    }
    chmod() { printf 'chmod %s\n' "$*" >> "$CASE_DIR/filesystem-calls"; }
    cp() { if [[ ${1:-} == -p ]]; then shift; fi; command cp "$@"; }
  fi
}
assert_equal() { [[ $1 == "$2" ]] || { printf 'Expected [%s], got [%s]\n' "$2" "$1" >&2; exit 1; }; }

if [[ ${1:-} == --failed-update-worker ]]; then
  prepare_case "$2"
  printf old-binary > "$WGS_BIN_DIR/wgsimple"
  printf old-custom-unit > "$WGS_UNIT_DIR/wgsimple.service"
  printf old-control > "$WGS_STATE_DIR/control.db"
  touch "$CASE_DIR/active" "$CASE_DIR/enabled"
  wgs_ready() { printf changed-control > "$WGS_STATE_DIR/control.db"; printf stale-journal > "$WGS_STATE_DIR/control.db-journal"; return 1; }
  trap wgs_finish EXIT
  wgs_install
  exit 0
fi

mkdir -p "$ROOT/.tools"
TEST_DIR=$(mktemp -d "$ROOT/.tools/install-tests.XXXXXX")
case $(cd "$TEST_DIR" && pwd -P) in "$ROOT"/.tools/install-tests.*) ;; *) exit 1 ;; esac
trap 'if [[ -d $TEST_DIR && ! -L $TEST_DIR ]]; then case $(cd "$TEST_DIR" && pwd -P) in "$ROOT"/.tools/install-tests.*) rm -rf -- "$TEST_DIR" ;; esac; fi' EXIT

(
  WGS_BINARY=
  wgs_arguments --interface wg_test-1 --no-start --no-deps --check
  assert_equal "$WGS_INTERFACE:$WGS_START:$WGS_DEPS:$WGS_CHECK" 'wg_test-1:0:0:1'
)
for invalid in 'wg0;id' '../wg0' '-wg0' 'abcdefghijklmnop'; do
  if (wgs_arguments --interface "$invalid") >/dev/null 2>&1; then printf '%s\n' 'Unsafe interface accepted' >&2; exit 1; fi
done
if (wgs_arguments --binary one --binary two) >/dev/null 2>&1; then exit 1; fi
printf '%s\n' 'PASS arguments and injection rejection'

(
  uname() { printf x86_64; }; assert_equal "$(wgs_arch)" amd64
  uname() { printf aarch64; }; assert_equal "$(wgs_arch)" arm64
  uname() { printf armv7l; }; if wgs_arch >/dev/null 2>&1; then exit 1; fi
)
(
  WGS_BINARY=$ROOT/bin/wgsimple-linux-amd64; wgs_validate_binary amd64
  if wgs_validate_binary arm64 >/dev/null 2>&1; then exit 1; fi
  WGS_BINARY=$ROOT/bin/wgsimple-linux-arm64; wgs_validate_binary arm64
  WGS_BINARY=$ROOT/bin/wgsimple.exe; if wgs_validate_binary amd64 >/dev/null 2>&1; then exit 1; fi
)
printf '%s\n' 'PASS architecture selection and actual ELF/Windows validation'

(
  wgs_distribution debian 13 Debian
  dependency_log=$TEST_DIR/dependencies
  tools_ready=1
  apt-get() { printf '%s\n' "$*" >> "$dependency_log"; if [[ $1 == install ]]; then tools_ready=1; fi; }
  command() { if [[ ${1:-} == -v ]]; then ((tools_ready)); return; fi; builtin command "$@"; }
  wgs_dependencies
  [[ ! -e $dependency_log ]]
  tools_ready=0
  WGS_CHECK=1
  wgs_dependencies >/dev/null
  [[ ! -e $dependency_log ]]
  WGS_CHECK=0; WGS_DEPS=0
  if wgs_dependencies >/dev/null 2>&1; then exit 1; fi
  [[ ! -e $dependency_log ]]
  WGS_DEPS=1
  wgs_dependencies >/dev/null
  [[ $(wc -l < "$dependency_log") -eq 2 ]]
  grep -q -- '--no-install-recommends wireguard-tools iproute2 curl kmod ca-certificates' "$dependency_log"
)
printf '%s\n' 'PASS dependency detection, no-deps and read-only check (mock package manager)'

mkdir "$TEST_DIR/first"
(
  prepare_case "$TEST_DIR/first"
  wgs_install >/dev/null
  assert_equal "$(cat "$WGS_BIN_DIR/wgsimple")" new-binary
  [[ -f $CASE_DIR/active && -f $CASE_DIR/enabled ]]
  grep -q -- '--listen 127.0.0.1:8787' "$WGS_UNIT_DIR/wgsimple.service"
  grep -q 'CapabilityBoundingSet=CAP_NET_ADMIN' "$WGS_UNIT_DIR/wgsimple.service"
  [[ ! -e $WGS_STATE_DIR/telemetry.db ]]
)
printf '%s\n' 'PASS first installation and private service defaults'

mkdir "$TEST_DIR/update"
(
  prepare_case "$TEST_DIR/update"
  printf old-binary > "$WGS_BIN_DIR/wgsimple"
  printf custom-service > "$WGS_UNIT_DIR/wgsimple.service"
  printf keep-client-keys > "$WGS_STATE_DIR/control.db"
  printf keep-vpn-config > "$WGS_CONFIG_DIR/wg0.conf"
  touch "$CASE_DIR/active" "$CASE_DIR/enabled"
  wgs_install >/dev/null
  assert_equal "$(cat "$WGS_UNIT_DIR/wgsimple.service")" custom-service
  assert_equal "$(cat "$WGS_STATE_DIR/control.db")" keep-client-keys
  assert_equal "$(cat "$WGS_CONFIG_DIR/wg0.conf")" keep-vpn-config
  assert_equal "$(cat "$WGS_TEMP/previous-binary")" old-binary
  [[ -f $CASE_DIR/active ]]
)
printf '%s\n' 'PASS update preserves service, client database and VPN config'

mkdir "$TEST_DIR/no-start"
(
  prepare_case "$TEST_DIR/no-start"
  WGS_START=0
  wgs_install >/dev/null
  [[ ! -f $CASE_DIR/active && ! -f $CASE_DIR/enabled ]]
)
printf '%s\n' 'PASS no-start mode'

mkdir "$TEST_DIR/rollback"
set +e
bash "$0" --failed-update-worker "$TEST_DIR/rollback" > "$TEST_DIR/rollback-output" 2>&1
worker_status=$?
set -e
[[ $worker_status -ne 0 ]]
assert_equal "$(cat "$TEST_DIR/rollback/bin/wgsimple")" old-binary
assert_equal "$(cat "$TEST_DIR/rollback/systemd/wgsimple.service")" old-custom-unit
assert_equal "$(cat "$TEST_DIR/rollback/state/control.db")" old-control
[[ ! -f $TEST_DIR/rollback/state/control.db-journal ]]
[[ -f $TEST_DIR/rollback/active && -f $TEST_DIR/rollback/enabled ]]
printf '%s\n' 'PASS failed startup rolls back binary, database and previous running state'

mkdir "$TEST_DIR/symlink"
(
  prepare_case "$TEST_DIR/symlink"
  printf untouched > "$CASE_DIR/protected"
  ln -s "$CASE_DIR/protected" "$WGS_BIN_DIR/wgsimple"
  if [[ ! -L $WGS_BIN_DIR/wgsimple ]]; then printf '%s\n' 'SKIP native symlink creation unavailable in this shell'; exit 0; fi
  if wgs_install >/dev/null 2>&1; then exit 1; fi
  assert_equal "$(cat "$CASE_DIR/protected")" untouched
  printf '%s\n' 'PASS symbolic-link destination rejected'
)
