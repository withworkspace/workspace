// Package pulls is the Reviews page's side of ws: the open pull requests
// across the workspace's repos, sorted by what each asks of you, with
// stacks as one thing; a pull request's changes read from git; and what ws
// keeps of each (your reviews of it, where it's docked). It has no UI: the
// web UI serves what it returns.
package pulls

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/github"
)

// Sections: what a pull request asks of you. A teammate's is in one of
// the first seven; yours, in one of the last three.
const (
	NeedsReview   = "needs-review"
	BackWithYou   = "back-with-you"
	WaitingOnThem = "waiting-on-them"
	Approved      = "approved"
	Others        = "others"
	Drafts        = "drafts"
	Automated     = "automated"

	Returned      = "returned"
	Ready         = "ready"
	WaitingReview = "waiting-review"
)

type sectionDef struct {
	key, title string
	yours      bool
	folded     bool
	counts     bool // it's in the sidebar's count
}

// sections are in the order they show, which is also how pressing each is
// (within theirs, and within yours).
var sections = []sectionDef{
	{key: NeedsReview, title: "Needs your review", counts: true},
	{key: BackWithYou, title: "Back with you", counts: true},
	{key: WaitingOnThem, title: "Waiting on them"},
	{key: Approved, title: "Approved"},
	{key: Others, title: "Everyone else's"},
	{key: Drafts, title: "Drafts", folded: true},
	{key: Automated, title: "Automated", folded: true},
	{key: Returned, title: "Returned to you", yours: true, counts: true},
	{key: Ready, title: "Ready to merge", yours: true, counts: true},
	{key: WaitingReview, title: "Waiting for review", yours: true},
}

func sectionOf(key string) (sectionDef, int) {
	for i, s := range sections {
		if s.key == key {
			return s, i
		}
	}
	return sectionDef{}, len(sections)
}

// PR is an open pull request, as the Reviews page shows it.
type PR struct {
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	Author string `json:"author"`
	URL    string `json:"url"`
	Branch string `json:"branch"`
	// Base is the branch it's for: a stack's layer is for the layer below.
	Base string `json:"base"`
	// Head is its commit on GitHub.
	Head  string `json:"head,omitempty"`
	Draft bool   `json:"draft,omitempty"`
	Bot   bool   `json:"bot,omitempty"`
	// Fork: its branch is in a fork, so it can be reviewed but not docked.
	Fork bool `json:"fork,omitempty"`
	Mine bool `json:"mine,omitempty"`

	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
	Files     int `json:"files"`
	// Checks is success, failure, pending, or "" for none.
	Checks string `json:"checks,omitempty"`
	// Merges is whether it merges cleanly into its base: clean, conflicts,
	// or "" while GitHub hasn't said (it works it out once asked, and
	// again after the base moves).
	Merges string `json:"merges,omitempty"`
	// Decision is GitHub's: approved, changes_requested, review_required,
	// or "" where no review is needed.
	Decision string `json:"decision,omitempty"`
	Created  string `json:"created,omitempty"`
	Updated  string `json:"updated,omitempty"`

	// State is the section it's in, and Words says why, in a few.
	State string `json:"state"`
	Words string `json:"words"`
	// Asked is how your review was asked for: "you", by name, or "team".
	Asked string `json:"asked,omitempty"`
	// Reviewed is your latest review of it on GitHub.
	Reviewed *MyReview `json:"reviewed,omitempty"`
	// Mission is the mission whose capsule it's from (yours only).
	Mission *MissionRef `json:"mission,omitempty"`
	// Capsule is the capsule its branch is checked out in here, if any.
	Capsule string `json:"capsule,omitempty"`
	// OnTopOf is the PR its base is the head of, when that's not the layer
	// below it in a stack: a branch off someone's PR.
	OnTopOf int `json:"onTopOf,omitempty"`
	// Merge is what ws offers to do to merge it, of yours on its own (a
	// stack's layer merges with its stack): "now", it's ready; "ready",
	// only waiting holds it (its checks, a review, its base's rules), so
	// it can be left to merge by itself once it can; "" neither.
	Merge string `json:"merge,omitempty"`
	// AutoMerge: it's set to merge by itself once it can.
	AutoMerge bool `json:"autoMerge,omitempty"`
}

