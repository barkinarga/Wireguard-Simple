package app

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// LoopbackListener checks the bind address, never a request's Host header.
func LoopbackListener(listen string) (bool, error) {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return false, err
	}
	ip := net.ParseIP(host)
	return host == "localhost" || ip != nil && ip.IsLoopback(), nil
}

func ValidatePanelOrigin(origin string, loopback bool) error {
	if origin == "" {
		return nil
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.User != nil || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("invalid canonical origin")
	}
	if !loopback && u.Scheme != "https" {
		return errors.New("public origin must use HTTPS")
	}
	return nil
}

// PanelTLS preserves explicit certificates and loopback HTTP. All other binds
// get a persistent private, self-signed identity; no public plaintext fallback.
// The caller holds the data directory lock and has opened its private store.
func PanelTLS(listen, origin, dir, cert, key string) (string, string, error) {
	loopback, err := LoopbackListener(listen)
	if err != nil {
		return "", "", err
	}
	if err := ValidatePanelOrigin(origin, loopback); err != nil {
		return "", "", err
	}
	if (cert == "") != (key == "") {
		return "", "", errors.New("TLS certificate and key must be specified together")
	}
	if cert != "" {
		if _, err := tls.LoadX509KeyPair(cert, key); err != nil {
			return "", "", errors.New("Could not load TLS certificate/key; no HTTP fallback")
		}
		return cert, key, nil
	}
	if loopback {
		return "", "", nil
	}
	path := filepath.Join(dir, "panel-tls.pem")
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
			return "", "", errors.New("Automatic TLS identity must be a private regular file (0600)")
		}
		pair, err := tls.LoadX509KeyPair(path, path)
		if err != nil {
			return "", "", errors.New("Invalid automatic TLS identity; restore panel-tls.pem or replace it while the service is stopped")
		}
		leaf, err := x509.ParseCertificate(pair.Certificate[0])
		if err != nil || time.Now().Before(leaf.NotBefore) || !time.Now().Before(leaf.NotAfter) {
			return "", "", errors.New("Automatic TLS certificate is outside its validity period; check the clock or replace panel-tls.pem while stopped")
		}
		if leaf.PublicKeyAlgorithm != x509.Ed25519 {
			return path, path, nil
		}
		// v0.4.2 generated Ed25519 identities, which Chrome does not advertise
		// in its TLS verification preferences. Migrate only our automatic file;
		// operator-supplied certificates above are never replaced.
	} else if !os.IsNotExist(err) {
		return "", "", err
	}
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return "", "", err
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "WireGuard Simple panel"},
		NotBefore:    now.Add(-5 * time.Minute), NotAfter: now.AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	addName := func(host string) {
		if ip := net.ParseIP(host); ip != nil {
			if !ip.IsUnspecified() {
				template.IPAddresses = append(template.IPAddresses, ip)
			}
		} else if host != "" && host != "localhost" {
			template.DNSNames = append(template.DNSNames, host)
		}
	}
	host, _, _ := net.SplitHostPort(listen)
	addName(host)
	if u, err := url.Parse(origin); err == nil {
		addName(u.Hostname())
	}
	if addresses, err := net.InterfaceAddrs(); err == nil {
		for _, address := range addresses {
			if ip, _, err := net.ParseCIDR(address.String()); err == nil {
				addName(ip.String())
			}
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &private.PublicKey, private)
	if err != nil {
		return "", "", err
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return "", "", err
	}
	// One atomic file prevents a crash between publication of certificate and key.
	data := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	data = append(data, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})...)
	if err := atomicFile(path, data, 0600); err != nil {
		return "", "", err
	}
	return path, path, nil
}
