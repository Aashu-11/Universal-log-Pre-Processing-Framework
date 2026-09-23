package listener

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/logkrama/logkrama/internal/collector/batch"
)

func drainN(t *testing.T, buf *batch.Buffer, n int) []string {
	t.Helper()
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		select {
		case ev := <-buf.Chan():
			out = append(out, string(ev.Payload))
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out draining event %d/%d", i, n)
		}
	}
	return out
}

func TestFileTailResumesFromCheckpointAfterRestart(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	checkpointPath := filepath.Join(dir, "checkpoint.json")

	if err := os.WriteFile(logPath, []byte("line-1\nline-2\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	buf1 := newTestBuffer(t, "file-1")
	f1 := NewFile(FileConfig{Dir: dir, Pattern: "*.log", ListenerID: "file-1", CheckpointPath: checkpointPath, PollInterval: 10 * time.Millisecond}, buf1, newTestMetrics())
	if err := f1.pollOnce(); err != nil {
		t.Fatalf("poll 1: %v", err)
	}

	got := drainN(t, buf1, 2)
	if got[0] != "line-1" || got[1] != "line-2" {
		t.Fatalf("got %v, want [line-1 line-2]", got)
	}

	// Simulate a restart: new File instance loading the same checkpoint
	// file. Append more lines first — only the NEW lines should be read,
	// never a repeat of line-1/line-2 and never a gap.
	appendFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open for append: %v", err)
	}
	if _, err := appendFile.WriteString("line-3\nline-4\n"); err != nil {
		t.Fatalf("append: %v", err)
	}
	appendFile.Close()

	buf2 := newTestBuffer(t, "file-1-restart")
	f2 := NewFile(FileConfig{Dir: dir, Pattern: "*.log", ListenerID: "file-1", CheckpointPath: checkpointPath, PollInterval: 10 * time.Millisecond}, buf2, newTestMetrics())
	cp, err := loadCheckpoint(checkpointPath)
	if err != nil {
		t.Fatalf("load checkpoint: %v", err)
	}
	f2.checkpoint = cp
	if err := f2.pollOnce(); err != nil {
		t.Fatalf("poll after restart: %v", err)
	}

	got2 := drainN(t, buf2, 2)
	if got2[0] != "line-3" || got2[1] != "line-4" {
		t.Fatalf("after restart got %v, want [line-3 line-4] (no duplicate, no gap)", got2)
	}
}

func TestFileTailHandlesPartialLineAcrossPolls(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	if err := os.WriteFile(logPath, []byte("complete-line\npartial-no-newline-yet"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	buf := newTestBuffer(t, "file-2")
	f := NewFile(FileConfig{Dir: dir, Pattern: "*.log", ListenerID: "file-2", CheckpointPath: filepath.Join(dir, "cp.json"), PollInterval: 10 * time.Millisecond}, buf, newTestMetrics())
	if err := f.pollOnce(); err != nil {
		t.Fatalf("poll: %v", err)
	}

	got := drainN(t, buf, 1)
	if got[0] != "complete-line" {
		t.Fatalf("got %q, want %q", got[0], "complete-line")
	}

	select {
	case ev := <-buf.Chan():
		t.Fatalf("unexpected event before the line was terminated: %q", ev.Payload)
	case <-time.After(100 * time.Millisecond):
		// expected: the partial line must not be emitted yet
	}

	appendFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open for append: %v", err)
	}
	appendFile.WriteString("\n")
	appendFile.Close()

	if err := f.pollOnce(); err != nil {
		t.Fatalf("poll 2: %v", err)
	}
	got2 := drainN(t, buf, 1)
	if got2[0] != "partial-no-newline-yet" {
		t.Fatalf("got %q", got2[0])
	}
}
