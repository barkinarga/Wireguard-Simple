# WireGuard Simple v0.4.14

Permit panel access through the managed VPN by default, while preserving application access restrictions.

- Fix the case where Internet forwarding worked but the same public panel URL timed out inside the tunnel: WAN permissions did not cover host input on the VPN interface/zone.
- With an application-owned NAT table, add only the configured non-loopback panel TCP port from the saved IPv4 VPN subnet on the VPN interface. Support native UFW and the owned firewalld VPN zone, in runtime and permanent configuration, without reload or changes to SSH, unrelated services or default policies.
- Retain strict zone ownership/custom-rule checks, idempotence and rollback of only newly added rules. Loopback listeners and administrator-managed networking do not gain VPN panel permissions automatically.
- Keep Public access as the fresh default. This server only still rejects VPN requests; Private networks only accepts the default private VPN source. In both public/private modes, an IP allowlist remains an additional restriction. Authentication and forwarded-header protections are unchanged.

Validation: backend suite/vet; scoped port, ownership, idempotence and rollback regressions; VPN-source HTTP mode/allowlist/authentication cases. Native UFW/firewalld release tests probe both WAN and real WireGuard panel access, all access modes and IP/CIDR allowlists, a denied listening host service, and firewall reload persistence. The reported CentOS server was repaired using the same narrow rule without restarting its VPN/panel; the user confirmed VPN panel access works.
