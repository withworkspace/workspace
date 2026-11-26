package team

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// A period's account: what the team's been working on, as a few topics,
// each the changes that are part of one piece of work. An agent writes it
// when it's asked for, from the gists of the period's changes and nothing
// else; who's on each topic is taken from its changes, not from what the
// agent says. It's kept in the records, by what the period's called.

// Topic is one thing the team's been working on.
type Topic struct {
	// Name is what the work is, in a few words, and Text what's been done
	// and where it stands, in a sentence or two.
	Name string `json:"name"`
	Text string `json:"text"`
	// Strands are the changes that are part of it, and People the keys of
	// those on them, whose they are first.
	Strands []string `json:"strands"`
	People  []string `json:"people"`
}

// Summary is a period's account, as kept.
type Summary struct {
	// Since is where the period starts, and At when it was written.
	Since time.Time `json:"since"`
	At    time.Time `json:"at"`
	// From names what it was written from: it's out of date when the
	// period's changes, or what's been written of them, are no longer
	// these.
	From   string  `json:"from"`
	Topics []Topic `json:"topics,omitempty"`
	Failed string  `json:"failed,omitempty"`
}

// SummaryView is a period's account as a timeline has it.
type SummaryView struct {
	At     *time.Time `json:"at,omitempty"`
	Topics []Topic    `json:"topics,omitempty"`
	// Stale: the period has changed since it was written. Writing: it's
	// being written now. Failed is why it couldn't be.
	Stale   bool   `json:"stale,omitempty"`
	Writing bool   `json:"writing,omitempty"`
	Failed  string `json:"failed,omitempty"`
	// Can is how many changes there are to write it from: those that have
	// been read.
	Can int `json:"can"`
}

// Change is one of a period's changes, as an agent is told of it for the
// account: numbered, so it can say which it means.
type Change struct {
	Number int
	Strand string
	Name   string
	Repos  []string
	People []string // as they're called
	State  string
	Gist   string
	Know   []Know
}

// SummaryAsk is what an agent is asked to write an account from.
type SummaryAsk struct {
	Since   time.Time
	Changes []Change
}

// changesGiven is the most changes an account is written from: the
// latest.
const changesGiven = 120

// changes are a timeline's changes that have been read, the latest first,
// each once: what an account is written from, with whose each is.
func changes(t *Timeline) ([]Change, map[string][]string) {
	short := map[string]string{}
	for _, p := range t.People {
		short[p.Key] = p.Short
	}
	var out []Change
	on := map[string][]string{}
	at := map[string]int{}
	for _, d := range t.Days {
		for _, it := range d.Items {
			if it.Bot || it.Gist == "" {
				continue
			}
			if _, seen := at[it.Strand]; !seen {
				if len(out) >= changesGiven {
					continue
				}
				repos := it.Repos
				if len(repos) == 0 {
					repos = []string{it.Repo}
				}
				i := len(out)
				at[it.Strand] = i
				out = append(out, Change{Number: i + 1, Strand: it.Strand, Name: it.Name, Repos: repos, State: it.State, Gist: it.Gist, Know: it.Know})
			}
			// Everyone on it in the period, whose it is first: the latest
			// row's people, then earlier rows' who aren't there yet.
			for _, key := range it.People {
				has := false
				for _, k := range on[it.Strand] {
					has = has || k == key
				}
				if !has {
					on[it.Strand] = append(on[it.Strand], key)
				}
			}
		}
	}
	for i := range out {
		for _, key := range on[out[i].Strand] {
			out[i].People = append(out[i].People, short[key])
		}
	}
	return out, on
}

// fromOf names what an account is written from: each change, where it
// stands and what's been written of it.
func fromOf(list []Change) string {
	h := sha1.New()
	for _, c := range list {
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00", c.Strand, c.State, c.Gist)
	}
	return hex.EncodeToString(h.Sum(nil)[:8])
}

