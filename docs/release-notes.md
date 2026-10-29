# WireGuard Simple v0.4.10

Show panel links even when the installed version is already up to date.

- Print the panel IPv4 URL and local URL after the already-current message, using the effective service listener and port.
- Share the same URL formatting with successful installation output. Respect custom ports and loopback-only HTTP/HTTPS listeners.
- Keep the already-current path read-only: no reinstall, service restart or setup-token display.
- Add regression coverage for current/newer installations, custom ports and loopback HTTP/TLS; verify the shared output helper in the executable installer payload.
