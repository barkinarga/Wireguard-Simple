# WireGuard Simple v0.4.6

Detect initial connection defaults and add optional per-client configuration controls.

- Fill a blank endpoint from one unambiguous public IPv4 assigned to the host, excluding the managed VPN. Preserve saved values; NAT-only or ambiguous hosts remain manual. No external IP lookup.
- New installations inherit the host's IPv4 DNS resolvers, using upstream systemd-resolved configuration instead of loopback stubs. Remove the fixed Cloudflare DNS default. If discovery fails, allow manual DNS or retaining device DNS.
- Preserve existing DNS settings; add Use server DNS to populate the Settings field before saving.
- Expand New client through Advanced settings for VPN address, DNS, AllowedIPs, endpoint/port, MTU, keepalive, listen port, FwMark and optional keys. Empty fields retain defaults. Apply overrides to that client only, including its download and QR code.
- Validate addresses, duplicates, keys and numeric bounds before applying changes; keep client secrets out of snapshots. Existing client records remain compatible.

Go tests cover detection, preservation, invalid input, API authorization, per-client persistence and exported configurations. TypeScript, all ten language catalogs and the embedded UI build are checked; browser testing covers the expanded form and client creation.