// Ref is how a PR is named: api#89.
func (p PR) Ref() string { return fmt.Sprintf("%s#%d", p.Repo, p.Number) }

// MyReview is your latest review of a PR on GitHub.
type MyReview struct {
	// State is approved, changes_requested, commented or dismissed.
	State  string `json:"state"`
	At     string `json:"at,omitempty"`
	Commit string `json:"commit,omitempty"`
	// Behind: the PR has commits you haven't reviewed.
	Behind bool `json:"behind,omitempty"`
}

// MissionRef is a mission a PR is from.
type MissionRef struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

// Stack is a stack of pull requests: one change that arrives as several,
// each layer building on the one below. It's a row in the list and one
// review, with its layers inside.
type Stack struct {
	Repo string `json:"repo"`
	// Number is GitHub's number for it; 0 for one ws worked out (a chain of
	// PRs each based on the last's branch), which WorkedOut says.
	Number    int  `json:"number,omitempty"`
	WorkedOut bool `json:"workedOut,omitempty"`
	// Title is its bottom layer's.
	Title  string `json:"title"`
	Author string `json:"author"`
	Mine   bool   `json:"mine,omitempty"`
	// Base is the branch the whole stack is for.
	Base string `json:"base"`
	// Layers are its open PRs, bottom first: the order to read them in.
	Layers []PR `json:"layers"`
	// Merged is how many layers have merged below them.
	Merged    int `json:"merged,omitempty"`
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
	// State and Words are its most pressing layer's.
	State string `json:"state"`
	Words string `json:"words"`
	// Open is the layer to open: the lowest that needs you.
	Open int `json:"open"`
	// OnTopOf is the PR its bottom layer is based on, when it's a branch
	// off another PR.
	OnTopOf int `json:"onTopOf,omitempty"`
	// Merges: it's yours, one of GitHub's, and every layer's ready to
	// merge, so it can be merged whole, from its top (never in part).
	Merges bool `json:"merges,omitempty"`
}

// Row is a row of the list: a pull request, or a stack of them.
type Row struct {
	Key     string `json:"key"`
	PR      *PR    `json:"pr,omitempty"`
	Stack   *Stack `json:"stack,omitempty"`
	updated string
	byName  bool
}

// Section is a group of rows that ask the same of you.
type Section struct {
	Key    string `json:"key"`
	Title  string `json:"title"`
	Yours  bool   `json:"yours,omitempty"`
	Folded bool   `json:"folded,omitempty"`
	Rows   []Row  `json:"rows"`
}

// List is the Reviews page: every open PR in the workspace's repos, in
// sections, theirs first.
type List struct {
	// At is when GitHub was last asked.
	At time.Time `json:"at"`
	Me string    `json:"me"`
	// Sections are those with something in them.
	Sections []Section `json:"sections"`
	// Count is what the sidebar counts: what needs your review or is back
	// with you, and your own PRs returned or ready where no mission
	// already says so.
	Count int `json:"count"`
	// Problems are repos GitHub couldn't be asked about, and why.
	Problems []string `json:"problems,omitempty"`
}

// Find is a PR in the list, with its stack if it's in one.
func (l *List) Find(repo string, number int) (*PR, *Stack) {
	if l == nil {
		return nil, nil
	}
	for si := range l.Sections {
		for ri := range l.Sections[si].Rows {
			r := &l.Sections[si].Rows[ri]
			if r.PR != nil && r.PR.Repo == repo && r.PR.Number == number {
				return r.PR, nil
			}
			if r.Stack != nil && r.Stack.Repo == repo {
				for li := range r.Stack.Layers {
					if r.Stack.Layers[li].Number == number {
						return &r.Stack.Layers[li], r.Stack
					}
				}
			}
		}
	}
	return nil, nil
}

