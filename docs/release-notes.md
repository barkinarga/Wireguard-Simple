WireGuard Simple v0.2.0 adds an English default interface, 10 embedded interface languages, and installation paths for seven Linux distributions.

- Interface languages: English, Turkish, German, French, Spanish, Italian, Portuguese, Russian, Arabic, and Simplified Chinese. The selector works before sign-in and in the top bar; the browser remembers the choice. Arabic has an RTL layout.
- Dates, numbers, relative times, common validation messages, and administrative events follow the selected language. Existing Turkish event history is supported. Low-level technical diagnostics remain English.
- The installer recognizes Debian, Ubuntu, RHEL, CentOS Stream, AlmaLinux, Rocky Linux, and Fedora; selects `apt-get`, `dnf`, or the EL `yum` fallback; and checks WireGuard kernel support without creating an interface.
- Missing packages are installed from enabled repositories. Existing curl providers are preserved. The installer does not add repositories, replace kernels, disable SELinux, or change firewalld/NAT/forwarding/DNS.
- Traffic history, persistent administrative history, and Prometheus remain off on a fresh installation. Existing keys and preferences are preserved on upgrade.

On the target Linux server:

```bash
curl -fL --retry 3 -o wgsimple-install.run https://github.com/barkinarga/Wireguard-Simple/releases/latest/download/wgsimple-install.run && sudo bash wgsimple-install.run
```

[English installation guide](https://github.com/barkinarga/Wireguard-Simple/blob/readme/INSTALL.md) · [Türkçe kurulum](https://github.com/barkinarga/Wireguard-Simple/blob/readme/KURULUM.md) · [Platform requirements](https://github.com/barkinarga/Wireguard-Simple/blob/readme/docs/platforms.md)

EL8 requires a prepared WireGuard-capable kernel/module. CentOS means Stream 9+; retired CentOS Linux and immutable ostree images are excluded. A systemd server on amd64/arm64 is required. Internet routing and UDP firewall policy must be configured separately. This remains an early development release: automated tests do not replace real VPN, SELinux, or reboot validation on every target distribution.

### Türkçe

Varsayılan arayüz İngilizce oldu; Türkçe dahil 10 dil, tarayıcıda hatırlanan dil seçimi ve Arapça için sağdan sola düzen eklendi. Kurucu artık Debian, Ubuntu, RHEL, CentOS Stream, AlmaLinux, Rocky Linux ve Fedora için doğru paket yolunu seçiyor. EL8'de uygun WireGuard çekirdeği/modülü önceden gerekebilir. Ek depo açılmaz; SELinux/firewalld kapatılmaz. Kalıcı kayıtlar yeni kurulumda kapalı kalır. Gerçek VPN bağlantısı ve dağıtım başına üretim doğrulaması henüz tamamlanmadı.
