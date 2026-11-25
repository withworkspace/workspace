package ops

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/outbox"
	"github.com/withworkspace/workspace/internal/pulls"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/team"
)

// What a crew's page asks of the crew the daemon holds: where its session
// is kept, saying something to it, answering what it asks, the menu on its
// screen, and its terminal. The host holds them (crew.Runs, the daemon's
// server); from a command, only saying something reaches them, over the
// daemon's socket (crew.Host.Say).
type CrewPages interface {
	Session(slug, id string) (CrewSession, error)
	Say(slug, id, text string) (string, error)
	Answer(slug, id string, a CrewAnswer) (string, error)
	Prompt(slug, id string) (PromptView, bool)
	Screen(slug, id string) (Screen, bool)
	Watch(slug, id string) (changes, ended <-chan struct{}, cancel func(), ok bool)
	Key(slug, id string, k TermKey) error
	Paste(slug, id, text string) error
	// SwitchModel puts running crew on another model, their session going
	// on (crew.Run's SwitchModel), and records it as theirs.
	SwitchModel(slug, id, model string) (string, error)
}

// CrewSession is where a crew's harness keeps its session: Path, or Why
// there's none to read. Final is that there never will be.
type CrewSession struct {
	Harness, Path, Why string
	Final              bool
}

// CrewAnswer answers what crew ask: crew.Answer.
type CrewAnswer = crew.Answer

// PromptView is what running crew are waiting on you for: crew.Prompt.
type PromptView = crew.Prompt

// Screen is a crew's screen: a line per row, with its colours as ANSI
// escapes, the cursor, and the terminal's size.
type Screen struct {
	Text                            string
	Width, Height, CursorX, CursorY int
	CursorVisible                   bool
	Title                           string
}

// TermKey is a key a page types into crew's terminal, as a browser names
// it (KeyboardEvent.key), with its modifiers.
type TermKey struct {
	Key              string
	Ctrl, Alt, Shift bool
}

// ErrNotOnScreen is an answer to what crew aren't asking: crew.ErrNotOnScreen.
var ErrNotOnScreen = crew.ErrNotOnScreen

// errNotHere is crew asked for where the host holds none in this process.
var errNotHere = errors.New("crew can't be reached from here")

// NoCrew is crew's pages where no crew are held: each says so. A test's
// fake starts from it.
type NoCrew struct{}

func (NoCrew) Session(slug, id string) (CrewSession, error) {
	return CrewSession{}, fmt.Errorf("%w: %v", ErrNotFound, errNotHere)
}
func (NoCrew) Say(slug, id, text string) (string, error)            { return "", errNotHere }
func (NoCrew) Answer(slug, id string, a CrewAnswer) (string, error) { return "", errNotHere }
func (NoCrew) Prompt(slug, id string) (PromptView, bool)            { return PromptView{}, false }
func (NoCrew) Screen(slug, id string) (Screen, bool)                { return Screen{}, false }
func (NoCrew) Key(slug, id string, k TermKey) error                 { return errNotHere }
func (NoCrew) Paste(slug, id, text string) error                    { return errNotHere }
func (NoCrew) SwitchModel(slug, id, model string) (string, error)   { return "", errNotHere }
func (NoCrew) Watch(slug, id string) (<-chan struct{}, <-chan struct{}, func(), bool) {
	return nil, nil, nil, false
}

// Crew is the crew the daemon holds, as their pages reach them.
func (w *Workspace) Crew() CrewPages { return crewPages{w} }

type crewPages struct{ w *Workspace }

// runs are the crew the host holds in this process: the daemon's.
func (w *Workspace) runs() (crew.Runs, bool) {
	r, ok := w.opts.Host.(crew.Runs)
	return r, ok
}

// crewOf is a mission's crew, by id, and its key on the host.
func (w *Workspace) crewOf(slug, id string) (mission.Crew, string, error) {
	m, err := w.load(slug)
	if err != nil {
		return mission.Crew{}, "", err
	}
	c, ok := m.CrewByID(id)
	if !ok {
		return mission.Crew{}, "", fmt.Errorf("%w: mission %s has no crew %s", ErrNotFound, slug, id)
	}
	return c, slug + "/" + id, nil
}

