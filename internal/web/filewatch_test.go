package web

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// A followed thing whose file's watched is told as the file's written,
// not at the hub's next poll, and the polls pass it by (no stat); one
// whose file isn't there yet is looked at each poll until it is, and
// watched from then.
func TestKeyedHub_WatchesAFollowedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "one.run")
	var stats atomic.Int32
	h := newKeyedHub("thing:", 50*time.Millisecond, func(string) string {
		stats.Add(1)
		info, err := os.Stat(path)
		if err != nil {
			return ""
		}
		return fmt.Sprint(info.Size(), info.ModTime().UnixNano())
	})
	h.watch = newFileWatch(t.Logf)
	h.pathOf = func(string) string { return path }
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go h.watch.run(ctx)
	go h.run(ctx)
	ch, stop := h.subscribe("one")
	t.Cleanup(stop)
	told := func(within time.Duration) bool {
		select {
		case <-ch:
			return true
		case <-time.After(within):
			return false
		}
	}

	// Not there yet: looked at each poll, and told once it's written.
	time.Sleep(200 * time.Millisecond)
	if n := stats.Load(); n < 3 {
		t.Errorf("a file not there yet was looked at %d times in 200 ms", n)
	}
	os.WriteFile(path, []byte("one\n"), 0o644)
	if !told(time.Second) {
		t.Fatal("made, and not told")
	}

	// Watched from then: the polls pass it by, and a write's told at once.
	time.Sleep(200 * time.Millisecond)
	stats.Store(0)
	time.Sleep(300 * time.Millisecond)
	if n := stats.Load(); n != 0 {
		t.Errorf("a watched file was looked at %d times by polls", n)
	}
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	start := time.Now()
	f.WriteString("two\n")
	f.Close()
	if !told(time.Second) {
		t.Fatal("written, and not told")
	}
	if took := time.Since(start); took > 300*time.Millisecond {
		t.Errorf("told %v after it was written", took)
	}

	// Replaced (written whole, and moved into place): watched again.
	os.WriteFile(path+".new", []byte("three\n"), 0o644)
	os.Rename(path+".new", path)
	if !told(time.Second) {
		t.Fatal("replaced, and not told")
	}
	time.Sleep(100 * time.Millisecond)
	f, _ = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString("four\n")
	f.Close()
	if !told(time.Second) {
		t.Fatal("written after it was replaced, and not told")
	}

	// Let go: nothing's watched.
	stop()
	time.Sleep(100 * time.Millisecond)
	h.watch.mu.Lock()
	defer h.watch.mu.Unlock()
	if len(h.watch.on) != 0 || len(h.watch.holders) != 0 {
		t.Errorf("still watched once nobody follows it: %v %v", h.watch.on, h.watch.holders)
	}
}
