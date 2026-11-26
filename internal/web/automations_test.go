package web

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/automation"
	"github.com/withworkspace/workspace/internal/events"
)

const drawingScript = `# Notify me if Brendan pushes a new drawing.
when = event("thing.added", kind = "sketch", by = "brendan")
may = ["notify"]

def run(e):
    notify("%s drew %s" % (e.by.short, e.title), open = e.thing)
`

// The Automations page lists yours, and one's page has its runs, each with
// what it did; it's turned off and on, dry-run again, and removed. Its
// followers are told when a run's kept, without asking again.
func TestAutomations_Page(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	e := &automation.Engine{Root: s.Root, Hooks: automation.Hooks{Notify: func(*automation.Automation, string, []string) error { return nil }}}
	if _, err := e.Add("drawing", []byte(drawingScript)); err != nil {
		t.Fatal(err)
	}

	conn, _, err := dialSocket(t, s, s.Info().Origin())
	if err != nil {
		t.Fatal(err)
	}
	st := talkTo(t, conn)
	st.ask(SocketAsk{Op: SocketFollow, Topic: "automations"})
	if got := st.next(3 * time.Second); got != "following automations" {
		t.Fatalf("following: %q", got)
	}
	if _, err := events.Emit(s.Root, events.Event{Kind: events.ThingAdded, Origin: events.Heard, By: events.ByPerson("brendan-ash"), Repo: "records", Thing: "records/sketches/queues/", ThingKind: "sketch", Title: "Queues"}); err != nil {
		t.Fatal(err)
	}
	e.RunDue()
	if got := st.next(3 * time.Second); got != "changed automations" {
		t.Fatalf("a run kept: %q", got)
	}

	var info AutomationsInfo
	asks(t, s, "GET", "/api/automations", nil, &info)
	if len(info.Automations) != 1 {
		t.Fatalf("listed %+v", info.Automations)
	}
	a := info.Automations[0]
	if a.Name != "drawing" || !a.On || a.About != "Notify me if Brendan pushes a new drawing." || a.Tally.Runs != 1 || a.Last == nil || a.Last.State != automation.Ran {
		t.Errorf("listed %+v (last %+v)", a, a.Last)
	}

	var page AutomationPage
	asks(t, s, "GET", "/api/automations/drawing", nil, &page)
	if page.Source != drawingScript || len(page.Runs) != 1 {
		t.Fatalf("its page: %+v", page)
	}
	r := page.Runs[0]
	if r.Event != "e1" || r.Says == "" || len(r.Did) != 1 || r.Did[0].What != "notify" || r.Did[0].Text != "Brendan drew Queues" {
		t.Errorf("its run: %+v", r)
	}

	if page.Outline == nil || len(page.Outline.Steps) != 1 || page.Outline.Steps[0].Kind != "notify" ||
		len(page.Steps) != 1 || page.Steps[0].At != page.Outline.Steps[0].At || page.Steps[0].Done != 1 {
		t.Errorf("its outline %+v, counted %+v", page.Outline, page.Steps)
	}

	if res := asks(t, s, "POST", "/api/automations/drawing/on", AutomationOn{On: false}, nil); res.StatusCode != http.StatusOK {
		t.Errorf("turning it off: %d", res.StatusCode)
	}
	if states, _ := automation.States(s.Root); states["drawing"].On {
		t.Error("it's on still")
	}

	var dry automation.DryRun
	asks(t, s, "POST", "/api/automations/drawing/dry-run", nil, &dry)
	if dry.Matched != 1 || dry.Ran != 1 || len(dry.Lines) != 1 || dry.Lines[0].Did[0] != "notify: Brendan drew Queues" {
		t.Errorf("dry run: %+v", dry)
	}

	if res := asks(t, s, "GET", "/api/automations/nope", nil, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("one there isn't: %d", res.StatusCode)
	}
	if res := asks(t, s, "DELETE", "/api/automations/drawing", nil, nil); res.StatusCode != http.StatusOK {
		t.Errorf("removing it: %d", res.StatusCode)
	}
	asks(t, s, "GET", "/api/automations", nil, &info)
	if len(info.Automations) != 0 {
		t.Errorf("removed, still listed: %+v", info.Automations)
	}
}

// An automation's output is served as written, a version at a time; an
// HTML one isn't sent, it's served as a frame's page under the frame's
// policy.
func TestAutomations_Output(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	e := &automation.Engine{Root: s.Root}
	for name, out := range map[string]string{"md": `output("# Hello\n\n" + chart([("2026-11-26T03:00:00Z", 50)]))`, "page": `output(html = "<h1>Hi</h1>")`} {
		src := "when = every(\"1h\")\nmay = [\"output\"]\n\ndef run(e):\n    " + out + "\n"
		if _, err := e.Add(name, []byte(src)); err != nil {
			t.Fatal(err)
		}
		if _, err := automation.RunNow(s.Root, name); err != nil {
			t.Fatal(err)
		}
	}
	e.RunDue()

	var md AutomationOutputInfo
	asks(t, s, "GET", "/api/automations/md/output", nil, &md)
	if md.Output == nil || md.Output.Format != automation.OutputMarkdown || !strings.HasPrefix(md.Output.Body, "# Hello\n\n```chart\n") || len(md.Versions) != 1 {
		t.Errorf("markdown: %+v", md)
	}
	var page AutomationOutputInfo
	asks(t, s, "GET", "/api/automations/page/output", nil, &page)
	if page.Output == nil || page.Output.Format != automation.OutputHTML || page.Output.Body != "" {
		t.Fatalf("html: %+v", page)
	}
	res := do(t, s, "GET", "/frame/automations/page/output/1", true, nil)
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), "<h1>Hi</h1>") || res.Header.Get("Content-Security-Policy") == "" {
		t.Errorf("frame %d %q %q", res.StatusCode, body, res.Header.Get("Content-Security-Policy"))
	}
	if res := do(t, s, "GET", "/frame/automations/md/output/1", true, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("a markdown one's frame: %d", res.StatusCode)
	}

	var list AutomationsInfo
	asks(t, s, "GET", "/api/automations", nil, &list)
	for _, a := range list.Automations {
		if a.Output != 1 || !a.Scheduled {
			t.Errorf("listed %s: output %d, scheduled %v", a.Name, a.Output, a.Scheduled)
		}
	}
}
