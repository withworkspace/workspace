package acp

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// update makes a session update from its JSON, as it would come.
func update(t *testing.T, js string) SessionUpdate {
	t.Helper()
	u, err := ParseUpdate(json.RawMessage(js))
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// A turn as an agent sends it: thinking, words, a tool call that's asked
// for, runs and finishes with a diff, more words.
func turnUpdates(t *testing.T) []SessionUpdate {
	return []SessionUpdate{
		update(t, `{"sessionUpdate":"agent_thought_chunk","content":{"type":"text","text":"Let me "}}`),
		update(t, `{"sessionUpdate":"agent_thought_chunk","content":{"type":"text","text":"look."}}`),
		update(t, `{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"I'll "}}`),
		update(t, `{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"fix it."}}`),
		update(t, `{"sessionUpdate":"tool_call","toolCallId":"t1","title":"Edit a.go","kind":"edit","status":"pending","locations":[{"path":"a.go","line":3}],"rawInput":{"path":"a.go"}}`),
		update(t, `{"sessionUpdate":"plan","entries":[{"content":"fix","priority":"high","status":"in_progress"},{"content":"test","priority":"medium","status":"pending"}]}`),
		update(t, `{"sessionUpdate":"tool_call_update","toolCallId":"t1","status":"in_progress"}`),
		update(t, `{"sessionUpdate":"tool_call_update","toolCallId":"t1","status":"completed","content":[{"type":"diff","path":"a.go","oldText":"x","newText":"y"}]}`),
		update(t, `{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"Done."}}`),
	}
}

func TestTranscriptFoldsATurn(t *testing.T) {
	var tr Transcript
	at := time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)
	tr.Sent(at, Text("fix a.go"))
	if tr.Reply() != "" {
		t.Errorf("a reply before the agent's said anything: %q", tr.Reply())
	}
	for i, u := range turnUpdates(t) {
		tr.Apply(at.Add(time.Duration(i)*time.Second), u)
		if i == 4 {
			if r := tr.Running(); len(r) != 1 || r[0].ID != "t1" || r[0].Status != StatusPending {
				t.Errorf("running after the call was asked for = %+v", r)
			}
		}
	}

	var kinds []string
	for _, it := range tr.Items {
		kinds = append(kinds, it.Kind+":"+it.Text)
	}
	want := []string{"user:fix a.go", "thought:Let me look.", "agent:I'll fix it.", "tool:", "agent:Done."}
	if !reflect.DeepEqual(kinds, want) {
		t.Errorf("items = %q, want %q", kinds, want)
	}
	c := tr.Call("t1")
	if c == nil {
		t.Fatal("no call t1")
	}
	// An update without a title or locations keeps the call's.
	if c.Title != "Edit a.go" || c.Kind != "edit" || c.Status != StatusCompleted || !c.Done() ||
		len(c.Locations) != 1 || *c.Locations[0].Line != 3 || string(c.Input) != `{"path":"a.go"}` {
		t.Errorf("call = %+v", c)
	}
	if len(c.Content) != 1 || c.Content[0].Type != "diff" || *c.Content[0].OldText != "x" || c.Content[0].NewText != "y" {
		t.Errorf("content = %+v", c.Content)
	}
	if !c.Updated.Equal(at.Add(7 * time.Second)) {
		t.Errorf("updated %v", c.Updated)
	}
	if tr.Items[3].At != at.Add(4*time.Second) {
		t.Errorf("the call's item began %v", tr.Items[3].At)
	}
	if len(tr.Running()) != 0 {
		t.Errorf("still running: %+v", tr.Running())
	}
	if tr.Reply() != "Done." {
		t.Errorf("reply = %q", tr.Reply())
	}
	if len(tr.Plan) != 2 || tr.Plan[0].Status != "in_progress" {
		t.Errorf("plan = %+v", tr.Plan)
	}
}

// Messages the agent names are kept apart, even back to back; a chunk
// that isn't text says what it is.
func TestTranscriptMessages(t *testing.T) {
	var tr Transcript
	for _, js := range []string{
		`{"sessionUpdate":"agent_message_chunk","messageId":"m1","content":{"type":"text","text":"one"}}`,
		`{"sessionUpdate":"agent_message_chunk","messageId":"m2","content":{"type":"text","text":"two"}}`,
		`{"sessionUpdate":"agent_message_chunk","messageId":"m2","content":{"type":"resource_link","uri":"file:///a.go","name":"a.go"}}`,
		`{"sessionUpdate":"user_message_chunk","content":{"type":"image","data":"…","mimeType":"image/png"}}`,
	} {
		tr.Apply(time.Time{}, update(t, js))
	}
	var texts []string
	for _, it := range tr.Items {
		texts = append(texts, it.Kind+":"+it.Text)
	}
	if want := []string{"agent:one", "agent:two[a.go]", "user:[image]"}; !reflect.DeepEqual(texts, want) {
		t.Errorf("items = %q, want %q", texts, want)
	}
}

