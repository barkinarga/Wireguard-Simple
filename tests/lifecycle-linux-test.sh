#!/usr/bin/env bash
# Destructive installation test, ONLY on a disposable GitHub-hosted VM.
# Real systemd services use a private network namespace; no host routes change.
set -euo pipefail
[[ ${GITHUB_ACTIONS:-} == true && ${RUNNER_ENVIRONMENT:-} == github-hosted && $EUID == 0 ]] || {
  echo 'A disposable GitHub-hosted Linux runner (root) is required' >&2; exit 1;
}
ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
candidate=$(realpath "${1:?candidate installer}")
previous=$(realpath "${2:?previous release installer}")
expected=${3:?candidate version}

if [[ ${4:-} != --inside ]]; then
  [[ ! -e /usr/local/bin/wgsimple && ! -e /var/lib/wgsimple &&
     ! -e /etc/systemd/system/wgsimple.service && ! -e /etc/systemd/system/wgsimple-network.service &&
     ! -e /etc/systemd/system/wgsimple.service.d && ! -e /etc/systemd/system/wgsimple-network.service.d &&
     ! -e /etc/wireguard/wg0.conf && ! -e /run/wgsimple-ci ]] || {
    echo 'Refusing to overwrite an existing installation' >&2; exit 1;
  }
  cleanup() {
    systemctl stop wgsimple.service wgsimple-network.service >/dev/null 2>&1 || true
    systemctl disable wgsimple.service wgsimple-network.service >/dev/null 2>&1 || true
    rm -f -- /usr/local/bin/wgsimple /etc/wireguard/wg0.conf /etc/systemd/system/wgsimple.service /etc/systemd/system/wgsimple-network.service
    rm -rf -- /var/lib/wgsimple /run/wgsimple-ci /etc/systemd/system/wgsimple.service.d /etc/systemd/system/wgsimple-network.service.d
    systemctl daemon-reload
    for ns in wgs-server wgs-internet wgs-client; do
      ip netns del "$ns" >/dev/null 2>&1 || true
    done
  }
  # Refuse a colliding namespace before installing the cleanup trap.
  for ns in wgs-server wgs-internet wgs-client; do
    ! ip netns list | awk '{print $1}' | grep -qx "$ns"
  done
  trap cleanup EXIT
  for ns in wgs-server wgs-internet wgs-client; do ip netns add "$ns"; ip -n "$ns" link set lo up; done
  ip link add wgs-wan type veth peer name wgs-uplink
  ip link set wgs-wan netns wgs-server
  ip link set wgs-uplink netns wgs-internet
  ip link add wgs-cwan type veth peer name wgs-cuplink
  ip link set wgs-cwan netns wgs-client
  ip link set wgs-cuplink netns wgs-internet
  ip -n wgs-internet link add br0 type bridge
  for link in wgs-uplink wgs-cuplink; do ip -n wgs-internet link set "$link" master br0; ip -n wgs-internet link set "$link" up; done
  ip -n wgs-internet link set br0 up
  ip -n wgs-internet addr add 198.18.0.1/24 dev br0
  ip -n wgs-internet addr add 203.0.113.9/32 dev lo
  ip -n wgs-server addr add 198.18.0.2/24 dev wgs-wan
  ip -n wgs-server link set wgs-wan up
  ip -n wgs-server route add default via 198.18.0.1
  ip -n wgs-client addr add 198.18.0.3/24 dev wgs-cwan
  ip -n wgs-client link set wgs-cwan up
  ip -n wgs-client route add default via 198.18.0.1
  install -d -m 0700 /run/wgsimple-ci
  for service in wgsimple wgsimple-network; do
    install -d -m 0755 "/etc/systemd/system/$service.service.d"
    printf '[Service]\nNetworkNamespacePath=/run/netns/wgs-server\n' > "/etc/systemd/system/$service.service.d/ci-network.conf"
  done
  ip netns exec wgs-server bash "$0" "$candidate" "$previous" "$expected" --inside
  exit
fi