// SummaryPrompt is what an agent is asked for a period's account: what
// topics are and how to reply (read by ParseSummary), then the changes.
// name is a repo as people call it.
func SummaryPrompt(ask SummaryAsk, name func(repo string) string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You're writing a short account of what a team has been working on since %s, for the people on it, from the changes below: pull requests and commits across the team's repos. Each has a number, what it's called, where it is, who's on it, where it stands, and what it does. You can't read or run anything: everything you have is below.\n\n", ask.Since.Format("Monday 2 January"))

	b.WriteString("**Group the changes into topics:** the few pieces of work the team has been on.\n")
	b.WriteString("- A topic is work with one aim, whoever did it and whichever repos it's in: a feature built in a backend and a frontend, a migration, a run of fixes to one thing, a sweep of dependency updates, decisions recorded about one subject.\n")
	b.WriteString("- Three to seven topics, the most consequential first. Fewer, if that's what there is.\n")
	b.WriteString("- A change that's part of nothing larger is left out, unless it's consequential by itself. No topic called \"other\", \"miscellaneous\" or \"various\", and no two unrelated changes put together to make one.\n")
	b.WriteString("- A change is in one topic at most.\n\n")

	b.WriteString("**For each topic:**\n")
	b.WriteString("- A name of a few words: what the work is, as someone on the team would call it. \"Bundle contents\", not \"Improvements to bundles\".\n")
	b.WriteString("- The numbers of its changes.\n")
	b.WriteString("- One or two sentences, of 45 words at most together, saying what it comes to and where it stands. Say exactly which part has landed and which is still in review (\"the API's half has landed; the BFF's is in review\"), not that they're in various states. If one of its changes has something worth knowing, say it plainly. Say the thing itself, not that work was done on it.\n")
	b.WriteString("- Don't name people: who's on each topic is shown beside it. Nothing about how much anyone did, no praise, no guess at why. No \"I\" or \"we\".\n\n")

	b.WriteString("**Reply in exactly this form,** one line a topic, with nothing before or after:\n\n")
	b.WriteString("```\ntopic: <name> | <its changes' numbers, with commas between> | <the sentences>\n```\n\n---\n\n**The changes,** the latest first:\n\n")

	states := map[string]string{InProgress: "a draft", InReview: "in review", Done: "landed", Shut: "closed"}
	for _, c := range ask.Changes {
		var repos []string
		for _, r := range c.Repos {
			repos = append(repos, name(r))
		}
		where := strings.Join(repos, ", ")
		if len(repos) > 3 {
			where = fmt.Sprintf("%d repos", len(repos))
		}
		fmt.Fprintf(&b, "%d. %s (%s; %s; %s)\n   %s\n", c.Number, c.Name, where, strings.Join(c.People, ", "), states[c.State], c.Gist)
		for _, k := range c.Know {
			fmt.Fprintf(&b, "   Worth knowing (%s): %s\n", k.Kind, k.Text)
		}
	}
	return b.String()
}

// ParseSummary reads an agent's account: a line "topic: name | numbers |
// text" for each. A topic that names no change it was given is left out,
// as is a change named twice: each is under the first topic that has it.
func ParseSummary(reply string, list []Change) ([]Topic, error) {
	byNumber := map[int]Change{}
	for _, c := range list {
		byNumber[c.Number] = c
	}
	taken := map[int]bool{}
	var topics []Topic
	for line := range strings.SplitSeq(reply, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "-*"))
		if !strings.HasPrefix(strings.ToLower(line), "topic:") {
			continue
		}
		parts := strings.SplitN(line[len("topic:"):], "|", 3)
		if len(parts) < 3 {
			continue
		}
		t := Topic{Name: strings.Trim(strings.TrimSpace(parts[0]), "*`"), Text: strings.TrimSpace(parts[2])}
		for _, f := range strings.FieldsFunc(parts[1], func(r rune) bool { return r < '0' || r > '9' }) {
			n, _ := strconv.Atoi(f)
			if c, ok := byNumber[n]; ok && !taken[n] {
				taken[n] = true
				t.Strands = append(t.Strands, c.Strand)
			}
		}
		if t.Name == "" || t.Text == "" || len(t.Strands) == 0 || len(topics) >= 8 {
			continue
		}
		topics = append(topics, t)
	}
	if len(topics) == 0 {
		return nil, fmt.Errorf("its reply had no topics: %s", clip(reply, 160))
	}
	return topics, nil
}

