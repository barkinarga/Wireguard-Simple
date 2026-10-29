# WireGuard Simple v0.4.9

Fix first network setup on CentOS cloud servers where NetworkManager explicitly binds the uplink to firewalld's default zone only at runtime.

- Treat only that redundant default-zone uplink binding as equivalent to an unbound permanent interface. Keep refusing real runtime-only ports, services, rules, non-default zone assignments and conflicting permanent bindings.
- Preserve the runtime interface binding instead of saving unrelated runtime state. Keep the checked reload and native scoped zone/policy setup.
- Save managed-network intent before setup so a normal retry recreates its network service after a failed first installation, while preserving partial application state. Existing manual-network installations remain unchanged.
- Add regressions for default-zone interface equivalence and rejected permission differences, a real firewalld test with runtime-only uplink binding, and installer recovery from a failed first network setup.

Existing minimal-distro executable-header tests, CentOS Stream 10 missing-tar coverage and real UFW/firewalld packet/reload tests remain release gates.

Also verified on a CentOS Stream 10 ARM64 cloud VM: normal retry after partial setup, active panel/network services, external panel HTTPS, a temporary namespace WireGuard client with real Internet ping/DNS/HTTPS, and preserved server VPN identity and SSH/default-zone binding. Temporary peers and namespaces were removed after validation.
