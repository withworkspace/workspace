package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/runs"
)

// The Runs page follows the runs over the socket: told when one starts and
// when it ends, not as it writes its output. A run's log is followed too,
// and told what's written to it, from as much as the page has. A ground or
// capsule followed on the Files page is open, for the runs started there.
func TestSocket_FollowsRuns(t *testing.T) {
	if _, err := runs.ReadProcs(); err != nil {
		t.Skipf("processes can't be read here: %v", err)
	}
	poll := transcriptPoll
	transcriptPoll = 20 * time.Millisecond
	t.Cleanup(func() { transcriptPoll = poll })
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	dir := filepath.Join(s.Root, "repos", "demo", "feat")
	os.MkdirAll(dir, 0o755)
	store := runs.Store{Root: s.Root}
	t.Cleanup(func() {
		list, _ := store.List()
		for _, r := range list {
			store.End(r)
		}
	})
	conn, _, err := dialSocket(t, s, s.Info().Origin())
	if err != nil {
		t.Fatal(err)
	}
	st := talkTo(t, conn)
	st.ask(SocketAsk{Op: SocketFollow, Topic: "runs"})
	if got := st.next(3 * time.Second); got != "following runs" {
		t.Fatalf("following: %q", got)
	}

	r, err := store.Start(runs.Start{Name: "web", Repo: "demo", Worktree: "feat", Dir: dir, Command: "sleep 300", Owner: runs.Owner{Kind: runs.ByNone}, TTL: time.Hour, By: "you"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got := st.next(3 * time.Second); got != "changed runs" {
		t.Fatalf("a run started: %q", got)
	}
	var info RunsInfo
	json.NewDecoder(do(t, s, "GET", "/api/runs", true, nil).Body).Decode(&info)
	if len(info.Runs) != 1 || info.Runs[0].Ref != r.Ref() {
		t.Fatalf("the runs: %+v", info.Runs)
	}

	// Its log: as much as there is, then what's written after.
	ref := r.Ref()
	st.ask(SocketAsk{Op: SocketFollow, Topic: "runlog:" + ref})
	if got := st.next(3 * time.Second); got != "following runlog:"+ref {
		t.Fatalf("following its log: %q", got)
	}
	var held RunLog
	json.NewDecoder(do(t, s, "GET", "/api/runs/log?ref="+ref, true, nil).Body).Decode(&held)
	f, _ := os.OpenFile(r.LogPath(), os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString("listening on :3000\n")
	f.Close()
	if got := st.next(3 * time.Second); got != "changed runlog:"+ref {
		t.Fatalf("written to: %q", got)
	}
	op := st.last.RunLog
	if op == nil || op.From != held.Size || op.Text != "listening on :3000\n" || op.Size != held.Size+int64(len(op.Text)) {
		t.Fatalf("what was written: %+v, holding %d", op, held.Size)
	}
	// Output isn't a change to the runs.
	if got := st.next(300 * time.Millisecond); got != "(nothing)" {
		t.Errorf("after its output: %q", got)
	}

	// A capsule followed on the Files page is open.
	st.ask(SocketAsk{Op: SocketFollow, Topic: "worktree:demo/feat"})
	if got := st.next(3 * time.Second); got != "following worktree:demo/feat" {
		t.Fatalf("following a capsule: %q", got)
	}
	if !s.filesFollowed.open()["demo/feat"] {
		t.Error("a capsule followed isn't open")
	}

	if err := store.End(r); err != nil {
		t.Fatal(err)
	}
	for got := st.next(3 * time.Second); got != "changed runs"; got = st.next(3 * time.Second) {
		if got == "(nothing)" {
			t.Fatal("a run ended, and the runs weren't told")
		}
	}
}
