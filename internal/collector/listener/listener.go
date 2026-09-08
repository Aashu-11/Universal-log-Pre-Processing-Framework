package listener

import "context"

// Listener is implemented by every ingest transport (UDP, TCP, TLS, HTTP,
// File). Serve blocks until ctx is canceled or an unrecoverable error
// occurs; Close is an additional/alternate way to stop it.
type Listener interface {
	ID() string
	Serve(ctx context.Context) error
	Close() error
}
