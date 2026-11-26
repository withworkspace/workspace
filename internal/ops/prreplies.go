package ops

import (
	"fmt"
	"strings"

	"github.com/withworkspace/workspace/internal/mission"
)

// Replies on a mission's pull requests: crew, resumed to answer what
// someone said there, draft a reply (ws mission reply), and ws posts it, as
// you, through GitHub. A reply to a person waits for you, where ws holds
// replies_need_me (on unless set off): a task of yours, to post it or not.
// A reply to a bot is posted at once.

// Replied is a reply drafted, and what became of it.
type Replied struct {
	Slug  string
	Reply mission.Reply
	// Posted: it's on GitHub; otherwise it waits for the user.
	Posted bool
}

func (r Replied) String() string {
	if r.Posted {
		return fmt.Sprintf("Posted the reply on %s.", r.Reply.Where())
	}
	return fmt.Sprintf("Drafted reply %d on %s: it waits for the user's approval before it's posted, as them.", r.Reply.N, r.Reply.Where())
}

// DraftReply drafts a reply on one of the mission's pull requests (prRef,
// api#12): in a review thread (thread, its id), or on the pull request's
// own conversation, to whoever said what it answers (to: their login, or
// for a thread, who's in it). Crew draft one; so may you. It's posted at
// once to a bot, or where ws doesn't hold replies_need_me; otherwise it
// waits for you.
func (w *Workspace) DraftReply(slug, prRef, thread, to, text string, by Actor) (Replied, error) {
	if by.Kind == Orchestrator || by.Kind == WS {
		return Replied{}, refused("not-yours", "crew draft replies on a pull request (resume them with what to answer, and they run ws mission reply), or the user does")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return Replied{}, invalid("say what the reply says")
	}
	m, err := w.load(slug)
	if err != nil {
		return Replied{}, err
	}
	repo, number, err := w.ParsePRRef(prRef)
	if err != nil {
		return Replied{}, invalid("%v", err)
	}
	ref := fmt.Sprintf("%s#%d", repo, number)
	if !missionPR(m, repo, number) {
		return Replied{}, refused("not-the-missions", "%s isn't a pull request from one of the mission's capsules", ref)
	}
	gh := w.GitHub()
	if gh == nil {
		return Replied{}, refused("no-github", "ws can't reach GitHub here, so it can't reply on %s", ref)
	}
	me, _ := gh.Viewer()
	var people []string
	if thread != "" {
		threads, err := gh.ReviewThreads(w.WS().RemoteOf(repo).Owner, w.WS().RemoteOf(repo).Name, number)
		if err != nil {
			return Replied{}, err
		}
		found := false
		for _, t := range threads {
			if t.ID != thread {
				continue
			}
			found = true
			for _, c := range t.Comments {
				if c.Author != me && !strings.EqualFold(c.Author, to) {
					people = append(people, c.Author)
				}
			}
		}
		if !found {
			return Replied{}, invalid("%s has no review thread %s: the mission's log names each thread something's said in (ws show -f llm)", ref, thread)
		}
	}
	if to = strings.TrimPrefix(strings.TrimSpace(to), "@"); to != "" {
		people = append(people, to)
	}
	if len(people) == 0 && thread == "" {
		return Replied{}, invalid("say who it's to (--to <login>), or the review thread it answers (--thread <id>)")
	}
	bots := len(people) > 0
	for _, p := range people {
		bots = bots && w.isBot(repo, number, p)
	}
	r, err := w.Missions.DraftReply(m.Slug, mission.Reply{PR: ref, Thread: thread, To: strings.Join(dedupe(people), ", "), Text: text, By: by.By()})
	if err != nil {
		return Replied{}, wrap(err)
	}
	out := Replied{Slug: m.Slug, Reply: r}
	if bots || !w.holding(GuaranteeRepliesNeedMe) || by.IsYou() {
		if err := w.postReply(m.Slug, r, by); err != nil {
			return out, err
		}
		out.Posted = true
		return out, nil
	}
	w.Missions.AppendLogAfter(m.Slug, mission.Event{Kind: mission.EventPR, What: mission.PREventReplyDrafted, Repo: repo, Crew: by.By(),
		Text: fmt.Sprintf("drafted a reply on %s, to %s: %s", r.Where(), r.To, clipString(oneLine(r.Text), 200))})
	task := fmt.Sprintf("%s drafted a reply on %s, to %s: “%s” Post it, as you?", by.By(), r.Where(), r.To, clipString(oneLine(r.Text), 600))
	if _, err := w.Missions.OpenTask(m.Slug, mission.Task{Verb: mission.TaskReply, Text: task, Reply: r.N, Options: []string{mission.ReplyPost, mission.ReplyDont}, By: by.By()}); err != nil {
		return out, wrap(err)
	}
	return out, nil
}

