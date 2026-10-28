# Kurucu doğrulaması — 28 Ekim 2026

Yeni kurulum yolu `release/wgsimple-install.run` dosyasıdır. Dosya amd64 ve arm64 uygulamalarını birlikte taşır; hedefte mimariyi seçer, gömülü paketin SHA-256 özetini denetler ve ayrı bir geliştirme ortamı kurmadan uygulamayı yükler.

Yerel doğrulama:

- 3 Node testi geçti: arşiv yolu güvenliği, boş argüman desteği, kurucudaki gerçek ikili dosyaların ve gömülü SHA-256 özetinin doğruluğu. Bozuk payload özeti eşleşmez.
- Oluşturulan arşiv işletim sisteminin `tar` aracıyla da başarıyla listelendi.
- İzole Bash senaryoları geçti: seçenek doğrulama, mimari/ELF kontrolü, eksik araç/no-deps/check akışları, ilk kurulum, mevcut yapılandırmayı koruyan güncelleme, başlatmama ve başarısız başlangıçta geri alma.
- Servis işlemleri ile paket yöneticisi bu testlerde sahteydi. Bütün test dosyaları proje içindeki geçici klasörlerdeydi. Windows ACL değişiklikleri yapılmadı; POSIX izin işlemleri Windows testlerinde taklit edildi.
- Windows Bash ortamında gerçek dosya symlink'i üretilemediği için symlink senaryosu atlandı. Linux CI iş akışında bu senaryo gerçek symlink ile çalıştırılır.
- Gerçek sistem paket kurulumu, systemd servisi başlatma ve VPN/ağ değişikliği bu Windows bilgisayarında yapılmadı.

GitHub iş akışı kaynak koddan arayüzü ve uygulamaları derler; Go race testlerini, kurucu senaryolarını ve paket kontrollerini Linux üzerinde çalıştırır. Sürüm yayımlama bu adımların başarılı olmasına bağlıdır. Sürüm komiti `release: v0.1.0` biçimindedir; sonradan yayımlanmış aynı sürümün dosyaları otomatik değiştirilmez.

Bu kontroller gerçek bir Linux sunucusunda üretim kurulumu, NAT/forwarding veya WireGuard istemci bağlantısı testi yerine geçmez.
