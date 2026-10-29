#!/usr/bin/env bash
# Pure dependency decisions: no packages, services or system paths are changed.
set -euo pipefail
ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
source "$ROOT/scripts/dependencies.sh"
wgs_is_root() { return 0; }
wgs_ca_available() { return 0; }
prepare() {
  WGS_FAMILY=rhel; WGS_MANAGER=dnf; WGS_IP_PACKAGE=iproute
  WGS_CHECK=0; WGS_DEPS=1; WGS_NETWORK=auto
  ready=0; calls=(); scenario=retry; attempts=0
  command() {
    if [[ ${1:-} == -v ]]; then
      case $2 in
        tar|gzip) ((ready)) ;;
        bsdtar) [[ $scenario == bsdtar ]] ;;
        *) return 0 ;;
      esac
      return
    fi
    builtin command "$@"
  }
  dnf() {
    calls+=("$*")
    if [[ $1 == makecache ]]; then [[ $scenario != failure ]]; return; fi
    attempts=$((attempts + 1))
    if [[ $scenario == failure ]]; then return 1; fi
    if [[ $scenario == retry && $attempts == 1 ]]; then return 1; fi
    if [[ $scenario != lying-manager ]]; then ready=1; fi
  }
}
(
  prepare
  wgs_dependencies >/dev/null 2>&1
  [[ ${#calls[@]} == 3 && $ready == 1 && $WGS_EXTRACTOR == tar ]]
  [[ ${calls[0]} == 'install -y --setopt=install_weak_deps=False tar gzip ca-certificates' ]]
  [[ ${calls[1]} == makecache && ${calls[2]} == "${calls[0]}" ]]
  [[ ${calls[*]} != *curl* ]]
)
printf '%s\n' 'PASS missing tar/gzip auto-install, metadata retry and existing curl preservation'
for mode in no-deps check; do
  (
    prepare
    if [[ $mode == check ]]; then WGS_CHECK=1; wgs_dependencies >/dev/null; else WGS_DEPS=0; if wgs_dependencies >/dev/null 2>&1; then exit 1; fi; fi
    [[ ${#calls[@]} == 0 && $ready == 0 ]]
  )
done
for scenario_name in failure lying-manager; do
  (
    prepare; scenario=$scenario_name
    if wgs_dependencies >/dev/null 2>&1; then printf 'Unsafe dependency success: %s\n' "$scenario"; exit 1; fi
    [[ $ready == 0 ]]
  )
done
printf '%s\n' 'PASS check/no-deps are read-only; repository failures and still-missing tools stop setup'
(
  prepare; scenario=bsdtar
  bsdtar() { calls+=("bsdtar $*"); }
  wgs_dependencies >/dev/null
  [[ ${#calls[@]} == 0 && $WGS_EXTRACTOR == bsdtar ]]
  wgs_extract_payload /fixture/payload.tar.gz /fixture/target
  [[ ${calls[0]} == 'bsdtar --extract --file /fixture/payload.tar.gz --directory /fixture/target --no-same-owner --no-same-permissions' ]]
)
printf '%s\n' 'PASS existing bsdtar safely replaces missing GNU tar/gzip'
(
  prepare; WGS_FAMILY=debian; WGS_MANAGER=apt-get
  apt-get() {
    calls+=("$*")
    if [[ $1 == update ]]; then [[ ${#calls[@]} != 1 ]]; return; fi
    attempts=$((attempts + 1)); if ((attempts == 1)); then return 1; fi
    ready=1
  }
  wgs_dependencies >/dev/null 2>&1
  [[ ${#calls[@]} == 4 && $ready == 1 ]]
  [[ ${calls[0]} == update && ${calls[2]} == update ]]
)
printf '%s\n' 'PASS apt refresh/install retry without disabling package verification'
