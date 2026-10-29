package app

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestPanelTLSBindBoundary(t *testing.T) {
	for _, origin := range []string{"http://203.0.113.5:8787", "https://vpn.example.com/path", "https://user@vpn.example.com", "https://vpn.example.com?x=1", "https://vpn.example.com#fragment"} {
		if _, _, err := PanelTLS("0.0.0.0:8787", origin, t.TempDir(), "", ""); err == nil {
			t.Fatalf("unsafe public origin accepted: %s", origin)
		}
	}
	if err := ValidatePanelOrigin("http://localhost:8787", true); err != nil {
		t.Fatal("local HTTP origin rejected", err)
	}
	for _, listen := range []string{"127.0.0.1:8787", "[::1]:8787", "[::ffff:127.0.0.1]:8787", "localhost:8787"} {
		dir := t.TempDir()
		cert, key, err := PanelTLS(listen, "", dir, "", "")
		if err != nil || cert != "" || key != "" {
			t.Fatalf("loopback HTTP changed for %s: %v", listen, err)
		}
		if _, err := os.Stat(filepath.Join(dir, "panel-tls.pem")); !os.IsNotExist(err) {
			t.Fatal("loopback unexpectedly creates certificate")
		}
	}
	for _, listen := range []string{"0.0.0.0:8787", ":8787", "[::]:8787", "192.168.1.2:8787", "203.0.113.5:8787", "vpn.example.com:8787"} {
		cert, key, err := PanelTLS(listen, "", t.TempDir(), "", "")
		if err != nil || cert == "" || key != cert {
			t.Fatalf("public listener lacks TLS: %s: %v", listen, err)
		}
		// Must reject before binding; these addresses need not exist locally.
		if err := Serve(context.Background(), http.NotFoundHandler(), listen, "", ""); err == nil {
			t.Fatalf("public plaintext accepted: %s", listen)
		}
	}
	for _, pair := range [][2]string{{"missing-cert", ""}, {"", "missing-key"}} {
		if _, _, err := PanelTLS("127.0.0.1:8787", "", t.TempDir(), pair[0], pair[1]); err == nil {
			t.Fatal("partial certificate pair accepted")
		}
		if err := Serve(context.Background(), http.NotFoundHandler(), "127.0.0.1:8787", pair[0], pair[1]); err == nil {
			t.Fatal("Serve accepted partial TLS pair")
		}
	}
}

func TestAutomaticTLSRejectsUnsafeIdentity(t *testing.T) {
	for _, future := range []bool{false, true} {
		dir := t.TempDir()
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		start, end := time.Now().Add(-48*time.Hour), time.Now().Add(-24*time.Hour)
		if future {
			start, end = time.Now().Add(24*time.Hour), time.Now().Add(48*time.Hour)
		}
		template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: start, NotAfter: end}
		der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
		if err != nil {
			t.Fatal(err)
		}
		privateDER, err := x509.MarshalPKCS8PrivateKey(private)
		if err != nil {
			t.Fatal(err)
		}
		data := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
		data = append(data, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})...)
		if err := os.WriteFile(filepath.Join(dir, "panel-tls.pem"), data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := PanelTLS("0.0.0.0:8787", "", dir, "", ""); err == nil {
			t.Fatal("automatic certificate outside validity period accepted")
		}
	}
	t.Run("symlink", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(t.TempDir(), "identity")
		if err := os.WriteFile(target, []byte("unchanged"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(dir, "panel-tls.pem")); err != nil {
			t.Skip("OS does not allow symlink creation")
		}
		if _, _, err := PanelTLS("0.0.0.0:8787", "", dir, "", ""); err == nil {
			t.Fatal("symlinked TLS identity accepted")
		}
		data, _ := os.ReadFile(target)
		if string(data) != "unchanged" {
			t.Fatal("symlink target modified")
		}
	})
}

