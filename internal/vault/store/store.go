// Package store defines the blob storage abstraction the Raw Vault writes
// through. Two implementations exist: Local (a plain filesystem tree, used
// for tests and for running the data plane without Docker) and MinIO
// (the pinned production target, S3-compatible). The Vault package only
// depends on the Store interface, so swapping backends never touches vault
// logic — see docs/DECISIONS.md for why this mattered during the build.
package store

import "context"

// Store is a minimal, content-addressed-by-key blob store: put whole
// objects, get whole objects back, list by prefix. The Raw Vault never needs
// partial reads or appends at the store layer — segments are sealed
// (immutable) before they are ever written out.
type Store interface {
	// Put writes data under key, overwriting any existing object.
	Put(ctx context.Context, key string, data []byte) error
	// Get returns the full contents of key, or an error satisfying
	// errors.Is(err, ErrNotFound) if it does not exist.
	Get(ctx context.Context, key string) ([]byte, error)
	// List returns every key with the given prefix, sorted lexically.
	List(ctx context.Context, prefix string) ([]string, error)
}

// ErrNotFound is returned by Get when key does not exist.
var ErrNotFound = errNotFound{}

type errNotFound struct{}

func (errNotFound) Error() string { return "store: key not found" }
