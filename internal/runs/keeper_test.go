package runs

import (
	"os"
	"strings"
	"testing"
	"time"
)

// What's kept moves as the files' stamp does: a run started, up, its
// output (not), ended, cleared away. And it's a lookup once kept.
func TestKeeper_MovesAsTheRunsDo(t *testing.T) {
	s, dir := setup(t)
	k := &Keeper{Store: s}
	if _, ok := k.Stamp(); ok {
		t.Fatal("kept before anything was looked at")
	}
	k.Look(time.Now())
	empty, ok := k.Stamp()
	if !ok {
		t.Fatal("not kept after a look")
	}
	if in, _ := k.StampIn("demo", "feat"); in != "" {
		t.Errorf("a place with no runs: %q", in)
	}

	r := start(t, s, dir, "web", listener()+" --port {port}", Owner{Kind: ByNone}, time.Hour)
	if !k.Look(time.Now()) {
		t.Fatal("a run started, and the look didn't say it moved")
	}
	started, _ := k.Stamp()
	if started == empty {
		t.Fatal("a run started, and the stamp didn't move")
	}
	if _, ok, kept := k.Running("demo", "feat", ""); !kept || !ok {
		t.Errorf("running: %v, kept %v", ok, kept)
	}
	if in, _ := k.StampIn("demo", "feat"); in == "" {
		t.Error("its place's stamp didn't move")
	}
	if in, _ := k.StampIn("demo", "other"); in != "" {
		t.Errorf("another place's stamp moved: %q", in)
	}

	// Coming up is found by asking (Probe), not by a file.
	if ready, err := s.Wait(r, 10*time.Second); err != nil || !ready {
		t.Fatalf("not up: %v", err)
	}
	var next time.Duration
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, next = k.Probe(time.Now())
		if _, starting, _ := k.Starting("demo", "feat", ""); !starting || time.Now().After(deadline) {
			break
		}
		time.Sleep(ProbeStarting)
	}
	if _, starting, _ := k.Starting("demo", "feat", ""); starting {
		t.Fatal("up, and still kept as starting")
	}
	if next != ProbeEvery {
		t.Errorf("once up, probed again in %v, not %v", next, ProbeEvery)
	}
	up, _ := k.Stamp()
	if up == started {
		t.Error("it came up, and the stamp didn't move")
	}

	f, _ := os.OpenFile(r.LogPath(), os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString("GET / 200\n")
	f.Close()
	k.Look(time.Now())
	k.Probe(time.Now())
	if now, _ := k.Stamp(); now != up {
		t.Error("its output moved the stamp")
	}

	v := k.Version()
	if err := s.End(r); err != nil {
		t.Fatal(err)
	}
	k.Look(time.Now())
	if now, _ := k.Stamp(); now == up {
		t.Error("it ended, and the stamp didn't move")
	}
	if k.Version() == v {
		t.Error("it ended, and the version didn't move")
	}
	if _, ok, _ := k.Running("demo", "feat", ""); ok {
		t.Error("ended, and still kept as running")
	}
}

// The folders watched are the store's down to each run's own, not below
// it (its reports) nor the earlier runs; its output isn't heeded.
func TestKeeper_Folders(t *testing.T) {
	s, dir := setup(t)
	r := task(t, s, dir, "test", "true")
	ended(t, s, r)
	task(t, s, dir, "test", "true") // the first goes behind it, in .past
	os.MkdirAll(r.ReportsDir()+"/cover", 0o755)
	k := &Keeper{Store: s}
	got := map[string]bool{}
	for _, f := range k.Folders() {
		got[f] = true
	}
	for _, want := range []string{s.Dir(), s.Dir() + "/demo", s.Dir() + "/demo/feat", r.Path} {
		if !got[want] {
			t.Errorf("%s isn't watched: %v", want, got)
		}
	}
	for f := range got {
		if f == r.ReportsDir() || f == s.Dir()+"/"+pastDir {
			t.Errorf("%s is watched", f)
		}
	}
	if k.Heeds(r.LogPath()) {
		t.Error("its output is heeded")
	}
	if !k.Heeds(r.Path + "/" + exitFile) {
		t.Error("its exit isn't heeded")
	}
}

// What happens to the runs is noted as it's seen, from the second look
// on: a run started, a service up, and how each ended.
func TestKeeper_Changes(t *testing.T) {
	s, dir := setup(t)
	before := task(t, s, dir, "lint", "true")
	ended(t, s, before)
	k := &Keeper{Store: s}
	k.Look(time.Now())
	if c := k.Changes(); len(c) != 0 {
		t.Fatalf("the first look said what's there as changes: %+v", c)
	}

	r := start(t, s, dir, "web", listener()+" --port {port}", Owner{Kind: ByNone}, time.Hour)
	k.Look(time.Now())
	if ready, err := s.Wait(r, 10*time.Second); err != nil || !ready {
		t.Fatalf("not up: %v", err)
	}
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(ProbeStarting) {
		k.Probe(time.Now())
		if _, starting, _ := k.Starting("demo", "feat", ""); !starting {
			break
		}
	}
	if err := s.End(r); err != nil {
		t.Fatal(err)
	}
	k.Look(time.Now())

	lint := task(t, s, dir, "test", "true")
	ended(t, s, lint)
	k.Look(time.Now())

	var got []string
	for _, c := range k.Changes() {
		got = append(got, c.Kind+" "+c.Run.Name+" "+c.Status)
	}
	want := []string{"started web ", "up web ", "ended web stopped", "started test ", "ended test passed"}
	if strings.Join(got, ", ") != strings.Join(want, ", ") {
		t.Errorf("changes:\n got %v\nwant %v", got, want)
	}
	if c := k.Changes(); len(c) != 0 {
		t.Errorf("said twice: %+v", c)
	}
}
