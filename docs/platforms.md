# Linux platform support

[English README](../README.md) · [Türkçe README](../README.tr.md) · [Install](../INSTALL.md)

The same statically built Go application and embedded interface are packaged for **Linux amd64 and arm64**. The installer recognizes these distribution families:

| System | Package manager | IP tools package | Notes |
| --- | --- | --- | --- |
| Debian, Ubuntu | `apt-get` | `iproute2` | systemd and a WireGuard-capable running kernel required |
| RHEL 8+ | `dnf`, then `yum` fallback | `iproute` | Enabled repositories/subscription must provide the required packages |
| AlmaLinux 8+, Rocky Linux 8+ | `dnf`, then `yum` fallback | `iproute` | Uses enabled distribution repositories |
| CentOS Stream 9+ | `dnf`, then `yum` fallback | `iproute` | Retired CentOS Linux 7/8 and Stream 8 are rejected |
| Fedora | `dnf` | `iproute` | Mutable, systemd-based installations; ostree images use a different deployment model |

The check is an installer compatibility boundary, not a statement of the distribution vendor's support policy. Real installations, VPN connectivity, SELinux enforcement, firewalld, reboot recovery, and low-resource behavior have **not yet been validated on each distribution**. Automated tests cover distribution selection and package/kernel/label operations with mocks, plus file installation and rollback in an isolated Linux CI workspace.

## Kernel and repository prerequisites

The installer first plans missing tools: `wireguard-tools`, `iproute2` or `iproute`, `curl`, and `kmod`; package installation also ensures `ca-certificates`. Existing tools are preserved. In particular, an existing `curl-minimal` provider is not replaced merely because another package is missing. RPM installs disable weak dependencies to avoid unnecessary packages.

The running kernel must already support WireGuard. The check uses `/sys/module/wireguard` or `modprobe --dry-run --quiet wireguard`; it does not create an interface, load a module, or change routes. EL8 kernels can require separately prepared WireGuard modules. Upstream documents EL8-specific module/repository options, which remain an administrator decision. The installer does not enable EPEL, ELRepo, COPR, change subscription settings, or replace a kernel. [WireGuard installation reference](https://www.wireguard.com/install/).

For RHEL 9/10, Red Hat documents `dnf install wireguard-tools` and `wg-quick` service configuration. Red Hat's documentation classifies WireGuard as a Technology Preview; this project's installer does not alter that vendor support status. [RHEL 9 networking](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/9/html-single/configuring_and_managing_networking/index), [RHEL 10 networking](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/10/html-single/configuring_and_managing_networking/index).

If your enabled repositories cannot resolve a required package, installation stops. Provide the missing tools from an administrator-approved source, then rerun the installer; `--no-deps` disables package installation. The panel requires a mutable system with systemd, so Fedora CoreOS/Silverblue-style ostree images are rejected by this installer.

CentOS Stream is the maintained CentOS line. CentOS Linux 7/8 and Stream 8 have reached end of life and are outside this installer's scope. [CentOS lifecycle](https://www.centos.org/cl-vs-cs/).

## SELinux, firewalls, and existing VPNs

- SELinux remains enabled. If `restorecon` is available, the installer applies the system's configured labels to the application, unit, and application/config directories. It does not install a permissive policy or disable enforcement. A customized policy may require administrator review of AVC denials.
- firewalld, nftables, iptables, NAT, forwarding, and DNS policies remain under administrator control. Only the management service is installed; it binds to loopback and is reached through SSH by default.
- Existing service settings and keys are preserved on upgrade. The panel's generated service uses an explicit system path including `sbin` directories, which avoids distribution-specific command lookup differences.
- Avoid having NetworkManager and this application independently modify the same WireGuard interface. Import the matching server configuration and choose a single owner for its configuration.

The package does not add a database service, frontend server, container runtime, or translation service. Optional traffic history, persistent administrative history, and metrics remain off on a fresh installation.

## Check without installing

```bash
bash wgsimple-install.run --check
```

The command extracts the installer into a temporary directory, checks architecture, distribution, prerequisites and kernel support, then removes temporary files. It does not invoke the package manager, install a service, load a kernel module, or change networking. Missing `kmod` is reported as a pending kernel check; installation verifies it after dependencies are available.
