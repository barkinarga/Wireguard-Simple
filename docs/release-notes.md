# WireGuard Simple v0.4.4

Make the installation summary easier to read and display the first-run setup token directly.

- Separate panel addresses, the setup token, certificate guidance, panel access and VPN networking with spaced headings.
- Use green for success, cyan for panel/setup headings and yellow for Note headings in interactive terminals. Redirected output, dumb terminals and NO_COLOR remain plain text.
- Display an existing setup token with a privacy reminder, without requiring cat or an editor. Completed setup does not display a token.
- Keep HTTPS behavior, existing clients, accounts and network settings unchanged.

Installer checks cover token display, missing and invalid token content, plain redirected output, installation and rollback.
