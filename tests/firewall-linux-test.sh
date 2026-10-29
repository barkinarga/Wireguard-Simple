#!/usr/bin/env bash
# Run only in a disposable privileged CI container with --network none.
set -euo pipefail
[[ ${WGS_FIREWALL_TEST_CONTAINER:-} == yes && -f /.dockerenv ]] || { echo 'Disposable firewall test container required'; exit 1; }
manager=${1:?ufw or firewalld}
binary=/workspace/bin/wgsimple-linux-amd64
export LC_ALL=C
mkdir -p /test/state /test/wireguard
ip link set lo up
ip netns add internet
ip netns add client
ip link add eth0 type veth peer name server-wan
ip link set server-wan netns internet
ip link add client-wan type veth peer name client-eth
ip link set client-wan netns internet
ip link set client-eth netns client
ip -n internet link add br0 type bridge
ip -n internet link set server-wan master br0
ip -n internet link set client-wan master br0
for link in lo br0 server-wan client-wan; do ip -n internet link set "$link" up; done
ip -n internet addr add 198.18.0.1/24 dev br0
ip -n internet addr add 203.0.113.9/32 dev lo
ip addr add 198.18.0.2/24 dev eth0
ip link set eth0 up
ip route add default via 198.18.0.1
ip -n client link set lo up
ip -n client addr add 198.18.0.3/24 dev client-eth
ip -n client link set client-eth up
ip -n client route add default via 198.18.0.1
if [[ $manager == ufw ]]; then
  ufw allow 22/tcp
  ufw --force enable
else
  mkdir -p /run/dbus
  dbus-daemon --system --fork
  firewalld --nofork --nopid > /test/firewalld.log 2>&1 &
  for attempt in {1..50}; do if firewall-cmd --state >/dev/null 2>&1; then break; fi; sleep .2; done
  firewall-cmd --state
  firewall-cmd --add-port=22/tcp
  firewall-cmd --permanent --add-port=22/tcp
  # A runtime-only rule must prevent initial zone creation and reload.
  firewall-cmd --add-port=12345/tcp
  if "$binary" --data-dir /test/state --config-dir /test/wireguard --prepare-network > /test/runtime-only.log 2>&1; then cat /test/runtime-only.log; exit 1; fi
  if ! grep -q 'runtime-only' /test/runtime-only.log; then cat /test/runtime-only.log; tail -n 20 /test/firewalld.log; exit 1; fi
  firewall-cmd --query-port=12345/tcp
  ! firewall-cmd --permanent --get-zones | tr ' ' '\n' | grep -qx wgsimple-vpn
  firewall-cmd --remove-port=12345/tcp
fi
prepare() { "$binary" --data-dir /test/state --config-dir /test/wireguard "$@"; }
prepare --prepare-network
prepare --listen 0.0.0.0:8787 --prepare-firewall
prepare --prepare-network # repeat without duplicate permissions
prepare --listen 0.0.0.0:8787 --prepare-firewall
if [[ $manager == ufw ]]; then
  ufw status | grep -q '22/tcp.*ALLOW'
  ufw status | grep -q '8787.*ALLOW'
  ufw status | grep -q '51820.*ALLOW'
else
  for scope in '' --permanent; do
    firewall-cmd $scope --zone=public --query-port=22/tcp
    firewall-cmd $scope --zone=public --query-port=8787/tcp
    firewall-cmd $scope --zone=public --query-port=51820/udp
    firewall-cmd $scope --zone=wgsimple-vpn --query-interface=wg0
  done
fi
# Add a real peer without exposing any test keys in logs.
umask 077
wg genkey > /test/client.key
wg pubkey < /test/client.key > /test/client.pub
python3 - <<'PY'
import sqlite3,json
from pathlib import Path
db=sqlite3.connect('/test/state/control.db')
server=json.loads(db.execute("select value from config where key='server'").fetchone()[0])
Path('/test/server.pub').write_text(server['publicKey'])
public=Path('/test/client.pub').read_text().strip()
peer=dict(id='integration',name='integration',address='10.64.0.2/32',publicKey=public,enabled=True,createdAt='2026-01-01T00:00:00Z')
db.execute('insert into peers(id,public_key,value) values(?,?,?)',('integration',public,json.dumps(peer)))
db.commit()
PY
prepare --prepare-network
echo "Client: create WireGuard interface"
ip -n client link add wgtest type wireguard
echo "Client: configure peer using a key pipe"
cat /test/client.key | ip netns exec client wg set wgtest private-key /dev/stdin peer "$(cat /test/server.pub)" allowed-ips 0.0.0.0/0 endpoint 198.18.0.2:51820 persistent-keepalive 1
ip -n client addr add 10.64.0.2/32 dev wgtest
ip -n client link set wgtest up
ip -n client route add 203.0.113.9/32 dev wgtest
ip netns exec client ping -c 3 -W 3 203.0.113.9
# Probe the actual panel from the external client namespace, not loopback.
"$binary" --data-dir /test/state --config-dir /test/wireguard --listen 0.0.0.0:8787 > /test/panel.log 2>&1 &
panel_pid=$!
trap 'kill "$panel_pid" 2>/dev/null || true' EXIT
for attempt in {1..30}; do
  if ip netns exec client curl --noproxy '*' -kfsS --max-time 1 https://198.18.0.2:8787/healthz > /test/health.json 2>/dev/null; then break; fi
  sleep .2
done
grep -q '"status":"ok"' /test/health.json
# Manager reloads must keep permissions and the independent scoped NAT table.
if [[ $manager == ufw ]]; then ufw reload; else firewall-cmd --reload; fi
ip netns exec client curl --noproxy '*' -kfsS --max-time 3 https://198.18.0.2:8787/healthz
ip netns exec client ping -c 2 -W 3 203.0.113.9
echo "PASS $manager: preserved SSH rule, panel access, real WireGuard handshake, forwarding, NAT, idempotence and firewall reload"
