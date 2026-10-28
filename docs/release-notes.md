WireGuard Simple v0.3.1 adds direct server-IP panel access on fresh installations and recoverable access controls.

- Fresh installations listen on all IPv4 interfaces at port 8787 and print loopback plus a detected server address. Existing service listeners are preserved on upgrade.
- Settings → Panel access offers public, loopback-only, and private-network modes, plus an optional IP/CIDR allowlist. Changes apply immediately; loopback remains available for recovery.
- A private panel-access.json file persists the policy. A stopped-service CLI reset clears only panel restrictions, preserving administrator credentials and VPN configuration.
- Unsaved panel access fields survive language changes and delayed settings responses.
- Authentication, setup tokens, CSRF checks, rate limits, and all 10 interface languages remain available.

Public HTTP is unencrypted. Use SSH tunneling or TLS on untrusted networks. Host/provider firewall rules must allow TCP 8787 for direct access. Address detection cannot discover an upstream NAT public address. No firewall, NAT, DNS, forwarding, or VPN interface rules are changed by the installer.

[Installation, upgrade, and recovery guide](https://github.com/barkinarga/Wireguard-Simple/blob/readme/INSTALL.md).

Automated tests cover access rules, authentication, CSRF, host/origin rejection, persistence/recovery, and mocked installation. Real VPN connectivity and full target-system networking validation remain outstanding.
