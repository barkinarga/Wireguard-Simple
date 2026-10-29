# WireGuard Simple v0.4.8

Fix installation on minimal CentOS Stream 10 hosts that do not have tar, and check prerequisites before unpacking the installer.

- Embed one shared dependency module in the executable package header and payload. Check archive/decompression, checksum, core utilities, text tools, CA certificates, systemctl and VPN/network commands before using them.
- Install missing packages with the supported distro's package manager, then verify commands and the CA bundle. Refresh repository metadata and retry one failed transaction once, without adding repositories or disabling TLS/package verification.
- Use an already-installed bsdtar when GNU tar/gzip are missing, or when GNU extraction fails. Always verify the archive checksum before extraction; never run a partially extracted installer.
- Keep --check and --no-deps from installing packages. Preserve existing curl providers and honor --no-network and --configure-network.
- Extend platform CI to CentOS Stream 10 and run the real self-extracting header against minimal distro containers, including missing-tar, no-deps/check and actual bsdtar extraction scenarios. The smoke payload verifies prerequisites without creating app services, VPN interfaces or application data.

Isolated tests cover transient/permanent package-manager failures, post-install missing commands, no-deps/check and extractor fallback. Existing application, installer, package integrity and real UFW/firewalld packet tests remain release gates.