// Session is where a crew's harness keeps its session: Claude Code's and
// pi's session files, ws's own record of an ACP crew's; Cursor's chats are
// in a database ws can't read.
func (p crewPages) Session(slug, id string) (CrewSession, error) {
	c, _, err := p.w.crewOf(slug, id)
	if err != nil {
		return CrewSession{}, err
	}
	sess := CrewSession{Harness: c.Harness}
	via := harness.CapsOf(c.Harness).Transcript
	if via == harness.TranscriptACP {
		// ws's own record of its session, kept beside the mission.
		sess.Path = p.w.Missions.ACPRecordPath(slug, id)
		return sess, nil
	}
	path, found := harness.SessionFile(c.Harness, c.Conversation)
	switch {
	case via == harness.TranscriptNone:
		sess.Final = true
		sess.Why = c.Harness + " keeps no transcript ws can read, so there's none: its terminal is what there is."
		if c.Harness == "cursor" {
			sess.Why = "Cursor keeps its chats in a database ws can't read, so there's no transcript: its terminal is what there is."
		}
	case c.Conversation == "":
		sess.Why = id + " recorded no session, so there's no transcript to read."
	case !found:
		sess.Why = id + "'s session file isn't where " + c.Harness + " keeps them yet (it may still be starting, or been cleared)."
	default:
		sess.Path = path
	}
	return sess, nil
}

func (p crewPages) Say(slug, id, text string) (string, error) { return p.w.Say(slug, id, text, AsYou) }

// Answer answers what running crew ask, as its run does: keys to the menu
// on a terminal's screen, or its agent's request.
func (p crewPages) Answer(slug, id string, a CrewAnswer) (string, error) {
	c, key, err := p.w.crewOf(slug, id)
	if err != nil {
		return "", err
	}
	runs, ok := p.w.runs()
	if !ok {
		return "", errNotHere
	}
	r, ok := runs.Run(key)
	if !ok {
		return "", fmt.Errorf("%s isn't running", id)
	}
	return r.Answer(c, a)
}

func (p crewPages) Prompt(slug, id string) (PromptView, bool) {
	runs, ok := p.w.runs()
	if !ok {
		return PromptView{}, false
	}
	r, ok := runs.Run(slug + "/" + id)
	if !ok {
		return PromptView{}, false
	}
	return r.Asking()
}

func (p crewPages) Screen(slug, id string) (Screen, bool) {
	runs, ok := p.w.runs()
	if !ok {
		return Screen{}, false
	}
	f, ok := runs.Screen(slug + "/" + id)
	if !ok {
		return Screen{}, false
	}
	return Screen{Text: f.Screen, Width: f.Width, Height: f.Height, CursorX: f.CursorX, CursorY: f.CursorY, CursorVisible: f.CursorVisible, Title: f.Title}, true
}

func (p crewPages) Watch(slug, id string) (<-chan struct{}, <-chan struct{}, func(), bool) {
	runs, ok := p.w.runs()
	if !ok {
		return nil, nil, nil, false
	}
	return runs.WatchScreen(slug + "/" + id)
}

// Key types a key into crew's terminal. Enter sends it a message: from
// then, the end of its turn is it answering you, as when you type in its
// terminal.
func (p crewPages) Key(slug, id string, k TermKey) error {
	runs, ok := p.w.runs()
	if !ok {
		return errNotHere
	}
	ev, ok := browserKey(k)
	if !ok {
		return errors.New("no such key: " + strconv.Quote(k.Key))
	}
	if err := runs.SendKey(slug+"/"+id, ev); err != nil {
		return err
	}
	if ev.Code == uv.KeyEnter && ev.Mod == 0 {
		p.w.Missions.TalkedTo(slug, id, "")
	}
	return nil
}

func (p crewPages) Paste(slug, id, text string) error {
	runs, ok := p.w.runs()
	if !ok {
		return errNotHere
	}
	return runs.Paste(slug+"/"+id, text)
}

func (p crewPages) SwitchModel(slug, id, model string) (string, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return "", errors.New("say which model")
	}
	_, key, err := p.w.crewOf(slug, id)
	if err != nil {
		return "", err
	}
	runs, ok := p.w.runs()
	if !ok {
		return "", errNotHere
	}
	r, ok := runs.Run(key)
	if !ok {
		return "", fmt.Errorf("%s isn't running", id)
	}
	said, err := r.SwitchModel(model)
	if err != nil {
		return "", err
	}
	return said, p.w.Missions.SetCrewModel(slug, id, model)
}

