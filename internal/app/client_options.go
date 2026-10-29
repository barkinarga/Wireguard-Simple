package app

import (
	"errors"
	"net/netip"
)

// Nil fields inherit the server defaults, including on old saved clients.
type ClientConfigOptions struct {
	DNS                 *string `json:"dns,omitempty"`
	AllowedIPs          string  `json:"allowedIPs,omitempty"`
	Endpoint            string  `json:"endpoint,omitempty"`
	Port                *int    `json:"port,omitempty"`
	MTU                 *int    `json:"mtu,omitempty"`
	PersistentKeepalive *int    `json:"persistentKeepalive,omitempty"`
	ListenPort          *int    `json:"listenPort,omitempty"`
	FwMark              *uint32 `json:"fwMark,omitempty"`
}

type CreatePeerOptions struct {
	ClientConfigOptions
	Address      string `json:"address,omitempty"`
	PrivateKey   string `json:"privateKey,omitempty"`
	PresharedKey string `json:"presharedKey,omitempty"`
}

func (o ClientConfigOptions) server(s Server) Server {
	if o.DNS != nil {
		s.DNS = *o.DNS
	}
	if o.AllowedIPs != "" {
		s.AllowedIPs = o.AllowedIPs
	}
	if o.Endpoint != "" {
		s.Endpoint = o.Endpoint
	}
	if o.Port != nil {
		s.Port = *o.Port
	}
	if o.MTU != nil {
		s.MTU = *o.MTU
	}
	return s
}

func (o CreatePeerOptions) validate(s Server, peers []Peer) (string, error) {
	if err := o.server(s).Validate(); err != nil {
		return "", err
	}
	if o.PersistentKeepalive != nil && (*o.PersistentKeepalive < 0 || *o.PersistentKeepalive > 65535) {
		return "", errors.New("Persistent keepalive must be 0–65535 seconds")
	}
	if o.ListenPort != nil && (*o.ListenPort < 0 || *o.ListenPort > 65535) {
		return "", errors.New("Client listen port must be 0–65535")
	}
	if o.PrivateKey != "" && !validKey(o.PrivateKey) || o.PresharedKey != "" && !validKey(o.PresharedKey) {
		return "", errors.New("Client keys must be base64-encoded 32-byte keys")
	}
	if o.Address == "" {
		return allocateIP(s, peers)
	}
	address, err := netip.ParsePrefix(o.Address)
	network, networkErr := netip.ParsePrefix(s.Address)
	if err != nil || networkErr != nil || !address.Addr().Is4() || address.Bits() != 32 || !network.Contains(address.Addr()) || address.Addr() == network.Addr() || address.Addr() == network.Masked().Addr() || !network.Contains(address.Addr().Next()) {
		return "", errors.New("Client address must be an available /32 in the server subnet")
	}
	for _, p := range peers {
		for _, raw := range splitCIDRs(p.Address) {
			if used, err := netip.ParsePrefix(raw); err != nil || used.Contains(address.Addr()) {
				return "", errors.New("Client address is already in use")
			}
		}
	}
	return address.String(), nil
}
