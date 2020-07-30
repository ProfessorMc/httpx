// Copyright (c) 2020 Bojan Zivanovic and contributors
// SPDX-License-Identifier: MIT

package httpx

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-systemd/activation"
	"golang.org/x/net/netutil"
)

// Server represents an HTTP server.
type Server struct {
	*http.Server

	// MaxConnections limits the number of accepted simultaneous connections.
	// Defaults to 0, indicating no limit.
	MaxConnections int
}

// NewServer creates a new HTTP server for the given address and handler.
//
// The addr is a TCP address in the form of "host:port" (e.g. "0.0.0.0:80")
// or a systemd socket name (e.g. "systemd:myapp-http").
// The handler can be nil, in which case http.DefaultServeMux is used.
func NewServer(addr string, handler http.Handler) *Server {
	if addr == "" {
		// Preserve the default documented by http.Server in stdlib.
		addr = ":http"
	}
	srv := &Server{
		Server: &http.Server{
			Addr:    addr,
			Handler: handler,
			// https://blog.cloudflare.com/exposing-go-on-the-internet/
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  120 * time.Second,
			TLSConfig: &tls.Config{
				NextProtos:       []string{"h2", "http/1.1"},
				MinVersion:       tls.VersionTLS12,
				CurvePreferences: []tls.CurveID{tls.CurveP256, tls.X25519},
				CipherSuites: []uint16{
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
					tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				},
				PreferServerCipherSuites: true,
			},
		},
	}

	return srv
}

// ListenAndServe listens on srv.Addr and calls Serve to handle incoming requests.
//
// Accepted connections are configured to enable TCP keep-alives.
//
// ListenAndServe always returns a non-nil error. After Shutdown or Close,
// the returned error is ErrServerClosed.
func (srv *Server) ListenAndServe() error {
	ln, err := srv.Listen()
	if err != nil {
		return err
	}
	return srv.Serve(ln)
}

// ListenAndServeTLS listens on srv.Addr and calls ServeTLS to handle incoming requests.
//
// Accepted connections are configured to enable TCP keep-alives.
//
// Filenames containing a certificate and matching private key for the
// server must be provided if neither the Server's TLSConfig.Certificates
// nor TLSConfig.GetCertificate are populated. If the certificate is
// signed by a certificate authority, the certFile should be the
// concatenation of the server's certificate, any intermediates, and
// the CA's certificate.
//
// ListenAndServeTLS always returns a non-nil error. After Shutdown or
// Close, the returned error is ErrServerClosed.
func (srv *Server) ListenAndServeTLS(certFile, keyFile string) error {
	ln, err := srv.Listen()
	if err != nil {
		return err
	}
	return srv.ServeTLS(ln, certFile, keyFile)
}

// Listen returns a TCP or systemd socket listener for srv.Addr.
func (srv *Server) Listen() (net.Listener, error) {
	var ln net.Listener
	if strings.HasPrefix(srv.Addr, "systemd:") {
		name := srv.Addr[8:]
		listeners, _ := activation.ListenersWithNames()
		listener, ok := listeners[name]
		if !ok {
			return nil, fmt.Errorf("listen systemd %s: socket not found", name)
		}
		ln = listener[0]
	} else {
		var err error
		ln, err = net.Listen("tcp", srv.Addr)
		if err != nil {
			return nil, err
		}
	}
	if srv.MaxConnections > 0 {
		ln = netutil.LimitListener(ln, srv.MaxConnections)
	}

	return ln, nil
}
