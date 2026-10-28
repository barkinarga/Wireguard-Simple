# Mimari ve araştırma notları

## Karar

Ürün kaynak kodu ve arayüzü sıfırdan geliştirilmiştir. İncelenen wg-easy, wireguard-manager ve wireguard-ui projelerinin kodları veya arayüz varlıkları bu projeye alınmamıştır. Genel amaçlı Go/TypeScript kütüphaneleri kendi lisanslarıyla kullanılır; tam bağımlılık sürümleri go.mod/go.sum ve package-lock.json içinde sabitlenir.

Üretimde Go HTTP sunucusu + gömülü statik arayüz + yerel SQLite kullanılır. WireGuard veri düzlemi Linux çekirdeğinde kalır. Kalıcı izleme, temel yapılandırmadan ayrıdır.

## İncelenen kaynaklar

27 Ekim 2026 araştırması:

- [wg-easy](https://github.com/wg-easy/wg-easy), incelenen master: `33fa6ce4029da1e9a7374df5014e1ca270279819`. Son release o tarihte [v15.5.0](https://github.com/wg-easy/wg-easy/releases/tag/v15.5.0). Node/Nuxt, SQLite, QR, istemci yönetimi, canlı grafikler, Prometheus, OIDC/2FA. [Canlı grafik tamponu](https://github.com/wg-easy/wg-easy/blob/33fa6ce4029da1e9a7374df5014e1ca270279819/src/app/stores/clients.ts) tarayıcıda 50 örnek tutar; [metrics uç noktaları](https://github.com/wg-easy/wg-easy/tree/33fa6ce4029da1e9a7374df5014e1ca270279819/src/server/routes/metrics) anlık sayaçları sunar. Bunlar tek başına kalıcı zaman serisi arşivi değildir. [Dockerfile](https://github.com/wg-easy/wg-easy/blob/33fa6ce4029da1e9a7374df5014e1ca270279819/Dockerfile) çalışma zamanında Node ve ağ araçlarını taşır. Bunlar ölçülmüş RAM karşılaştırması değildir.
- [wireguard-manager](https://github.com/complexorganizations/wireguard-manager), incelenen main: `e0f481f416d699599a26345e72b1c6f20053bae2`. [Bash kurucu](https://github.com/complexorganizations/wireguard-manager/blob/e0f481f416d699599a26345e72b1c6f20053bae2/wireguard-manager.sh) çoklu dağıtım desteği ve terminal yönetimi sunar. Disk kontrolü >1 GB ister; bu gerçek kurulu boyut ölçümü değildir. Başlangıç seçimleri Unbound ve engelleme listesine yönelir; ek paketler ve DNS yapılandırması nedeniyle küçük betik boyutu, küçük sistem etkisi anlamına gelmez.
- [wireguard-ui](https://github.com/ngoduykhanh/wireguard-ui), incelenen master: `2fdafd34ca6c8f7f1415a3a1d89498bb575a7171`. Go, gömülü arayüz, JSON dosyaları ve wgctrl kullanır. [Durum işleyicisi](https://github.com/ngoduykhanh/wireguard-ui/blob/2fdafd34ca6c8f7f1415a3a1d89498bb575a7171/handler/routes.go) son handshake'in 3 dakikadan yeni olmasını aktiflik tahmini olarak kullanır. Son yayımlanan sürüm o tarihte [v0.6.2, 7 Ocak 2024](https://github.com/ngoduykhanh/wireguard-ui/releases/tag/v0.6.2). Eski yayın tarihi tek başına bir güvenlik açığı bulgusu değildir.
- [Resmî WireGuard wg belgesi](https://git.zx2c4.com/wireguard-tools/about/src/man/wg.8): çalışma zamanı alanları endpoint, son handshake, RX/TX sayaçlarıdır. `syncconf` mevcut peer oturumlarını gereksiz yeniden başlatmadan farkları uygular. Dump özel anahtarlar içerdiği için ham çıktısı loglanmaz veya API'ye dönmez.

Bu araştırmada projelerin gerçek sunucu üzerindeki kaynak tüketimi ölçülmedi. Kaynak ve kurulum mimarisi karşılaştırıldı; Go seçimi bir kaynak hedefidir, henüz karşılaştırmalı benchmark sonucu değildir.

## Bileşenler

```text
Tarayıcı: TypeScript + CSS + yerel SVG grafikler
   │ aynı origin, HttpOnly oturum ve CSRF
Go HTTP API (gömülü UI, genel ağda TLS zorunlu)
   ├── Kontrol: istemciler, anahtarlar, sunucu, tercihler
   │       └── control.db (küçük SQLite, özel Linux izinleri)
   ├── Ortak toplayıcı: wg dump → yalnızca izinli alanlara dönüştürme
   │       ├── sınırlı RAM grafiği ve olay tamponu
   │       └── isteğe bağlı telemetry.db
   └── Linux adaptörü: sabit programlar + ayrı argümanlar
           └── Linux WireGuard arayüzü
```

- Demo adaptörü Linux komutlarını çağırmaz; Windows'ta canlı mod girişte reddedilir.
- Kontrol değişiklikleri tek kilitte seri uygulanır. WireGuard uygulaması başarısızsa SQLite kontrol verisi değiştirilmez. Uygulama başarılı olup SQLite commit başarısız olursa eski yapılandırma geri uygulanır; geri alma da başarısızsa açık hata dönülür. Bu, dosya/DB/kernel arasında güç kesintisine karşı tam bir dağıtık transaction garantisi değildir.
- Aynı veri klasörüne ikinci süreç, işletim sisteminin dosya kilidiyle engellenir. Kilit süreç kapanınca işletim sistemi tarafından bırakılır.
- Yeni peer için ağ ve yayın adresleri dışlanır; kayıtlı adreslerle çakışma engellenir. Yönlendirilmiş alt ağ içeren içe aktarımlarda otomatik adres dağıtımı reddedilir.
- Çalışan arayüzün anahtarı kontrol verisiyle eşleşmeden değiştirilmez. Yeni arayüz oluşturma açık bir canlı yönetim işlemi gerektirir.

## Güvenlik ve kaynak sınırları

Varsayılan dinleme loopback'tir. Demo genel ağa açılamaz. Canlı genel dinleme TLS ve canonical origin gerektirir. Host ve Origin denetimi, SameSite Strict HttpOnly cookie ve CSRF belirteci birlikte kullanılır. Yönetici parola doğrulaması tek eşzamanlı Argon2id işlemiyle sınırlandırılır. Oturumlar, giriş IP tablosu, istek gövdeleri, peer sayısı ve RAM örnekleri üst sınırlıdır. Özel anahtarlar yalnızca özel dosyalarda ve açıkça istenen istemci dışa aktarma/QR yanıtında bulunur. Bütün HTTP yanıtları no-store kullanır; dış varlık veya analitik çağrısı yoktur.

Geçmiş ayrı SQLite dosyasıdır. WAL yerine DELETE journal, küçük cache, RAM geçici alanı ve `max_page_count` kullanılır. Toplam geçmiş bütçesinin yarısından azı DB ana dosyasına ayrılır; eş boyuta kadar işlem günlüğüne pay bırakılır. Otomatik vacuum ve eski kayıt temizliği normal iş akışındadır. Kalıcı geçmiş varsayılanı kapalıdır ve kapalı varsayılanlarla telemetry.db oluşturulmadığı test edilir. DB kontrol dosyasının küçük üst sınırı geçmiş bütçesinden bağımsızdır.

Bu bütçe Go derleyicisi, geliştirme önbelleği, tarayıcı, işletim sistemi günlükleri veya yöneticinin haricî yedeklerini kapsamaz. Uygulama derleme zamanı araçları Linux dağıtım paketine dahil edilmez.

## Doğrulama kapsamı

Otomatik testler: kapalı kayıtların disk dosyası oluşturmaması, özel anahtarların durum API'sine sızmaması, CSRF, Host/Origin, ilk kurulum ve parola akışı, giriş sınırı, istemci yaşam döngüsü/QR, saklama/bütçe, restart sonrası olaylar, hatalı uygulamanın kontrol verisini koruması, sayaç sıfırlanması, hook reddi ve mod ayrımı.

Yerel tarayıcı testi: istemci oluşturma, ayrıntı/QR görüntüleme, tercihleri kaydetme, mobil düzen ve konsol hataları. Linux çıktıları çapraz derlemeyle kontrol edilir. Gerçek Linux netlink/komut yetkileri, NAT, yeniden başlatma ve kısıtlı disk koşulları ayrı hedef ortam testidir.