// Yours is each PR in the list that's yours or asks for your review: a
// stack's layers each, as it is.
// All is every open PR the list has, a stack's layers each.
func (l *List) All() []PR {
	if l == nil {
		return nil
	}
	var out []PR
	for _, sec := range l.Sections {
		for _, r := range sec.Rows {
			if r.PR != nil {
				out = append(out, *r.PR)
			}
			if r.Stack != nil {
				out = append(out, r.Stack.Layers...)
			}
		}
	}
	return out
}

func (l *List) Yours() []PR {
	if l == nil {
		return nil
	}
	var out []PR
	yours := func(pr PR) bool { return pr.Mine || pr.Asked != "" }
	for _, sec := range l.Sections {
		for _, r := range sec.Rows {
			if r.PR != nil && yours(*r.PR) {
				out = append(out, *r.PR)
			}
			if r.Stack != nil {
				for _, layer := range r.Stack.Layers {
					if yours(layer) || r.Stack.Mine {
						out = append(out, layer)
					}
				}
			}
		}
	}
	return out
}

func sameLogin(a, b string) bool { return a != "" && strings.EqualFold(a, b) }

// activityGrace is how long after your review a PR's last update still
// counts as the review itself, not something since.
const activityGrace = 90 * time.Second

// classify says which section a PR is in, and why. asked is whether
// GitHub's search found it asking for your review (by name, or a team of
// yours).
func classify(gp github.PR, me string, asked bool) (state, words, how string, mine *MyReview) {
	var my *github.LatestReview
	for i := range gp.Latest {
		if sameLogin(gp.Latest[i].Author, me) && gp.Latest[i].State != "PENDING" {
			my = &gp.Latest[i]
		}
	}
	if my != nil {
		mine = &MyReview{State: strings.ToLower(my.State), At: my.SubmittedAt, Commit: my.Commit,
			Behind: my.Commit != "" && gp.HeadRefOid != "" && my.Commit != gp.HeadRefOid}
	}
	if sameLogin(gp.Author, me) {
		state, words = classifyMine(gp)
		return state, words, "", nil
	}
	for _, r := range gp.Requested {
		if sameLogin(r.Login, me) {
			how = "you"
		}
	}
	if how == "" && asked {
		how = "team"
	}
	atHead := mine != nil && mine.Commit != "" && mine.Commit == gp.HeadRefOid
	switch {
	case gp.IsDraft:
		return Drafts, "draft", how, mine
	case how != "" && !atHead:
		words = "needs your review"
		if how == "team" {
			words = "needs a review from your team"
		}
		if mine != nil {
			words = "needs your review again"
		}
		return NeedsReview, words, how, mine
	case mine != nil && mine.State == "approved":
		words = "you approved"
		if mine.Behind {
			words = "you approved, new commits since"
		}
		return Approved, words, how, mine
	case mine != nil && mine.State == "dismissed":
		return BackWithYou, "your review was dismissed: new commits since", how, mine
	case mine != nil && (mine.State == "changes_requested" || mine.State == "commented"):
		said := "you asked for changes"
		if mine.State == "commented" {
			said = "you commented"
		}
		switch {
		case mine.Behind:
			return BackWithYou, said + ", new commits since", how, mine
		case since(my.SubmittedAt, gp.UpdatedAt) > activityGrace:
			return BackWithYou, said + ", new replies since", how, mine
		}
		return WaitingOnThem, said, how, mine
	case gp.Bot:
		return Automated, "opened by a bot", how, mine
	}
	return Others, "nobody's asked you", how, mine
}