// postReply posts a reply drafted on a mission's pull request, as you.
func (w *Workspace) postReply(slug string, r mission.Reply, by Actor) error {
	repo, number, err := w.ParsePRRef(r.PR)
	if err != nil {
		return err
	}
	gh := w.GitHub()
	if gh == nil {
		return refused("no-github", "ws can't reach GitHub here, so it can't post the reply")
	}
	if r.Thread != "" {
		err = gh.ReplyToThread(w.WS().RemoteOf(repo).Owner, w.WS().RemoteOf(repo).Name, number, r.Thread, r.Text)
	} else {
		body := r.Text
		if r.To != "" && !strings.Contains(r.To, ",") && !strings.Contains(body, "@"+r.To) {
			body = "@" + r.To + " " + body
		}
		err = gh.Comment(w.WS().RemoteOf(repo).Owner, w.WS().RemoteOf(repo).Name, number, body)
	}
	if err != nil {
		return fmt.Errorf("posting the reply on %s: %w", r.Where(), err)
	}
	w.Missions.SettleReply(slug, r.N, true)
	w.Missions.AppendLogAfter(slug, mission.Event{Kind: mission.EventPR, What: mission.PREventReplied, Repo: repo, Crew: by.By(),
		Text: fmt.Sprintf("replied on %s, to %s: %s", r.Where(), firstNonEmpty(r.To, "the thread"), clipString(oneLine(r.Text), 200))})
	return nil
}

// answerReply answers a reply task: post it, or don't. Anything else is
// answered as any task is (ok false).
func (w *Workspace) answerReply(m *mission.Mission, t mission.Task, by Actor, text string) (Answered, bool, error) {
	out := Answered{Slug: m.Slug}
	r, ok := m.ReplyByN(t.Reply)
	if !ok {
		return out, false, nil
	}
	switch {
	case strings.EqualFold(text, mission.ReplyPost):
		if err := w.postReply(m.Slug, r, by); err != nil {
			return out, true, err
		}
		out.Next = "posted it on " + r.Where()
	case strings.EqualFold(text, mission.ReplyDont):
		if err := w.Missions.SettleReply(m.Slug, r.N, false); err != nil {
			return out, true, wrap(err)
		}
		repo, _, _ := strings.Cut(r.PR, "#")
		w.Missions.AppendLogAfter(m.Slug, mission.Event{Kind: mission.EventPR, What: mission.PREventReplyDismissed, Repo: repo, Crew: by.By(),
			Text: fmt.Sprintf("the reply drafted on %s isn't posted", r.Where())})
		out.Next = "it isn't posted"
	default:
		return out, false, nil
	}
	w.youActed(m.Slug, by)
	return out, true, nil
}

// isBot reports whether who is a bot, as GitHub names them (dependabot[bot])
// or as the PR's conversation says.
func (w *Workspace) isBot(repo string, number int, who string) bool {
	if strings.HasSuffix(strings.ToLower(who), "[bot]") {
		return true
	}
	if gh := w.GitHub(); gh != nil {
		if said, err := gh.Discussion(w.WS().RemoteOf(repo).Owner, w.WS().RemoteOf(repo).Name, number); err == nil {
			for _, s := range said {
				if strings.EqualFold(s.Author, who) && s.Bot {
					return true
				}
			}
		}
	}
	return false
}

// missionPR reports whether repo#number is a pull request from one of the
// mission's capsules, as last seen.
func missionPR(m *mission.Mission, repo string, number int) bool {
	for _, c := range m.Capsules {
		if c.Repo == repo && c.SeenPR != nil && c.SeenPR.Number == number {
			return true
		}
	}
	return false
}

func dedupe(list []string) []string {
	var out []string
	for _, s := range list {
		if s != "" && !containsFold(out, s) {
			out = append(out, s)
		}
	}
	return out
}

func containsFold(list []string, s string) bool {
	for _, x := range list {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}
