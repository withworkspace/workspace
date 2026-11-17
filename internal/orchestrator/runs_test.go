package orchestrator

import (
	"github.com/withworkspace/workspace/internal/store/storetest"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
)

// Claude Code's stream-json, as 2.1.319 printed it: a call that ran, one
// refused, and the final message with the refusal listed.
const claudeSample = `{"type":"system","subtype":"init"}
{"type":"assistant","message":{"content":[{"type":"text","text":"Looking."},{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"ws mission show -f llm","description":"x"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","content":"Mission: cache","is_error":false}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t2","name":"Bash","input":{"command":"git -C repos/demo/.ground log -1"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t2","content":"Permission to use Bash has been denied because Claude Code is running in don't ask mode.","is_error":true}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t3","name":"Read","input":{"file_path":"/w/brief.md"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t3","content":[{"type":"text","text":"## Goal"}]}]}}
{"type":"result","subtype":"success","result":"I'd flag it, once Bash is available.","is_error":false,"permission_denials":[{"tool_name":"Bash","tool_use_id":"t2"}]}
`

// A run that did nothing is told so when it's run again: what it said,
// and what was refused, from its stream as its harness reads it.
func TestRetryNote(t *testing.T) {
	r := harness.ReadStream(harness.StreamClaude, claudeSample)
	note := RetryNote(Report{Output: r.Output, Calls: r.Calls})
	if !strings.Contains(note, "Only these were refused, each on its own (they aren't on your allowed list): `git -C repos/demo/.ground log -1`") {
		t.Errorf("retry note:\n%s", note)
	}
}

func TestRuns_KeptNewestFirst(t *testing.T) {
	store := mission.Store{Root: storetest.Own(t, t.TempDir()).Root()}
	m, _ := store.Create("Cache")
	at := time.Date(2026, 11, 9, 16, 0, 0, 0, time.UTC)
	for i := range keepRuns + 2 {
		r := RunRecord{Started: at.Add(time.Duration(i) * time.Minute), For: "c1 (done)", Prompt: "the prompt", Output: "said"}
		if err := SaveRun(store, m.Slug, &r); err != nil {
			t.Fatal(err)
		}
	}
	same := RunRecord{Started: at.Add(time.Duration(keepRuns+1) * time.Minute), Retry: true}
	SaveRun(store, m.Slug, &same) // the same second: a second id
	runs := Runs(store, m.Slug)
	if len(runs) != keepRuns || runs[0].ID != same.ID || !runs[0].Retry || runs[0].Prompt != "" {
		t.Fatalf("%d runs, newest %+v", len(runs), runs[0])
	}
	if !strings.HasSuffix(same.ID, "-2") {
		t.Errorf("a run in the same second got id %q", same.ID)
	}
	r, err := LoadRun(store, m.Slug, runs[1].ID)
	if err != nil || r.Prompt != "the prompt" {
		t.Errorf("load = %+v, %v", r, err)
	}
	if _, err := LoadRun(store, m.Slug, "../mission"); err == nil {
		t.Error("loaded outside the runs folder")
	}
}
