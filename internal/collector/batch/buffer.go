// Package batch implements the bounded ring buffer, disk-backed spool and
// backpressure signalling shared by every listener.
package batch

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/logkrama/logkrama/internal/collector"
)

// ErrBackpressure is returned by Push when both the in-memory buffer and the
// disk spool are full. TCP/HTTP listeners must translate this into pausing
// reads / returning 429 — never into dropping the event.
var ErrBackpressure = errors.New("batch: backpressure — buffer and spool both full")

// Config controls one Buffer's capacity and overflow behavior.
type Config struct {
	Capacity       int     // in-memory channel capacity (event count)
	SpoolThreshold float64 // fraction of Capacity at which new events start spilling to disk, e.g. 0.8
	MaxSpoolEvents int64   // spool depth (event count) at which Push starts returning ErrBackpressure
	SpoolDir       string  // directory for the overflow spool file
	ListenerID     string  // for metrics labeling
}

func (c Config) withDefaults() Config {
	if c.Capacity <= 0 {
		c.Capacity = 10000
	}
	if c.SpoolThreshold <= 0 {
		c.SpoolThreshold = 0.8
	}
	if c.MaxSpoolEvents <= 0 {
		c.MaxSpoolEvents = 500000
	}
	return c
}

// Buffer is a bounded ring buffer (an in-memory channel) backed by a disk
// spool for overflow. Consumers drain via Chan(); when the channel has room,
// a background goroutine refills it from the spool first, preserving rough
// FIFO order across the memory/disk boundary.
type Buffer struct {
	cfg   Config
	ch    chan collector.RawEvent
	spool *spool

	spoolDepth atomic.Int64
	closed     atomic.Bool
	done       chan struct{}
}

// New creates a Buffer. It must be closed with Close when no longer needed,
// which removes the spool file.
func New(cfg Config) (*Buffer, error) {
	cfg = cfg.withDefaults()
	sp, err := newSpool(cfg.SpoolDir, cfg.ListenerID)
	if err != nil {
		return nil, fmt.Errorf("batch: open spool: %w", err)
	}
	b := &Buffer{
		cfg:   cfg,
		ch:    make(chan collector.RawEvent, cfg.Capacity),
		spool: sp,
		done:  make(chan struct{}),
	}
	go b.feedFromSpool()
	return b, nil
}

// Push accepts one event. It never blocks the caller waiting on downstream
// consumers: it either lands in the memory channel, is durably spooled to
// disk, or (only once both are full) returns ErrBackpressure so the caller
// can push back on its upstream (pause TCP reads, HTTP 429).
func (b *Buffer) Push(ev collector.RawEvent) error {
	if b.closed.Load() {
		return errors.New("batch: buffer closed")
	}

	utilization := float64(len(b.ch)) / float64(cap(b.ch))
	if utilization < b.cfg.SpoolThreshold {
		select {
		case b.ch <- ev:
			return nil
		default:
			// Raced with another producer; fall through to spool.
		}
	}

	if b.spoolDepth.Load() >= b.cfg.MaxSpoolEvents {
		return ErrBackpressure
	}
	if err := b.spool.Write(ev); err != nil {
		return fmt.Errorf("batch: spool write: %w", err)
	}
	b.spoolDepth.Add(1)
	return nil
}

// TryPush is the UDP-path variant: it never spools and never blocks — full
// means dropped, immediately, so drops stay bounded and visible in metrics
// rather than silently building an unbounded disk queue for a
// connectionless, loss-tolerant protocol.
func (b *Buffer) TryPush(ev collector.RawEvent) (dropped bool) {
	select {
	case b.ch <- ev:
		return false
	default:
		return true
	}
}

// Chan returns the channel consumers should range over to drain events.
func (b *Buffer) Chan() <-chan collector.RawEvent {
	return b.ch
}

// SpoolDepth returns the current number of events sitting in the disk spool.
func (b *Buffer) SpoolDepth() int64 {
	return b.spoolDepth.Load()
}

// Len returns the current in-memory channel occupancy.
func (b *Buffer) Len() int {
	return len(b.ch)
}

// Cap returns the in-memory channel capacity.
func (b *Buffer) Cap() int {
	return cap(b.ch)
}

func (b *Buffer) feedFromSpool() {
	for {
		select {
		case <-b.done:
			return
		default:
		}

		if b.spoolDepth.Load() == 0 {
			if !b.spool.WaitForData(b.done) {
				return
			}
			continue
		}

		ev, ok, err := b.spool.ReadNext()
		if err != nil || !ok {
			continue
		}

		select {
		case b.ch <- ev:
			b.spoolDepth.Add(-1)
		case <-b.done:
			return
		}
	}
}

// Close stops the spool feeder and removes the spool file.
func (b *Buffer) Close() error {
	if b.closed.CompareAndSwap(false, true) {
		close(b.done)
	}
	return b.spool.Close()
}
