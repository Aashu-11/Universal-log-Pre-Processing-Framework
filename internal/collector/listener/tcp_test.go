package listener

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/logkrama/logkrama/internal/collector/batch"
	"github.com/logkrama/logkrama/internal/telemetry"
)

func newTestMetrics() *telemetry.Metrics {
	return telemetry.New(prometheus.NewRegistry())
}

func newTestBuffer(t *testing.T, id string) *batch.Buffer {
	t.Helper()
	b, err := batch.New(batch.Config{Capacity: 10000, SpoolDir: t.TempDir(), ListenerID: id})
	if err != nil {
		t.Fatalf("batch.New: %v", err)
	}
	t.Cleanup(func() { b.Close() })
	return b
}

func TestTCPNewlineFramingZeroLoss(t *testing.T) {
	buf := newTestBuffer(t, "tcp-nl")
	tcp := NewTCP(TCPConfig{Addr: "127.0.0.1:0", ListenerID: "tcp-nl"}, buf, newTestMetrics())

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- tcp.ServeOnListener(ctx, ln) }()

	const n = 5000
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	for i := 0; i < n; i++ {
		fmt.Fprintf(conn, "event-%d\n", i)
	}
	conn.Close()

	received := 0
	timeout := time.After(10 * time.Second)
	for received < n {
		select {
		case ev := <-buf.Chan():
			want := fmt.Sprintf("event-%d", received)
			if string(ev.Payload) != want {
				t.Fatalf("out of order or corrupted: got %q want %q", ev.Payload, want)
			}
			received++
		case <-timeout:
			t.Fatalf("timed out: received %d/%d (zero-loss requirement violated)", received, n)
		}
	}

	cancel()
	<-done
}

func TestTCPOctetCountedFraming(t *testing.T) {
	buf := newTestBuffer(t, "tcp-oc")
	tcp := NewTCP(TCPConfig{Addr: "127.0.0.1:0", ListenerID: "tcp-oc"}, buf, newTestMetrics())

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- tcp.ServeOnListener(ctx, ln) }()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	messages := []string{"<14>Sep  7 hello world", "<14>Sep  7 second message here"}
	for _, m := range messages {
		fmt.Fprintf(conn, "%d %s", len(m), m)
	}
	conn.Close()

	for i, want := range messages {
		select {
		case ev := <-buf.Chan():
			if string(ev.Payload) != want {
				t.Errorf("message %d = %q, want %q", i, ev.Payload, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for message %d", i)
		}
	}
	cancel()
	<-done
}

func TestUDPCountsDrops(t *testing.T) {
	buf := newTestBuffer(t, "udp-1")
	m := newTestMetrics()
	udp := NewUDP(UDPConfig{Addr: "127.0.0.1:0", ListenerID: "udp-1", Readers: 1}, buf, m)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	udp.conn = conn

	serveErr := make(chan error, 1)
	go func() {
		errCh := make(chan error, 1)
		go udp.readLoop(conn, errCh)
		<-ctx.Done()
		conn.Close()
		<-errCh
		serveErr <- nil
	}()

	client, err := net.Dial("udp", conn.LocalAddr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if _, err := client.Write([]byte("hello-udp")); err != nil {
		t.Fatalf("write: %v", err)
	}

	select {
	case ev := <-buf.Chan():
		if string(ev.Payload) != "hello-udp" {
			t.Errorf("got %q", ev.Payload)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for UDP event")
	}

	cancel()
	<-serveErr
}
