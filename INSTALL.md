# Install WireGuard Simple

[English README](README.md) · [Türkçe kurulum](KURULUM.md)

These steps are for your **Debian/Ubuntu Linux server**, with systemd and a WireGuard-capable kernel, on amd64 or arm64. The installer refuses Windows and WSL. Your personal computer only needs SSH and a browser to access the panel.

## 1. Download and install

Run on the server:

```bash
curl -fL --retry 3 -o wgsimple-install.run https://github.com/barkinarga/Wireguard-Simple/releases/latest/download/wgsimple-install.run && sudo bash wgsimple-install.run
```

If `curl` is missing, install it first:

```bash
sudo apt-get update && sudo apt-get install -y curl ca-certificates
```

Alternatively, download `wgsimple-install.run` from the [latest release](https://github.com/barkinarga/Wireguard-Simple/releases/latest), copy it to the server, and run `sudo bash wgsimple-install.run` there. To install without downloading missing system packages, provide those packages yourself and use `--no-deps`.

The installer chooses the architecture, verifies the embedded payload's checksum, installs missing `wireguard-tools`, `iproute2`, and `curl`, then starts `wgsimple.service`. It installs no Go compiler, Node.js, Docker, or database service. Temporary extraction files are cleaned up.

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

Enter it in the setup screen and create your administrator account. Keep the token private. Its file is removed after setup; future logins use your username and password. The current setup interface is in Turkish.

The default panel is only reachable through loopback and your SSH tunnel. The installer does not expose a public HTTP port.

## 3. Prepare VPN connectivity

In **Ayarlar → Endpoint** (Settings → Endpoint), enter your server's public IP address or domain. Use **İstemci ekle** (Add client) to create a device and obtain its configuration or QR code.

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
