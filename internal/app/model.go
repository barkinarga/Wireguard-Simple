package app

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"strings"
	"time"
)

type Settings struct {
	HistoryEnabled bool `json:"historyEnabled"`
	AuditEnabled   bool `json:"auditEnabled"`
	MetricsEnabled bool `json:"metricsEnabled"`
	RetentionDays  int  `json:"retentionDays"`
	BudgetMB       int  `json:"budgetMB"`
	SampleSeconds  int  `json:"sampleSeconds"`
}

func DefaultSettings() Settings { return Settings{RetentionDays: 7, BudgetMB: 32, SampleSeconds: 60} }
func (s Settings) Validate() error {
	if s.RetentionDays < 1 || s.RetentionDays > 30 {
		return errors.New("Retention must be 1–30 days")
	}
	if s.BudgetMB < 16 || s.BudgetMB > 512 {
		return errors.New("Storage budget must be 16–512 MB")
	}
	if s.SampleSeconds < 30 || s.SampleSeconds > 3600 {
		return errors.New("Sampling interval must be 30–3600 seconds")
	}
	return nil
}

type Server struct {
	Name       string `json:"name"`
	Interface  string `json:"interface"`
	Address    string `json:"address"`
	Endpoint   string `json:"endpoint"`
	Port       int    `json:"port"`
	DNS        string `json:"dns"`
	AllowedIPs string `json:"allowedIPs"`
	MTU        int    `json:"mtu"`
	PrivateKey string `json:"privateKey,omitempty"`
	PublicKey  string `json:"publicKey"`
}

func DefaultServer(iface string) Server {
	priv, pub, _ := newKeyPair()
	return Server{Name: "Main server", Interface: iface, Address: "10.64.0.1/24", Port: 51820, DNS: "1.1.1.1, 1.0.0.1", AllowedIPs: "0.0.0.0/0", MTU: 1420, PrivateKey: priv, PublicKey: pub}
}

var interfacePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,14}$`)

func ValidInterface(s string) bool { return interfacePattern.MatchString(s) }
func validName(s string) bool {
	if len([]rune(s)) < 1 || len([]rune(s)) > 64 {
		return false
	}
	for _, r := range s {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}

func (s Server) Validate() error {
	if !validName(s.Name) || !ValidInterface(s.Interface) {
		return errors.New("Invalid server or interface name")
	}
	p, err := netip.ParsePrefix(s.Address)
	if err != nil || !p.Addr().Is4() || p.Bits() < 16 || p.Bits() > 30 {
		return errors.New("Server address must be an IPv4 network between /16 and /30")
	}
	if s.Port < 1 || s.Port > 65535 || s.MTU < 1280 || s.MTU > 1500 {
		return errors.New("Invalid port or MTU")
	}
	if s.Endpoint != "" {
		if len(s.Endpoint) > 253 || strings.ContainsAny(s.Endpoint, "\r\n\t /#;=\"'") {
			return errors.New("Endpoint must be an IP address or domain only")
		}
		if ip, e := netip.ParseAddr(s.Endpoint); e != nil || !ip.IsValid() {
			for _, part := range strings.Split(s.Endpoint, ".") {
				if len(part) < 1 || len(part) > 63 || strings.HasPrefix(part, "-") || strings.HasSuffix(part, "-") {
					return errors.New("Invalid endpoint domain")
				}
				for _, c := range part {
					if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
						return errors.New("Invalid endpoint domain")
					}
				}
			}
		}
	}
	for _, d := range strings.Split(s.DNS, ",") {
		if _, err := netip.ParseAddr(strings.TrimSpace(d)); err != nil {
			return errors.New("DNS addresses must be IP addresses")
		}
	}
	if err := validateCIDRs(s.AllowedIPs); err != nil {
		return err
	}
	if !validKey(s.PrivateKey) || !validKey(s.PublicKey) {
		return errors.New("Invalid server key")
	}
	if pub, err := publicFromPrivate(s.PrivateKey); err != nil || pub != s.PublicKey {
		return errors.New("Server key pair does not match")
	}
	return nil
}

type Peer struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Address      string     `json:"address"`
	PublicKey    string     `json:"publicKey"`
	PrivateKey   string     `json:"privateKey,omitempty"`
	PresharedKey string     `json:"presharedKey,omitempty"`
	Enabled      bool       `json:"enabled"`
	CreatedAt    time.Time  `json:"createdAt"`
	ExpiresAt    *time.Time `json:"expiresAt"`
}

type PeerView struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Address       string     `json:"address"`
	PublicKey     string     `json:"publicKey"`
	Enabled       bool       `json:"enabled"`
	CreatedAt     time.Time  `json:"createdAt"`
	ExpiresAt     *time.Time `json:"expiresAt"`
	HasConfig     bool       `json:"hasConfig"`
	Status        string     `json:"status"`
	Endpoint      string     `json:"endpoint"`
	LastHandshake *time.Time `json:"lastHandshake"`
	RxBytes       uint64     `json:"rxBytes"`
	TxBytes       uint64     `json:"txBytes"`
	RxRate        float64    `json:"rxRate"`
	TxRate        float64    `json:"txRate"`
}

type RuntimePeer struct {
	PublicKey string
	Endpoint  string
	Handshake time.Time
	Rx, Tx    uint64
}

type Point struct {
	Time   time.Time `json:"time"`
	Rx     float64   `json:"rx"`
	Tx     float64   `json:"tx"`
	Active int       `json:"active"`
}

type Event struct {
	ID     string    `json:"id"`
	Time   time.Time `json:"time"`
	Level  string    `json:"level"`
	Action string    `json:"action"`
	Detail string    `json:"detail"`
}

type Resources struct {
	AppMemory       uint64  `json:"appMemory"`
	Goroutines      int     `json:"goroutines"`
	HostMemoryTotal uint64  `json:"hostMemoryTotal"`
	HostMemoryUsed  uint64  `json:"hostMemoryUsed"`
	DiskFree        uint64  `json:"diskFree"`
	DiskTotal       uint64  `json:"diskTotal"`
	HostCPU         float64 `json:"hostCPU"`
	ControlBytes    int64   `json:"controlBytes"`
	TelemetryBytes  int64   `json:"telemetryBytes"`
}

type Snapshot struct {
	Mode          string     `json:"mode"`
	Version       string     `json:"version"`
	StartedAt     time.Time  `json:"startedAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	Server        Server     `json:"server"`
	Settings      Settings   `json:"settings"`
	Peers         []PeerView `json:"peers"`
	Points        []Point    `json:"points"`
	Events        []Event    `json:"events"`
	Resources     Resources  `json:"resources"`
	Warnings      []string   `json:"warnings"`
	InterfaceUp   bool       `json:"interfaceUp"`
	HistoryPaused bool       `json:"historyPaused"`
}

