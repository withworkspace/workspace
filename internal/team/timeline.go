package team

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// The timeline: the events by day, newest first, with each pull request,
// and each run of one person's commits to one repo in a day, as one thing
// (a strand). What's said of each is in ws's own words: an agent is asked
// in later, for strands with more to say.

// Where a strand stands.
const (
	InProgress = "in-progress" // a draft
	InReview   = "in-review"   // open
	Done       = "landed"      // merged, or on the default branch
	Shut       = "closed"      // closed without merging
)

// Timeline is what the Team page shows.
type Timeline struct {
	// At is when git and GitHub were last looked at, and Since where the
	// timeline starts.
	At     time.Time `json:"at"`
	Since  time.Time `json:"since"`
	Days   []Day     `json:"days"`
	People []Person  `json:"people"`
	// GitHub: pull requests were asked for. Without it, the timeline is
	// what git knows.
	GitHub   bool     `json:"github"`
	Problems []string `json:"problems,omitempty"`
	// Seen is when you last looked at the page, if you have: what's since
	// is new to you.
	Seen *time.Time `json:"seen,omitempty"`
	// Know is everything worth knowing in the period, the latest first,
	// and Narrative how what agents write stands (gist.go).
	Know      []Known    `json:"know,omitempty"`
	Narrative *Narrative `json:"narrative,omitempty"`
	// Summary is the period's account: what the team's been working on,
	// as an agent wrote it when asked (summary.go).
	Summary *SummaryView `json:"summary,omitempty"`
}

// Day is one day's strands, the latest to change first.
type Day struct {
	// Date is the day, as 2026-11-10, in the clock's own zone.
	Date  string `json:"date"`
	Items []Item `json:"items"`
}

// Item is a strand on one day: what happened to it then.
type Item struct {
	Key string `json:"key"`
	// Strand names the piece of work: a pull request, or one person's
	// commits to one repo that day.
	Strand string `json:"strand"`
	Kind   string `json:"kind"` // pr or commits
	Repo   string `json:"repo"`
	PR     int    `json:"pr,omitempty"`
	// Repos and PRs are for one change made in several repos, a pull
	// request in each: every repo it's in and each of its pull requests.
	// Repo is then the first of them, and PR is 0.
	Repos []string   `json:"repos,omitempty"`
	PRs   []StrandPR `json:"prs,omitempty"`
	// Name is what it's called: its title, without its "feat:".
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
	// State is where the strand stands now, not as it stood that day.
	State string `json:"state"`
	// People are the keys of those on it that day: whose it is first.
	People []string `json:"people"`
	// Words say what happened, in a sentence or a few.
	Words string    `json:"words"`
	At    time.Time `json:"at"`
	// Bot: it's a bot's, folded away with the day's other automated ones.
	Bot       bool    `json:"bot,omitempty"`
	Additions int     `json:"additions,omitempty"`
	Deletions int     `json:"deletions,omitempty"`
	Events    []Event `json:"events"`

	// What an agent wrote of the strand, once it has (gist.go): what the
	// change does, in a sentence, and what's worth knowing of it, if
	// anything is. Narrative says where that stands: "" (written, or
	// there's none to write), writing, failed (GistError says why) or
	// unwritten (it can be asked for).
	Gist      string `json:"gist,omitempty"`
	Know      []Know `json:"know,omitempty"`
	Narrative string `json:"narrative,omitempty"`
	GistError string `json:"gistError,omitempty"`

	// Touches are the watches it touches (watch.go): a guide, a skill,
	// CI, lint settings. It's a change to how things are done.
	Touches []Touch `json:"touches,omitempty"`

	// Near is where it's near you: capsules of yours that have changed
	// files it changed too, once it's landed. Whoever shows the timeline
	// fills it in: it's about your checkouts, not the team.
	Near []Near `json:"near,omitempty"`
}

// Near is a capsule of yours that has changed some of the files a change
// did.
type Near struct {
	Capsule string   `json:"capsule"`
	Files   []string `json:"files"`
}

