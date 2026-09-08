package listener

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ulpf/ulpf/internal/collector"
	"github.com/ulpf/ulpf/internal/collector/batch"
	"github.com/ulpf/ulpf/internal/telemetry"
)

// FileConfig configures the directory file-tail listener.
type FileConfig struct {
	Dir            string
	Pattern        string // glob, e.g. "*.log"
	ListenerID     string
	CheckpointPath string
	PollInterval   time.Duration
}

func (c FileConfig) withDefaults() FileConfig {
	if c.Pattern == "" {
		c.Pattern = "*.log"
	}
	if c.PollInterval <= 0 {
		c.PollInterval = time.Second
	}
	if c.CheckpointPath == "" {
		c.CheckpointPath = filepath.Join(c.Dir, ".ulpf-checkpoint.json")
	}
	return c
}

// File polls a directory and tails matching files, resuming from a
// persisted byte-offset checkpoint on restart so no line is skipped or
// duplicated across a restart boundary. Only fully newline-terminated lines
// ever advance the committed offset — a line still being written when we
// poll is left for the next pass, never read as a truncated partial.
type File struct {
	cfg        FileConfig
	buf        *batch.Buffer
	metrics    *telemetry.Metrics
	checkpoint map[string]int64
}

func NewFile(cfg FileConfig, buf *batch.Buffer, m *telemetry.Metrics) *File {
	cfg = cfg.withDefaults()
	return &File{cfg: cfg, buf: buf, metrics: m, checkpoint: map[string]int64{}}
}

func (f *File) ID() string { return f.cfg.ListenerID }

func (f *File) Serve(ctx context.Context) error {
	cp, err := loadCheckpoint(f.cfg.CheckpointPath)
	if err != nil {
		return fmt.Errorf("listener/file: load checkpoint: %w", err)
	}
	f.checkpoint = cp

	ticker := time.NewTicker(f.cfg.PollInterval)
	defer ticker.Stop()

	for {
		if err := f.pollOnce(); err != nil {
			fmt.Printf("listener/file: poll error: %v\n", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (f *File) pollOnce() error {
	matches, err := filepath.Glob(filepath.Join(f.cfg.Dir, f.cfg.Pattern))
	if err != nil {
		return fmt.Errorf("glob: %w", err)
	}

	changed := false
	for _, path := range matches {
		n, err := f.tailFile(path)
		if err != nil {
			fmt.Printf("listener/file: tail %s: %v\n", path, err)
			continue
		}
		if n > 0 {
			changed = true
		}
	}
	if changed {
		return saveCheckpoint(f.cfg.CheckpointPath, f.checkpoint)
	}
	return nil
}

func (f *File) tailFile(path string) (int, error) {
	fh, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer fh.Close()

	info, err := fh.Stat()
	if err != nil {
		return 0, err
	}

	offset := f.checkpoint[path]
	if info.Size() < offset {
		offset = 0 // file was truncated/rotated; restart from the beginning
	}
	if info.Size() == offset {
		return 0, nil
	}

	if _, err := fh.Seek(offset, 0); err != nil {
		return 0, err
	}

	remaining := info.Size() - offset
	data := make([]byte, remaining)
	if _, err := readFullFile(fh, data); err != nil {
		return 0, err
	}

	lineStart := 0
	committed := offset
	count := 0
	for i, b := range data {
		if b != '\n' {
			continue
		}
		line := data[lineStart:i]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		f.emit(path, line)
		count++
		lineStart = i + 1
		committed = offset + int64(lineStart)
	}

	f.checkpoint[path] = committed
	return count, nil
}

func (f *File) emit(path string, payload []byte) {
	item := make([]byte, len(payload))
	copy(item, payload)

	ev := collector.RawEvent{
		Envelope: collector.Envelope{
			ListenerID: f.cfg.ListenerID,
			PeerIP:     path, // file source: envelope's "peer" is the file path
			ReceivedAt: time.Now(),
			ByteLength: len(item),
		},
		Payload: item,
	}
	f.metrics.IngestBytesTotal.WithLabelValues(f.cfg.ListenerID).Add(float64(len(item)))

	for {
		if err := f.buf.Push(ev); err == nil {
			f.metrics.EventsReceivedTotal.WithLabelValues(f.cfg.ListenerID, path).Inc()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func readFullFile(f *os.File, buf []byte) (int, error) {
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

func loadCheckpoint(path string) (map[string]int64, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]int64{}, nil
	}
	if err != nil {
		return nil, err
	}
	var cp map[string]int64
	if err := json.Unmarshal(b, &cp); err != nil {
		return nil, err
	}
	return cp, nil
}

func saveCheckpoint(path string, cp map[string]int64) error {
	b, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (f *File) Close() error { return nil }
