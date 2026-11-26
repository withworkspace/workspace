package automation

import (
	"fmt"
	"strings"

	"github.com/withworkspace/workspace/internal/events"
)

// The guide is what a conversation's agent is given to write one with:
// ws automation kinds prints it, and ws automation add --propose with no
// file. It's the surface a model writes against, so it says every kind
// of event there is, what each says, and every builtin.

// kindAbout is what each kind of event is, and the fields it says beyond
// those every one does (by, at, ref, title, text).
var kindAbout = map[string][2]string{
	events.CapsuleLifted:        {"a capsule lifted, by this ws", "repo, capsule, mission, branch"},
	events.CapsuleLanded:        {"a capsule's commits landed on its repo's default branch", "repo, capsule, mission"},
	events.CapsuleBurned:        {"a capsule burned", "repo, capsule, mission"},
	events.CommitsLanded:        {"commits landed on a repo's default branch, by anyone (heard)", "repo, branch, commits (each sha, subject, by), paths"},
	events.PROpened:             {"a pull request opened", "repo, pr (api#212), title, branch, mission"},
	events.PRReviewRequested:    {"your review asked for on a pull request (heard)", "repo, pr, title"},
	events.PRApproved:           {"a pull request approved (heard)", "repo, pr, title"},
	events.PRChangesRequested:   {"changes asked for on a pull request (heard)", "repo, pr, title"},
	events.PRChecksFailed:       {"a pull request's checks went red", "repo, pr, title, mission"},
	events.PRChecksPassed:       {"a pull request's checks went green (heard)", "repo, pr, title"},
	events.PRMerged:             {"a pull request merged", "repo, pr, title, mission"},
	events.PRClosed:             {"a pull request closed without merging (heard)", "repo, pr, title"},
	events.MissionStarted:       {"a mission started", "mission"},
	events.MissionFlagged:       {"a mission's crew flagged something for you", "mission"},
	events.MissionWaiting:       {"a mission waits on something outside it", "mission"},
	events.MissionEnded:         {"a mission ended", "mission"},
	events.CrewStarted:          {"crew started on a mission", "mission"},
	events.CrewAsked:            {"crew asked you something", "mission"},
	events.CrewDone:             {"crew handed their work over", "mission"},
	events.BundleHanded:         {"a verification bundle handed over, for you", "mission, repo, capsule"},
	events.ChecksJudged:         {"a step's checks run and judged", "mission"},
	events.ConversationAnswered: {"a conversation's agent said something", "conversation, mission"},
	events.ProposalMade:         {"a proposal made in a conversation", "conversation, proposal, kind (its kind: a plan, jira:issues…)"},
	events.ProposalAccepted:     {"you accepted a proposal", "conversation, proposal, kind"},
	events.ProposalDeclined:     {"you declined a proposal", "conversation, proposal, kind"},
	events.ArtifactAdded:        {"an artifact added", "artifact"},
	events.ArtifactPromoted:     {"an artifact promoted", "artifact"},
	events.DocAdded:             {"a document added, by this ws", "doc (records:docs/x.md)"},
	events.DocAmended:           {"a document amended, by this ws", "doc"},
	events.DocChanged:           {"a space's document added, changed or deleted on its default branch, by anyone (heard)", "repo, doc, path, status (added, changed, deleted)"},
	events.ThingAdded:           {"a thing added: a folder with a thing.toml in a space (a sketch), or a plugin's (a Jira issue) (heard)", "repo, thing, kind (sketch, jira:issue)"},
	events.ThingChanged:         {"a thing changed (heard)", "thing, kind, changes (each field, from, to); a field by its name matches what it changed to (status = \"Done\")"},
	events.RunStarted:           {"a run started (a recipe, a task)", "repo, run (its name, or deploy.ground.api), mission"},
	events.RunUp:                {"a service run answering on its port", "repo, run"},
	events.RunEnded:             {"a run ended", "repo, run, status (passed, failed (exit 1), stopped, gone)"},
	events.DecisionMade:         {"a decision made on a mission", "mission"},
}

// Guide is how an automation's written and proposed, for a conversation's
// agent.
func Guide() string {
	var b strings.Builder
	b.WriteString(`An automation is what ws does when something happens: a small Starlark file, run in the daemon on the workspace's events (ws events lists them). You propose it; ws checks it and dry-runs it over the last 30 days, and the user reads what it would have done before they turn it on.

  # Notify me if Brendan pushes a new drawing.      <- the user's words, as a comment: the card shows them
  when = event("thing.added", kind = "sketch", by = "brendan")
  may = ["notify"]

  def run(e):
      notify("%s drew %s" % (e.by.short, e.title), open = e.thing)

- when: which events it's for. event(kind, field = value, ...), each value a string, which may be a pattern (status = "failed*", path = "docs/*"). by is a person's name (brendan matches Brendan Ash) or "me". A list of event()s is any of them. ws matches it without running the script, so say as much of what it's for here as you can.
- Pace, in event(): settle = "10m" runs once its events have been quiet ten minutes, with all of them as e.batch; at_most = "1/h" runs it at most once an hour (what's past it is held); per = "pr" keeps them apart by a field (each pull request, each thing).
- may: what it may do. "notify" (a notification in the user's feed) and "converse" (start a conversation of the workspace's, for an agent to look into something and answer the user). Nothing else reaches outside a script.
- def run(e): called with each event it's for. e has the event's fields (e.kind, e.by.who, e.by.short, e.title, e.text, e.pr, e.thing, e.status, e.changes, e.commits, e.paths, e.ref, e.refs, e.batch). Without run, the event's notified as ws events says it.
- Builtins in run: notify(text, open = ref, refs = [...]); converse(text, about = [...]), for judgement (why did CI fail?) rather than deciding in the script; show(ref), what ws show says of it (show(e.pr).head, .body, .links); events(kind, since = "7d", field = value, ...), what's happened, for counting; refs_in(text), the refs a text names (each .ref, .kind); log(text), a line in its run's record; wait(event(...), timeout = "2h"), which parks the run till an event it's for comes after the one it's running on (it's then that event) or its time's up (None), at most "7d"; test it for None to say what happens either way.
- Loops: what it causes carries its chain (a conversation it starts, and that conversation's events). It isn't run on an event its chain already includes, or one 3 automations deep: say as much in when (by, conversation) where it's for what people do, not what automations do.
- Starlark is Python-like, with no while, no recursion, no imports, no files, network or clock. Keep it short: what can be said in when belongs there.

The kinds of event, and what each says beyond by, at, ref, title and text:
`)
	for _, k := range events.Kinds {
		about, ok := kindAbout[k]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "- %s: %s. %s\n", k, about[0], about[1])
	}
	b.WriteString(`
Propose it, as the user asked it (its name is a few lowercase words with hyphens; the first comment is their words):

  ws automation add --propose --name <name> - <<'EOF'
  # <what the user asked, in their words>
  when = event(...)
  ...
  EOF

ws checks it: what's wrong is said, and nothing's proposed till it's right. The card shows the user its dry run. To change one, propose it again with --revises <n>.
`)
	return b.String()
}