// StrandPR is one of the pull requests of a change made in several repos.
type StrandPR struct {
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	Name   string `json:"name"`
	URL    string `json:"url,omitempty"`
	State  string `json:"state"`
}

// joinWithin is how far apart two pull requests on the same branch name
// may be opened and still be one change.
const joinWithin = 14 * 24 * time.Hour

var conventional = regexp.MustCompile(`^(?i)(feat|fix|docs|chore|refactor|test|tests|ci|build|perf|style|revert)(\([^)]*\))?!?:\s*`)

// plainName is a title as a strand is called: without its conventional
// prefix or its pull request's number, starting with a capital.
func plainName(title string) string {
	// A merge commit with nothing under its subject says only where it
	// came from: its branch's name is the nearest thing to a title.
	if mergedPR.MatchString(title) {
		branch := title[strings.LastIndex(title, " ")+1:]
		title = strings.NewReplacer("-", " ", "_", " ").Replace(branch[strings.LastIndex(branch, "/")+1:])
	}
	_, title = prOfSubject(title)
	title = strings.TrimSpace(conventional.ReplaceAllString(title, ""))
	if r, size := utf8.DecodeRuneInString(title); size > 0 && unicode.IsLower(r) {
		title = string(unicode.ToUpper(r)) + title[size:]
	}
	return title
}

