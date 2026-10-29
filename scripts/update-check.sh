# Embedded in the self-extracting installer; safe to source in isolated tests.
wgs_release_version() {
  [[ $1 =~ ^v?([0-9]{1,8})\.([0-9]{1,8})\.([0-9]{1,8})$ ]]
}
wgs_version_newer() {
  local left=${1#v} right=${2#v} i
  local -a a b
  wgs_release_version "$left" && wgs_release_version "$right" || return 1
  IFS=. read -r -a a <<< "$left"
  IFS=. read -r -a b <<< "$right"
  for i in 0 1 2; do
    if ((10#${a[i]} > 10#${b[i]})); then return 0; fi
    if ((10#${a[i]} < 10#${b[i]})); then return 1; fi
  done
  return 1
}
wgs_download() {
  curl --proto '=https' --proto-redir '=https' --fail --silent --show-error \
    --location --retry 2 --connect-timeout 10 --max-time 180 --output "$2" "$1"
}
wgs_installed_version() {
  [[ -x /usr/local/bin/wgsimple ]] || return 1
  /usr/local/bin/wgsimple --version 2>/dev/null
}
wgs_check_release() {
  local work=$1 bundled=$2; shift 2
  local arg redirect latest installed= checksum result
  local repo=https://github.com/barkinarga/Wireguard-Simple
  for arg in "$@"; do
    case $arg in --no-update-check|--check) return 0 ;; esac
  done
  command -v curl >/dev/null 2>&1 || {
    printf '%s\n' 'Release checking requires curl. Install curl or use --no-update-check for this embedded package.' >&2; return 1;
  }
  printf '%s\n' 'Checking the latest WireGuard Simple release...'
  # Resolve once, then pin BOTH downloads to the same release to avoid races.
  if ! redirect=$(curl --proto '=https' --fail --silent --show-error \
    --retry 2 --connect-timeout 10 --max-time 30 --output /dev/null \
    --write-out '%{redirect_url}' "$repo/releases/latest"); then
    printf '%s\n' 'Could not check GitHub. Installation unchanged. Retry later or use --no-update-check for the embedded package.' >&2; return 1
  fi
  latest=${redirect#"$repo/releases/tag/"}
  if [[ $redirect != "$repo/releases/tag/$latest" ]] || ! wgs_release_version "$latest"; then
    printf '%s\n' 'GitHub returned an unsupported release URL/version; nothing installed.' >&2; return 1
  fi
  # Only the default, argument-free run is a no-op when already current.
  # Explicit installer options must still run (for example --configure-network).
  if (($# == 0)); then
    installed=$(wgs_installed_version) || installed=
    if wgs_release_version "$installed" && ! wgs_version_newer "$latest" "$installed"; then
      printf 'WireGuard Simple %s is already up to date.\n\n' "$installed"
      wgs_panel_links
      return 10
    fi
  fi
  if ! wgs_version_newer "$latest" "$bundled"; then return 0; fi
  printf 'Downloading WireGuard Simple %s...\n' "$latest"
  if ! wgs_download "$repo/releases/download/$latest/wgsimple-install.run" "$work/latest.run" || \
     ! wgs_download "$repo/releases/download/$latest/SHA256SUMS" "$work/latest.sums"; then
    printf '%s\n' 'Update download failed; installation unchanged.' >&2; return 1
  fi
  # Select exactly one fixed filename; never trust paths from a remote sums file.
  checksum=$(awk '$2 == "wgsimple-install.run" && length($1) == 64 && $1 !~ /[^0-9a-f]/ { hash=$1; count++ } END { if (count == 1) print hash; else exit 1 }' "$work/latest.sums") || {
    printf '%s\n' 'Invalid release checksum file; nothing installed.' >&2; return 1;
  }
  if ! printf '%s  %s\n' "$checksum" "$work/latest.run" | sha256sum --check --status -; then
    printf '%s\n' 'Release checksum verification failed; nothing installed.' >&2; return 1
  fi
  # The child uses its verified payload directly; no recursive update checks.
  bash "$work/latest.run" --no-update-check "$@" || { result=$?; return "$result"; }
  return 11
}