// summariesKept is how many periods' accounts are kept: the latest
// written.
const summariesKept = 12

// Summarise has a period's account written, in the background, from the
// changes in it that have been read. The period is kept under what it's
// called (today, 7d, "" for as far back as is kept), so the last seven
// days' account is still the last seven days' tomorrow, and out of date.
// It answers whether it started: not when one's being written, there's
// nothing to write it from, or nothing here writes them.
func (s *Service) Summarise(snap *Snapshot, since time.Time, key string) bool {
	if s.Summariser == nil || snap == nil {
		return false
	}
	t := s.Timeline(snap, since)
	list, on := changes(t)
	s.mu.Lock()
	if len(list) == 0 || s.summarising[key] {
		s.mu.Unlock()
		return false
	}
	if s.summarising == nil {
		s.summarising = map[string]bool{}
	}
	s.summarising[key] = true
	s.version++
	s.mu.Unlock()
	go func() {
		sum := Summary{Since: t.Since, At: s.now(), From: fromOf(list)}
		reply, err := s.Summariser(SummaryAsk{Since: t.Since, Changes: list})
		if err == nil {
			sum.Topics, err = ParseSummary(reply, list)
		}
		if err != nil {
			sum.Failed = err.Error()
		}
		// Who's on a topic is who's on its changes, whose they are first.
		for i := range sum.Topics {
			seen := map[string]bool{}
			for pass := range 2 {
				for _, strand := range sum.Topics[i].Strands {
					for n, k := range on[strand] {
						if (pass == 0) == (n == 0) && !seen[k] {
							seen[k] = true
							sum.Topics[i].People = append(sum.Topics[i].People, k)
						}
					}
				}
			}
		}
		// One that failed leaves what was written before, with why.
		s.keepSummary(key, sum)
		s.mu.Lock()
		delete(s.summarising, key)
		s.version++
		s.mu.Unlock()
	}()
	return true
}

// TryAccount has a period's account written without keeping it: for
// seeing what an agent is given and what it says. Dry, or with nothing
// to write it, it stops at what would be asked.
func (s *Service) TryAccount(snap *Snapshot, since time.Time, dry bool) (SummaryAsk, []Topic, error) {
	t := s.Timeline(snap, since)
	list, _ := changes(t)
	ask := SummaryAsk{Since: t.Since, Changes: list}
	if len(list) == 0 {
		return ask, nil, fmt.Errorf("none of the period's changes has been read: there's nothing to write it from (ws team --since … --read reads them)")
	}
	if dry || s.Summariser == nil {
		return ask, nil, nil
	}
	reply, err := s.Summariser(ask)
	if err != nil {
		return ask, nil, err
	}
	topics, err := ParseSummary(reply, list)
	return ask, topics, err
}

// Account fills a timeline in with its period's account (the period as
// Summarise was told it's called), if there's one, and whether it still
// stands.
func (s *Service) Account(t *Timeline, key string) {
	list, _ := changes(t)
	sum, ok := s.summaryOf(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	v := &SummaryView{Can: len(list), Writing: s.summarising[key]}
	if ok {
		at := sum.At
		v.At, v.Topics, v.Failed = &at, sum.Topics, sum.Failed
		v.Stale = len(sum.Topics) > 0 && sum.From != fromOf(list)
	}
	if s.Summariser != nil || v.At != nil {
		t.Summary = v
	}
}
