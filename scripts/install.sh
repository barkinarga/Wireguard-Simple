#!/usr/bin/env bash
# Target Debian/Ubuntu Linux server only. Safe to source for isolated tests.
set -euo pipefail

WGS_BIN_DIR=/usr/local/bin
WGS_STATE_DIR=/var/lib/wgsimple
WGS_CONFIG_DIR=/etc/wireguard
WGS_UNIT_DIR=/etc/systemd/system
WGS_INTERFACE=wg0
WGS_BINARY=
WGS_START=1
WGS_DEPS=1
WGS_CHECK=0
WGS_TEMP=
WGS_ROLLBACK=0
WGS_WAS_ACTIVE=0
WGS_WAS_ENABLED=0
WGS_UNIT_EXISTED=0
WGS_BINARY_EXISTED=0
WGS_START_ATTEMPTED=0

wgs_message() { printf '%s\n' "$*"; }
wgs_fail() { printf 'Hata: %s\n' "$*" >&2; return 1; }
wgs_usage() {
  cat <<'HELP'
WireGuard Simple — Debian/Ubuntu kurulumu

Kullanım:
  sudo bash install.sh [UYGULAMA_DOSYASI] [seçenekler]

Seçenekler:
  --binary DOSYA       Kurulacak yerel Linux uygulaması
  --interface AD       Yeni kurulum için WireGuard arayüzü (varsayılan: wg0)
  --no-start           Dosyaları kur; servisi başlatma/etkinleştirme
  --no-deps            Eksik paketleri otomatik kurma
  --check              Yalnızca ortamı ve dosyayı kontrol et; değişiklik yapma
  --help              Bu açıklamayı göster

Varsayılan: eksik araçları kurar, paneli kurar ve başlatır.
Mevcut istemciler, anahtarlar ve servis dosyası güncellemede korunur.
VPN arayüzü, NAT, forwarding, DNS ve güvenlik duvarı değiştirilmez.
HELP
}
wgs_arguments() {
  while (($#)); do
    case $1 in
      --binary) [[ $# -ge 2 && -z $WGS_BINARY ]] || { wgs_fail '--binary tek bir dosya ile kullanılmalı'; return 1; }; WGS_BINARY=$2; shift 2 ;;
      --interface) [[ $# -ge 2 ]] || { wgs_fail '--interface için ad gerekli'; return 1; }; WGS_INTERFACE=$2; shift 2 ;;
      --no-start) WGS_START=0; shift ;;
      --no-deps) WGS_DEPS=0; shift ;;
      --check) WGS_CHECK=1; shift ;;
      --help|-h) wgs_usage; return 2 ;;
      --*) wgs_fail "Bilinmeyen seçenek: $1"; return 1 ;;
      *) [[ -z $WGS_BINARY ]] || { wgs_fail 'Yalnızca bir uygulama dosyası belirtin'; return 1; }; WGS_BINARY=$1; shift ;;
    esac
  done
  [[ $WGS_INTERFACE =~ ^[a-zA-Z0-9][a-zA-Z0-9_-]{0,14}$ ]] || { wgs_fail 'Arayüz adı geçersiz'; return 1; }
}
wgs_arch() {
  case $(uname -m) in x86_64|amd64) printf amd64 ;; aarch64|arm64) printf arm64 ;; *) wgs_fail 'Desteklenen mimariler: amd64 ve arm64'; return 1 ;; esac
}
wgs_platform() {
  [[ $(uname -s) == Linux ]] || { wgs_fail 'Bu kurucu yalnızca Linux sunucusu içindir; Windows üzerinde çalışmaz'; return 1; }
  if [[ -r /proc/sys/kernel/osrelease ]] && grep -qi microsoft /proc/sys/kernel/osrelease; then
    wgs_fail 'Bu kurucu WSL üzerinde çalıştırılmaz; hedef Linux sunucusunu kullanın'; return 1
  fi
  [[ -f /etc/os-release ]] || { wgs_fail 'Linux dağıtımı belirlenemedi'; return 1; }
  local ID=
  source /etc/os-release
  [[ $ID == debian || $ID == ubuntu ]] || { wgs_fail 'Bu kurucu Debian ve Ubuntu içindir'; return 1; }
  command -v systemctl >/dev/null 2>&1 || { wgs_fail 'systemd/systemctl gerekiyor'; return 1; }
  [[ -d /run/systemd/system ]] || { wgs_fail 'systemd çalışan bir Linux sunucusu gerekiyor'; return 1; }
  wgs_arch >/dev/null
}
wgs_validate_binary() {
  local arch=$1 magic machine
  [[ -f $WGS_BINARY && -r $WGS_BINARY ]] || { wgs_fail 'Uygulama dosyası bulunamadı'; return 1; }
  magic=$(od -An -tx1 -N6 "$WGS_BINARY" | tr -d ' \n')
  [[ $magic == 7f454c460201 ]] || { wgs_fail 'Dosya 64 bit Linux ELF uygulaması değil'; return 1; }
  machine=$(od -An -tx1 -j18 -N2 "$WGS_BINARY" | tr -d ' \n')
  case "$arch:$machine" in amd64:3e00|arm64:b700) ;; *) wgs_fail 'Uygulama mimarisi sunucuyla eşleşmiyor'; return 1 ;; esac
}
wgs_dependencies() {
  local missing=0 tool
  for tool in wg wg-quick ip curl; do command -v "$tool" >/dev/null 2>&1 || missing=1; done
  if ((missing == 0)); then return 0; fi
  if ((WGS_CHECK)); then wgs_message 'Eksik araçlar kurulum sırasında tamamlanacak (wireguard-tools, iproute2, curl).'; return 0; fi
  ((WGS_DEPS)) || { wgs_fail 'Eksik araçlar var. wireguard-tools, iproute2 ve curl kurun veya --no-deps seçeneğini kaldırın'; return 1; }
  wgs_message 'Gerekli küçük sistem paketleri kuruluyor…'
  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends wireguard-tools iproute2 curl ca-certificates
}
wgs_write_unit() {
  cat > "$WGS_UNIT_DIR/wgsimple.service" <<UNIT
[Unit]
Description=WireGuard Simple management console
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
UMask=0077
ExecStart=$WGS_BIN_DIR/wgsimple --mode live --listen 127.0.0.1:8787 --data-dir $WGS_STATE_DIR --interface $WGS_INTERFACE
Restart=on-failure
RestartSec=5
WorkingDirectory=$WGS_STATE_DIR
Environment=GOMEMLIMIT=128MiB
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
PrivateDevices=true
ProtectKernelModules=true
ProtectKernelLogs=true
ProtectControlGroups=true
RestrictSUIDSGID=true
LockPersonality=true
RestrictRealtime=true
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK
CapabilityBoundingSet=CAP_NET_ADMIN
ReadWritePaths=$WGS_STATE_DIR $WGS_CONFIG_DIR
MemoryHigh=128M
MemoryMax=192M
TasksMax=64
StandardOutput=null
StandardError=null

[Install]
WantedBy=multi-user.target
UNIT
  chmod 0644 "$WGS_UNIT_DIR/wgsimple.service"
}
wgs_ready() {
  local attempt
  for attempt in {1..20}; do
    if systemctl is-active --quiet wgsimple.service && curl --noproxy '*' --fail --silent --max-time 2 http://127.0.0.1:8787/healthz >/dev/null 2>&1; then return 0; fi
    sleep 1
  done
  wgs_fail 'Panel sağlık kontrolüne yanıt vermedi; önceki kurulum geri alınacak. Özel servis ayarlarınız varsa --no-start kullanın'
}
wgs_replace_binary() {
  local source=$1 staged
  staged=$(mktemp "$WGS_BIN_DIR/.wgsimple.XXXXXX")
  if ! install -m 0755 "$source" "$staged"; then rm -f -- "$staged"; return 1; fi
  if ! mv -f -- "$staged" "$WGS_BIN_DIR/wgsimple"; then rm -f -- "$staged"; return 1; fi
}
wgs_rollback() {
  set +e
  local failed=0
  wgs_message 'Kurulum tamamlanamadı; önceki uygulama ve servis durumu geri yükleniyor.' >&2
  if ((WGS_WAS_ACTIVE || WGS_START_ATTEMPTED)); then
    if ! systemctl stop wgsimple.service >/dev/null 2>&1; then wgs_message "Servis durdurulamadı. Yedekler korunuyor: $WGS_TEMP" >&2; return 1; fi
  fi
  if ((WGS_BINARY_EXISTED)); then wgs_replace_binary "$WGS_TEMP/previous-binary" || failed=1; else rm -f -- "$WGS_BIN_DIR/wgsimple" || failed=1; fi
  if ((WGS_UNIT_EXISTED)); then cp -p -- "$WGS_TEMP/previous-unit" "$WGS_UNIT_DIR/wgsimple.service" || failed=1; else systemctl disable wgsimple.service >/dev/null 2>&1 || true; rm -f -- "$WGS_UNIT_DIR/wgsimple.service" || failed=1; fi
  if [[ -f $WGS_TEMP/previous-control.db ]]; then
    rm -f -- "$WGS_STATE_DIR/control.db-journal" "$WGS_STATE_DIR/control.db-wal" "$WGS_STATE_DIR/control.db-shm" || return 1
    local suffix
    for suffix in '' -journal -wal -shm; do
      if [[ -f $WGS_TEMP/previous-control.db$suffix ]]; then install -m 0600 "$WGS_TEMP/previous-control.db$suffix" "$WGS_STATE_DIR/control.db$suffix" || failed=1; fi
    done
  fi
  systemctl daemon-reload || failed=1
  if ((WGS_WAS_ENABLED)); then systemctl enable wgsimple.service >/dev/null 2>&1 || failed=1; elif ((WGS_UNIT_EXISTED)); then systemctl disable wgsimple.service >/dev/null 2>&1 || failed=1; fi
  if ((WGS_WAS_ACTIVE)); then systemctl start wgsimple.service || failed=1; fi
  if ((failed)); then wgs_message "Geri alma tam tamamlanamadı. Yedekler korunuyor: $WGS_TEMP" >&2; return 1; fi
  return 0
}
wgs_finish() {
  local status=$?
  trap - EXIT
  if ((status != 0 && WGS_ROLLBACK)); then if ! wgs_rollback; then exit "$status"; fi; fi
  if [[ -n $WGS_TEMP && -d $WGS_TEMP && ! -L $WGS_TEMP ]]; then
    case $(cd "$WGS_TEMP" && pwd -P) in /tmp/wgsimple-install.*) rm -rf -- "$WGS_TEMP" ;; *) printf '%s\n' 'Geçici klasör otomatik temizlenmedi.' >&2 ;; esac
  fi
  exit "$status"
}
wgs_install() {
  local destination=$WGS_BIN_DIR/wgsimple unit=$WGS_UNIT_DIR/wgsimple.service
  [[ ! -L $destination && ! -L $unit && ! -L $WGS_STATE_DIR && ! -L $WGS_CONFIG_DIR ]] || { wgs_fail 'Kurulum hedeflerinde sembolik bağlantı bulundu; otomatik değişiklik yapılmadı'; return 1; }
  if systemctl is-active --quiet wgsimple.service; then WGS_WAS_ACTIVE=1; fi
  if systemctl is-enabled --quiet wgsimple.service; then WGS_WAS_ENABLED=1; fi
  if [[ -f $destination ]]; then WGS_BINARY_EXISTED=1; cp -p -- "$destination" "$WGS_TEMP/previous-binary"; fi
  if [[ -f $unit ]]; then WGS_UNIT_EXISTED=1; cp -p -- "$unit" "$WGS_TEMP/previous-unit"; fi
  WGS_ROLLBACK=1
  if ((WGS_WAS_ACTIVE)); then systemctl stop wgsimple.service; fi
  local suffix
  for suffix in '' -journal -wal -shm; do
    if [[ -f $WGS_STATE_DIR/control.db$suffix ]]; then cp -p -- "$WGS_STATE_DIR/control.db$suffix" "$WGS_TEMP/previous-control.db$suffix"; fi
  done
  install -d -m 0755 "$WGS_BIN_DIR" "$WGS_UNIT_DIR"
  install -d -m 0700 "$WGS_STATE_DIR" "$WGS_CONFIG_DIR"
  wgs_replace_binary "$WGS_BINARY"
  if ((WGS_UNIT_EXISTED == 0)); then wgs_write_unit; else wgs_message 'Mevcut servis ayarları korunuyor.'; fi
  systemctl daemon-reload
  if ((WGS_START)); then WGS_START_ATTEMPTED=1; systemctl enable --now wgsimple.service; wgs_ready; fi
  WGS_ROLLBACK=0
  wgs_message ''
  wgs_message 'WireGuard Simple kuruldu.'
  if ((WGS_START)); then wgs_message 'Panel hazır: http://127.0.0.1:8787'; else wgs_message 'Paneli başlatmak için: sudo systemctl enable --now wgsimple.service'; fi
  wgs_message 'Kendi bilgisayarınızdan: ssh -L 8787:127.0.0.1:8787 KULLANICI@SUNUCU_IP'
  wgs_message "İlk kurulum anahtarı varsa sunucuda kendiniz okuyun: sudo cat $WGS_STATE_DIR/setup-token"
  wgs_message 'İstemci verileri korunur; trafik geçmişi ve log tercihleriniz değiştirilmez.'
  wgs_message 'NAT, DNS, forwarding, güvenlik duvarı ve VPN arayüzleri değiştirilmedi.'
}
wgs_main() {
  local result=0 arch script_dir
  wgs_arguments "$@" || result=$?
  if ((result == 2)); then return 0; elif ((result != 0)); then return "$result"; fi
  export PATH=/usr/sbin:/usr/bin:/sbin:/bin
  wgs_platform
  arch=$(wgs_arch)
  if [[ -z $WGS_BINARY ]]; then
    script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
    if [[ -f $script_dir/wgsimple ]]; then WGS_BINARY=$script_dir/wgsimple; else WGS_BINARY=$script_dir/../bin/wgsimple-linux-$arch; fi
  fi
  wgs_validate_binary "$arch"
  if ((WGS_CHECK)); then wgs_dependencies; wgs_message "Kontrol tamam: $arch / $WGS_INTERFACE. Hiçbir değişiklik yapılmadı."; return 0; fi
  [[ $EUID -eq 0 ]] || { wgs_fail 'Hedef Linux sunucusunda sudo ile çalıştırın'; return 1; }
  umask 077
  WGS_TEMP=$(mktemp -d /tmp/wgsimple-install.XXXXXX)
  trap wgs_finish EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
  wgs_dependencies
  wgs_install
}

if [[ ${BASH_SOURCE[0]} == "$0" ]]; then wgs_main "$@"; fi