func token(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func newKeyPair() (string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	b[0] &= 248
	b[31] &= 127
	b[31] |= 64
	priv := base64.StdEncoding.EncodeToString(b)
	pub, err := publicFromPrivate(priv)
	return priv, pub, err
}
func publicFromPrivate(s string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	k, err := ecdh.X25519().NewPrivateKey(b)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(k.PublicKey().Bytes()), nil
}
func validKey(s string) bool {
	b, e := base64.StdEncoding.DecodeString(s)
	return e == nil && len(b) == 32
}
func validateCIDRs(s string) error {
	if len(s) > 2048 || strings.TrimSpace(s) == "" {
		return errors.New("Allowed IPs is empty or too long")
	}
	for _, v := range strings.Split(s, ",") {
		if _, e := netip.ParsePrefix(strings.TrimSpace(v)); e != nil {
			return errors.New("Allowed IPs must contain valid CIDR addresses")
		}
	}
	return nil
}

func splitCIDRs(s string) []string {
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
func publicServer(s Server) Server { s.PrivateKey = ""; return s }
func clientConfig(s Server, p Peer) (string, error) {
	if p.PrivateKey == "" {
		return "", errors.New("The imported client's private key is not available on the server")
	}
	if s.Endpoint == "" {
		return "", errors.New("Set the server endpoint first")
	}
	psk := ""
	if p.PresharedKey != "" {
		psk = "PresharedKey = " + p.PresharedKey + "\n"
	}
	return fmt.Sprintf("[Interface]\nPrivateKey = %s\nAddress = %s\nDNS = %s\nMTU = %d\n\n[Peer]\nPublicKey = %s\n%sAllowedIPs = %s\nEndpoint = %s\nPersistentKeepalive = 25\n", p.PrivateKey, p.Address, s.DNS, s.MTU, s.PublicKey, psk, s.AllowedIPs, net.JoinHostPort(s.Endpoint, fmt.Sprint(s.Port))), nil
}
