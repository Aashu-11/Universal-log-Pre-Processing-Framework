package listener

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"github.com/ulpf/ulpf/internal/collector/batch"
	"github.com/ulpf/ulpf/internal/telemetry"
)

// TLSConfig configures the syslog TLS/6514-style listener.
type TLSConfig struct {
	TCPConfig
	CertFile string
	KeyFile  string
}

// TLS wraps TCP's accept/frame/emit logic with a TLS-terminating listener.
// SNI is captured per connection and carried into every emitted event's
// envelope.
type TLS struct {
	cfg      TLSConfig
	inner    *TCP
	listener net.Listener
}

func NewTLS(cfg TLSConfig, buf *batch.Buffer, m *telemetry.Metrics) *TLS {
	return &TLS{cfg: cfg, inner: NewTCP(cfg.TCPConfig, buf, m)}
}

func (t *TLS) ID() string { return t.cfg.ListenerID }

func (t *TLS) Serve(ctx context.Context) error {
	cert, err := tls.LoadX509KeyPair(t.cfg.CertFile, t.cfg.KeyFile)
	if err != nil {
		return fmt.Errorf("listener/tls: load cert/key: %w", err)
	}
	tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}

	inner, err := net.Listen("tcp", t.cfg.Addr)
	if err != nil {
		return fmt.Errorf("listener/tls: listen %q: %w", t.cfg.Addr, err)
	}
	ln := tls.NewListener(inner, tlsCfg)
	t.listener = ln

	return t.inner.ServeOnListener(ctx, ln)
}

func (t *TLS) Close() error {
	if t.listener != nil {
		return t.listener.Close()
	}
	return nil
}
