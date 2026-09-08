package listener

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPIngestNDJSON(t *testing.T) {
	buf := newTestBuffer(t, "http-1")
	h := NewHTTP(HTTPConfig{ListenerID: "http-1"}, buf, newTestMetrics())

	srv := httptest.NewServer(http.HandlerFunc(h.handleIngest))
	defer srv.Close()

	body := "{\"a\":1}\n{\"a\":2}\n{\"a\":3}\n"
	resp, err := http.Post(srv.URL, "application/x-ndjson", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", resp.StatusCode)
	}
	var ir ingestResponse
	if err := json.NewDecoder(resp.Body).Decode(&ir); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if ir.EventsQueued != 3 {
		t.Errorf("events_queued = %d, want 3", ir.EventsQueued)
	}

	for i := 0; i < 3; i++ {
		select {
		case ev := <-buf.Chan():
			want := "{\"a\":" + string(rune('1'+i)) + "}"
			if string(ev.Payload) != want {
				t.Errorf("event %d = %q, want %q", i, ev.Payload, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for event %d", i)
		}
	}
}

func TestHTTPIngestGzip(t *testing.T) {
	buf := newTestBuffer(t, "http-2")
	h := NewHTTP(HTTPConfig{ListenerID: "http-2"}, buf, newTestMetrics())

	srv := httptest.NewServer(http.HandlerFunc(h.handleIngest))
	defer srv.Close()

	var gzBuf bytes.Buffer
	gz := gzip.NewWriter(&gzBuf)
	gz.Write([]byte("{\"gz\":true}\n"))
	gz.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL, &gzBuf)
	req.Header.Set("Content-Encoding", "gzip")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", resp.StatusCode)
	}

	select {
	case ev := <-buf.Chan():
		if string(ev.Payload) != `{"gz":true}` {
			t.Errorf("got %q", ev.Payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}
}

func TestHTTPRateLimit(t *testing.T) {
	buf := newTestBuffer(t, "http-3")
	h := NewHTTP(HTTPConfig{ListenerID: "http-3", RateLimitPerSec: 1, RateBurst: 1}, buf, newTestMetrics())
	srv := httptest.NewServer(http.HandlerFunc(h.handleIngest))
	defer srv.Close()

	// First request consumes the single burst token.
	resp1, err := http.Post(srv.URL, "application/x-ndjson", bytes.NewBufferString("{}\n"))
	if err != nil {
		t.Fatalf("post 1: %v", err)
	}
	resp1.Body.Close()

	// Second, immediate request should be rate limited.
	resp2, err := http.Post(srv.URL, "application/x-ndjson", bytes.NewBufferString("{}\n"))
	if err != nil {
		t.Fatalf("post 2: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", resp2.StatusCode)
	}
}

func TestHTTPServeLifecycle(t *testing.T) {
	buf := newTestBuffer(t, "http-4")
	h := NewHTTP(HTTPConfig{Addr: "127.0.0.1:0", ListenerID: "http-4"}, buf, newTestMetrics())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.Serve(ctx) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not shut down in time")
	}
}
