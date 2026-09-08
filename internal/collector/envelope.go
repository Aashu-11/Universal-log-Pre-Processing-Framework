// Package collector implements the multi-protocol log ingest layer (syslog
// UDP/TCP/TLS, HTTP bulk, file tail) that feeds the Raw Vault.
package collector

import "time"

// Envelope is the per-event metadata captured at ingest time, independent of
// the event's own content — who sent it, how, and when it was received.
type Envelope struct {
	ListenerID string    `json:"listener_id"`
	PeerIP     string    `json:"peer_ip,omitempty"`
	PeerPort   int       `json:"peer_port,omitempty"`
	ReceivedAt time.Time `json:"received_at"`
	ByteLength int       `json:"byte_length"`
	TLSSNI     string    `json:"tls_sni,omitempty"`
	Truncated  bool      `json:"truncated,omitempty"`
}

// RawEvent is one unparsed event plus its envelope, as produced by a
// Listener and consumed by the batch buffer.
type RawEvent struct {
	Envelope Envelope
	Payload  []byte
}
