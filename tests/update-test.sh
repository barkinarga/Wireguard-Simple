#!/usr/bin/env bash
# No real network, packages, services or installation paths are modified.
set -euo pipefail
ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
source "$ROOT/scripts/update-check.sh"
mkdir -p "$ROOT/.tools"
TEST_DIR=$(mktemp -d "$ROOT/.tools/update-tests.XXXXXX")
case $(cd "$TEST_DIR" && pwd -P) in "$ROOT"/.tools/update-tests.*) ;; *) exit 1 ;; esac
trap 'case $(cd "$TEST_DIR" && pwd -P) in "$ROOT"/.tools/update-tests.*) rm -rf -- "$TEST_DIR" ;; esac' EXIT

curl() {
  printf '%s\n' "$*" >> "$CASE_DIR/requests"
  local output= arg url=${!#} previous=
  for arg in "$@"; do
    if [[ $previous == --output ]]; then output=$arg; fi
    previous=$arg
  done
  if [[ $url == */releases/latest ]]; then
    if [[ $SCENARIO == unavailable ]]; then return 22; fi
    if [[ $SCENARIO == bad-url ]]; then printf 'https://example.invalid/releases/tag/v9.9.9'; return; fi
    printf 'https://github.com/barkinarga/Wireguard-Simple/releases/tag/v%s' "$LATEST"
  elif [[ $url == */releases/download/v"$LATEST"/wgsimple-install.run ]]; then
    [[ $SCENARIO != failed-download ]] || return 22
    cp "$CASE_DIR/fixture.run" "$output"
  elif [[ $url == */releases/download/v"$LATEST"/SHA256SUMS ]]; then
    local hash
    hash=$(sha256sum "$CASE_DIR/fixture.run"); hash=${hash%% *}
    if [[ $SCENARIO == corrupt ]]; then hash=$(printf '%064d' 0); fi
    printf '%s  wgsimple-install.run\n' "$hash" > "$output"
    if [[ $SCENARIO == duplicate ]]; then printf '%s  wgsimple-install.run\n' "$hash" >> "$output"; fi
  else printf 'Unexpected URL: %s\n' "$url" >&2; return 1
  fi
}
wgs_installed_version() { [[ -n $INSTALLED ]] || return 1; printf '%s\n' "$INSTALLED"; }
prepare() {
  CASE_DIR=$TEST_DIR/$1
  mkdir "$CASE_DIR"
  export CHILD_ARGS=$CASE_DIR/child-args
  SCENARIO=normal; LATEST=0.4.2; INSTALLED=0.4.0
  printf 'printf "%%s\\n" "$@" > "$CHILD_ARGS"\n' > "$CASE_DIR/fixture.run"
}
check_result() {
  local expected=$1; shift
  local result=0
  wgs_check_release "$CASE_DIR" 0.4.1 "$@" > "$CASE_DIR/output" 2>&1 || result=$?
  [[ $result == "$expected" ]] || { cat "$CASE_DIR/output"; printf 'Expected %s, got %s\n' "$expected" "$result"; exit 1; }
}

wgs_version_newer v1.10.0 1.9.99
! wgs_version_newer 1.2.3 v1.2.3
! wgs_version_newer 1.2.3 2.0.0
! wgs_release_version 'v1.2.3;id'
! wgs_release_version 'v1.2.3/../../other'
printf '%s\n' 'PASS numeric version ordering and release validation'

prepare update
check_result 11 --no-deps --interface wg1
grep -Fxq -- '--no-update-check' "$CHILD_ARGS"
grep -Fxq -- '--interface' "$CHILD_ARGS"
grep -Fxq -- 'wg1' "$CHILD_ARGS"
[[ $(wc -l < "$CASE_DIR/requests") -eq 3 ]]
printf '%s\n' 'PASS verified pinned update, option forwarding and recursion prevention'

for mode in corrupt duplicate failed-download bad-url unavailable; do
  prepare "$mode"; SCENARIO=$mode
  check_result 1
  [[ ! -e $CHILD_ARGS ]]
done
printf '%s\n' 'PASS corruption, duplicate sums, failed downloads and invalid/unavailable releases never execute'

for mode in current newer; do
  prepare "$mode"
  INSTALLED=$LATEST
  [[ $mode != newer ]] || INSTALLED=10.0.0
  check_result 10
  [[ $(wc -l < "$CASE_DIR/requests") -eq 1 && ! -e $CHILD_ARGS ]]
done
printf '%s\n' 'PASS current/newer installations are untouched without downgrading'

for option in --no-update-check --check; do
  prepare "skip-${option#--}"
  check_result 0 "$option"
  [[ ! -e $CASE_DIR/requests && ! -e $CHILD_ARGS ]]
done
prepare bundled
LATEST=0.4.1; INSTALLED=
check_result 0
[[ ! -e $CHILD_ARGS ]]
prepare explicit
LATEST=0.4.1; INSTALLED=0.4.1
check_result 0 --configure-network
prepare child-failure
printf 'exit 23\n' > "$CASE_DIR/fixture.run"
check_result 23
printf '%s\n' 'PASS check/offline modes, embedded fresh install, explicit repair and child failure propagation'