// Say gives a mission's crew a message: typed into, or sent to, crew the
// daemon's running; an ended crew is resumed with it. From you, the end of
// its turn is then yours (TalkedTo), unless it answers a question they
// asked (AnsweredCrew); from ws or the orchestrator, it's logged as typed
// in, and the orchestrator's (Nudged). It says what it did.
func (w *Workspace) Say(slug, id, text string, by Actor) (string, error) {
	c, key, err := w.crewOf(slug, id)
	if err != nil {
		return "", err
	}
	if w.opts.Host == nil {
		return "", errNotHere
	}
	said, err := w.opts.Host.Say(key, text)
	switch {
	case err == nil:
		if by.IsYou() {
			w.answeredCrew(slug, c, text)
			// The answer to a question of theirs isn't you taking them
			// over: they carry on, watched as they were.
			if answered, _ := w.Missions.AnsweredCrew(slug, id, text); !answered {
				w.Missions.TalkedTo(slug, id, text)
			}
		} else if err := w.Missions.Nudged(slug, id, text, by.By()); err != nil {
			return said, err
		}
		return said, nil
	case !errors.Is(err, crew.ErrNotRunning):
		return "", err
	case c.Ended == nil:
		return "", fmt.Errorf("%s isn't running in this daemon: talk to it in the terminal it's in", id)
	case c.Conversation == "":
		return "", fmt.Errorf("%s recorded no session, so it can't be resumed", id)
	}
	// Ended: resumed, with what was said as its first message.
	if err := w.Missions.SetResumeWithBy(slug, id, text, by.By()); err != nil {
		return "", err
	}
	l, err := w.Launch(slug, Dispatch{Resume: id}, by)
	if err != nil {
		w.Missions.SetResumeWith(slug, id, "")
		return "", err
	}
	if by.IsYou() {
		w.answeredCrew(slug, c, text)
	}
	return l.String(), nil
}

// answeredCrew keeps what you tell crew who'd asked you something (ws
// mission ask) as a decision of yours, with their question as its why, as
// your answer to the orchestrator's question is kept. Crew record what
// they take from it themselves, but that's their word for yours: ws
// knowing you said it is what lets it stand over a check the plan wrote
// before you did.
func (w *Workspace) answeredCrew(slug string, c mission.Crew, text string) {
	text = strings.TrimSpace(text)
	if c.Status != mission.CrewAsking || strings.TrimSpace(c.Note) == "" || text == "" {
		return
	}
	w.Missions.Decide(slug, mission.Decision{What: text, By: AsYou.By(), Asked: &mission.Asked{By: c.ID, Text: oneLine(c.Note)}})
}

// browserKeys are the keys a browser names (KeyboardEvent.key) that aren't
// characters.
var browserKeys = map[string]rune{
	"Enter": uv.KeyEnter, "Tab": uv.KeyTab, "Backspace": uv.KeyBackspace, "Escape": uv.KeyEscape,
	"ArrowUp": uv.KeyUp, "ArrowDown": uv.KeyDown, "ArrowLeft": uv.KeyLeft, "ArrowRight": uv.KeyRight,
	"Home": uv.KeyHome, "End": uv.KeyEnd, "PageUp": uv.KeyPgUp, "PageDown": uv.KeyPgDown,
	"Delete": uv.KeyDelete, "Insert": uv.KeyInsert,
}

func init() {
	for i := range 12 {
		browserKeys["F"+strconv.Itoa(i+1)] = uv.KeyF1 + rune(i)
	}
}

// browserKey is a key the browser named, as a key press for crew's
// terminal, which encodes it for the modes the crew has on.
func browserKey(k TermKey) (uv.KeyPressEvent, bool) {
	var mod uv.KeyMod
	if k.Shift {
		mod |= uv.ModShift
	}
	if k.Alt {
		mod |= uv.ModAlt
	}
	if k.Ctrl {
		mod |= uv.ModCtrl
	}
	if code, ok := browserKeys[k.Key]; ok {
		return uv.KeyPressEvent{Code: code, Mod: mod}, true
	}
	r := []rune(k.Key)
	if len(r) != 1 || !unicode.IsPrint(r[0]) {
		return uv.KeyPressEvent{}, false
	}
	ev := uv.KeyPressEvent{Code: unicode.ToLower(r[0]), Mod: mod}
	if r[0] == ' ' {
		ev.Code = uv.KeySpace
	}
	if unicode.ToLower(r[0]) != r[0] {
		ev.ShiftedCode = r[0]
	}
	// A character typed (shift included) is text; with ctrl or alt it's
	// a combination, encoded from its code.
	if !k.Ctrl && !k.Alt {
		ev.Text = string(r)
	}
	return ev, true
}

// ProposalEdits are what you changed in a proposal before accepting it.
type ProposalEdits struct {
	What, Why, Title, Brief, Text, Plan, Note string
	// Items are a plugin's proposal's items accepted, by number: none,
	// every one still open.
	Items []int
}

// PRQuestion is a question about a pull request: what you ask, and the
// lines it's about, if it's about some (at Commit).
type PRQuestion struct {
	Body, Path, Side string
	Line, EndLine    int
	Commit           string
}

// SendToCrew hands threads on a mission's document to its crew to change
// it (none named: every Change thread open on ref that crew don't have),
// saying who has them.
func (w *Workspace) SendToCrew(slug, ref string, ids []string) (string, error) {
	m, err := w.load(slug)
	if err != nil {
		return "", err
	}
	if len(ids) == 0 {
		if ids, err = openChangesToSend(conversation.For(w.Root(), slug), ref); err != nil {
			return "", err
		}
	}
	return w.sendToCrew(m, ids)
}

