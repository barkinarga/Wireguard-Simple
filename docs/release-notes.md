# WireGuard Simple v0.4.5

Shorten the installation summary and restore panel URLs after upgrades.

- Show a compact success line, panel/local URLs, the existing setup token, a short certificate note and the TCP/UDP firewall reminders.
- Read effective service arguments to display URLs after upgrades while preserving existing service settings. Respect custom listener ports and loopback HTTP/TLS.
- Keep colored headings and URLs in interactive terminals, with plain redirected output and NO_COLOR support.
- Keep HTTPS, accounts, clients, keys and VPN networking behavior unchanged.

Installer tests cover fresh and upgraded URL output, effective service arguments, custom ports, loopback schemes, setup tokens and rollback.
