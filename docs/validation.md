# İlk sürüm doğrulaması

Tarih: 27 Ekim 2026. Geliştirme ortamı: Windows/amd64, proje klasörüne taşınabilir olarak indirilen resmî Go 1.27.3 ve Node.js. Windows'a WireGuard, Docker, ağ sürücüsü veya sistem servisi kurulmadı. Linux için kurulum betikleri yalnızca dosya olarak hazırlandı ve bu bilgisayarda çalıştırılmadı.

## Geçen kontroller

- TypeScript: `npm run check`.
- Gömülü arayüz derlemesi: `npm run build`.
- Go: `go test ./... -count=1 -cover` — uygulama paketindeki 18 test geçti. Uygulama paketi satır/ifade kapsamı %63,1; Linux'a özgü komut yürütme yollarının gerçekten çalıştırıldığı anlamına gelmez.
- Go statik kontrolü: `go vet ./...`.
- Linux kurulum ve derleme betiklerinin Bash yazım kontrolü (`bash -n`); betikler çalıştırılmadı.
- CGO kapalı Windows/amd64, Linux/amd64 ve Linux/arm64 derlemeleri.
- Windows'ta `--mode live` başlangıçta reddedildi; bu denemede veri klasörü dahi oluşturulmadı.
- Varsayılan demoda `telemetry.db` oluşturulmadı. Kontrol veritabanı gözlenen örnek çalışma alanında 28.672 bayttı; bu genel bir kapasite garantisi değildir.
- Yerel tarayıcı: istemci oluşturma, ayrıntı ve QR görüntüleme, depolama tercihlerini kaydetme.
- 390 × 844 mobil düzen: menü açma, istemci/genel bakış sayfaları arasında gezinme ve menünün kapanması. Mobil testten sonra viewport varsayılanına döndürüldü.
- Son tarayıcı konsolu kontrolünde hata veya uyarı görülmedi.

## Testlerin kapsadığı davranışlar

1. Kapalı varsayılanların geçmiş dosyası oluşturmaması.
2. Durum API'sinde özel anahtar bulunmaması.
3. İstemci oluşturma/QR/dışa aktarma/duraklatma/silme ve CSRF denetimi.
4. Host, Origin ve cross-site istek reddi.
5. İlk kurulum, parola doğrulama ve parola değişince oturum iptali.
6. Giriş denemesi sınırı.
7. Geçmiş bütçesi, bütçe küçültme, süre temizliği ve kontrol verisini koruyan geçmiş silme.
8. Yeniden başlatma sonrası kalıcı yönetim olayları.
9. WireGuard uygulaması başarısız olunca kontrol verisinin değişmemesi.
10. Sayaç sıfırlanmasında geçersiz hız üretilmemesi.
11. Hook içeren içe aktarmanın reddi ve mevcut kayıtların korunması.
12. Dump ayrıştırma ve yapılandırma/endpoint/arayüz doğrulaması.
13. Demo ve canlı veri klasörlerinin ayrılığı.
14. Eşzamanlı olay/durum isteklerinde RAM olay tamponunun üst sınırı.
15. Metriklerin başlangıçta kapalı olması.
16. Metrik anahtarının salt okunurluğu, özetiyle saklanması ve yenilenince eski anahtarın geçersizliği.
17. Veri klasörünün ikinci sürece karşı kilitlenmesi.
18. Çalışan arayüz adres/MTU bilgilerinin eşleşme denetimi.

## Paket boyutları

Son doğrulanan derleme:

| Dosya | Bayt |
| --- | ---: |
| Windows/amd64 uygulaması | 12.480.000 |
| Linux/amd64 uygulaması | 12.517.536 |
| Linux/arm64 uygulaması | 11.796.640 |
| Derlenmiş arayüz JavaScript | 49.491 |
| Arayüz CSS | 45.317 |
| HTML | 807 |

Bu boyutlar geliştirme araçlarını veya Linux'un WireGuard/iproute2 paketlerini kapsamaz. RAM göstergesi Go çalışma zamanı tahsisidir; Linux RSS veya karşılaştırmalı benchmark sonucu olarak sunulmaz.

## Henüz doğrulanmayanlar

- Gerçek Linux çekirdeğinde istemci bağlantısı, `wg-quick`/`wg syncconf` işlemleri ve hata durumunda runtime geri alma.
- NAT/forwarding/güvenlik duvarı ve sistem yeniden başlatma entegrasyonu.
- Gerçek düşük disk, büyük istemci sayısı ve uzun süreli yük altında kaynak tüketimi.
- Docker/Compose çalıştırması; ortamda Docker bulunmadığı için örnek dosyalar runtime testi görmedi.
- Go race detector: bu Windows ortamında C derleyicisi bulunmadığından çalıştırılmadı. Eşzamanlılık senaryosu normal Go testleriyle denetlendi.

Ekran görüntüleri yalnızca yerel çalışma alanında tutulur; herkese açık repoya eklenmez. Geçici önizleme sunucusu doğrulamadan sonra kapatılır; demoyu `start-demo.ps1` ile yeniden açabilirsiniz.

