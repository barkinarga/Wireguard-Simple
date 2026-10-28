# WireGuard Simple v0.4.0

V2 is now the default interface for every installation and upgrade. The existing
VPN management features and configuration are preserved.

- Simplified navigation around the sidebar; removed workspace placeholders,
  slogan cards, theme-preview controls, and repeated settings shortcuts.
- Clearer client tables, controls, forms, and dialogs. Secondary text is at least
  12px in V2, including the server information card and public keys.
- Responsive desktop/mobile layouts, keyboard focus, reduced-motion support,
  and all ten existing interface languages are retained.
- Removed the activity retention explanation strip and demo banner/footer text.
- The executable now defaults to live Linux mode. Explicit `--mode demo` remains
  available for development and Windows previews; removing labels does not
  turn simulated data into a real VPN connection.
- The original visual design remains available at `/?design=v1`.

## Upgrade

Back up the existing installation, then run the new `wgsimple-install.run` on the
Linux server. Administrator credentials, client keys, expiry dates, monitoring
preferences, and networking remain under the existing upgrade behavior. See
[INSTALL.md](INSTALL.md) for installation, backup, recovery, and network options.

The V2 interface was checked on desktop and mobile. An ARM64 build containing
these UI changes was installed on an existing Ubuntu test server in live mode;
service health passed and the WireGuard peer set and configuration were verified
unchanged. This update does not add new cross-distribution or external-device
connectivity validation beyond the existing platform notes.
