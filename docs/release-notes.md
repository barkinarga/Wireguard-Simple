# WireGuard Simple v0.4.16

This security update covers client configuration exports, sign-in limits behind a reverse proxy, and managed VPN startup.

- Reject DNS values with line breaks or NUL characters. Configuration downloads and QR codes also check DNS values saved by older versions before exporting them.
- Use `--trusted-proxy` to separate sign-in limits by IPv4 address or IPv6 /64 behind a configured reverse proxy. Set the proxy to overwrite `X-Real-IP` with the client's address as described in the [installation guide](https://github.com/barkinarga/Wireguard-Simple/blob/v0.4.16/INSTALL.md#panel-access-and-recovery).
- Skip expired clients when restoring the managed WireGuard interface at startup.

Run the current installer to upgrade an existing installation. See the [installation guide](https://github.com/barkinarga/Wireguard-Simple/blob/v0.4.16/INSTALL.md#update) for update steps.
