# WireGuard Simple v0.4.3

Fix Chrome ERR_SSL_VERSION_OR_CIPHER_MISMATCH when opening the self-signed HTTPS panel.

- Automatically generated certificates now use ECDSA P-256 with SHA-256, compatible with Chrome TLS signature preferences.
- On startup, the automatic Ed25519 certificate created by v0.4.2 is atomically replaced once with an ECDSA identity. Its fingerprint changes; verify the new fingerprint over SSH before accepting the browser warning.
- Existing browser-compatible automatic identities persist unchanged. Invalid identities still fail closed and operator-supplied custom certificates are never modified.
- Public HTTPS requirements, Secure session cookies, loopback HTTP, VPN clients, keys and network behavior are unchanged.

Regression tests check the browser-compatible certificate algorithms, automatic legacy migration, persistence after migration, and preservation of explicit custom certificates, alongside the existing HTTPS setup/login and plaintext-rejection tests.