// What the session is like now, and an update to a call it hasn't seen.
func TestTranscriptSession(t *testing.T) {
	var tr Transcript
	for _, js := range []string{
		`{"sessionUpdate":"available_commands_update","availableCommands":[{"name":"simplify","description":"Find low-info comments"}]}`,
		`{"sessionUpdate":"current_mode_update","currentModeId":"plan"}`,
		`{"sessionUpdate":"config_option_update","configOptions":[{"id":"model","name":"Model","category":"model","type":"select","currentValue":"default[]","options":[{"value":"default[]","name":"Auto"}]}]}`,
		`{"sessionUpdate":"session_info_update","title":"File And List"}`,
		`{"sessionUpdate":"session_info_update","updatedAt":"2026-11-11T13:19:36Z"}`,
		`{"sessionUpdate":"usage_update","used":1200,"size":300000}`,
		`{"sessionUpdate":"notice","severity":"warning","title":"Context nearly full"}`,
		`{"sessionUpdate":"subagent_spawned","agentId":"a1"}`,
		`{"sessionUpdate":"tool_call_update","toolCallId":"late","status":"failed"}`,
	} {
		tr.Apply(time.Time{}, update(t, js))
	}
	if len(tr.Commands) != 1 || tr.Commands[0].Name != "simplify" || tr.Mode != "plan" || tr.Title != "File And List" ||
		tr.Used != 1200 || tr.Size != 300000 || len(tr.Config) != 1 || tr.Config[0].Pick("auto") != "default[]" {
		t.Errorf("session = %+v", tr)
	}
	if len(tr.Items) != 2 || tr.Items[0].Kind != ItemNotice || tr.Items[0].Severity != "warning" || tr.Items[0].Text != "Context nearly full" {
		t.Fatalf("items = %+v", tr.Items)
	}
	if c := tr.Items[1].Tool; c == nil || c.ID != "late" || c.Status != StatusFailed || !c.Done() {
		t.Errorf("a call first heard of in an update = %+v", tr.Items[1])
	}
}

// A clone doesn't change with the transcript, and a transcript read back
// from JSON goes on updating its calls in place.
func TestTranscriptCloneAndJSON(t *testing.T) {
	var tr Transcript
	ups := turnUpdates(t)
	for _, u := range ups[:5] {
		tr.Apply(time.Time{}, u)
	}
	c := tr.Clone()
	for _, u := range ups[5:] {
		tr.Apply(time.Time{}, u)
	}
	if c.Call("t1").Status != StatusPending || len(c.Items) != 3 || c.Plan != nil {
		t.Errorf("the clone changed: %+v", c)
	}

	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var back Transcript
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	back.Apply(time.Time{}, ups[7])
	if len(back.Items) != 3 || back.Call("t1").Status != StatusCompleted {
		t.Errorf("read back and updated: %+v", back.Items)
	}
}

// A record written as a session goes, read back as it grows (a line half
// written left for later), replays into the transcript the session made.
func TestRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s1.jsonl")
	rec, err := AppendRecord(path)
	if err != nil {
		t.Fatal(err)
	}
	var direct Transcript
	at := time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)
	rec.Write(Event{At: at, Session: "s1"})
	rec.Write(Event{At: at, Sent: []ContentBlock{Text("fix a.go")}})
	direct.Sent(at, Text("fix a.go"))
	ups := turnUpdates(t)
	for _, u := range ups[:4] {
		rec.Write(Event{At: at, Update: u.Raw})
		direct.Apply(at, u)
	}
	// A line half written, as a reader might catch it.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`{"at":"2026-11-11T09:00:00Z","upd`)
	f.Close()

	events, off, err := ReadRecordFile(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 6 || events[0].Session != "s1" || events[0].At != at {
		t.Fatalf("read %d events: %+v", len(events), events)
	}
	followed := Replay(nil, events)

	// The rest of that line, and the turn's rest.
	f, _ = os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	f.WriteString(`ate":` + string(ups[4].Raw) + "}\n")
	f.Close()
	direct.Apply(at, ups[4])
	for _, u := range ups[5:] {
		rec.Write(Event{At: at, Update: u.Raw})
		direct.Apply(at, u)
	}
	rec.Write(Event{At: at, Stop: "end_turn"})
	rec.Write(Event{At: at, Ended: "exited"})
	if err := rec.Close(); err != nil {
		t.Fatal(err)
	}

	more, off2, err := ReadRecordFile(path, off)
	if err != nil || off2 <= off {
		t.Fatalf("read on: %v, offset %d → %d", err, off, off2)
	}
	if len(more) != 7 || more[5].Stop != "end_turn" || more[6].Ended != "exited" {
		t.Fatalf("read on %d events: %+v", len(more), more)
	}
	Replay(followed, more)
	if !reflect.DeepEqual(followed.Items, direct.Items) || !reflect.DeepEqual(followed.Plan, direct.Plan) {
		t.Errorf("followed:\n%+v\ndirect:\n%+v", followed.Items, direct.Items)
	}
	if rest, off3, err := ReadRecordFile(path, off2); err != nil || len(rest) != 0 || off3 != off2 {
		t.Errorf("at the end: %d events, offset %d, %v", len(rest), off3, err)
	}
	if none, _, err := ReadRecordFile(filepath.Join(t.TempDir(), "nope.jsonl"), 0); err != nil || none != nil {
		t.Errorf("a record not there yet: %v, %v", none, err)
	}
}

type failingWriter struct{ n int }

func (w *failingWriter) Write(p []byte) (int, error) {
	w.n++
	return 0, errors.New("disk full")
}

// A record that can't be written says so, and stops trying.
func TestRecorderKeepsItsError(t *testing.T) {
	w := &failingWriter{}
	rec := NewRecorder(w)
	if err := rec.Write(Event{Stop: "end_turn"}); err == nil {
		t.Fatal("a failed write said nothing")
	}
	rec.Write(Event{Stop: "end_turn"})
	if rec.Err() == nil || w.n != 1 {
		t.Errorf("err %v, tried %d times", rec.Err(), w.n)
	}
}
