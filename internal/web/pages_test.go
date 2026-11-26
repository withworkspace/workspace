package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/ops"
	pages "github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/workspace"
)

func pageWrite(t *testing.T, s *Server, method, path string, v any, into any) *http.Response {
	t.Helper()
	data, _ := json.Marshal(v)
	res := do(t, s, method, path, true, func(r *http.Request) {
		r.Body = io.NopCloser(bytes.NewReader(data))
		r.ContentLength = int64(len(data))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", s.Info().Origin())
	})
	if res.StatusCode == 200 && into != nil {
		json.NewDecoder(res.Body).Decode(into)
	}
	return res
}

// You start a page, save it as you write, and ask for a cell: it's
// numbered at once, filled when the page says to, and filled again when
// you ask for a change. The page sees a run's calls as it goes.
func TestPagesAPI(t *testing.T) {
	s, _ := docServer(t)
	var asked []string
	answersInto(s, &asked)

	var d PageDetail
	if res := pageWrite(t, s, "POST", "/api/pages", map[string]any{"title": " Token   cache "}, &d); res.StatusCode != 200 || d.ID != "p1" || d.Title != "Token cache" || d.Text != "# Token cache\n" {
		t.Fatalf("new: %d %+v", res.StatusCode, d)
	}

	text := "# Token cache\n\nThe routes:\n"
	var saved PageSaved
	if res := pageWrite(t, s, "PUT", "/api/pages/p1", map[string]any{"text": text, "base": d.Version}, &saved); res.StatusCode != 200 || saved.Version == d.Version {
		t.Fatalf("save: %d %+v", res.StatusCode, saved)
	}
	if res := pageWrite(t, s, "PUT", "/api/pages/p1", map[string]any{"text": "# Other\n", "base": d.Version}, nil); res.StatusCode != http.StatusConflict {
		t.Errorf("a save from the version before: %d", res.StatusCode)
	}
	if res := pageWrite(t, s, "PUT", "/api/pages/p7", map[string]any{"text": "x", "base": ""}, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("saving no such page: %d", res.StatusCode)
	}

	// A cell: numbered, and not filled until the page asks.
	var cell PageCell
	if res := pageWrite(t, s, "POST", "/api/pages/p1/cells", map[string]any{"prompt": " "}, nil); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("a cell asking for nothing: %d", res.StatusCode)
	}
	if res := pageWrite(t, s, "POST", "/api/pages/p1/cells", map[string]any{"prompt": "a table of the existing routes"}, &cell); res.StatusCode != 200 || cell.Conversation == nil || cell.ID != "w1" || cell.Kind != conversation.Cell || cell.Anchor.Page != "p1" {
		t.Fatalf("a cell: %d %+v", res.StatusCode, cell)
	}
	if len(asked) != 0 {
		t.Errorf("filled before the page asked: %v", asked)
	}
	if res := pageWrite(t, s, "POST", "/api/pages/p1/cells/w1/fill", nil, nil); res.StatusCode != 200 || len(asked) != 1 || asked[0] != "/w1" {
		t.Fatalf("fill: %d %v", res.StatusCode, asked)
	}

	// While it's filling, its calls so far.
	store := conversation.ForWorkspace(s.Root)
	store.StartAnswering("w1", os.Getpid())
	os.WriteFile(store.RunPath("w1"), []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"u1","name":"Read","input":{"file_path":"/ws/repos/api/.ground/routes.go"}}]}}`+"\n"), 0o644)
	var got PageDetail
	json.NewDecoder(do(t, s, "GET", "/api/pages/p1", true, nil).Body).Decode(&got)
	if got.Text != text || len(got.Cell) != 1 || got.Cell[0].Run == nil || len(got.Cell[0].Run.Calls) != 1 {
		t.Errorf("the page while its cell fills: %+v", got)
	}
	if list := Pages(s.Root); len(list) != 1 || list[0].Cells != 0 || list[0].Filling != 1 {
		t.Errorf("listed, the cell not in its text yet: %+v", list)
	}
	store.EndAnswering("w1")

	// Written from a file: the cell says so, and says when it's changed.
	routes := filepath.Join(s.Root, "repos", "api", ".ground", "routes.go")
	os.MkdirAll(filepath.Dir(routes), 0o755)
	os.WriteFile(routes, []byte("package api\n"), 0o644)
	src, ok := conversation.SourceOf(s.Root, routes)
	if _, own := conversation.SourceOf(s.Root, filepath.Join(s.Root, ".ws", "pages", "p1.md")); !ok || own || src.Path != "repos/api/.ground/routes.go" {
		t.Fatalf("a source: %+v %v, the page itself: %v", src, ok, own)
	}
	store.Say("w1", conversation.Turn{By: conversation.Agent, At: time.Now(), Body: "| Route |\n|---|\n| /tokens |", Sources: []conversation.Source{src}})
	withCell := text + "\n<!-- ws:cell w1 v1 | a table of the existing routes -->\n\n| Route |\n|---|\n| /tokens |\n\n<!-- /ws:cell -->\n"
	pageWrite(t, s, "PUT", "/api/pages/p1", map[string]any{"text": withCell, "base": saved.Version}, nil)
	json.NewDecoder(do(t, s, "GET", "/api/pages/p1", true, nil).Body).Decode(&got)
	if cs := got.Cell[0].Sources; len(cs) != 1 || cs[0].Repo != "api" || cs[0].Worktree != ".ground" || cs[0].File != "routes.go" || cs[0].State != "" {
		t.Errorf("what it was written from: %+v", cs)
	}
	if list := Pages(s.Root); list[0].Cells != 1 || list[0].Stale != 0 {
		t.Errorf("listed, filled: %+v", list)
	}
	os.WriteFile(routes, []byte("package api\n\nfunc Routes() {}\n"), 0o644)
	json.NewDecoder(do(t, s, "GET", "/api/pages/p1", true, nil).Body).Decode(&got)
	if cs := got.Cell[0].Sources; cs[0].State != conversation.SourceChanged {
		t.Errorf("its file changed: %+v", cs)
	}
	if list := Pages(s.Root); list[0].Stale != 1 {
		t.Errorf("listed, stale: %+v", list)
	}
	os.Remove(routes)
	json.NewDecoder(do(t, s, "GET", "/api/pages/p1", true, nil).Body).Decode(&got)
	if cs := got.Cell[0].Sources; cs[0].State != conversation.SourceGone {
		t.Errorf("its file gone: %+v", cs)
	}

	// Filled, there's nothing to fill; asking for a change fills it again.
	pageWrite(t, s, "POST", "/api/pages/p1/cells/w1/fill", nil, nil)
	if len(asked) != 1 {
		t.Errorf("filled again with nothing asked: %v", asked)
	}
	if res := pageWrite(t, s, "POST", "/api/pages/p1/cells/w1/say", map[string]any{"body": "group them by service", "edited": "| Route |\n|---|\n| /tokens (cached) |\n"}, &cell); res.StatusCode != 200 || len(cell.Turns) != 3 || len(asked) != 2 {
		t.Fatalf("a change: %d %+v %v", res.StatusCode, cell, asked)
	}
	if cell.Turns[2].Edited != "| Route |\n|---|\n| /tokens (cached) |" {
		t.Errorf("the cell as you'd edited it: %q", cell.Turns[2].Edited)
	}
	var cells struct {
		Cell []PageCell `json:"cell"`
	}
	json.NewDecoder(do(t, s, "GET", "/api/pages/p1/cells", true, nil).Body).Decode(&cells)
	if len(cells.Cell) != 1 || len(cells.Cell[0].Turns) != 3 {
		t.Errorf("the page's cells: %+v", cells)
	}

	// One the agent couldn't fill is filled when it's tried again.
	store.Say("w1", conversation.Turn{By: conversation.Agent, At: time.Now(), Body: "| Service | Route |"})
	pageWrite(t, s, "POST", "/api/pages/p1/cells/w1/say", map[string]any{"body": "and their handlers"}, nil)
	store.Say("w1", conversation.Turn{By: conversation.WS, At: time.Now(), Body: "The agent couldn't answer: exit status 1"})
	if res := pageWrite(t, s, "POST", "/api/pages/p1/cells/w1/fill", nil, nil); res.StatusCode != 200 || len(asked) != 4 {
		t.Errorf("trying a failed cell again: %d %v", res.StatusCode, asked)
	}

	// A cell is its page's, and isn't a conversation in the list.
	pageWrite(t, s, "POST", "/api/pages", map[string]any{}, nil)
	if res := pageWrite(t, s, "POST", "/api/pages/p2/cells/w1/say", map[string]any{"body": "x"}, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("another page's cell: %d", res.StatusCode)
	}
	if talks := Talks(s.Root); len(talks) != 0 {
		t.Errorf("cells listed as conversations: %+v", talks)
	}
	if res := do(t, s, "GET", "/api/conversations/w1", true, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("a cell read as a conversation: %d", res.StatusCode)
	}

	if res := pageWrite(t, s, "DELETE", "/api/pages/p2", nil, nil); res.StatusCode != 200 {
		t.Errorf("remove: %d", res.StatusCode)
	}
	if res := do(t, s, "GET", "/api/pages/p2", true, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("a removed page: %d", res.StatusCode)
	}
}

// A cell is out of date when a cell above it changes; one that's about the
// whole page (a check), when the page around it does.
func TestPageCellsMoved(t *testing.T) {
	s, _ := docServer(t)
	var d PageDetail
	pageWrite(t, s, "POST", "/api/pages", map[string]any{"title": "Routes"}, &d)
	var first, second, check PageCell
	pageWrite(t, s, "POST", "/api/pages/p1/cells", map[string]any{"prompt": "the routes"}, &first)
	pageWrite(t, s, "POST", "/api/pages/p1/cells", map[string]any{"prompt": "which of these need auth"}, &second)
	if res := pageWrite(t, s, "POST", "/api/pages/p1/cells", map[string]any{"prompt": "what's open", "goal": "check"}, &check); res.StatusCode != 200 || check.Goal != "check" {
		t.Fatalf("a check: %d %+v", res.StatusCode, check)
	}
	// A question (>?): asked, and what's said in it after can ask too. An
	// answer isn't something written, and a question above isn't something
	// a cell was written beside.
	var q PageCell
	pageWrite(t, s, "POST", "/api/pages/p1/cells", map[string]any{"prompt": "why per tenant?", "ask": true}, &q)
	if q.Goal != conversation.GoalAsk || !q.Turns[0].Ask {
		t.Fatalf("a question: %+v", q)
	}
	conversation.ForWorkspace(s.Root).Say(q.ID, conversation.Turn{By: conversation.Agent, At: time.Now(), Body: "Because tokens are.", Answer: true})
	pageWrite(t, s, "POST", "/api/pages/p1/cells/"+q.ID+"/say", map[string]any{"body": "and per user?", "ask": true}, &q)
	if len(q.Turns) != 3 || !q.Turns[2].Ask || q.Written() != nil {
		t.Errorf("a follow-up question: %+v", q.Turns)
	}
	withQ := "<!-- ws:cell " + q.ID + " v0 ask | why per tenant? -->\n\n<!-- /ws:cell -->\n\n<!-- ws:cell w1 v1 | x -->\n\nBody.\n\n<!-- /ws:cell -->\n"
	if cs := pages.Cells([]byte(withQ)); len(cs) != 2 || cs[0].ID != q.ID || cs[0].Body != "" || pages.Above([]byte(withQ), "w1") != pages.Above([]byte("<!-- ws:cell w1 v1 | x -->\n\nBody.\n\n<!-- /ws:cell -->\n"), "w1") {
		t.Errorf("a question in a page's text: %+v", cs)
	}

	// A question about a pass's finding: a question, with no place in the
	// page, so not one of its cells as it's counted.
	var fq PageCell
	pageWrite(t, s, "POST", "/api/pages/p1/cells", map[string]any{"prompt": "what counts as fast?", "finding": "check:abc"}, &fq)
	if fq.Goal != conversation.GoalAsk || !fq.Turns[0].Ask || fq.Anchor.Finding != "check:abc" {
		t.Fatalf("a question about a finding: %+v", fq)
	}

	// One made around words of yours keeps them.
	var around PageCell
	pageWrite(t, s, "POST", "/api/pages/p1/cells", map[string]any{"prompt": "tighten this", "holds": "We want, more or less, a cache.\n"}, &around)
	if around.ID != "w6" || around.Turns[0].Edited != "We want, more or less, a cache." {
		t.Errorf("a cell around your words: %+v", around)
	}
	if res := pageWrite(t, s, "POST", "/api/pages/p1/cells", map[string]any{"prompt": "x", "goal": "Not A Word"}, nil); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("a goal that isn't a word: %d", res.StatusCode)
	}
	cell := func(id, v, body string) string {
		return "<!-- ws:cell " + id + " v" + v + " | x -->\n\n" + body + "\n\n<!-- /ws:cell -->\n\n"
	}
	text := "# Routes\n\nWords.\n\n" + cell("w1", "1", "- /tokens") + cell("w2", "1", "- /tokens does") + cell("w3", "1", "- [ ] which service?")
	version := d.Version
	save := func(next string) {
		t.Helper()
		var saved PageSaved
		if res := pageWrite(t, s, "PUT", "/api/pages/p1", map[string]any{"text": next, "base": version}, &saved); res.StatusCode != 200 {
			t.Fatalf("save: %d", res.StatusCode)
		}
		version, text = saved.Version, next
	}
	save(text)
	store := conversation.ForWorkspace(s.Root)
	for _, id := range []string{"w1", "w2", "w3"} {
		store.Say(id, conversation.Turn{By: conversation.Agent, At: time.Now(), Body: "…", Above: pages.Above([]byte(text), id), Around: pages.Around([]byte(text), id)})
	}
	moved := func() map[string]string {
		var got PageDetail
		json.NewDecoder(do(t, s, "GET", "/api/pages/p1", true, nil).Body).Decode(&got)
		out := map[string]string{}
		for _, c := range got.Cell {
			out[c.ID] = c.Moved
		}
		return out
	}
	if m := moved(); m["w1"] != "" || m["w2"] != "" || m["w3"] != "" {
		t.Fatalf("just written: %v", m)
	}

	// Words outside any cell: only the check is about them.
	save(strings.Replace(text, "Words.", "Other words.", 1))
	if m := moved(); m["w1"] != "" || m["w2"] != "" || m["w3"] != "page" {
		t.Errorf("after words changed: %v", m)
	}
	// The first cell written again: the one below it is out of date too.
	save(strings.Replace(text, "- /tokens\n", "- /tokens\n- /refresh\n", 1))
	if m := moved(); m["w1"] != "" || m["w2"] != "above" || m["w3"] != "page" {
		t.Errorf("after w1 changed: %v", m)
	}
	if list := Pages(s.Root); list[0].Cells != 3 || list[0].Stale != 2 {
		t.Errorf("listed: %+v", list)
	}
	// Ticking the check's own box changes nothing it was written beside.
	before := moved()
	save(strings.Replace(text, "- [ ] which", "- [x] which", 1))
	if m := moved(); m["w3"] != before["w3"] || m["w2"] != before["w2"] {
		t.Errorf("after its own box was ticked: %v", m)
	}
}

// A page's passes are listed whether or not they've run. Running one
// starts its agent; what it found is served with where each finding
// stands, settling one holds when it's run again, and none of it is in the
// page's text.
func TestPagePassesAPI(t *testing.T) {
	s, _ := docServer(t)
	var asked []string
	idsAnsweredInto(s, &asked)
	var d PageDetail
	pageWrite(t, s, "POST", "/api/pages", map[string]any{"title": "Greet by name"}, &d)
	list := func() []PagePass {
		var got struct {
			Passes []PagePass `json:"passes"`
		}
		json.NewDecoder(do(t, s, "GET", "/api/pages/p1/passes", true, nil).Body).Decode(&got)
		return got.Passes
	}
	if ps := list(); len(ps) != 3 || ps[0].Name != "check" || ps[0].ID != "" || ps[0].Runs != 0 || ps[2].Name != "cold" || !ps[2].Cold {
		t.Fatalf("before any's run: %+v", ps)
	}

	var p PagePass
	if res := pageWrite(t, s, "POST", "/api/pages/p1/passes/nope/run", nil, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("no such pass: %d", res.StatusCode)
	}
	if res := pageWrite(t, s, "POST", "/api/pages/p1/passes/check/run", nil, &p); res.StatusCode != 200 || p.ID != "w1" || len(asked) != 1 {
		t.Fatalf("run: %d %+v %v", res.StatusCode, p, asked)
	}
	store := conversation.ForWorkspace(s.Root)
	summary, fs := conversation.ParseFindings("Not ready.\n\n## question\n> by name\nWhich name?\n\n## missing\nWho reviews it.\n")
	store.Say("w1", conversation.Turn{By: conversation.Agent, At: time.Now(), Body: summary, Findings: fs, Around: d.Version})

	check := list()[0]
	if check.Runs != 1 || check.Summary != "Not ready." || len(check.Findings) != 2 || check.Findings[0].State != conversation.FindingOpen || check.Moved || check.Failed != "" {
		t.Fatalf("after its run: %+v", check)
	}
	if info := Pages(s.Root); info[0].Findings != 2 || info[0].Cells != 0 {
		t.Errorf("listed: %+v", info)
	}
	// Nothing of it is in the page.
	var got PageDetail
	json.NewDecoder(do(t, s, "GET", "/api/pages/p1", true, nil).Body).Decode(&got)
	if got.Text != d.Text || len(got.Cell) != 0 {
		t.Errorf("the page after a pass: %q %+v", got.Text, got.Cell)
	}

	// Settled, by key; a state that isn't one is refused.
	key := check.Findings[0].Key
	if res := pageWrite(t, s, "POST", "/api/pages/p1/passes/check/findings/"+key, map[string]any{"state": "done"}, &p); res.StatusCode != 200 || p.Findings[0].State != "done" || p.Findings[1].State != "open" {
		t.Fatalf("settle: %d %+v", res.StatusCode, p.Findings)
	}
	if res := pageWrite(t, s, "POST", "/api/pages/p1/passes/check/findings/"+key, map[string]any{"state": "maybe"}, nil); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("a state that isn't one: %d", res.StatusCode)
	}
	if res := pageWrite(t, s, "POST", "/api/pages/p1/passes/cold/findings/"+key, map[string]any{"state": "done"}, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("settling in a pass that's not run: %d", res.StatusCode)
	}

	// The page changes: the pass says so. Run again, what was settled holds
	// for the finding on the same words.
	pageWrite(t, s, "PUT", "/api/pages/p1", map[string]any{"text": d.Text + "\nMore.\n", "base": d.Version}, nil)
	if check = list()[0]; !check.Moved {
		t.Errorf("after the page changed: %+v", check)
	}
	pageWrite(t, s, "POST", "/api/pages/p1/passes/check/run", nil, &p)
	if len(asked) != 2 || len(p.Findings) != 2 {
		t.Fatalf("run again: %v %+v", asked, p)
	}
	_, again := conversation.ParseFindings("## question\n> by name\nStill: which name?\n\n## ambiguity\n> More.\nMore what?\n")
	store.Say("w1", conversation.Turn{By: conversation.Agent, At: time.Now(), Body: "Closer.", Findings: again})
	check = list()[0]
	if check.Runs != 2 || len(check.Findings) != 2 || check.Findings[0].State != "done" || check.Findings[0].Text != "Still: which name?" || check.Findings[1].Kind != "ambiguity" || check.Findings[1].State != "open" {
		t.Errorf("after running again: %+v", check)
	}
	// A run that couldn't: it says why, and what it last found stands.
	store.Say("w1", conversation.Turn{By: conversation.You, At: time.Now(), Body: "Run it again."})
	store.Say("w1", conversation.Turn{By: conversation.WS, At: time.Now(), Body: "The agent couldn't answer: exit status 1"})
	if check = list()[0]; check.Failed == "" || len(check.Findings) != 2 {
		t.Errorf("after a run that failed: %+v", check)
	}
	// Passes aren't cells, or conversations in the list.
	if cells := pageCells(s.Root, "p1"); len(cells) != 0 || len(Talks(s.Root)) != 0 {
		t.Errorf("a pass listed as something else: %+v", cells)
	}
}

// An outline: the cell that planned it is served with its blocks, and each
// block as a cell of its own, waiting its turn until its agent's on it.
// What's said of a block is said in the outline, of that block, and the
// page says once that it's put the blocks in.
func TestPageOutlineAPI(t *testing.T) {
	s, _ := docServer(t)
	var asked []string
	idsAnsweredInto(s, &asked)
	var d PageDetail
	pageWrite(t, s, "POST", "/api/pages", map[string]any{"title": "Token cache"}, &d)
	var plan PageCell
	if res := pageWrite(t, s, "POST", "/api/pages/p1/cells", map[string]any{"prompt": "add sections on dependencies and roll out", "goal": "plan"}, &plan); res.StatusCode != 200 || plan.Goal != conversation.GoalPlan {
		t.Fatalf("asked of the page: %d %+v", res.StatusCode, plan)
	}
	pageWrite(t, s, "POST", "/api/pages/p1/cells/w1/fill", nil, nil)
	store := conversation.ForWorkspace(s.Root)
	_, blocks, _ := conversation.ParseOutline("<!-- outline -->\n=== block\nafter: Constraints\nheading: Dependencies\nsummary: What it needs.\n=== block\nafter: end\nheading: Roll out\nsummary: How it goes out.\n")
	store.Say("w1", conversation.Turn{By: conversation.Agent, At: time.Now(), Body: "Two sections.", Blocks: blocks, Answer: true})

	cells := func() map[string]PageCell {
		var got struct {
			Cell []PageCell `json:"cell"`
		}
		json.NewDecoder(do(t, s, "GET", "/api/pages/p1/cells", true, nil).Body).Decode(&got)
		out := map[string]PageCell{}
		for _, c := range got.Cell {
			out[c.ID] = c
		}
		return out
	}
	got := cells()
	if len(got) != 3 || len(got["w1"].Blocks) != 2 || got["w1"].Placed != nil {
		t.Fatalf("planned: %+v", got)
	}
	if b := got["w1-b1"]; b.Of != "w1" || b.Kind != conversation.Cell || len(b.Turns) != 1 || b.Turns[0].Body != "What it needs." || !b.Queued {
		t.Errorf("its first block: %+v", b)
	}

	// The page puts them in, and says so.
	if res := pageWrite(t, s, "POST", "/api/pages/p1/cells/w1/placed", nil, nil); res.StatusCode != 200 {
		t.Fatalf("placed: %d", res.StatusCode)
	}
	if cells()["w1"].Placed == nil {
		t.Error("not marked as placed")
	}
	if res := pageWrite(t, s, "POST", "/api/pages/p1/cells/w1-b9/say", map[string]any{"body": "x"}, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("a block it didn't plan: %d", res.StatusCode)
	}

	// Its agent's on the first: that one's being written, the other waits.
	store.StartAnswering("w1", os.Getpid())
	got = cells()
	if !got["w1-b1"].Answering || got["w1-b1"].Queued || got["w1-b2"].Answering || !got["w1-b2"].Queued || got["w1-b1"].Run == nil {
		t.Errorf("while the first is written: b1 %+v, b2 %+v", got["w1-b1"], got["w1-b2"])
	}
	store.EndAnswering("w1")
	store.Say("w1", conversation.Turn{By: conversation.Agent, At: time.Now(), Body: "It needs the token service.", Block: "b1"})

	// Said of a block, in the outline; its agent's asked to answer.
	before := len(asked)
	var b1 PageCell
	if res := pageWrite(t, s, "POST", "/api/pages/p1/cells/w1-b1/say", map[string]any{"body": "name the version", "edited": "It needs the token service (mine)."}, &b1); res.StatusCode != 200 || b1.ID != "w1-b1" || len(b1.Turns) != 3 || b1.Turns[2].Edited == "" {
		t.Fatalf("said of a block: %d %+v", res.StatusCode, b1)
	}
	if len(asked) != before+1 || asked[len(asked)-1] != "w1" {
		t.Errorf("who's asked to answer: %v", asked)
	}
	if c, _ := store.Load("w1"); c.Last().Block != "b1" || !c.YourTurn() {
		t.Errorf("the outline's latest turn: %+v", c.Last())
	}

	// In the page's text, its blocks are its cells.
	text := d.Text + "\n## Dependencies\n\n<!-- ws:cell w1-b1 v1 | What it needs. -->\n\nIt needs the token service.\n\n<!-- /ws:cell -->\n\n## Roll out\n\n<!-- ws:cell w1-b2 v0 | How it goes out. -->\n\n<!-- /ws:cell -->\n"
	pageWrite(t, s, "PUT", "/api/pages/p1", map[string]any{"text": text, "base": d.Version}, nil)
	if cs := pages.Cells([]byte(text)); len(cs) != 2 || cs[0].ID != "w1-b1" || cs[1].ID != "w1-b2" {
		t.Errorf("the page's cells: %+v", cs)
	}
	if list := Pages(s.Root); list[0].Cells != 2 {
		t.Errorf("listed: %+v", list[0])
	}
}

// A page started as one of the workspace's kinds of document says so; one
// it hasn't is refused. Filing goes to what files it, and says what it did.
func TestPagesAsKinds(t *testing.T) {
	s, _ := docServer(t)
	setOps(s, testOps(s, ops.Options{}, func(_ *config.Config, ws *workspace.Workspace) {
		ws.Kinds = []*workspace.Kind{{Name: "bp", Title: "Build proposal"}}
	}))
	var asked ops.PageFile
	fake(s).filePage = func(id string, r ops.PageFile) (*ops.PageFiling, error) {
		asked = r
		if id != "p1" {
			return nil, pages.ErrNotFound
		}
		return &ops.PageFiling{Kind: r.Kind, Ref: "records:bp/2026-11-11-x.md", How: "land", Message: "Filed: landed on main"}, nil
	}

	var d PageDetail
	if res := pageWrite(t, s, "POST", "/api/pages", map[string]any{"title": "Portfolio", "kind": "bp"}, &d); res.StatusCode != 200 || d.Kind != "bp" || d.Text != "# Portfolio\n" {
		t.Fatalf("new as a bp: %d %+v", res.StatusCode, d)
	}
	if list := Pages(s.Root); len(list) != 1 || list[0].Kind != "bp" {
		t.Errorf("listed: %+v", list)
	}
	if res := pageWrite(t, s, "POST", "/api/pages", map[string]any{"title": "x", "kind": "adr"}, nil); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("a kind it hasn't: %d", res.StatusCode)
	}

	var f PageFiling
	if res := pageWrite(t, s, "POST", "/api/pages/p1/file", map[string]any{"kind": "bp", "title": "Portfolio", "dry": true}, &f); res.StatusCode != 200 || f.Ref != "records:bp/2026-11-11-x.md" || !asked.Dry || asked.Title != "Portfolio" {
		t.Errorf("file: %d %+v, asked %+v", res.StatusCode, f, asked)
	}
	if res := pageWrite(t, s, "POST", "/api/pages/p9/file", map[string]any{"kind": "bp"}, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("a page that isn't there: %d", res.StatusCode)
	}
}

// A kind's passes are tabs on its pages and no others; the built-in ones
// are on every page; one that can't be run says why, and isn't.
func TestPagePassesOfAKind(t *testing.T) {
	s, _ := docServer(t)
	skills := filepath.Join(s.Root, "skills")
	os.MkdirAll(filepath.Join(skills, "build-proposals"), 0o755)
	os.WriteFile(filepath.Join(skills, "build-proposals", "SKILL.md"), []byte("---\nname: build-proposals\n---\n"), 0o644)
	setOps(s, testOps(s, ops.Options{}, func(c *config.Config, ws *workspace.Workspace) {
		c.Pages.Passes = map[string]config.PassConfig{
			"share": {Skill: "build-proposals"},
			"lint":  {Skill: "vale"},
			"tone":  {Looks: "Find what reads harshly."},
		}
		ws.AgentsSkills = skills
		ws.Kinds = []*workspace.Kind{{Name: "bp", Title: "Build proposal", Passes: []string{"share", "lint"}}}
	}))
	var d PageDetail
	pageWrite(t, s, "POST", "/api/pages", map[string]any{"title": "A BP", "kind": "bp"}, &d)
	pageWrite(t, s, "POST", "/api/pages", map[string]any{"title": "Notes"}, nil)
	names := func(id string) string {
		var out []string
		for _, p := range s.pagePasses(id, nil) {
			out = append(out, p.Name)
		}
		return strings.Join(out, " ")
	}

	if got := names("p1"); got != "check formal cold tone share lint" {
		t.Errorf("a BP's passes: %s", got)
	}
	if got := names("p2"); got != "check formal cold tone" {
		t.Errorf("a plain page's passes: %s", got)
	}
	if res := pageWrite(t, s, "POST", "/api/pages/p2/passes/share/run", map[string]any{}, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("a kind's pass run on another page: %d", res.StatusCode)
	}
	if res := pageWrite(t, s, "POST", "/api/pages/p1/passes/lint/run", map[string]any{}, nil); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("a pass that can't be run: %d", res.StatusCode)
	}
}

// A file opens as the page it's edited in (or its mission's document); a
// save refused because the file's moved on is a 409, to bring it in.
func TestPagesTiedToFiles(t *testing.T) {
	s, _ := docServer(t)
	fake(s).openFile = func(ref string) (string, *ops.DocMission, error) {
		if ref == "records:m.md" {
			return "", &ops.DocMission{Slug: "m", Ref: "records:m.md"}, nil
		}
		return "p4", nil, nil
	}
	fake(s).savePage = func(id, message string) (*ops.DocSaved, error) {
		if message == "late" {
			return nil, fmt.Errorf("%w: docs/plan.md has changed", ErrChangedSince)
		}
		return &ops.DocSaved{Message: "Saved: landed on main"}, nil
	}
	fake(s).bringIn = func(id string) (*ops.PageBroughtIn, error) {
		return &ops.PageBroughtIn{Changed: true, Clashes: 1}, nil
	}

	var opened struct {
		ID      string      `json:"id"`
		Mission *DocMission `json:"mission"`
	}
	if res := pageWrite(t, s, "POST", "/api/pages/open", map[string]any{"ref": "records:docs/plan.md"}, &opened); res.StatusCode != 200 || opened.ID != "p4" {
		t.Errorf("open: %d %+v", res.StatusCode, opened)
	}
	if pageWrite(t, s, "POST", "/api/pages/open", map[string]any{"ref": "records:m.md"}, &opened); opened.Mission == nil || opened.Mission.Slug != "m" {
		t.Errorf("a mission's: %+v", opened)
	}
	var saved DocSaved
	if res := pageWrite(t, s, "POST", "/api/pages/p4/save", map[string]any{}, &saved); res.StatusCode != 200 || saved.Message != "Saved: landed on main" {
		t.Errorf("save: %d %+v", res.StatusCode, saved)
	}
	if res := pageWrite(t, s, "POST", "/api/pages/p4/save", map[string]any{"message": "late"}, nil); res.StatusCode != http.StatusConflict {
		t.Errorf("a file that's moved on: %d", res.StatusCode)
	}
	var in PageBroughtIn
	if res := pageWrite(t, s, "POST", "/api/pages/p4/bring-in", map[string]any{}, &in); res.StatusCode != 200 || in.Clashes != 1 {
		t.Errorf("bring in: %d %+v", res.StatusCode, in)
	}
}
