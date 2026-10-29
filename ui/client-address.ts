// A preview of the server allocator, never a reservation. Creation is validated
// again under the server's change lock, including changes from other sessions.
type PeerAddress = { address: string };
type Network = { ip: number; bits: number; start: number; end: number };
function ipv4(raw: string): number | null {
  if (!/^\d{1,3}(\.\d{1,3}){3}$/.test(raw)) return null;
  const parts = raw.split('.');
  if (parts.some(p => Number(p) > 255 || (p.length > 1 && p[0] === '0'))) return null;
  return parts.reduce((n, p) => n * 256 + Number(p), 0);
}
function network(raw: string): Network | null {
  const parts = raw.split('/');
  if (parts.length !== 2 || !/^(0|[1-9]\d?)$/.test(parts[1])) return null;
  const ip = ipv4(parts[0]), bits = Number(parts[1]);
  if (ip === null || bits > 32) return null;
  const size = 2 ** (32 - bits), start = Math.floor(ip / size) * size;
  return { ip, bits, start, end: start + size - 1 };
}
function format(ip: number): string { return [24, 16, 8, 0].map(shift => Math.floor(ip / 2 ** shift) % 256).join('.'); }
export function normalizeClientAddress(raw: string): string {
  const value = raw.trim();
  return ipv4(value) !== null ? `${value}/32` : value;
}
export function clientAddressPreview(server: string, peers: PeerAddress[]) {
  const subnet = network(server);
  if (!subnet) return { subnet: '', next: '' };
  const used = new Set([subnet.ip]);
  for (const peer of peers) {
    for (const raw of peer.address.split(',').map(v => v.trim()).filter(Boolean)) {
      const item = network(raw);
      if (!item || item.bits !== 32) return { subnet: `${format(subnet.start)}/${subnet.bits}`, next: '' };
      used.add(item.ip);
    }
  }
  let ip = subnet.start + 1;
  while (ip < subnet.end && used.has(ip)) ip++;
  return { subnet: `${format(subnet.start)}/${subnet.bits}`, next: ip < subnet.end ? `${format(ip)}/32` : '' };
}
export function clientAddressIssue(server: string, peers: PeerAddress[], raw: string) {
  if (!raw.trim()) {
    if (peers.some(peer => peer.address.split(',').some(raw => { const used = network(raw.trim()); return !used || used.bits !== 32; })))
      return 'Enter an IPv4 address in the VPN subnet, with /32.';
    return clientAddressPreview(server, peers).next ? '' : 'No available client address in the server subnet';
  }
  const subnet = network(server), candidate = network(normalizeClientAddress(raw));
  if (!subnet || !candidate || candidate.bits !== 32 || candidate.ip <= subnet.start || candidate.ip >= subnet.end)
    return 'Enter an IPv4 address in the VPN subnet, with /32.';
  if (candidate.ip === subnet.ip) return 'This address belongs to the VPN server.';
  for (const peer of peers) {
    for (const raw of peer.address.split(',').map(v => v.trim()).filter(Boolean)) {
      const used = network(raw);
      if (!used || candidate.ip >= used.start && candidate.ip <= used.end) return 'Client address is already in use';
    }
  }
  return '';
}
