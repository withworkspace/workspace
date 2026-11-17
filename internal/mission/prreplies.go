package mission

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// Replies on a mission's pull requests: crew draft one to what someone
// said there (ws mission reply), and ws posts it, as you, through GitHub.
// A reply to a person waits for you first ([missions] replies_need_me, on
// unless set off): it's a task of yours, to post or not.

// Reply is a reply drafted on one of the mission's pull requests.
type Reply struct {
	N int `toml:"n" json:"n"`
	// PR is the pull request (api#12), Thread the review thread it
	// answers ("" for the pull request's own conversation), and To who
	// it's to, as they're named on GitHub.
	PR     string    `toml:"pr" json:"pr"`
	Thread string    `toml:"thread,omitempty" json:"thread,omitempty"`
	To     string    `toml:"to,omitempty" json:"to,omitempty"`
	Text   string    `toml:"text" json:"text"`
	By     string    `toml:"by" json:"by"` // who drafted it
	At     time.Time `toml:"at" json:"at"`
	// Posted is when it was posted; Dropped, that it won't be.
	Posted  *time.Time `toml:"posted,omitempty" json:"posted,omitempty"`
	Dropped bool       `toml:"dropped,omitempty" json:"dropped,omitempty"`
}

// Where is where a reply goes, in words: "api#12" or "thread PRRT_1 on
// api#12".
func (r Reply) Where() string {
	if r.Thread != "" {
		return "thread " + r.Thread + " on " + r.PR
	}
	return r.PR
}

// ReplyByN is the reply numbered n.
func (m *Mission) ReplyByN(n int) (Reply, bool) {
	i := slices.IndexFunc(m.Replies, func(r Reply) bool { return r.N == n })
	if i < 0 {
		return Reply{}, false
	}
	return m.Replies[i], true
}

// DraftReply keeps a reply drafted on a pull request, numbered, and logs
// it.
func (s Store) DraftReply(slug string, r Reply) (Reply, error) {
	r.Text = strings.TrimSpace(r.Text)
	if r.Text == "" {
		return Reply{}, fmt.Errorf("the reply is empty")
	}
	if _, err := s.Update(slug, func(m *Mission) error {
		r.N = len(m.Replies) + 1
		if r.At.IsZero() {
			r.At = now()
		}
		m.Replies = append(m.Replies, r)
		return nil
	}); err != nil {
		return Reply{}, err
	}
	return r, nil
}

// SettleReply records reply n posted (or, not posted, dropped).
func (s Store) SettleReply(slug string, n int, posted bool) error {
	_, err := s.Update(slug, func(m *Mission) error {
		for i := range m.Replies {
			if m.Replies[i].N != n {
				continue
			}
			if posted {
				t := now()
				m.Replies[i].Posted = &t
			} else {
				m.Replies[i].Dropped = true
			}
			m.Tasks = slices.DeleteFunc(m.Tasks, func(t Task) bool { return t.Reply == n })
			return nil
		}
		return fmt.Errorf("mission %s has no reply %d", slug, n)
	})
	return err
}

// CountRound counts another round of fixing step n's pull request (its
// checks gone red, a conflict), in repo for a step that runs in several,
// and says how many there have been.
func (s Store) CountRound(slug string, n int, repo string) (int, error) {
	brief, _ := s.ReadBrief(slug)
	rounds := 0
	_, err := s.Update(slug, func(m *Mission) error {
		states, _ := m.StageProgress(brief)
		st, err := PickStage(states, n)
		if err != nil {
			return err
		}
		r := recordFor(m, m.Route(brief), st.Stage)
		for i := range r.Instances {
			if r.Instances[i].Repo == repo {
				r.Instances[i].Rounds++
				rounds = r.Instances[i].Rounds
				return nil
			}
		}
		r.Rounds++
		rounds = r.Rounds
		return nil
	})
	return rounds, err
}
