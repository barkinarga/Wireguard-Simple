# WireGuard Simple v0.4.7

Configure supported active local firewalls during installation while retaining the provider-firewall reminders.

- Detect UFW/firewalld, preserve default policies and SSH rules, and add scoped IPv4 permissions for the actual panel TCP and saved WireGuard UDP ports. Loopback panels do not receive an external TCP permission. Disabled firewalls stay disabled.
- Add native UFW route permissions or a dedicated firewalld VPN zone/egress policy for the managed subnet and uplink. Keep NAT scoped in the application-owned nftables table.
- Preserve existing rules on repeat setup and roll back new additions when a firewall operation fails. Refuse mixed/custom/ambiguous configurations instead of resetting or bypassing them.
- Before the initial firewalld object reload, compare runtime/permanent configuration and refuse to discard runtime-only changes. Never run complete-reload or runtime-to-permanent.
- Keep manual provider-firewall warnings. --no-network opts out of automatic firewall and VPN network changes; --no-start does not change firewall state.

Tests cover manager detection, custom-rule refusal, native command scoping, existing permissions, rollback and installer behavior. Dedicated Linux container checks exercise real UFW/firewalld, WireGuard packets, panel HTTPS access, VPN forwarding/NAT and reload persistence before release publication.