// The Reviews and Team pages' writes: posting your review of a pull
// request to GitHub, a reply in one of its threads or a comment on it
// (sent behind, in the daemon); noting you've seen
// the Team page; and having gists and a period's account written. Each is
// the service's (Options.Reviews, Options.Team: the daemon's, which its
// pages read from too).

var errNoTeam = fmt.Errorf("%w: there's no Team page here", ErrNotFound)

// SubmitPR posts your review of a pull request, at the head it was written
// on.
func (w *Workspace) SubmitPR(repo string, number int, head, verdict, summary string) (*pulls.Submitted, error) {
	svc, err := w.reviews()
	if err != nil {
		return nil, err
	}
	return svc.Submit(repo, number, head, verdict, summary)
}

// RespondPR is your reply in one of a pull request's threads, or
// resolving or reopening it, and is its threads with that. In the daemon
// it's shown at once and sent to GitHub behind (Options.Outbox).
func (w *Workspace) RespondPR(repo string, number int, thread string, what review.Response, body string) (*review.Review, error) {
	svc, err := w.reviews()
	if err != nil {
		return nil, err
	}
	threads, err := svc.Respond(repo, number, thread, what, body)
	if err != nil {
		return nil, err
	}
	// What the follow-up said of a thread is settled with it.
	switch what {
	case review.Resolve:
		w.followedUp(repo, number, thread, true)
	case review.Reopen:
		w.followedUp(repo, number, thread, false)
	}
	return threads, nil
}

// CommentPR is a comment of yours on a pull request itself, and is what's
// been said on it with that: sent as RespondPR's is.
func (w *Workspace) CommentPR(repo string, number int, body string) (*pulls.Discussion, error) {
	svc, err := w.reviews()
	if err != nil {
		return nil, err
	}
	return svc.Comment(repo, number, body)
}

// ResendPR sends something said from a pull request's page that GitHub
// didn't take again; DiscardPR lets go of it.
func (w *Workspace) ResendPR(repo string, number int, id int64) error {
	svc, err := w.reviews()
	if err != nil {
		return err
	}
	return unsent(svc.Resend(repo, number, id))
}

func (w *Workspace) DiscardPR(repo string, number int, id int64) error {
	svc, err := w.reviews()
	if err != nil {
		return err
	}
	return unsent(svc.Discard(repo, number, id))
}

// unsent is err as ops says it: one that's no longer waiting isn't found.
func unsent(err error) error {
	if errors.Is(err, outbox.ErrNotFound) {
		return fmt.Errorf("%w: it's no longer waiting to be sent", ErrNotFound)
	}
	return err
}

// DeclinePR takes your name off a pull request's reviewers on GitHub: it
// isn't yours to review, so it stops needing you.
func (w *Workspace) DeclinePR(repo string, number int) error {
	svc, err := w.reviews()
	if err != nil {
		return err
	}
	return svc.Decline(repo, number)
}

// MarkTeamSeen notes you've looked at the Team page: what's there now
// isn't new to you next time.
func (w *Workspace) MarkTeamSeen() error {
	if w.opts.Team == nil {
		return errNoTeam
	}
	return w.opts.Team.MarkSeen()
}

// Narrate has the gists of a look's changes written (strands, or every one
// since since that hasn't one), and says how many it started.
func (w *Workspace) Narrate(snap *team.Snapshot, strands []string, since time.Time) int {
	if w.opts.Team == nil {
		return 0
	}
	return w.opts.Team.Narrate(snap, strands, since)
}

// Summarise has a period's account written, and says whether it started.
func (w *Workspace) Summarise(snap *team.Snapshot, since time.Time, key string) bool {
	if w.opts.Team == nil {
		return false
	}
	return w.opts.Team.Summarise(snap, since, key)
}

// AsksPRs is whether there's an agent here to ask about a pull request.
func (w *Workspace) AsksPRs() bool {
	_, _, err := Roles(w.Config()).ForConversation()
	return err == nil
}

// AskPR starts an ask about a pull request (docking it first, so the
// agent can read it): the review's ask bar.
func (w *Workspace) AskPR(repo string, number int, q PRQuestion) (*conversation.Conversation, error) {
	svc, err := w.reviews()
	if err != nil {
		return nil, err
	}
	return w.askPR(svc, repo, number, q)
}

// DockPR checks a pull request out here, as ws dock does, and says what it
// did.
func (w *Workspace) DockPR(repo string, number int) (string, error) {
	svc, err := w.reviews()
	if err != nil {
		return "", err
	}
	return w.openHere(svc, repo, number)
}
