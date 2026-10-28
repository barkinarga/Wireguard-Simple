# WireGuard Simple — basit kurulum

Bu adımlar **Debian/Ubuntu Linux sunucusu** içindir. Windows bilgisayarına WireGuard kurmayın. Kurucu Windows ve WSL üzerinde çalışmayı reddeder.

## 1. Tek dosyayı sunucuya gönder

**İnternetten doğrudan kurulum:** Sunucuda `curl` varsa aşağıdaki tek satır indirir ve kurar; sonra 3. adımdan devam edin:

```bash
curl -fL --retry 3 -o wgsimple-install.run https://github.com/barkinarga/Wireguard-Simple/releases/latest/download/wgsimple-install.run && sudo bash wgsimple-install.run
```

`curl` yoksa önce `sudo apt-get update && sudo apt-get install -y curl ca-certificates` çalıştırın veya aşağıdaki dosya aktarım yolunu kullanın.

Projedeki `release/wgsimple-install.run` dosyasını sunucuya kopyalayın. amd64/arm64 seçimi otomatiktir. Örneğin, kendi bilgisayarınızda proje klasöründen:

```powershell
scp .\release\wgsimple-install.run KULLANICI@SUNUCU_IP:~/
```

`KULLANICI` ve `SUNUCU_IP` alanlarını kendi bilgilerinizle değiştirin. SCP yerine kullandığınız dosya aktarım uygulaması da olur.

## 2. Sunucuda tek komutla kur

SSH ile sunucuya bağlanın ve dosyanın bulunduğu klasörde çalıştırın:

```bash
sudo bash wgsimple-install.run
```

Kurucu doğru mimariyi seçer, paketin bütünlüğünü doğrular, eksik `wireguard-tools`, `iproute2`, `curl` araçlarını kurar ve panel servisini otomatik başlatır. Sunucuya Go, Node.js, Docker veya ayrı bir veritabanı servisi kurulmaz. Kurulumun geçici dosyaları işlem sonunda temizlenir.

Yeni kurulumda trafik geçmişi, kalıcı log ve Prometheus kapalıdır. Var olan kurulumun istemcileri, anahtarları, servis dosyası ve kayıt tercihleri korunur. Panel başlatılamazsa kurucu önceki uygulamayı ve servis durumunu geri yüklemeye çalışır.

## 3. Paneli aç

Kendi bilgisayarınızda SSH tünelini açın ve açık bırakın:

```powershell
ssh -L 8787:127.0.0.1:8787 KULLANICI@SUNUCU_IP
```

Tarayıcıda **http://127.0.0.1:8787** adresini açın.

İlk giriş için sunucudaki terminalde:

```bash
sudo cat /var/lib/wgsimple/setup-token
```

Görünen anahtarı kurulum ekranına girin ve kendi yönetici hesabınızı oluşturun. Anahtarı paylaşmayın. Hesap oluşturulduktan sonra bu dosya kaldırılır; tekrar girişlerde kullanıcı adı ve parolanız kullanılır.

Panel dışarıya açık HTTP portu açmaz; SSH tüneli üzerinden erişilir. Kendi bilgisayarınızda başka bir demo 8787 portunu kullanıyorsa önce kapatın veya tünelin ilk portunu `8788` yapıp tarayıcıda `http://127.0.0.1:8788` kullanın.

## 4. VPN bağlantısını hazırla

Panelde **Ayarlar → Endpoint** alanına sunucunuzun genel IP adresini veya alan adını yazın. Ardından **İstemci ekle** ile cihaz oluşturup QR kodunu veya yapılandırmayı kullanın.

**Tek komut paneli ve gerekli WireGuard araçlarını kurar.** VPN'in internet çıkışı için sunucudaki forwarding/NAT ve UDP güvenlik duvarı kuralları ayrıca hazırlanmalıdır. Kurucu bunları, DNS ayarlarını veya mevcut VPN arayüzlerini otomatik değiştirmez. İlk sürümün gerçek Linux VPN bağlantı testi henüz tamamlanmamıştır.

## Güncelleme

Yeni `wgsimple-install.run` dosyasını sunucuya gönderip aynı komutu çalıştırın:

```bash
sudo bash wgsimple-install.run
```

Yönetim paneli kısa süreliğine yeniden başlatılır; mevcut VPN arayüzü kapatılmaz. Mevcut yönetici hesabınız ve istemcileriniz korunur.

## Gerektiğinde

Kurulum yapmadan ortam kontrolü:

```bash
bash wgsimple-install.run --check
```

Yalnızca dosyaları kurma, paneli başlatmama:

```bash
sudo bash wgsimple-install.run --no-start
```

Otomatik paket kurulumunu kapatma:

```bash
sudo bash wgsimple-install.run --no-deps
```

Paneli kontrol etme / yeniden başlatma / durdurma:

```bash
sudo systemctl status wgsimple.service
sudo systemctl restart wgsimple.service
sudo systemctl stop wgsimple.service
```

Servis özel HTTPS/origin ayarlarıyla çalışıyorsa otomatik loopback sağlık kontrolü uyumlu olmayabilir. Bu kurulumlarda `--no-start` ile dosyaları kurup servisi kendi ayarlarınızla başlatın. Yeni kurulumda farklı WireGuard arayüzü için `--interface wg1` kullanılabilir; güncellemede mevcut servis dosyasındaki arayüz korunur.

Kurulumdan sonra yüklediğiniz `.run` dosyasını silebilirsiniz. Güncelleme için aynı indirme/kurulum komutu kullanılır. Kaynak kodu: [barkinarga/Wireguard-Simple](https://github.com/barkinarga/Wireguard-Simple).