[[ $(ip netns identify $$) == wgs-server && -d /run/wgsimple-ci ]] || exit 1
umask 077
work=/run/wgsimple-ci
trap 'echo "Lifecycle failed at line $LINENO (private installer output is not published)" >&2; systemctl show wgsimple.service wgsimple-network.service --property=ActiveState,SubState,Result,ExecMainStatus >&2' ERR
reset_install() {
  systemctl stop wgsimple.service wgsimple-network.service >/dev/null 2>&1 || true
  systemctl disable wgsimple.service wgsimple-network.service >/dev/null 2>&1 || true
  rm -f -- /usr/local/bin/wgsimple /etc/wireguard/wg0.conf /etc/systemd/system/wgsimple.service /etc/systemd/system/wgsimple-network.service
  rm -rf -- /var/lib/wgsimple
  ip link del wg0 >/dev/null 2>&1 || true
  nft delete table ip wgsimple_internet >/dev/null 2>&1 || true
  systemctl daemon-reload
  systemctl reset-failed wgsimple.service wgsimple-network.service 2>/dev/null || true
}
install_package() {
  # The package is unmodified. Disable only its remote latest-release lookup.
  bash "$1" --no-update-check > "$work/installer.log" 2>&1
  systemctl is-active --quiet wgsimple.service
  systemctl is-enabled --quiet wgsimple.service
  openssl x509 -in /var/lib/wgsimple/panel-tls.pem -out "$work/panel.crt"
}
api() { python3 "$ROOT/tests/lifecycle-api.py" "$1" "$work"; }
vpn() {
  ip -n wgs-client link del wgtest >/dev/null 2>&1 || true
  ip -n wgs-client link add wgtest type wireguard
  ip netns exec wgs-client wg setconf wgtest "$work/client-wg.conf" 2> "$work/vpn-errors.log"
  ip -n wgs-client addr add 10.64.0.2/32 dev wgtest
  ip -n wgs-client link set wgtest up
  ip -n wgs-client route replace 203.0.113.9/32 dev wgtest
  ip netns exec wgs-client ping -c 3 -W 3 203.0.113.9 >/dev/null
  wg show wg0 latest-handshakes | awk '$2 > 0 {ok=1} END {exit !ok}'
  echo 'PASS exported client configuration: actual WireGuard handshake and NAT traffic'
}

install_package "$candidate"
[[ $(/usr/local/bin/wgsimple --version) == "$expected" ]]
api create
vpn
echo 'PASS clean install of the exact candidate package and real systemd services'

# A separate clean installation of the previous published stable release is
# the upgrade baseline; this does not pretend that a reinstall is an upgrade.
reset_install
install_package "$previous"
old_version=$(/usr/local/bin/wgsimple --version)
[[ $old_version == $(< "${previous%/*}/version") ]]
[[ $old_version != "$expected" ]]
api create
vpn
old_hash=$(sha256sum /usr/local/bin/wgsimple | awk '{print $1}')
payload_line=$(awk '$0 == "__WGS_PAYLOAD__" { print NR + 1; exit }' "$candidate")
[[ $payload_line =~ ^[0-9]+$ ]]
tail -n +"$payload_line" "$candidate" | tar -xzOf - "wgsimple-$(dpkg --print-architecture)" > "$work/candidate-binary"
candidate_hash=$(sha256sum "$work/candidate-binary" | awk '{print $1}')
[[ $candidate_hash != "$old_hash" ]]

# Reject only the candidate binary at service startup. The real installer must
# restore the old binary/database/unit and start the old service successfully.
printf '#!/bin/sh\n[ "$(sha256sum /usr/local/bin/wgsimple | cut -d" " -f1)" != "%s" ]\n' "$candidate_hash" > "$work/allow-old-binary"
chmod 0700 "$work/allow-old-binary"
printf '[Service]\nRestart=no\nExecStartPre=/run/wgsimple-ci/allow-old-binary\n' > /etc/systemd/system/wgsimple.service.d/ci-failure.conf
systemctl daemon-reload
if bash "$candidate" --no-update-check > "$work/failed-update.log" 2>&1; then
  echo 'Candidate unexpectedly succeeded despite injected startup failure' >&2; exit 1
fi
[[ $(sha256sum /usr/local/bin/wgsimple | awk '{print $1}') == "$old_hash" ]]
for attempt in {1..20}; do
  if curl --noproxy '*' --cacert "$work/panel.crt" -fsS --max-time 2 https://127.0.0.1:8787/healthz >/dev/null 2>&1; then break; fi
  sleep 1
done
systemctl is-active --quiet wgsimple.service
api verify
vpn
echo 'PASS failed real update restored the previous running installation'
rm -f /etc/systemd/system/wgsimple.service.d/ci-failure.conf
systemctl daemon-reload
systemctl reset-failed wgsimple.service wgsimple-network.service 2>/dev/null || true
install_package "$candidate"
[[ $(/usr/local/bin/wgsimple --version) == "$expected" ]]
[[ $(sha256sum /usr/local/bin/wgsimple | awk '{print $1}') == "$candidate_hash" ]]
api verify
vpn
echo 'PASS previous stable release upgraded to the exact candidate package'
