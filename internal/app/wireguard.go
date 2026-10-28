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
		return nil, fmt.Errorf("%s operation failed; check the interface and permissions", name)
	}
	if len(b) > 2*1024*1024 {
		return nil, errors.New("WireGuard response exceeds the size limit")
	}
	return b, nil
}
func (l *LiveEngine) Read(ctx context.Context, s Server, _ []Peer) ([]RuntimePeer, bool, error) {
	if runtime.GOOS != "linux" {
		return nil, false, errors.New("Live WireGuard mode is only supported on Linux")
	}
	b, err := command(ctx, "wg", []string{"show", s.Interface, "dump"}, "")
	if err != nil {
		return nil, false, err
	}
	header := strings.Split(strings.SplitN(string(b), "\n", 2)[0], "\t")
	if len(header) != 4 || header[1] != s.PublicKey {
		return nil, false, errors.New("Existing WireGuard interface uses a different key; import the server configuration")
	}
	peers, err := parseDump(string(b))
	return peers, err == nil, err
}
func parseDump(raw string) ([]RuntimePeer, error) {
	out := []RuntimePeer{}
	scanner := bufio.NewScanner(strings.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 64*1024)
	if !scanner.Scan() {
		return nil, errors.New("WireGuard status response is empty")
	}
	if len(strings.Split(scanner.Text(), "\t")) != 4 {
		return nil, errors.New("Invalid WireGuard status header")
	}
	for scanner.Scan() {
		p := strings.Split(scanner.Text(), "\t")
		if len(p) != 8 || !validKey(p[0]) {
			return nil, errors.New("Invalid WireGuard client status response")
		}
		hs, e := strconv.ParseInt(p[4], 10, 64)
		if e != nil || hs < 0 {
			return nil, errors.New("Invalid WireGuard handshake information")
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
		return errors.New("WireGuard changes are disabled on this computer")
	}
	if err := s.Validate(); err != nil {
		return err
	}
	if !ValidInterface(s.Interface) {
		return errors.New("Invalid interface")
	}
	if err := os.MkdirAll(l.ConfigDir, 0700); err != nil {
		return err
	}
	path := filepath.Join(l.ConfigDir, s.Interface+".conf")
	if st, err := os.Lstat(path); err == nil && st.Mode()&os.ModeSymlink != 0 {
		return errors.New("Configuration file cannot be a symbolic link")
	}
	current, existsErr := command(ctx, "wg", []string{"show", s.Interface, "public-key"}, "")
	if existsErr == nil && strings.TrimSpace(string(current)) != s.PublicKey {
		return errors.New("Interface uses a different server key; import the existing configuration first")
	}
	var runtimeBackup []byte
	if existsErr == nil {
		addresses, err := command(ctx, "ip", []string{"-json", "address", "show", "dev", s.Interface}, "")
		if err != nil || !interfaceMatches(addresses, s) {
			return errors.New("Running interface address/MTU differs; reconcile during maintenance")
		}
		runtimeBackup, err = command(ctx, "wg", []string{"showconf", s.Interface}, "")
		if err != nil {
			return errors.New("Could not read existing WireGuard state for rollback")
		}
	} else if _, err := command(ctx, "ip", []string{"link", "show", "dev", s.Interface}, ""); err == nil {
		return errors.New("A network interface with this name already exists; no automatic changes made")
	}
	old, readErr := os.ReadFile(path)
	if readErr != nil && !os.IsNotExist(readErr) {
		return errors.New("Could not read existing configuration")
	}
	if readErr == nil && existsErr != nil {
		return errors.New("Existing configuration preserved; start with wg-quick and import it first")
	}
	// For an existing interface reject disruptive address/MTU changes. wg syncconf
	// can update peers and listen port without restarting existing sessions.
	if existsErr == nil && readErr == nil {
		previous, _, e := parseImport(string(old), s.Interface)
		if e != nil {
			return errors.New("Could not safely validate existing configuration")
		}
		if previous.Address != s.Address || previous.MTU != s.MTU {
			return errors.New("Change the running interface address/MTU during maintenance")
		}
	}
	if err := atomicFile(path, []byte(serverConfig(s, peers, true)), 0600); err != nil {
		return errors.New("Could not save WireGuard configuration")
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
			return errors.New("WireGuard operation and rollback failed; check the interface from the Linux console")
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
		return Server{}, nil, errors.New("File must be at most 256 KB")
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
				return s, nil, errors.New("Only one interface can be imported")
			}
			seenInterface = true
			section = "interface"
			continue
		}
		if line == "[Peer]" {
			if len(peers) >= 1024 {
				return s, nil, errors.New("Client limit is 1024")
			}
			peers = append(peers, Peer{ID: token(12), Name: fmt.Sprintf("Imported %d", len(peers)+1), Enabled: true, CreatedAt: time.Now().UTC()})
			peer = &peers[len(peers)-1]
			section = "peer"
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return s, nil, errors.New("Invalid configuration line")
		}
		k, v := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if section == "interface" {
			if seen[k] {
				return s, nil, errors.New("Duplicate interface field: " + k)
			}
			seen[k] = true
			switch k {
			case "PrivateKey":
				s.PrivateKey = v
			case "ListenPort":
				n, e := strconv.Atoi(v)
				if e != nil {
					return s, nil, errors.New("Invalid port")
				}
				s.Port = n
			case "Address":
				s.Address = v
			case "MTU":
				n, e := strconv.Atoi(v)
				if e != nil {
					return s, nil, errors.New("Invalid MTU")
				}
				s.MTU = n
			case "DNS":
				s.DNS = v
			default:
				return s, nil, errors.New("Unsupported interface field: " + k + " (hooks and commands are not accepted)")
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
					return s, nil, errors.New("Custom keepalive is not supported by this import version")
				}
			default:
				return s, nil, errors.New("Unsupported client field: " + k)
			}
		}
		if section == "" {
			return s, nil, errors.New("The [Interface] section must come first")
		}
	}
	if err := scanner.Err(); err != nil {
		return s, nil, errors.New("Could not read configuration file")
	}
	pub, err := publicFromPrivate(s.PrivateKey)
	if err != nil {
		return s, nil, errors.New("Invalid server private key")
	}
	s.PublicKey = pub
	if err = s.Validate(); err != nil {
		return s, nil, err
	}
	keys := map[string]bool{}
	addresses := map[string]bool{}
	for _, p := range peers {
		if !validKey(p.PublicKey) || (p.PresharedKey != "" && !validKey(p.PresharedKey)) {
			return s, nil, errors.New("Invalid client key")
		}
		if keys[p.PublicKey] || addresses[p.Address] {
			return s, nil, errors.New("Duplicate client key or address")
		}
		keys[p.PublicKey] = true
		addresses[p.Address] = true
		if err = validateCIDRs(p.Address); err != nil {
			return s, nil, err
		}
	}
	return s, peers, nil
}