// classifyMine says where one of your own PRs stands.
func classifyMine(gp github.PR) (state, words string) {
	switch {
	case gp.ReviewDecision == "CHANGES_REQUESTED":
		return Returned, "changes requested"
	case gp.StatusRollup == "failure":
		return Returned, "checks failing"
	case gp.Mergeable == "CONFLICTING":
		// Approved and green or not, it can't be merged as it is.
		return Returned, "conflicts with " + firstNonEmpty(gp.BaseRefName, "base")
	case gp.IsDraft:
		return WaitingReview, "draft"
	case gp.StatusRollup == "pending":
		return WaitingReview, "checks running"
	case gp.MergeState == "BLOCKED" && gp.ReviewDecision != "APPROVED":
		// Its base branch's rules want a review (GitHub's review decision
		// can be empty while a ruleset does). That you could merge it
		// anyway, as an admin, doesn't make it ready.
		return WaitingReview, "waiting for review"
	case gp.MergeState == "BLOCKED":
		return WaitingReview, "approved, but blocked by branch rules"
	case gp.ReviewDecision == "APPROVED":
		return ifMerges(gp, "approved")
	case gp.ReviewDecision == "" && len(gp.Requested) == 0:
		return ifMerges(gp, "needs no review")
	}
	return WaitingReview, "waiting for review"
}

// ifMerges is where one of yours that nothing else holds up stands: ready
// to merge, unless GitHub is still working out whether it merges cleanly
// (it does so once asked, and after its base moves). Until it's said,
// it's waiting: it may turn out to conflict, and something that needs you
// shouldn't show for a moment and go.
func ifMerges(gp github.PR, words string) (state, why string) {
	if gp.Mergeable == "UNKNOWN" {
		return WaitingReview, "checking for merge conflicts"
	}
	return Ready, words
}

// mergesOf is GitHub's word on whether a pull request merges cleanly,
// as a PR says it: clean, conflicts, or "" while it's still unknown.
func mergesOf(mergeable string) string {
	switch mergeable {
	case "MERGEABLE":
		return "clean"
	case "CONFLICTING":
		return "conflicts"
	}
	return ""
}

// since is how long b is after a (RFC 3339 times), or 0 when it can't be
// told.
func since(a, b string) time.Duration {
	ta, err1 := time.Parse(time.RFC3339, a)
	tb, err2 := time.Parse(time.RFC3339, b)
	if err1 != nil || err2 != nil {
		return 0
	}
	return tb.Sub(ta)
}

// Local is what the workspace knows that GitHub doesn't.
type Local struct {
	// Missions are the missions with a capsule on a branch, by
	// "repo/branch".
	Missions map[string]MissionRef
	// Capsules are the capsules checked out here, by "repo/branch".
	Capsules map[string]string
}

// repoPRs is what GitHub said of one repo.
type repoPRs struct {
	repo   string
	prs    []github.PR
	stacks []github.Stack
}

// build sorts every repo's open PRs into the list. asked are the PRs
// GitHub's search found asking for your review.
func build(me string, repos []repoPRs, asked []github.PRRef, local Local) *List {
	askedOf := map[string]bool{}
	for _, a := range asked {
		askedOf[fmt.Sprintf("%s#%d", a.Repo, a.Number)] = true
	}
	var rows []Row
	for _, rp := range repos {
		rows = append(rows, repoRows(me, rp, askedOf, local)...)
	}
	l := &List{Me: me, Sections: []Section{}}
	for _, def := range sections {
		sec := Section{Key: def.key, Title: def.title, Yours: def.yours, Folded: def.folded, Rows: []Row{}}
		for _, r := range rows {
			if rowState(r) == def.key {
				sec.Rows = append(sec.Rows, r)
			}
		}
		if len(sec.Rows) == 0 {
			continue
		}
		// Those asked of you by name come first; then the latest to change.
		sort.SliceStable(sec.Rows, func(i, j int) bool {
			a, b := sec.Rows[i], sec.Rows[j]
			if a.byName != b.byName {
				return a.byName
			}
			return a.updated > b.updated
		})
		if def.counts {
			for _, r := range sec.Rows {
				if !def.yours || !rowHasMission(r) {
					l.Count++
				}
			}
		}
		l.Sections = append(l.Sections, sec)
	}
	return l
}

func rowState(r Row) string {
	if r.Stack != nil {
		return r.Stack.State
	}
	return r.PR.State
}

