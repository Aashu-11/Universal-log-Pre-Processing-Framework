package batch

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/ulpf/ulpf/internal/collector"
)

// spool is a simple disk-backed FIFO of RawEvents: one growing file,
// written by Push (via Write) and read sequentially by the buffer's feeder
// goroutine (via ReadNext). It exists purely to absorb bursts larger than
// the in-memory channel without dropping TCP/HTTP events — see
// docs/DECISIONS.md for the documented limitation that the spool does not
// survive a process restart (acceptable for a prototype whose durability
// guarantee is the Raw Vault, not this overflow buffer).
type spool struct {
	mu   sync.Mutex
	cond *sync.Cond
	file *os.File
	w    *bufio.Writer

	readFile *os.File
	readAt   int64
	writeAt  int64

	path string
}

func newSpool(dir, listenerID string) (*spool, error) {
	if dir == "" {
		dir = os.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir spool dir: %w", err)
	}
	path := filepath.Join(dir, fmt.Sprintf("ulpf-spool-%s-%d.bin", listenerID, os.Getpid()))

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open spool file: %w", err)
	}
	rf, err := os.Open(path)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("open spool file for reading: %w", err)
	}

	s := &spool{file: f, w: bufio.NewWriter(f), readFile: rf, path: path}
	s.cond = sync.NewCond(&s.mu)
	return s, nil
}

// Write appends one event: [envelope JSON len][envelope JSON][payload len][payload].
func (s *spool) Write(ev collector.RawEvent) error {
	envJSON, err := json.Marshal(ev.Envelope)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(envJSON)))
	if _, err := s.w.Write(lenBuf[:]); err != nil {
		return err
	}
	if _, err := s.w.Write(envJSON); err != nil {
		return err
	}
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(ev.Payload)))
	if _, err := s.w.Write(lenBuf[:]); err != nil {
		return err
	}
	if _, err := s.w.Write(ev.Payload); err != nil {
		return err
	}
	if err := s.w.Flush(); err != nil {
		return err
	}

	s.writeAt += int64(4 + len(envJSON) + 4 + len(ev.Payload))
	s.cond.Broadcast()
	return nil
}

// ReadNext reads the next event after the last one returned, if any is
// available yet. ok is false if the reader has caught up to the writer.
func (s *spool) ReadNext() (collector.RawEvent, bool, error) {
	s.mu.Lock()
	if s.readAt >= s.writeAt {
		s.mu.Unlock()
		return collector.RawEvent{}, false, nil
	}
	s.mu.Unlock()

	var lenBuf [4]byte
	if _, err := readFull(s.readFile, lenBuf[:]); err != nil {
		return collector.RawEvent{}, false, err
	}
	envLen := binary.BigEndian.Uint32(lenBuf[:])
	envBytes := make([]byte, envLen)
	if _, err := readFull(s.readFile, envBytes); err != nil {
		return collector.RawEvent{}, false, err
	}

	if _, err := readFull(s.readFile, lenBuf[:]); err != nil {
		return collector.RawEvent{}, false, err
	}
	payloadLen := binary.BigEndian.Uint32(lenBuf[:])
	payload := make([]byte, payloadLen)
	if _, err := readFull(s.readFile, payload); err != nil {
		return collector.RawEvent{}, false, err
	}

	var env collector.Envelope
	if err := json.Unmarshal(envBytes, &env); err != nil {
		return collector.RawEvent{}, false, fmt.Errorf("unmarshal spooled envelope: %w", err)
	}

	s.mu.Lock()
	s.readAt += int64(4 + len(envBytes) + 4 + len(payload))
	s.mu.Unlock()

	return collector.RawEvent{Envelope: env, Payload: payload}, true, nil
}

// WaitForData blocks until a Write happens or done is closed. Returns false
// if done fired first.
func (s *spool) WaitForData(done <-chan struct{}) bool {
	woke := make(chan struct{})
	go func() {
		s.mu.Lock()
		for s.readAt >= s.writeAt {
			s.cond.Wait()
			select {
			case <-done:
				s.mu.Unlock()
				return
			default:
			}
		}
		s.mu.Unlock()
		close(woke)
	}()

	select {
	case <-woke:
		return true
	case <-done:
		s.mu.Lock()
		s.cond.Broadcast() // wake the helper goroutine above so it can exit
		s.mu.Unlock()
		return false
	}
}

func (s *spool) Close() error {
	s.file.Close()
	s.readFile.Close()
	return os.Remove(s.path)
}

func readFull(f *os.File, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := f.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
