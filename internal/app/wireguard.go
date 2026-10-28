package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Engine interface {
	Read(context.Context, Server, []Peer) ([]RuntimePeer, bool, error)
	Apply(context.Context, Server, []Peer) error
}

type DemoEngine struct{ started time.Time }

func NewDemoEngine() *DemoEngine { return &DemoEngine{started: time.Now()} }
func (d *DemoEngine) Read(_ context.Context, s Server, peers []Peer) ([]RuntimePeer, bool, error) {
	out := []RuntimePeer{}
	elapsed := uint64(time.Since(d.started).Seconds())
	for i, p := range peers {
		if !p.Enabled {
			continue
		}
		handshake := time.Now().Add(-time.Duration(15+i*11) * time.Second)
		rx := uint64(270+i*131)*1024*1024 + elapsed*uint64(17000+i*11000)
		tx := uint64(65+i*43)*1024*1024 + elapsed*uint64(6000+i*3000)
		if i == 3 {
			handshake = time.Now().Add(-4 * time.Hour)
			rx = 92432
			tx = 48293
		}
		out = append(out, RuntimePeer{PublicKey: p.PublicKey, Endpoint: fmt.Sprintf("198.51.100.%d:%d", 24+i*7, 42000+i*419), Handshake: handshake, Rx: rx, Tx: tx})
	}
	return out, true, nil
}
func (d *DemoEngine) Apply(context.Context, Server, []Peer) error { return nil }

type LiveEngine struct{ ConfigDir string }