// View is the timeline of some events from a time on. Events from before
// it still say where a strand stands.
func View(events []Event, cfg map[string]PersonConfig, since time.Time, loc *time.Location) *Timeline {
	if loc == nil {
		loc = time.Local
	}
	roster := Resolve(events, cfg)
	who := func(e Event) (key, short string) {
		if p := roster.Of(e); p != nil {
			return p.Key, p.Short
		}
		return "", "Someone"
	}

	// Where each pull request stands and whose it is, from everything
	// known of it.
	state := map[string]string{}
	owner := map[string]string{}
	known := map[string]StrandPR{}
	for _, e := range events {
		if e.PR == 0 {
			continue
		}
		k := prKey(e.Repo, e.PR)
		pr := known[k]
		pr.Repo, pr.Number, pr.Name = e.Repo, e.PR, plainName(e.Title)
		if e.URL != "" {
			pr.URL = e.URL
		}
		known[k] = pr
		if e.Kind == Opened || (e.Kind == Merged && owner[k] == "") {
			owner[k], _ = who(e)
		}
		switch {
		case e.Kind == Merged:
			state[k] = Done
		case e.Kind == Closed:
			state[k] = Shut
		case state[k] == "" || state[k] == InProgress || state[k] == InReview:
			if e.Kind == Opened {
				state[k] = InReview
				if e.Draft {
					state[k] = InProgress
				}
			} else if state[k] == "" {
				state[k] = InReview
			}
		}
	}

	strands, members := strandsOf(events, func(e Event) string { key, _ := who(e); return key }, loc)
	type slot struct{ day, strand string }
	items := map[slot]*Item{}
	var order []slot
	for _, e := range events {
		if e.At.Before(since) {
			continue
		}
		day := e.At.In(loc).Format("2006-01-02")
		key, _ := who(e)
		s := slot{day, strands[e.ID]}
		it := items[s]
		if it == nil {
			it = &Item{Key: s.strand + "@" + day, Strand: s.strand, Repo: e.Repo, PR: e.PR, Kind: "commits", State: Done}
			if e.PR != 0 {
				it.Kind = "pr"
				keys := members[s.strand]
				if len(keys) == 0 {
					keys = []string{prKey(e.Repo, e.PR)}
				} else {
					// One change in several repos: every pull request of
					// it, whichever had something happen that day.
					it.PR = 0
					in := map[string]bool{}
					for _, k := range keys {
						pr := known[k]
						pr.State = state[k]
						it.PRs = append(it.PRs, pr)
						if !in[pr.Repo] {
							in[pr.Repo] = true
							it.Repos = append(it.Repos, pr.Repo)
						}
					}
					it.Repo = it.Repos[0]
				}
				it.State = standing(keys, state)
				// Whose it is comes first, whoever did something that day.
				if o := owner[keys[0]]; o != "" {
					it.People = []string{o}
				}
			}
			items[s] = it
			order = append(order, s)
		}
		e.Who = key
		it.Events = append(it.Events, e)
		if e.At.After(it.At) {
			it.At = e.At
		}
		if e.URL != "" {
			it.URL = e.URL
		}
	}

	days := map[string]*Day{}
	var dates []string
	for _, s := range order {
		it := items[s]
		describe(it, who)
		d := days[s.day]
		if d == nil {
			d = &Day{Date: s.day}
			days[s.day] = d
			dates = append(dates, s.day)
		}
		d.Items = append(d.Items, *it)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	t := &Timeline{Since: since, Days: []Day{}, People: roster.People()}
	if t.People == nil {
		t.People = []Person{}
	}
	for _, date := range dates {
		d := days[date]
		sort.SliceStable(d.Items, func(i, j int) bool { return d.Items[i].At.After(d.Items[j].At) })
		t.Days = append(t.Days, *d)
	}
	return t
}

// standing is where a strand of some pull requests stands: in review
// while any is, else in progress while any is a draft, else landed if any
// did, else closed.
func standing(keys []string, state map[string]string) string {
	have := map[string]bool{}
	for _, k := range keys {
		have[state[k]] = true
	}
	for _, s := range []string{InReview, InProgress, Done, Shut} {
		if have[s] {
			return s
		}
	}
	return InReview
}

// joined finds the pull requests that are one change made in several
// repos: the same branch name, opened by the same person, in more than
// one repo, each within two weeks of the last. It answers each one's
// strand, and each such strand's pull requests, the first opened first.
// The events are oldest first.
func joined(events []Event, who func(Event) string) (map[string]string, map[string][]string) {
	type opened struct {
		key, repo string
		at        time.Time
	}
	type on struct{ branch, who string }
	groups := map[on][]opened{}
	var order []on
	for _, e := range events {
		if e.Kind != Opened || e.Branch == "" || e.Bot {
			continue
		}
		g := on{e.Branch, who(e)}
		if _, ok := groups[g]; !ok {
			order = append(order, g)
		}
		groups[g] = append(groups[g], opened{prKey(e.Repo, e.PR), e.Repo, e.At})
	}
	of := map[string]string{}
	members := map[string][]string{}
	for _, g := range order {
		prs := groups[g]
		for from := 0; from < len(prs); {
			to := from + 1
			for to < len(prs) && prs[to].at.Sub(prs[to-1].at) <= joinWithin {
				to++
			}
			repos := map[string]bool{}
			for _, pr := range prs[from:to] {
				repos[pr.repo] = true
			}
			if len(repos) > 1 {
				strand := "b:" + g.branch + ":" + prs[from].key
				for _, pr := range prs[from:to] {
					of[pr.key] = strand
					members[strand] = append(members[strand], pr.key)
				}
			}
			from = to
		}
	}
	return of, members
}

// describeJoined says what happened that day to a change made in several
// repos: what each person did, and in how many of them.
func describeJoined(it *Item, who func(Event) (key, short string), person func(string)) {
	// It's called what its pull requests are, if they agree; else by the
	// branch they share, which is the one name they all have.
	it.Name = it.PRs[0].Name
	for _, pr := range it.PRs {
		if pr.Name != it.Name {
			if branch := it.Events[0].Branch; branch != "" {
				it.Name = branchName(branch)
			}
			break
		}
	}
	size := map[string][2]int{}
	type act struct{ who, did string }
	in := map[act]map[string]bool{}
	var order []act
	say := func(who, did, repo string) {
		a := act{who, did}
		if in[a] == nil {
			in[a] = map[string]bool{}
			order = append(order, a)
		}
		in[a][repo] = true
	}
	// Someone who gave a verdict on a pull request isn't also said to
	// have commented on it.
	gave := map[string]bool{}
	for _, e := range it.Events {
		if key, _ := who(e); e.Kind == Reviewed && e.Verdict != Commented {
			gave[key+" "+prKey(e.Repo, e.PR)] = true
		}
	}
	for _, e := range it.Events {
		key, short := who(e)
		size[prKey(e.Repo, e.PR)] = [2]int{e.Additions, e.Deletions}
		if e.Kind != Reviewed {
			person(key)
			it.Bot = it.Bot || e.Bot
		}
		switch e.Kind {
		case Opened:
			say(short, "opened", e.Repo)
		case Ready:
			say(short, "marked ready for review", e.Repo)
		case Reviewed:
			person(key)
			switch {
			case e.Verdict == Approved:
				say(short, "approved", e.Repo)
			case e.Verdict == Changes:
				say(short, "asked for changes", e.Repo)
			case !gave[key+" "+prKey(e.Repo, e.PR)]:
				say(short, "commented", e.Repo)
			}
		case Merged:
			say(short, "landed", e.Repo)
		case Closed:
			say("", "Closed without merging", e.Repo)
		}
	}
	for _, s := range size {
		it.Additions += s[0]
		it.Deletions += s[1]
	}
	var said []string
	for _, a := range order {
		where := ""
		for repo := range in[a] {
			where = "in " + repo
		}
		if n := len(in[a]); n > 1 {
			where = fmt.Sprintf("in %d repos", n)
		}
		if a.who == "" {
			said = append(said, a.did+" "+where)
		} else {
			said = append(said, a.who+" "+a.did+" "+where)
		}
	}
	it.Words = strings.Join(said, ". ") + "."
}

var (
	// betweenWords is a dash or underscore between two words of a branch
	// name: not one inside a date or a version.
	betweenWords = regexp.MustCompile(`([^0-9])[-_]|[-_]([^0-9])`)
)

// branchName is a branch's name as a title: its last part, with spaces
// for the dashes between words, starting with a capital.
func branchName(branch string) string {
	name := branch[strings.LastIndex(branch, "/")+1:]
	name = betweenWords.ReplaceAllString(name, "$1 $2")
	if r, size := utf8.DecodeRuneInString(name); size > 0 && unicode.IsLower(r) {
		name = string(unicode.ToUpper(r)) + name[size:]
	}
	return name
}

// strandsOf names the strand each event is of, by its id. A pull
// request's is itself, or the change it's part of where the same change
// was made in several repos (joined), whose pull requests are answered
// too. A commit with no pull request is with those of its person's
// commits to its repo that day that touch the same files, named for the
// first of them: so a document added and then amended is one thing, and
// five documents added in a day are five. The events are oldest first.
func strandsOf(events []Event, who func(Event) string, loc *time.Location) (map[string]string, map[string][]string) {
	out := make(map[string]string, len(events))
	of, members := joined(events, who)
	type run struct{ repo, who, day string }
	runs := map[run][]int{}
	var order []run
	for i, e := range events {
		if e.PR != 0 {
			if strand, ok := of[prKey(e.Repo, e.PR)]; ok {
				out[e.ID] = strand
			} else {
				out[e.ID] = "pr:" + prKey(e.Repo, e.PR)
			}
			continue
		}
		r := run{e.Repo, who(e), e.At.In(loc).Format("2006-01-02")}
		if _, ok := runs[r]; !ok {
			order = append(order, r)
		}
		runs[r] = append(runs[r], i)
	}
	for _, r := range order {
		of := runs[r]
		// first[n] is the earliest commit that n is joined to.
		first := make([]int, len(of))
		var find func(int) int
		find = func(n int) int {
			if first[n] != n {
				first[n] = find(first[n])
			}
			return first[n]
		}
		touched := map[string]int{}
		for n, i := range of {
			first[n] = n
			for _, path := range events[i].Paths {
				if m, ok := touched[path]; ok {
					if a, b := find(m), find(n); a < b {
						first[b] = a
					} else {
						first[a] = b
					}
				} else {
					touched[path] = n
				}
			}
		}
		for n, i := range of {
			sha := events[of[find(n)]].Commit
			if len(sha) > 8 {
				sha = sha[:8]
			}
			out[events[i].ID] = "c:" + r.repo + ":" + r.who + ":" + r.day + ":" + sha
		}
	}
	return out, members
}

// listed joins things as a sentence lists them: a, b and c.
func listed(things []string) string {
	switch n := len(things); n {
	case 0:
		return ""
	case 1:
		return things[0]
	default:
		return strings.Join(things[:n-1], ", ") + " and " + things[n-1]
	}
}

// describe fills in what an item is called, who's on it and what's said
// of it, from its events.
func describe(it *Item, who func(Event) (key, short string)) {
	seen := map[string]bool{}
	person := func(key string) {
		if key != "" && !seen[key] {
			seen[key] = true
			it.People = append(it.People, key)
		}
	}
	for _, key := range it.People {
		seen[key] = true
	}
	if it.People == nil {
		it.People = []string{}
	}
	if it.Kind == "commits" {
		key, short := who(it.Events[0])
		person(key)
		last := it.Events[len(it.Events)-1]
		// It's called by the first of its commits: what was added, before
		// it was amended.
		it.Name = plainName(it.Events[0].Title)
		it.Bot = true
		for _, e := range it.Events {
			it.Additions += e.Additions
			it.Deletions += e.Deletions
			it.Bot = it.Bot && e.Bot
		}
		on := "the default branch"
		if last.Branch != "" {
			on = last.Branch
		}
		if n := len(it.Events); n == 1 {
			it.Words = fmt.Sprintf("%s landed straight on %s.", short, on)
		} else {
			var subjects []string
			for i := 0; i < n && len(subjects) < 3; i++ {
				subjects = append(subjects, plainName(it.Events[i].Title))
			}
			more := ""
			if n > len(subjects) {
				more = fmt.Sprintf(", and %d more", n-len(subjects))
			}
			it.Words = fmt.Sprintf("%s landed %d commits straight on %s: %s%s.", short, n, on, strings.Join(subjects, "; "), more)
		}
		return
	}

	if len(it.PRs) > 1 {
		describeJoined(it, who, person)
		return
	}

	// A pull request: whose it is, then what each person did, in order,
	// once each.
	latest := it.Events[len(it.Events)-1]
	it.Name = plainName(latest.Title)
	it.Additions, it.Deletions = latest.Additions, latest.Deletions
	for _, e := range it.Events {
		if e.Kind != Reviewed {
			key, _ := who(e)
			person(key)
			it.Bot = it.Bot || e.Bot
		}
	}
	// A person's reviews in a day are one: their verdict, if they gave
	// one, or that they commented.
	verdict := map[string]string{}
	for _, e := range it.Events {
		if e.Kind != Reviewed {
			continue
		}
		key, _ := who(e)
		if e.Verdict != Commented || verdict[key] == "" {
			verdict[key] = e.Verdict
		}
	}
	// What each did, in order: someone's next thing joins their last.
	type act struct{ who, did string }
	var acts []act
	say := func(who, did string) { acts = append(acts, act{who, did}) }
	reviewed := map[string]bool{}
	for _, e := range it.Events {
		key, short := who(e)
		switch e.Kind {
		case Opened:
			say(short, "opened")
		case Ready:
			say(short, "marked ready for review")
		case Reviewed:
			if reviewed[key] {
				continue
			}
			reviewed[key] = true
			person(key)
			switch verdict[key] {
			case Approved:
				say(short, "approved")
			case Changes:
				say(short, "asked for changes")
			default:
				say(short, "commented")
			}
		case Merged:
			say(short, "landed")
		case Closed:
			say("", "Closed without merging")
		}
	}
	var said []string
	for i := 0; i < len(acts); {
		if acts[i].who == "" {
			said = append(said, acts[i].did)
			i++
			continue
		}
		var did []string
		j := i
		for ; j < len(acts) && acts[j].who == acts[i].who; j++ {
			did = append(did, acts[j].did)
		}
		said = append(said, acts[i].who+" "+listed(did))
		i = j
	}
	it.Words = strings.Join(said, ". ") + "."
}
