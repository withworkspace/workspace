package harness

import (
	"encoding/json"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// A session file read as it grows, a piece at a time and cut anywhere (in
// the middle of a line too), comes to what reading it whole does; and each
// read's From is right: what's before it is as the read before said.
func TestTranscriptReader_AsItGrows(t *testing.T) {
	for _, c := range []struct{ harness, file string }{{"claude", "claude-session.jsonl"}, {"pi", "pi-session.jsonl"}} {
		whole, err := os.ReadFile(filepath.Join("testdata", c.file))
		if err != nil {
			t.Fatal(err)
		}
		want := asJSON(t, ReadTranscript(c.harness, whole))
		for seed := range uint64(20) {
			rng := rand.New(rand.NewPCG(seed, 7))
			path := filepath.Join(t.TempDir(), c.file)
			os.WriteFile(path, nil, 0o644)
			r := NewTranscriptReader(c.harness)
			var prev []TranscriptEntry
			for at := 0; at < len(whole); {
				n := min(len(whole)-at, 1+rng.IntN(len(whole)/5+1))
				f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
				f.Write(whole[at : at+n])
				f.Close()
				at += n
				got, err := r.Read(path)
				if err != nil {
					t.Fatal(err)
				}
				if got.From > len(prev) || got.From > len(got.Entries) {
					t.Fatalf("%s: from %d of %d, after %d", c.harness, got.From, len(got.Entries), len(prev))
				}
				if asJSON(t, got.Entries[:got.From]) != asJSON(t, prev[:got.From]) {
					t.Fatalf("%s seed %d: entries before %d changed, but From says not", c.harness, seed, got.From)
				}
				prev = got.Entries
			}
			if got := asJSON(t, prev); got != want {
				t.Fatalf("%s seed %d: read as it grew, it isn't what it is read whole", c.harness, seed)
			}
		}
	}
}

// A file shorter than what's been read was replaced: it's read again from
// its start.
func TestTranscriptReader_Restarts(t *testing.T) {
	whole, _ := os.ReadFile("testdata/pi-session.jsonl")
	path := filepath.Join(t.TempDir(), "s.jsonl")
	os.WriteFile(path, whole, 0o644)
	r := NewTranscriptReader("pi")
	if _, err := r.Read(path); err != nil {
		t.Fatal(err)
	}
	first := whole[:len(whole)/2]
	first = first[:lastLine(first)]
	os.WriteFile(path, first, 0o644)
	got, err := r.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Restarted || got.From != 0 || asJSON(t, got.Entries) != asJSON(t, ReadTranscript("pi", first)) {
		t.Errorf("replaced: restarted %v, from %d, %d entries", got.Restarted, got.From, len(got.Entries))
	}
}

func lastLine(b []byte) int {
	for i, v := range slices.Backward(b) {
		if v == '\n' {
			return i + 1
		}
	}
	return 0
}

func asJSON(t *testing.T, v []TranscriptEntry) string {
	t.Helper()
	if v == nil {
		v = []TranscriptEntry{} // none is none, nil or not
	}
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