func command(ctx context.Context, name string, args []string, input string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	// Never include stdout/stderr in errors: wg dump contains private keys.
	b, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s işlemi tamamlanamadı; arayüz ve yetkileri kontrol edin", name)
	}
	if len(b) > 2*1024*1024 {
		return nil, errors.New("WireGuard yanıtı boyut sınırını aşıyor")
	}
	return b, nil
}
func (l *LiveEngine) Read(ctx context.Context, s Server, _ []Peer) ([]RuntimePeer, bool, error) {
	if runtime.GOOS != "linux" {
		return nil, false, errors.New("canlı WireGuard modu yalnızca Linux üzerinde çalışır")
	}
	b, err := command(ctx, "wg", []string{"show", s.Interface, "dump"}, "")
	if err != nil {
		return nil, false, err
	}
	header := strings.Split(strings.SplitN(string(b), "\n", 2)[0], "\t")
	if len(header) != 4 || header[1] != s.PublicKey {
		return nil, false, errors.New("mevcut WireGuard arayüzü farklı bir anahtar kullanıyor; sunucu yapılandırmasını içe aktarın")
	}
	peers, err := parseDump(string(b))
	return peers, err == nil, err
}
func parseDump(raw string) ([]RuntimePeer, error) {
	out := []RuntimePeer{}
	scanner := bufio.NewScanner(strings.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 64*1024)
	if !scanner.Scan() {
		return nil, errors.New("WireGuard durum yanıtı boş")
	}
	if len(strings.Split(scanner.Text(), "\t")) != 4 {
		return nil, errors.New("WireGuard durum başlığı geçersiz")
	}
	for scanner.Scan() {
		p := strings.Split(scanner.Text(), "\t")
		if len(p) != 8 || !validKey(p[0]) {
			return nil, errors.New("WireGuard istemci durum yanıtı geçersiz")
		}
		hs, e := strconv.ParseInt(p[4], 10, 64)
		if e != nil || hs < 0 {
			return nil, errors.New("WireGuard handshake bilgisi geçersiz")
		}
		rx, e := strconv.ParseUint(p[5], 10, 64)
		if e != nil {
			return nil, e
		}
		tx, e := strconv.ParseUint(p[6], 10, 64)
		if e != nil {
			return nil, e
		}
		var t time.Time
		if hs > 0 {
			t = time.Unix(hs, 0)
		}
		ep := p[2]
		if ep == "(none)" {
			ep = ""
		}
		out = append(out, RuntimePeer{PublicKey: p[0], Endpoint: ep, Handshake: t, Rx: rx, Tx: tx})
	}
	return out, scanner.Err()
}
func serverConfig(s Server, peers []Peer, quick bool) string {
	var b strings.Builder
	b.WriteString("# Managed by WireGuard Simple. Do not edit while the panel is running.\n[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\nListenPort = %d\n", s.PrivateKey, s.Port)
	if quick {
		fmt.Fprintf(&b, "Address = %s\nMTU = %d\n", s.Address, s.MTU)
	}
	for _, p := range peers {
		if !p.Enabled {
			continue
		}
		fmt.Fprintf(&b, "\n[Peer]\nPublicKey = %s\nAllowedIPs = %s\n", p.PublicKey, p.Address)
		if p.PresharedKey != "" {
			fmt.Fprintf(&b, "PresharedKey = %s\n", p.PresharedKey)
		}
	}
	return b.String()
}
func (l *LiveEngine) Apply(ctx context.Context, s Server, peers []Peer) error {
	if runtime.GOOS != "linux" {
		return errors.New("bu bilgisayarda WireGuard değişiklikleri devre dışı")
	}
	if err := s.Validate(); err != nil {
		return err
	}
	if !ValidInterface(s.Interface) {
		return errors.New("geçersiz arayüz")
	}
	if err := os.MkdirAll(l.ConfigDir, 0700); err != nil {
		return err
	}
	path := filepath.Join(l.ConfigDir, s.Interface+".conf")
	if st, err := os.Lstat(path); err == nil && st.Mode()&os.ModeSymlink != 0 {
		return errors.New("sembolik bağlantı olan yapılandırma dosyası kullanılamaz")
	}
	current, existsErr := command(ctx, "wg", []string{"show", s.Interface, "public-key"}, "")
	if existsErr == nil && strings.TrimSpace(string(current)) != s.PublicKey {
		return errors.New("arayüz başka bir sunucu anahtarı kullanıyor; önce mevcut yapılandırmayı içe aktarın")
	}
	var runtimeBackup []byte
	if existsErr == nil {
		addresses, err := command(ctx, "ip", []string{"-json", "address", "show", "dev", s.Interface}, "")
		if err != nil || !interfaceMatches(addresses, s) {
			return errors.New("çalışan arayüzün adres/MTU bilgisi farklı; bakım sırasında eşleştirin")
		}
		runtimeBackup, err = command(ctx, "wg", []string{"showconf", s.Interface}, "")
		if err != nil {
			return errors.New("mevcut WireGuard durumu geri alma için okunamadı")
		}
	} else if _, err := command(ctx, "ip", []string{"link", "show", "dev", s.Interface}, ""); err == nil {
		return errors.New("bu isimde bir ağ arayüzü zaten var; otomatik değişiklik yapılmadı")
	}
	old, readErr := os.ReadFile(path)
	if readErr != nil && !os.IsNotExist(readErr) {
		return errors.New("mevcut yapılandırma okunamadı")
	}
	if readErr == nil && existsErr != nil {
		return errors.New("mevcut yapılandırma korunuyor; önce arayüzü wg-quick ile başlatın ve yapılandırmayı içe aktarın")
	}
	// For an existing interface reject disruptive address/MTU changes. wg syncconf
	// can update peers and listen port without restarting existing sessions.
	if existsErr == nil && readErr == nil {
		previous, _, e := parseImport(string(old), s.Interface)
		if e != nil {
			return errors.New("mevcut yapılandırma güvenle doğrulanamadı")
		}
		if previous.Address != s.Address || previous.MTU != s.MTU {
			return errors.New("çalışan arayüzün adres/MTU değişikliği bakım sırasında yapılmalı")
		}
	}
	if err := atomicFile(path, []byte(serverConfig(s, peers, true)), 0600); err != nil {
		return errors.New("WireGuard yapılandırması kaydedilemedi")
	}
	var err error
	if existsErr == nil {
		_, err = command(ctx, "wg", []string{"syncconf", s.Interface, "/dev/stdin"}, serverConfig(s, peers, false))
	} else {
		_, err = command(ctx, "wg-quick", []string{"up", path}, "")
	}
	if err != nil {
		cleanupCtx := context.WithoutCancel(ctx)
		var rollbackErr error
		if existsErr == nil {
			_, rollbackErr = command(cleanupCtx, "wg", []string{"syncconf", s.Interface, "/dev/stdin"}, string(runtimeBackup))
		} else if key, e := command(cleanupCtx, "wg", []string{"show", s.Interface, "public-key"}, ""); e == nil && strings.TrimSpace(string(key)) == s.PublicKey {
			_, rollbackErr = command(cleanupCtx, "wg-quick", []string{"down", path}, "")
		}
		var fileErr error
		if readErr == nil {
			fileErr = atomicFile(path, old, 0600)
		} else {
			fileErr = os.Remove(path)
		}
		if rollbackErr != nil || (fileErr != nil && !os.IsNotExist(fileErr)) {
			return errors.New("WireGuard işlemi ve geri alma tamamlanamadı; arayüzü Linux konsolundan kontrol edin")
		}
		return err
	}
	return nil
}

