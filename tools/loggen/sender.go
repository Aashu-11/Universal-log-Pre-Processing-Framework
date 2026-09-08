package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
)

// Sender delivers one already-formatted log line to the collector.
type Sender interface {
	Send(line string) error
	Close() error
}

// udpSender is connectionless: every Send is one datagram.
type udpSender struct {
	conn *net.UDPConn
}

func newUDPSender(addr string) (Sender, error) {
	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		return nil, err
	}
	return &udpSender{conn: conn}, nil
}

func (s *udpSender) Send(line string) error {
	_, err := s.conn.Write([]byte(line))
	return err
}
func (s *udpSender) Close() error { return s.conn.Close() }

// tcpSender keeps one persistent connection, newline-framed.
type tcpSender struct {
	conn net.Conn
}

func newTCPSender(addr string) (Sender, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &tcpSender{conn: conn}, nil
}

func (s *tcpSender) Send(line string) error {
	_, err := fmt.Fprintf(s.conn, "%s\n", line)
	return err
}
func (s *tcpSender) Close() error { return s.conn.Close() }

// httpSender batches lines into gzip NDJSON POSTs for efficiency at high
// EPS — one HTTP request per httpBatchSize lines.
type httpSender struct {
	url    string
	client *http.Client
	buf    []string
	batch  int
}

const httpBatchSize = 200

func newHTTPSender(addr string) (Sender, error) {
	return &httpSender{url: "http://" + addr + "/v1/ingest", client: &http.Client{}, batch: httpBatchSize}, nil
}

func (s *httpSender) Send(line string) error {
	s.buf = append(s.buf, line)
	if len(s.buf) < s.batch {
		return nil
	}
	return s.flush()
}

func (s *httpSender) flush() error {
	if len(s.buf) == 0 {
		return nil
	}
	var body bytes.Buffer
	gz := gzip.NewWriter(&body)
	for _, l := range s.buf {
		gz.Write([]byte(l))
		gz.Write([]byte("\n"))
	}
	gz.Close()
	s.buf = s.buf[:0]

	req, err := http.NewRequest(http.MethodPost, s.url, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Content-Type", "application/x-ndjson")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("http sender: status %d", resp.StatusCode)
	}
	return nil
}

func (s *httpSender) Close() error { return s.flush() }

// countingSender wraps a Sender to track sent/error counts for the live
// throughput report.
type countingSender struct {
	inner Sender
	sent  *atomic.Int64
	errs  *atomic.Int64
}

func (c *countingSender) Send(line string) error {
	if err := c.inner.Send(line); err != nil {
		c.errs.Add(1)
		return err
	}
	c.sent.Add(1)
	return nil
}
func (c *countingSender) Close() error { return c.inner.Close() }
