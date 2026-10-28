package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"wireguard-simple/internal/app"
	"wireguard-simple/internal/web"
)

var version = "0.4.0-dev"

func run() error {
	mode := flag.String("mode", "live", "demo (no network changes) or live (Linux only)")
	listen := flag.String("listen", "127.0.0.1:8787", "HTTP listen address")
	dir := flag.String("data-dir", "", "private application data directory (demo: .data/demo, live: /var/lib/wgsimple)")
	iface := flag.String("interface", "wg0", "managed WireGuard interface")
	configDir := flag.String("config-dir", "/etc/wireguard", "WireGuard configuration directory (live Linux only)")
	origin := flag.String("origin", "", "canonical panel origin, e.g. https://vpn.example.com")
	cert := flag.String("tls-cert", "", "TLS certificate file")
	key := flag.String("tls-key", "", "TLS private key file")
	allowHTTP := flag.Bool("allow-http", false, "allow live public HTTP (unencrypted); prefer TLS on untrusted networks")
	resetAccess := flag.String("reset-panel-access", "", "reset panel policy to public, local, or lan, then exit (stop service first)")
	prepareNetwork := flag.Bool("prepare-network", false, "prepare managed IPv4 interface, forwarding and NAT, then exit (root, live mode, stop panel first)")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return nil
	}
	if *mode != "demo" && *mode != "live" {
		return fmt.Errorf("mode must be demo or live")
	}
	if *mode == "live" && runtime.GOOS != "linux" {
		return fmt.Errorf("live mode is Linux-only; no WireGuard operation was performed")
	}
	if !app.ValidInterface(*iface) {
		return fmt.Errorf("invalid interface name")
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	loopback := host == "localhost" || (ip != nil && ip.IsLoopback())
	if !loopback && (*mode == "demo" || (!*allowHTTP && (*cert == "" || *key == "" || *origin == ""))) {
		return fmt.Errorf("public listeners require live mode, TLS certificate/key, and an explicit HTTPS origin")
	}
	if (*cert == "") != (*key == "") {
		return fmt.Errorf("TLS certificate and key must be specified together")
	}
	if *origin != "" {
		u, e := url.Parse(*origin)
		if e != nil || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.User != nil || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("invalid canonical origin")
		}
		if !loopback && !*allowHTTP && u.Scheme != "https" {
			return fmt.Errorf("public origin must use HTTPS")
		}
	}
	if *dir == "" {
		if *mode == "demo" {
			*dir = ".data/demo"
		} else {
			*dir = "/var/lib/wgsimple"
		}
	}
	absolute, err := filepath.Abs(*dir)
	if err != nil {
		return err
	}
	unlock, err := app.LockData(absolute)
	if err != nil {
		return err
	}
	defer unlock()
	if *resetAccess != "" {
		if err := app.ResetPanelAccess(absolute, *resetAccess); err != nil {
			return err
		}
		fmt.Println("Panel access reset. Start the service to apply it; administrator credentials are unchanged.")
		return nil
	}
	store, err := app.OpenStore(absolute, *iface)
	if err != nil {
		return err
	}
	defer store.Close()
	actual, err := store.Server()
	if err != nil {
		return err
	}
	if actual.Interface != *iface {
		return fmt.Errorf("data directory belongs to interface %s", actual.Interface)
	}
	if *prepareNetwork {
		if *mode != "live" {
			return fmt.Errorf("network preparation requires live mode")
		}
		_ = os.Setenv("LC_ALL", "C")
		if err := app.PrepareInternet(context.Background(), store, *configDir); err != nil {
			return err
		}
		fmt.Println("Managed IPv4 VPN interface, forwarding and NAT are ready.")
		return nil
	}
	var engine app.Engine = app.NewDemoEngine()
	if *mode == "live" {
		engine = &app.LiveEngine{ConfigDir: *configDir}
	}
	service, err := app.NewService(store, engine, *mode, version)
	if err != nil {
		return err
	}
	server, err := app.NewHTTP(service, web.Files(), *origin, *cert != "" || len(*origin) >= 8 && (*origin)[:8] == "https://")
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go service.Run(ctx)
	scheme := "http"
	if *cert != "" {
		scheme = "https"
	}
	fmt.Printf("WireGuard Simple %s · %s\nLocal: %s://%s\n", version, *mode, scheme, *listen)
	if *mode == "demo" {
		fmt.Println("Demo: simulated peers; no VPN, DNS, route, or firewall changes.")
	} else if !store.HasAdmin() {
		fmt.Printf("First setup: read the private token file %s locally. Do not share it.\n", filepath.Join(absolute, "setup-token"))
	}
	return app.Serve(ctx, server.Handler(), *listen, *cert, *key)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
