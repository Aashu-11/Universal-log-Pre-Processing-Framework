// Package listener implements the individual network/file listeners: UDP,
// TCP, TLS and HTTP bulk ingest, plus directory file-tail.
package listener

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/ulpf/ulpf/internal/collector"
	"github.com/ulpf/ulpf/internal/collector/batch"
	"github.com/ulpf/ulpf/internal/telemetry"
)

// UDPConfig configures the syslog UDP listener.
type UDPConfig struct {
	Addr       string // e.g. ":5514"
	ListenerID string
	// Readers is the number of concurrent goroutines calling ReadFrom on
	// the same socket. UDP sockets support concurrent readers safely (the
	// kernel serializes delivery), so this is a portable stand-in for
	// Linux's recvmmsg-based batched reads — see docs/DECISIONS.md.
	Readers        int
	ReadBufferSize int // SO_RCVBUF, default 16MB per CLAUDE.md
}

// UDP is the syslog UDP/514-style listener. Every datagram is exactly one
// event (no framing/joining — UDP syslog is inherently one-message-per-
// packet). Drops are always counted, never silent.
type UDP struct {
	cfg     UDPConfig
	buf     *batch.Buffer
	metrics *telemetry.Metrics
	conn    *net.UDPConn
}

func NewUDP(cfg UDPConfig, buf *batch.Buffer, m *telemetry.Metrics) *UDP {
	if cfg.Readers <= 0 {
		cfg.Readers = 4
	}
	if cfg.ReadBufferSize <= 0 {
		cfg.ReadBufferSize = 16 << 20
	}
	return &UDP{cfg: cfg, buf: buf, metrics: m}
}

func (u *UDP) ID() string { return u.cfg.ListenerID }

// Serve binds the socket and blocks, reading datagrams with u.cfg.Readers
// concurrent goroutines, until ctx is canceled.
func (u *UDP) Serve(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp", u.cfg.Addr)
	if err != nil {
		return fmt.Errorf("listener/udp: resolve %q: %w", u.cfg.Addr, err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("listener/udp: listen %q: %w", u.cfg.Addr, err)
	}
	u.conn = conn
	if err := conn.SetReadBuffer(u.cfg.ReadBufferSize); err != nil {
		// Not fatal — some platforms/permissions cap this — but worth
		// knowing about, so callers can inspect via logs/metrics rather
		// than the listener refusing to start.
		fmt.Printf("listener/udp: SetReadBuffer(%d) failed on %s: %v\n", u.cfg.ReadBufferSize, u.cfg.Addr, err)
	}

	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		conn.Close()
		close(done)
	}()

	errCh := make(chan error, u.cfg.Readers)
	for i := 0; i < u.cfg.Readers; i++ {
		go u.readLoop(conn, errCh)
	}

	<-done
	for i := 0; i < u.cfg.Readers; i++ {
		<-errCh
	}
	return nil
}

func (u *UDP) readLoop(conn *net.UDPConn, done chan<- error) {
	buf := make([]byte, 65535)
	for {
		n, peer, err := conn.ReadFromUDP(buf)
		if err != nil {
			done <- nil
			return
		}
		if n == 0 {
			continue
		}
		payload := make([]byte, n)
		copy(payload, buf[:n])

		peerIP, peerPort := "", 0
		if peer != nil {
			peerIP = peer.IP.String()
			peerPort = peer.Port
		}

		ev := collector.RawEvent{
			Envelope: collector.Envelope{
				ListenerID: u.cfg.ListenerID,
				PeerIP:     peerIP,
				PeerPort:   peerPort,
				ReceivedAt: time.Now(),
				ByteLength: n,
			},
			Payload: payload,
		}

		u.metrics.IngestBytesTotal.WithLabelValues(u.cfg.ListenerID).Add(float64(n))
		if dropped := u.buf.TryPush(ev); dropped {
			u.metrics.UDPDropsTotal.Inc()
			continue
		}
		u.metrics.EventsReceivedTotal.WithLabelValues(u.cfg.ListenerID, peerIP).Inc()
	}
}

// Close stops the listener (also achieved by canceling Serve's context).
func (u *UDP) Close() error {
	if u.conn != nil {
		return u.conn.Close()
	}
	return nil
}
