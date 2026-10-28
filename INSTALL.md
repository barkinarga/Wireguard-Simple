# Install WireGuard Simple

[README](README.md)

These steps are for your **Debian, Ubuntu, RHEL, CentOS Stream, AlmaLinux, Rocky Linux, or Fedora server**, with systemd and a WireGuard-capable kernel, on amd64 or arm64. The installer refuses Windows and WSL. Your personal computer only needs SSH and a browser to access the panel. See the [distribution requirements](docs/platforms.md), especially for EL8 and immutable Fedora images.

## 1. Download and install

Run on the server:

```bash
curl -fL --retry 3 -o wgsimple-install.run https://github.com/barkinarga/Wireguard-Simple/releases/latest/download/wgsimple-install.run && sudo bash wgsimple-install.run
```

If `curl` is missing, install it first on Debian/Ubuntu:

```bash
sudo apt-get update && sudo apt-get install -y curl ca-certificates
```

On the RPM-based distributions, use `sudo dnf install -y curl ca-certificates` (or `yum` on an EL system without `dnf`). If the server already provides `curl`, including `curl-minimal`, keep that provider. You can also transfer the installer file from your computer.

Alternatively, download `wgsimple-install.run` from the [latest release](https://github.com/barkinarga/Wireguard-Simple/releases/latest), copy it to the server, and run `sudo bash wgsimple-install.run` there. To install without downloading missing system packages, provide those packages yourself and use `--no-deps`.

The installer chooses the architecture, verifies the embedded payload's checksum, installs missing WireGuard tools, IP tools (`iproute2` or `iproute`), `curl`, and `kmod`, then checks kernel support and starts `wgsimple.service`. It uses your enabled repositories; if a package is unavailable, it stops with an explanation. It adds no external repository, kernel, Go compiler, Node.js, Docker, or database service. Temporary extraction files are cleaned up.

Traffic history, persistent administrative logs, and Prometheus are **off on a fresh installation**. Existing settings and client keys are preserved on upgrade. If startup fails, the installer attempts to restore the previous application and service state.

## 2. Open the panel

On your own computer, replace `user@server` with your SSH login and keep this connection open:

```bash
ssh -L 8787:127.0.0.1:8787 user@server
```

Open **http://127.0.0.1:8787**. If that local port is already used by a demo or another program, use `ssh -L 8788:127.0.0.1:8787 user@server` and open **http://127.0.0.1:8788** instead.

For first-run setup, read the token on the server:

```bash
sudo cat /var/lib/wgsimple/setup-token
```

Enter it in the setup screen and create your administrator account. Keep the token private. Its file is removed after setup; future logins use your username and password. The interface defaults to English; a language selector is available before sign-in and in the top bar, with 10 embedded languages.

On a fresh installation, the panel also listens on `0.0.0.0:8787`. Open the detected `http://SERVER_IP:8787` address directly if TCP 8787 is allowed by the host/provider firewall. Detection uses an address assigned to the server; behind NAT, use the mapped public address. No external IP lookup service is called. HTTP sends passwords, session cookies, and configurations without encryption. Use the SSH tunnel or configure TLS on untrusted networks. The installer does not automatically open firewall ports.

## 3. Prepare VPN connectivity

In **Settings → Endpoint**, enter your server's public IP address or domain. Use **Add client** to create a device and obtain its configuration or QR code.

**The one-command installer sets up the management panel and tools.** Internet access through the VPN additionally requires IP forwarding, NAT, and UDP firewall rules appropriate for your server. Configure these separately. The installer does not change them, DNS, or existing VPN interfaces.

Live management actions can create or update the selected WireGuard interface. For an existing VPN, import its matching server configuration before managing clients. Server files do not contain client private keys, so imported clients cannot receive regenerated client configurations from that import alone.

Real Linux VPN connectivity, routing, and reboot recovery have not yet completed end-to-end validation for this early release. Test on a dedicated Linux server before relying on it.

## Update

Run the same download-and-install command again to install the latest published release. The panel briefly restarts; existing clients, administrator credentials, and recording preferences are preserved. The installer does not take down an existing WireGuard interface. You can delete the downloaded `.run` file after installation.

## Useful commands

| Task | Command |
| --- | --- |
| Check the environment without installing | `bash wgsimple-install.run --check` |
| Install files without starting the panel | `sudo bash wgsimple-install.run --no-start` |
| Skip automatic system package installation | `sudo bash wgsimple-install.run --no-deps` |
| Choose an interface on a fresh installation | `sudo bash wgsimple-install.run --interface wg1` |
| Check the panel service | `sudo systemctl status wgsimple.service` |
| Restart the panel | `sudo systemctl restart wgsimple.service` |
| Stop the panel | `sudo systemctl stop wgsimple.service` |

An upgrade preserves the interface in the existing service file. With custom HTTPS/origin settings, the installer's loopback health check may not apply; install using `--no-start` and start the service with your own configuration.

Stopping the panel leaves an existing WireGuard interface running, but disables monitoring and client expiry enforcement until the panel runs again. Automatic VPN startup after reboot must be enabled separately once its configuration has been validated.

[Back to README](README.md)

## Panel access and recovery

In **Settings → Panel access**, choose **Public access**, **This server only**, or **Private networks only**. Optionally add up to 64 IP addresses or CIDRs, separated by commas or newlines. The allowlist intersects with the selected mode. Loopback is always admitted. Private networks mean RFC1918 IPv4 and unique-local IPv6 source addresses, including VPN networks; this is not a location check.

The policy takes effect on every new request, including sign-in, static files, metrics, and health checks. It is stored in `/var/lib/wgsimple/panel-access.json` with private permissions and persists through restart/upgrade. Blocking sources returns HTTP 403; it does not close the listening socket or change firewall rules. Source IP comes from the TCP connection; forwarded headers are ignored. A local reverse proxy therefore appears as loopback: enforce source restrictions in the proxy itself.

If you lock yourself out, run over SSH:

```bash
sudo systemctl stop wgsimple.service
sudo /usr/local/bin/wgsimple --mode live --data-dir /var/lib/wgsimple --reset-panel-access public
sudo systemctl start wgsimple.service
```

Use `local` instead of `public` to recover to loopback-only admission, or `lan` for private networks. This clears the allowlist without changing accounts, passwords, VPN keys, or history. Alternatively, while stopped, edit `panel-access.json` to `{"mode":"public","allowlist":[]}` and restart. Invalid files prevent startup rather than silently exposing the panel.

### Upgrading a previous loopback-only installation

Upgrades preserve the existing service listener. If it still uses `--listen 127.0.0.1:8787`, the panel policy cannot make that socket public. After upgrading to v0.3.0 or newer, run `sudo systemctl edit --full wgsimple.service` and replace only `--listen 127.0.0.1:8787` with `--listen 0.0.0.0:8787 --allow-http` in `ExecStart`, preserving all other arguments. Then run:

```bash
sudo systemctl daemon-reload
sudo systemctl restart wgsimple.service
```

For TLS instead of unencrypted public HTTP, omit `--allow-http` and supply `--tls-cert`, `--tls-key`, and an explicit HTTPS `--origin`. That canonical origin controls which address the browser must use.
