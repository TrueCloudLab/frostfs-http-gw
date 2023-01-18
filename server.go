package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"
)

type (
	ServerInfo struct {
		Address string
		TLS     ServerTLSInfo
	}

	ServerTLSInfo struct {
		Enabled  bool
		CertFile string
		KeyFile  string
	}

	Server interface {
		Address() string
		Listener() net.Listener
		UpdateCert(certFile, keyFile string) error
	}

	server struct {
		address     string
		listener    net.Listener
		tlsProvider *certProvider
	}

	certProvider struct {
		Enabled bool

		mu       sync.RWMutex
		certPath string
		keyPath  string
		cert     *tls.Certificate
	}
)

func (s *server) Address() string {
	return s.address
}

func (s *server) Listener() net.Listener {
	return s.listener
}

func (s *server) UpdateCert(certFile, keyFile string) error {
	return s.tlsProvider.UpdateCert(certFile, keyFile)
}

func newServer(ctx context.Context, serverInfo ServerInfo) (*server, error) {
	var lic net.ListenConfig
	ln, err := lic.Listen(ctx, "tcp", serverInfo.Address)
	if err != nil {
		return nil, fmt.Errorf("could not prepare listener: %w", err)
	}

	tlsProvider := &certProvider{
		Enabled: serverInfo.TLS.Enabled,
	}

	if serverInfo.TLS.Enabled {
		if err = tlsProvider.UpdateCert(serverInfo.TLS.CertFile, serverInfo.TLS.KeyFile); err != nil {
			return nil, fmt.Errorf("failed to update cert: %w", err)
		}

		ln = tls.NewListener(ln, &tls.Config{
			GetCertificate: tlsProvider.GetCertificate,
		})
	}

	return &server{
		address:     serverInfo.Address,
		listener:    ln,
		tlsProvider: tlsProvider,
	}, nil
}

func (p *certProvider) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	if !p.Enabled {
		return nil, errors.New("cert provider: disabled")
	}

	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cert, nil
}

func (p *certProvider) UpdateCert(certPath, keyPath string) error {
	if !p.Enabled {
		return fmt.Errorf("tls disabled")
	}

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return fmt.Errorf("cannot load TLS key pair from certFile '%s' and keyFile '%s': %w", certPath, keyPath, err)
	}

	p.mu.Lock()
	p.certPath = certPath
	p.keyPath = keyPath
	p.cert = &cert
	p.mu.Unlock()
	return nil
}

func (p *certProvider) FilePaths() (string, string) {
	if !p.Enabled {
		return "", ""
	}

	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.certPath, p.keyPath
}
