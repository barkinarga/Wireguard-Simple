# Linux paketi — hızlı başlangıç

Bu arşiv hedef Debian/Ubuntu **Linux sunucusu** içindir. Windows veya WSL üzerinde kurulum yapmayın. Paketteki uygulama dosyası sunucunun mimarisiyle eşleşmelidir.

Arşivi bir klasöre açtıktan sonra sunucuda tek komut:

```bash
sudo bash install.sh
```

Gerekli araçlar otomatik tamamlanır ve panel servisi başlatılır. Go, Node.js, Docker veya ayrı bir veritabanı sunucusu kurulmaz. Güncellemede mevcut istemciler, anahtarlar ve servis ayarları korunur.

Kendi bilgisayarınızdan SSH tüneli açın:

```bash
ssh -L 8787:127.0.0.1:8787 KULLANICI@SUNUCU_IP
```

Tarayıcı: **http://127.0.0.1:8787**.

İlk hesap için sunucuda kurulum anahtarını kendiniz okuyun:

```bash
sudo cat /var/lib/wgsimple/setup-token
```

Bu anahtarı panelde kullanarak kendi yönetici hesabınızı oluşturun. Anahtarı paylaşmayın. Kalıcı trafik/log geçmişi yeni kurulumda kapalıdır.

**Kurucu paneli ve WireGuard araçlarını kurar.** İnternet çıkışı için forwarding/NAT ve UDP güvenlik duvarı ayarları ayrıca hazırlanmalıdır. Kurucu mevcut VPN arayüzlerini, DNS veya güvenlik duvarını değiştirmez. Gerçek Linux VPN bağlantı testi henüz tamamlanmamıştır.

Başlatmadan kurulum için `sudo bash install.sh --no-start`, otomatik paket kurulumunu kapatmak için `--no-deps`, değişiklik yapmadan kontrol için `bash install.sh --check` kullanılabilir. Eski `install-linux.sh` adı da uyumluluk için bulunur; yanında `install.sh` dosyası olmalıdır.

Tek dosyalık evrensel kurucu olan `wgsimple-install.run` için adımlar `KURULUM.md` dosyasındadır. Ayrıntılı özellikler ve sınırlar `README.md` içindedir.