func interfaceMatches(raw []byte, s Server) bool {
	var links []struct {
		MTU       int `json:"mtu"`
		Addresses []struct {
			Local  string `json:"local"`
			Prefix int    `json:"prefixlen"`
		} `json:"addr_info"`
	}
	if json.Unmarshal(raw, &links) != nil || len(links) != 1 || links[0].MTU != s.MTU {
		return false
	}
	for _, addr := range links[0].Addresses {
		if fmt.Sprintf("%s/%d", addr.Local, addr.Prefix) == s.Address {
			return true
		}
	}
	return false
}

// Import configuration data only. Hooks and other shell-bearing directives are
// rejected; no imported command is ever executed.
func parseImport(raw, iface string) (Server, []Peer, error) {
	if len(raw) > 256*1024 {
		return Server{}, nil, errors.New("dosya en fazla 256 KB olabilir")
	}
	s := DefaultServer(iface)
	s.Endpoint = ""
	s.PrivateKey = ""
	s.PublicKey = ""
	peers := []Peer{}
	section := ""
	seenInterface := false
	seen := map[string]bool{}
	var peer *Peer
	scanner := bufio.NewScanner(strings.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 64*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(strings.SplitN(scanner.Text(), "#", 2)[0])
		if line == "" {
			continue
		}
		if line == "[Interface]" {
			if seenInterface {
				return s, nil, errors.New("yalnızca bir arayüz içe aktarılabilir")
			}
			seenInterface = true
			section = "interface"
			continue
		}
		if line == "[Peer]" {
			if len(peers) >= 1024 {
				return s, nil, errors.New("istemci sınırı 1024")
			}
			peers = append(peers, Peer{ID: token(12), Name: fmt.Sprintf("İçe aktarılan %d", len(peers)+1), Enabled: true, CreatedAt: time.Now().UTC()})
			peer = &peers[len(peers)-1]
			section = "peer"
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return s, nil, errors.New("yapılandırma satırı geçersiz")
		}
		k, v := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if section == "interface" {
			if seen[k] {
				return s, nil, errors.New("yinelenen arayüz alanı: " + k)
			}
			seen[k] = true
			switch k {
			case "PrivateKey":
				s.PrivateKey = v
			case "ListenPort":
				n, e := strconv.Atoi(v)
				if e != nil {
					return s, nil, errors.New("port geçersiz")
				}
				s.Port = n
			case "Address":
				s.Address = v
			case "MTU":
				n, e := strconv.Atoi(v)
				if e != nil {
					return s, nil, errors.New("MTU geçersiz")
				}
				s.MTU = n
			case "DNS":
				s.DNS = v
			default:
				return s, nil, errors.New("desteklenmeyen arayüz alanı: " + k + " (hook ve komutlar kabul edilmez)")
			}
		}
		if section == "peer" && peer != nil {
			switch k {
			case "PublicKey":
				peer.PublicKey = v
			case "PresharedKey":
				peer.PresharedKey = v
			case "AllowedIPs":
				peer.Address = v
			case "PersistentKeepalive":
				if v != "0" && v != "25" {
					return s, nil, errors.New("özel keepalive ayarı bu içe aktarma sürümünde desteklenmiyor")
				}
			default:
				return s, nil, errors.New("desteklenmeyen istemci alanı: " + k)
			}
		}
		if section == "" {
			return s, nil, errors.New("önce [Interface] bölümü gelmeli")
		}
	}
	if err := scanner.Err(); err != nil {
		return s, nil, errors.New("yapılandırma dosyası okunamadı")
	}
	pub, err := publicFromPrivate(s.PrivateKey)
	if err != nil {
		return s, nil, errors.New("sunucu özel anahtarı geçersiz")
	}
	s.PublicKey = pub
	if err = s.Validate(); err != nil {
		return s, nil, err
	}
	keys := map[string]bool{}
	addresses := map[string]bool{}
	for _, p := range peers {
		if !validKey(p.PublicKey) || (p.PresharedKey != "" && !validKey(p.PresharedKey)) {
			return s, nil, errors.New("istemci anahtarı geçersiz")
		}
		if keys[p.PublicKey] || addresses[p.Address] {
			return s, nil, errors.New("yinelenen istemci anahtarı veya adresi")
		}
		keys[p.PublicKey] = true
		addresses[p.Address] = true
		if err = validateCIDRs(p.Address); err != nil {
			return s, nil, err
		}
	}
	return s, peers, nil
}
