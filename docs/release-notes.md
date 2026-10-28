Bağımsız Go sunucusu ve gömülü Türkçe web arayüzüyle ilk geliştirme sürümü.

- İstemci yönetimi, QR ve yapılandırma indirme, canlı trafik ve yönetim olayları.
- Varsayılanı kapalı, süre ve disk sınırı olan geçmiş/log kayıtları.
- Yönetici girişi ve isteğe bağlı salt okunur Prometheus erişimi.
- Debian/Ubuntu için amd64/arm64 seçimini otomatik yapan tek dosyalık kurucu.

Hedef Linux sunucusunda kurulum:

```bash
curl -fL --retry 3 -o wgsimple-install.run https://github.com/barkinarga/Wireguard-Simple/releases/latest/download/wgsimple-install.run && sudo bash wgsimple-install.run
```

`curl` yoksa paketi tarayıcıdan indirip sunucuya aktarın veya önce `sudo apt-get update && sudo apt-get install -y curl ca-certificates` çalıştırın.

[Basit kurulum kılavuzu](https://github.com/barkinarga/Wireguard-Simple/blob/readme/KURULUM.md)

Kurucu paneli ve gerekli araçları kurar; mevcut VPN arayüzlerine, NAT/forwarding/DNS veya güvenlik duvarına dokunmaz. İnternet çıkışı için gerekli ağ politikaları ayrıca hazırlanmalıdır. Bu sürümün gerçek Linux VPN bağlantısı ve üretim koşulları testi henüz tamamlanmamıştır.
