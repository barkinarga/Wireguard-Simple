WireGuard Simple v0.3.2 fixes new-interface startup on Ubuntu 26.04.1 ARM64.

- Start new managed interfaces with direct ip/wg commands instead of a wg-quick subprocess. This avoids a reproduced AppArmor child-profile transition failure under NoNewPrivileges.
- Preserve AppArmor, NoNewPrivileges, the capability bound, and the other supplied service restrictions.
- Remove an interface created by the current operation if configuration/address/link setup fails. Do not delete an interface when creation itself fails. Existing-interface updates continue through wg syncconf.
- Keep wg-quick-compatible configuration files for external startup. Fresh routed-subnet setups outside the server VPN network require external startup and import rather than silently missing routes.

Validation: Go tests and vet; live startup under the service restrictions on an Ubuntu 26.04.1 ARM64 KVM server; a real WireGuard handshake and three tunnel ICMP replies using a temporary client namespace on that host. The temporary client, key, interface, and namespace were removed afterward. External-device internet access, provider firewall traversal, reboot recovery, and other distribution live VPN paths remain unverified.

[Installation guide](https://github.com/barkinarga/Wireguard-Simple/blob/readme/INSTALL.md). Existing accounts, keys, listener settings, and panel access policy are preserved by the installer.
