package listener

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/logkrama/logkrama/internal/collector"
	"github.com/logkrama/logkrama/internal/collector/batch"
	"github.com/logkrama/logkrama/internal/collector/framing"
	"github.com/logkrama/logkrama/internal/telemetry"
)

// TCPConfig configures the syslog TCP listener. Framing (RFC6587
// octet-counted vs. newline-delimited) is auto-detected per connection.
type TCPConfig struct {
	Addr           string
	ListenerID     string
	MultilineStart string // only applies to newline-framed connections
	MaxEventBytes  int
}

// TCP is the syslog TCP/601-style listener. Both RFC6587 octet-counted
// framing ("<len> <msg>") and plain newline framing are supported and
// detected per connection by peeking the first bytes. TCP never drops: a
// full buffer pauses reads on the connection (natural TCP backpressure)
// instead of discarding data.
type TCP struct {
	cfg      TCPConfig
	buf      *batch.Buffer
	metrics  *telemetry.Metrics
	listener net.Listener
}

func NewTCP(cfg TCPConfig, buf *batch.Buffer, m *telemetry.Metrics) *TCP {
	return &TCP{cfg: cfg, buf: buf, metrics: m}
}

func (t *TCP) ID() string { return t.cfg.ListenerID }

func (t *TCP) Serve(ctx context.Context) error {
	ln, err := net.Listen("tcp", t.cfg.Addr)
	if err != nil {
		return fmt.Errorf("listener/tcp: listen %q: %w", t.cfg.Addr, err)
	}
	return t.ServeOnListener(ctx, ln)
}

// ServeOnListener runs the accept loop on a caller-supplied listener — used
// directly by TCP.Serve, and by TLS.Serve which wraps a tls.Listener around
// the same accept/frame/emit logic.
func (t *TCP) ServeOnListener(ctx context.Context, ln net.Listener) error {
	t.listener = ln

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return fmt.Errorf("listener/tcp: accept: %w", err)
			}
		}
		go t.handleConn(ctx, conn)
	}
}

func (t *TCP) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	peer, _, _ := net.SplitHostPort(conn.RemoteAddr().String())

	sni := ""
	if tc, ok := conn.(*tls.Conn); ok {
		if err := tc.HandshakeContext(ctx); err != nil {
			return
		}
		sni = tc.ConnectionState().ServerName
	}

	r := bufio.NewReaderSize(conn, 64*1024)
	octetCounted, err := detectOctetCounting(r)
	if err != nil {
		return
	}

	joiner := framing.New(framing.Config{MultilineStart: t.cfg.MultilineStart, MaxEventBytes: t.cfg.MaxEventBytes})

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var payload []byte
		var readErr error
		if octetCounted {
			payload, readErr = readOctetCountedMessage(r)
		} else {
			payload, readErr = readLine(r)
		}
		if readErr != nil {
			if res, ok := joiner.Flush(); ok {
				t.emit(res, peer, sni)
			}
			return
		}

		if octetCounted {
			t.emit(framing.Result{Payload: payload}, peer, sni)
			continue
		}
		if res, ok := joiner.Feed(payload); ok {
			t.emit(res, peer, sni)
		}
	}
}

func (t *TCP) emit(res framing.Result, peer, sni string) {
	ev := collector.RawEvent{
		Envelope: collector.Envelope{
			ListenerID: t.cfg.ListenerID,
			PeerIP:     peer,
			ReceivedAt: time.Now(),
			ByteLength: len(res.Payload),
			Truncated:  res.Truncated,
			TLSSNI:     sni,
		},
		Payload: res.Payload,
	}
	t.metrics.IngestBytesTotal.WithLabelValues(t.cfg.ListenerID).Add(float64(len(res.Payload)))

	for {
		err := t.buf.Push(ev)
		if err == nil {
			t.metrics.EventsReceivedTotal.WithLabelValues(t.cfg.ListenerID, peer).Inc()
			return
		}
		// Backpressure: never drop on TCP. Pause briefly and retry — this
		// stalls handleConn's read loop, which stalls TCP reads on this
		// connection, which is exactly the backpressure signal we want the
		// remote sender to see via TCP flow control.
		time.Sleep(50 * time.Millisecond)
	}
}

// detectOctetCounting peeks the first bytes of a connection: RFC6587
// octet-counted framing starts with an ASCII decimal length followed by a
// space ("123 <14>Sep..."). Newline framing typically starts with '<' (the
// syslog PRI) or arbitrary text. A leading digit is treated as
// octet-counting; anything else falls back to newline framing.
func detectOctetCounting(r *bufio.Reader) (bool, error) {
	b, err := r.Peek(1)
	if err != nil {
		return false, err
	}
	return b[0] >= '0' && b[0] <= '9', nil
}

func readOctetCountedMessage(r *bufio.Reader) ([]byte, error) {
	lenBytes, err := r.ReadBytes(' ')
	if err != nil {
		return nil, err
	}
	length := 0
	for _, c := range lenBytes[:len(lenBytes)-1] {
		if c < '0' || c > '9' {
			return nil, fmt.Errorf("listener/tcp: invalid octet-count digit %q", c)
		}
		length = length*10 + int(c-'0')
	}
	msg := make([]byte, length)
	if _, err := readFullReader(r, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func readFullReader(r *bufio.Reader, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func readLine(r *bufio.Reader) ([]byte, error) {
	line, err := r.ReadBytes('\n')
	if err != nil {
		if len(line) > 0 {
			return trimEOL(line), nil
		}
		return nil, err
	}
	return trimEOL(line), nil
}

func trimEOL(line []byte) []byte {
	line = line[:len(line)-1] // drop \n
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}
	out := make([]byte, len(line))
	copy(out, line)
	return out
}

func (t *TCP) Close() error {
	if t.listener != nil {
		return t.listener.Close()
	}
	return nil
}
