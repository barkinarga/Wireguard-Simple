# WireGuard Simple v0.4.15

Fix missing automatic panel permissions on Ubuntu 22.04 with active UFW and nftables 1.0.2.

The older nftables JSON output omits table comments. After first network preparation, the ownership guard could therefore reject the application's own NAT table and stop before the separate panel-port permission step, leaving VPN forwarding/UDP rules but no panel TCP input rules.

- When the reserved IPv4 table has no JSON comment, verify its exact table-level ownership marker using the read-only native nft listing and restore only that metadata for the existing firewall inspection.
- Do not infer ownership from a reserved name, accept nested/foreign markers, override a conflicting JSON comment, or bypass custom input/forward filter checks.
- Apply the compatibility read to both network setup/retry and panel firewall preparation. Keep UFW active and preserve SSH, existing rules and the managed VPN's scoped panel admission.
- Add legacy JSON/comment regression cases and Ubuntu 22.04 executable-package plus real UFW/WireGuard/panel policy tests alongside Ubuntu 24.04 and firewalld.

Validation: backend suite/vet, legacy ownership rejection tests and required native firewall/platform release gates. The reported Ubuntu 22.04.5 ARM64 host showed the exact JSON/native-listing mismatch; its missing panel rules were restored without restarting the panel or VPN. A private-data test on that host reproduced the installed-version rejection and confirmed the candidate succeeds without changing existing UFW rules or the running panel process. Temporary test data was removed.
