# WireGuard Simple v0.4.2

Public panel access now uses HTTPS automatically, without requiring Cloudflare, a domain name, or an external certificate service.

- Non-loopback listeners create a server-specific, persistent self-signed TLS certificate when no custom certificate is supplied. Open `https://SERVER_IP:8787` and verify the certificate fingerprint over SSH before accepting the expected browser warning.
- The private combined certificate/key stays in `<data-dir>/panel-tls.pem` with private permissions and is reused across restarts and upgrades. Invalid automatic identities stop startup instead of falling back to HTTP.
- Existing public service files containing `--allow-http` now use HTTPS too; the flag remains accepted only for compatibility. An explicit public HTTP origin must be changed to HTTPS. Loopback-only HTTP and custom TLS certificates remain supported.
- HTTPS setup/login sessions use Secure cookies. Plaintext requests cannot reach management handlers on the HTTPS listener.
- The installer generates a HTTPS-enabled service, prints HTTPS addresses, and checks local readiness with support for self-signed HTTPS and preserved loopback HTTP services.
- Existing VPN clients, keys, expiry behavior and network-access behavior are unchanged. This release does not add VPN access profiles or a separate control service.

Self-signed TLS encrypts the connection, but blindly accepting a warning does not authenticate the server against an active intermediary. Verify the fingerprint over a trusted SSH connection, explicitly trust the verified certificate, or configure a trusted custom certificate.

Regression coverage includes real HTTPS setup/login through the production server, plaintext rejection, Secure cookies, certificate persistence, invalid/expired identities, private permissions and symlinks where supported, custom TLS, IPv4/IPv6 listener classification, and installer readiness/upgrade preservation.
