# WireGuard Simple v0.3.3

Fresh installations now prepare IPv4 VPN internet access instead of installing only the panel. A dedicated root bootstrap starts the managed WireGuard interface, enables IPv4 forwarding and installs subnet/interface-scoped NAT through the detected IPv4 uplink, including after boot. The panel keeps its existing service restrictions.

- Existing keys, administrator credentials and client expiry settings are preserved. New clients remain enabled by default; an elapsed expiry intentionally disables a client.
- Existing installations retain their networking. Use `--configure-network` to opt in, or `--no-network` for administrator-managed setup.
- Active firewalld/UFW, custom filtering, multiple uplinks and conflicting configurations stop automatic preparation with an actionable error. Existing firewall rules are never flushed.
- Provider firewall UDP access and public endpoint configuration remain explicit requirements.
- Added regression checks for NAT scope, firewall conflicts, default-route ambiguity and installer network-bootstrap failures.

Ubuntu 26 ARM64 testing includes real WireGuard traffic, internet ping, DNS and HTTPS through a temporary same-host client namespace. External-device connectivity, complete machine reboot recovery and all-distribution firewall behavior are not yet fully validated.