// rowHasMission is whether a mission already speaks for a row: its
// headline says what the PR needs.
func rowHasMission(r Row) bool {
	if r.PR != nil {
		return r.PR.Mission != nil
	}
	for _, l := range r.Stack.Layers {
		if l.Mission == nil {
			return false
		}
	}
	return true
}

// repoRows are one repo's rows: its stacks (GitHub's, then chains ws
// works out from what each PR is based on), then the PRs in neither.
func repoRows(me string, rp repoPRs, askedOf map[string]bool, local Local) []Row {
	byNumber := map[int]*PR{}
	byHead := map[string]*PR{} // by branch: what a layer above is based on
	var order []*PR
	for _, gp := range rp.prs {
		p := &PR{
			Repo: rp.repo, Number: gp.Number, Title: gp.Title, Author: gp.Author, URL: gp.URL, Branch: gp.HeadRefName, Base: gp.BaseRefName,
			Head: gp.HeadRefOid, Draft: gp.IsDraft, Bot: gp.Bot, Fork: gp.IsCrossRepository, Mine: sameLogin(gp.Author, me),
			Additions: gp.Additions, Deletions: gp.Deletions, Files: gp.ChangedFiles, Checks: gp.StatusRollup,
			Decision: strings.ToLower(gp.ReviewDecision), Created: gp.CreatedAt, Updated: gp.UpdatedAt,
			Merges: mergesOf(gp.Mergeable), AutoMerge: gp.AutoMergeRequest != nil,
		}
		p.State, p.Words, p.Asked, p.Reviewed = classify(gp, me, askedOf[p.Ref()])
		key := rp.repo + "/" + gp.HeadRefName
		if p.Mine {
			if m, ok := local.Missions[key]; ok {
				p.Mission = &m
			}
		}
		if !p.Fork {
			p.Capsule = local.Capsules[key]
			byHead[gp.HeadRefName] = p
		}
		byNumber[gp.Number] = p
		order = append(order, p)
	}

	var rows []Row
	used := map[int]bool{}
	// GitHub's stacks.
	for _, gs := range rp.stacks {
		var layers []*PR
		merged := 0
		for _, sp := range gs.PRs {
			if p := byNumber[sp.Number]; p != nil && !used[sp.Number] {
				layers = append(layers, p)
			} else if sp.Merged {
				merged++
			}
		}
		if len(layers) == 0 {
			continue
		}
		layers = orderLayers(layers)
		for _, p := range layers {
			used[p.Number] = true
		}
		rows = append(rows, stackRow(rp.repo, gs.Number, firstNonEmpty(gs.Base, layers[0].Base), layers, merged, 0))
	}
	stacked := map[int]bool{}
	for n := range used {
		stacked[n] = true
	}
	// Chains GitHub doesn't know about: a PR whose base is another open
	// PR's branch is a layer on top of it. Where two sit on the same one,
	// each starts a stack of its own, "on top of" it.
	children := map[int][]*PR{}
	parent := map[int]*PR{}
	for _, p := range order {
		if used[p.Number] || p.Fork {
			continue
		}
		if below := byHead[p.Base]; below != nil && below.Number != p.Number {
			children[below.Number] = append(children[below.Number], p)
			parent[p.Number] = below
		}
	}
	var chain func(start *PR, onTopOf int)
	chain = func(start *PR, onTopOf int) {
		layers := []*PR{start}
		used[start.Number] = true
		cur := start
		for {
			next := children[cur.Number]
			if len(next) != 1 {
				break
			}
			cur = next[0]
			layers = append(layers, cur)
			used[cur.Number] = true
		}
		if len(layers) == 1 {
			start.OnTopOf = onTopOf
			rows = append(rows, prRow(start))
		} else {
			base := start.Base
			row := stackRow(rp.repo, 0, base, layers, 0, onTopOf)
			rows = append(rows, row)
		}
		for _, p := range children[cur.Number] { // two or more: each its own
			chain(p, cur.Number)
		}
	}
	for _, p := range order {
		if used[p.Number] {
			continue
		}
		switch below := parent[p.Number]; {
		case below == nil:
			chain(p, 0)
		case stacked[below.Number]: // on a layer of one of GitHub's stacks: its own row
			chain(p, below.Number)
		}
		// Otherwise it's reached from its parent's chain.
	}
	// Anything a cycle of bases left out.
	for _, p := range order {
		if !used[p.Number] {
			used[p.Number] = true
			rows = append(rows, prRow(p))
		}
	}
	return rows
}