func TestServeHTTPSSetupLoginAndPlaintextRejection(t *testing.T) {
	_, service, base := fixture(t, "live")
	cert, key, err := PanelTLS("0.0.0.0:8787", "", t.TempDir(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	pair, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(leaf)
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: time.Second}
	handler, err := NewHTTP(service, base.files, "", cert != "")
	if err != nil {
		t.Fatal(err)
	}
	// Reserve an available loopback port; production Serve owns the real listener.
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := reservation.Addr().String()
	_ = reservation.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, handler.Handler(), address, cert, key) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(7 * time.Second):
			t.Error("HTTPS server did not shut down")
		}
	})
	url := "https://" + address
	ready := false
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		response, err := client.Get(url + "/healthz")
		if err == nil {
			_ = response.Body.Close()
			ready = response.StatusCode == http.StatusOK
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ready {
		t.Fatal("production HTTPS server not ready")
	}
	for _, route := range []string{"/api/setup", "/api/login"} {
		body := `{"username":"admin","password":"test-password-only-123"}`
		if route == "/api/setup" {
			body = `{"username":"admin","password":"test-password-only-123","token":"` + handler.setupToken + `"}`
		}
		response, err := client.Post(url+route, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
			t.Fatalf("HTTPS auth flow failed: %s: %d", route, response.StatusCode)
		}
		cookies := response.Cookies()
		if len(cookies) == 0 || !cookies[len(cookies)-1].Secure || !cookies[len(cookies)-1].HttpOnly {
			t.Fatal("production authentication did not set secure cookie")
		}
	}
	response, err := client.Get("http://" + address + "/api/session")
	if err == nil {
		defer response.Body.Close()
		if response.StatusCode != http.StatusBadRequest {
			t.Fatal("HTTPS listener served the plaintext session route")
		}
	}
}

func TestPanelTLSIdentityPersistsAndFailsClosed(t *testing.T) {
	dir := t.TempDir()
	cert, key, err := PanelTLS("0.0.0.0:8787", "https://vpn.example.com:8787", dir, "", "")
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(cert)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := PanelTLS("0.0.0.0:8787", "", dir, "", ""); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(cert)
	if !bytes.Equal(original, after) {
		t.Fatal("identity changed on restart")
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(cert)
		if info.Mode().Perm() != 0600 {
			t.Fatal("private TLS identity not 0600")
		}
		if err := os.Chmod(cert, 0644); err != nil {
			t.Fatal(err)
		}
		if _, _, err := PanelTLS("0.0.0.0:8787", "", dir, "", ""); err == nil {
			t.Fatal("insecure automatic key permissions accepted")
		}
		_ = os.Chmod(cert, 0600)
	}
	customCert, customKey, err := PanelTLS("0.0.0.0:8787", "", t.TempDir(), cert, key)
	if err != nil || customCert != cert || customKey != key {
		t.Fatal("explicit TLS identity changed", err)
	}
	if err := os.WriteFile(cert, []byte("invalid certificate"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := PanelTLS("0.0.0.0:8787", "", dir, "", ""); err == nil {
		t.Fatal("corrupt certificate did not fail closed")
	}
	if _, _, err := PanelTLS("0.0.0.0:8787", "", dir, cert, key); err == nil {
		t.Fatal("corrupt explicit certificate did not fail closed")
	}
	data, _ := os.ReadFile(cert)
	if string(data) != "invalid certificate" {
		t.Fatal("corrupt identity silently replaced")
	}
}

func TestAutomaticTLSHandshakeAndSecureSession(t *testing.T) {
	cert, key, err := PanelTLS("0.0.0.0:8787", "https://vpn.example.com:8787", t.TempDir(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	pair, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"127.0.0.1", "::1", "localhost", "vpn.example.com"} {
		if err := leaf.VerifyHostname(host); err != nil {
			t.Fatal(err)
		}
	}
	auth := NewAuth(cert != "")
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			t.Error("plaintext reached authentication handler")
		}
		auth.create(w, "admin")
		_, _ = io.WriteString(w, "encrypted")
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(leaf)
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	response, err := client.Get(server.URL)
	if err != nil {
		t.Fatal("trusted self-signed identity did not handshake", err)
	}
	defer response.Body.Close()
	cookies := response.Cookies()
	if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly {
		t.Fatal("HTTPS session is not secure")
	}
	data, _ := io.ReadAll(response.Body)
	if string(data) != "encrypted" {
		t.Fatal("HTTPS application response failed")
	}
}
