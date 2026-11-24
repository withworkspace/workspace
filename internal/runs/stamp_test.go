package runs

import (
	"os"
	"testing"
	"time"
)

// The stamp moves when a run starts, comes up, ends and is cleared away:
// not as it writes its output, which a service does all the time.
func TestStamp(t *testing.T) {
	s, dir := setup(t)
	empty := s.Stamp()
	r := start(t, s, dir, "web", listener()+" --port {port}", Owner{Kind: ByNone}, time.Hour)
	if ready, err := s.Wait(r, 10*time.Second); err != nil || !ready {
		t.Fatalf("not up: %v", err)
	}
	up := s.Stamp()
	if up == empty {
		t.Fatal("a run started, and the stamp didn't move")
	}
	f, _ := os.OpenFile(r.LogPath(), os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString("GET / 200\n")
	f.Close()
	if s.Stamp() != up {
		t.Error("its output moved the stamp")
	}
	if err := s.End(r); err != nil {
		t.Fatal(err)
	}
	if s.Stamp() == up {
		t.Error("it ended, and the stamp didn't move")
	}
}

// A log is read from where it was read to: what's written after, and its
// size; a log begun again is said to be.
func TestReadFrom(t *testing.T) {
	path := t.TempDir() + "/out.log"
	os.WriteFile(path, []byte("one\ntwo\n"), 0o644)
	text, size, more, gone, err := ReadFrom(path, 4, 1<<20)
	if err != nil || text != "two\n" || size != 8 || more || gone {
		t.Errorf("from 4: %q %d %v %v %v", text, size, more, gone, err)
	}
	if _, _, more, _, _ := ReadFrom(path, 0, 3); !more {
		t.Error("more than max wasn't said")
	}
	os.WriteFile(path, []byte("x\n"), 0o644)
	if _, size, _, gone, _ := ReadFrom(path, 8, 1<<20); !gone || size != 2 {
		t.Errorf("begun again: gone %v, size %d", gone, size)
	}
}