func prRow(p *PR) Row {
	p.Merge = mergeOffer(p)
	return Row{Key: p.Ref(), PR: p, updated: p.Updated, byName: p.Asked == "you"}
}

// mergeOffer is what's offered to merge a PR of yours on its own (see
// PR.Merge): what's in Waiting for review is waiting, but a draft.
func mergeOffer(p *PR) string {
	switch {
	case !p.Mine:
		return ""
	case p.State == Ready:
		return "now"
	case p.State == WaitingReview && !p.Draft && p.Merges != "conflicts":
		return "ready"
	}
	return ""
}

// orderLayers puts a stack's layers bottom first, by what each is based
// on; where that doesn't say, they stay as GitHub listed them.
func orderLayers(layers []*PR) []*PR {
	byHead := map[string]*PR{}
	for _, p := range layers {
		byHead[p.Branch] = p
	}
	above := map[int]*PR{} // the layer based on each
	var bottoms []*PR
	for _, p := range layers {
		if below := byHead[p.Base]; below != nil && below != p && above[below.Number] == nil {
			above[below.Number] = p
		} else {
			bottoms = append(bottoms, p)
		}
	}
	var out []*PR
	seen := map[int]bool{}
	for _, b := range bottoms {
		for p := b; p != nil && !seen[p.Number]; p = above[p.Number] {
			seen[p.Number] = true
			out = append(out, p)
		}
	}
	for _, p := range layers {
		if !seen[p.Number] {
			out = append(out, p)
		}
	}
	return out
}

// needsYou are the sections that ask something of you now.
var needsYou = map[string]bool{NeedsReview: true, BackWithYou: true, Returned: true, Ready: true}

// stackRow makes a stack of layers (bottom first) a row: named for its
// bottom layer, in its most pressing layer's section.
func stackRow(repo string, number int, base string, layers []*PR, merged, onTopOf int) Row {
	s := &Stack{Repo: repo, Number: number, WorkedOut: number == 0, Title: layers[0].Title, Author: layers[0].Author, Mine: layers[0].Mine,
		Base: base, Merged: merged, OnTopOf: onTopOf, Open: layers[0].Number}
	best := len(sections) + 1
	updated := ""
	byName := false
	opened := false
	for _, p := range layers {
		s.Layers = append(s.Layers, *p)
		s.Additions += p.Additions
		s.Deletions += p.Deletions
		def, at := sectionOf(p.State)
		// A stack is theirs or yours, as its bottom layer is: a layer of the
		// other kind doesn't place it.
		if def.yours == s.Mine && at < best {
			best, s.State, s.Words = at, p.State, fmt.Sprintf("#%d %s", p.Number, p.Words)
		}
		if needsYou[p.State] && !opened {
			s.Open, opened = p.Number, true
		}
		if p.Updated > updated {
			updated = p.Updated
		}
		byName = byName || p.Asked == "you"
	}
	if s.State == "" {
		s.State, s.Words = layers[0].State, layers[0].Words
	}
	s.Merges = s.Mine && number > 0 && !slices.ContainsFunc(s.Layers, func(p PR) bool { return p.State != Ready })
	key := fmt.Sprintf("%s/stack-%d", repo, number)
	if number == 0 {
		key = fmt.Sprintf("%s/chain-%d", repo, layers[0].Number)
	}
	return Row{Key: key, Stack: s, updated: updated, byName: byName}
}

func firstNonEmpty(a ...string) string {
	for _, s := range a {
		if s != "" {
			return s
		}
	}
	return ""
}
