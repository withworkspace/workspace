package team

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/pulls"
)

// What an agent writes of a strand: its gist (what the change does, in a
// sentence) and what's worth knowing of it, if anything is, for someone who
// didn't write it. It's written from the change itself (its title and
// description, the files it touches and its diff), once when a pull request
// opens and again when it lands, and kept in the records, by strand.

// Kinds of thing worth knowing.
const (
	KnowContract   = "contract"   // something other code calls or reads has changed shape
	KnowAction     = "action"     // something people must do
	KnowConvention = "convention" // how things are done here has changed
	KnowRemoved    = "removed"    // something was taken away or deprecated
	KnowBehaviour  = "behaviour"  // what users or callers see has changed
)

// KnowKinds are the kinds, in the order they're told to an agent.
var KnowKinds = []string{KnowContract, KnowAction, KnowConvention, KnowRemoved, KnowBehaviour}

// Know is one thing worth knowing of a change: what kind of thing, the
// changed file it rests on, and what it is, in a sentence.
type Know struct {
	Kind string `json:"kind"`
	// In is the repo File is in, for a change made in several repos.
	In   string `json:"in,omitempty"`
	File string `json:"file"`
	Text string `json:"text"`
}

// Known is something worth knowing, with the strand it's of: for the
// list at the head of a period.
type Known struct {
	Know
	Strand string `json:"strand"`
	// Item is the key of the strand's latest row in the period.
	Item   string    `json:"item"`
	Name   string    `json:"name"`
	Repo   string    `json:"repo"`
	PR     int       `json:"pr,omitempty"`
	People []string  `json:"people"`
	At     time.Time `json:"at"`
}

// Narrative is how what agents write stands.
type Narrative struct {
	// On: an agent writes gists here. Off says why not, when it doesn't.
	On  bool   `json:"on"`
	Off string `json:"off,omitempty"`
	// Writing is how many are being written now, and Unwritten how many
	// rows in the period have none and could be asked for.
	Writing   int `json:"writing"`
	Unwritten int `json:"unwritten"`
}

// What a gist is of.
const (
	OfOpen    = "open"    // a pull request, as it stood when it was first seen open
	OfLanded  = "landed"  // a pull request, as it merged
	OfCommits = "commits" // commits with no pull request
)

// Gist is what was written of a strand.
type Gist struct {
	Strand string `json:"strand"`
	// Of is what it was written of, and From the commits: a gist is out
	// of date when its strand's are no longer these.
	Of   string    `json:"of"`
	From string    `json:"from"`
	At   time.Time `json:"at"`
	Gist string    `json:"gist,omitempty"`
	Know []Know    `json:"know,omitempty"`
	// Failed is why it couldn't be written, and Tries how often it's
	// been tried.
	Failed string `json:"failed,omitempty"`
	Tries  int    `json:"tries,omitempty"`
}

// Subject is a strand to write a gist of: what it is, and where its
// change is to be read.
type Subject struct {
	Strand string
	Kind   string // pr or commits
	Repo   string
	PR     int
	Title  string
	// Body is a pull request's description.
	Body string
	// Subjects are the commits' subjects, for commits with no pull request.
	Subjects []string
	Of       string
	From     string
	// Commits are the commits on the default branch whose changes it is:
	// a pull request's merge, or the commits themselves. An open pull
	// request has none: its branch is fetched, against Base.
	Commits []string
	Branch  string
	Base    string
	At      time.Time
	Bot     bool
	// Parts are its pull requests, when it's one change made in several
	// repos: each is read as a subject of its own, and they're given
	// together. Repo is then the first of them.
	Parts []Subject
}

// subjects are the strands some events could have gists written of: a
// pull request that's open, or merged in a commit git has; a change made
// in several repos, from its pull requests that are; and each run of one
// person's commits to a repo in a day. Closed pull requests have none.
func subjects(events []Event, prs map[string]PR, cfg map[string]PersonConfig, loc *time.Location) []Subject {
	roster := Resolve(events, cfg)
	strands, members := strandsOf(events, func(e Event) string {
		if p := roster.Of(e); p != nil {
			return p.Key
		}
		return ""
	}, loc)

	// Each pull request and each run of commits, as a subject of its own.
	by := map[string]*Subject{}
	var order []string
	closed := map[string]bool{}
	for _, e := range events {
		key := strands[e.ID]
		if e.PR != 0 {
			key = "pr:" + prKey(e.Repo, e.PR)
		}
		s := by[key]
		if s == nil {
			s = &Subject{Strand: key, Repo: e.Repo, PR: e.PR, Kind: "commits", Of: OfCommits, Bot: true}
			if e.PR != 0 {
				s.Kind, s.Of, s.From, s.Bot = "pr", OfOpen, OfOpen, false
				if pr, ok := prs[prKey(e.Repo, e.PR)]; ok {
					s.Body, s.Branch, s.Base = pr.Body, pr.HeadRefName, pr.BaseRefName
				}
			}
			by[key] = s
			order = append(order, key)
		}
		if e.At.After(s.At) {
			s.At = e.At
		}
		switch {
		case e.PR == 0:
			if s.Title == "" {
				s.Title = e.Title
			}
			s.Subjects = append(s.Subjects, e.Title)
			s.Commits = append(s.Commits, e.Commit)
			s.Bot = s.Bot && e.Bot
		case e.Kind == Reviewed:
		default:
			s.Title = e.Title
			s.Bot = s.Bot || e.Bot
			if e.Kind == Merged {
				s.Of, s.From, s.Commits = OfLanded, e.Commit, nil
				if e.Commit != "" {
					s.Commits = []string{e.Commit}
				}
			}
			closed[key] = closed[key] || e.Kind == Closed
		}
	}
	// readable is whether a subject's change is there to read: not one
	// closed without merging, nor merged in a commit git hasn't yet.
	readable := func(key string) bool {
		s := by[key]
		return s != nil && !(closed[key] && s.Of != OfLanded) && !(s.Of == OfLanded && len(s.Commits) == 0)
	}

	part := map[string]string{}
	for strand, keys := range members {
		for _, k := range keys {
			part["pr:"+k] = strand
		}
	}
	done := map[string]bool{}
	var out []Subject
	for _, key := range order {
		s := by[key]
		strand, joined := part[key]
		switch {
		case joined && done[strand]:
		case joined:
			// One change in several repos: its pull requests that can be
			// read, together. It's of them as they're first seen, and
			// then of them once none is still open.
			done[strand] = true
			whole := Subject{Strand: strand, Kind: "pr", Of: OfLanded}
			var from []string
			waiting := false
			for _, k := range members[strand] {
				p := by["pr:"+k]
				if p == nil {
					continue
				}
				if p.At.After(whole.At) {
					whole.At = p.At
				}
				whole.Bot = whole.Bot || p.Bot
				if !readable("pr:" + k) {
					// Merged, and not fetched yet: it isn't all landed.
					waiting = waiting || !closed["pr:"+k]
					continue
				}
				if whole.Title == "" {
					whole.Repo, whole.Title, whole.Body = p.Repo, p.Title, p.Body
				}
				if p.Of == OfOpen {
					whole.Of = OfOpen
				}
				whole.Parts = append(whole.Parts, *p)
				from = append(from, k+"@"+p.From)
			}
			if len(whole.Parts) == 0 {
				continue
			}
			if waiting {
				whole.Of = OfOpen
			}
			if whole.Of == OfOpen {
				// While any is open, it's of which pull requests there are.
				from = append([]string(nil), members[strand]...)
			}
			sort.Strings(from)
			sum := sha1.Sum([]byte(whole.Of + " " + strings.Join(from, " ")))
			whole.From = hex.EncodeToString(sum[:8])
			out = append(out, whole)
		case !readable(key):
		default:
			if s.Of == OfCommits {
				sum := sha1.Sum([]byte(strings.Join(s.Commits, " ")))
				s.From = hex.EncodeToString(sum[:8])
			}
			out = append(out, *s)
		}
	}
	return out
}

// FileChange is a file a change touches, and by how much.
type FileChange struct {
	// Repo is the repo it's in, for a change made in several: "" where
	// there's only the one.
	Repo      string
	Path      string
	Additions int
	Deletions int
}

// Name is how a file is written: its path, after its repo and a colon
// where the change is in several.
func (f FileChange) Name() string {
	if f.Repo != "" {
		return f.Repo + ":" + f.Path
	}
	return f.Path
}

// Material is a change as an agent is given it to read: the files it
// touches, and its diff, with what's not worth reading left out.
type Material struct {
	Files []FileChange
	Diff  string
	// Clipped: some of the diff was left out, for length.
	Clipped bool
}

const (
	// diffPerFile and diffInAll are how much of a change's diff is given,
	// in bytes: a file's, and all of it.
	diffPerFile = 6 * 1024
	diffInAll   = 48 * 1024
)

// noise is whether a file's diff isn't worth reading: what a tool wrote.
func noise(path string) bool {
	base := path[strings.LastIndex(path, "/")+1:]
	switch base {
	case "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "go.sum", "Cargo.lock", "poetry.lock", "uv.lock", "Gemfile.lock", "composer.lock":
		return true
	}
	for _, suffix := range []string{".lock", ".min.js", ".min.css", ".map", ".snap", ".svg", ".pb.go"} {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	for _, dir := range []string{"dist/", "vendor/", "node_modules/", "__snapshots__/"} {
		if strings.HasPrefix(path, dir) || strings.Contains(path, "/"+dir) {
			return true
		}
	}
	return false
}

// lockfile is whether a file is one a package manager writes, and
// manifest whether it's where dependencies are declared.
func lockfile(path string) bool {
	base := path[strings.LastIndex(path, "/")+1:]
	switch base {
	case "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "go.sum", "Cargo.lock", "poetry.lock", "uv.lock", "Gemfile.lock", "composer.lock", "packages.lock.json":
		return true
	}
	return strings.HasSuffix(base, ".lock")
}

func manifest(path string) bool {
	base := path[strings.LastIndex(path, "/")+1:]
	switch base {
	case "package.json", "pnpm-workspace.yaml", "pyproject.toml", "go.mod", "Cargo.toml", "Gemfile", "composer.json", "Directory.Packages.props", ".npmrc":
		return true
	}
	return strings.HasSuffix(base, ".csproj") || (strings.HasPrefix(base, "requirements") && strings.HasSuffix(base, ".txt"))
}

// DependenciesOnly reports whether a change is to dependencies and
// nothing else: every file is a manifest or a lockfile, and a lockfile is
// among them (a manifest changed alone may be a setting, not a
// dependency). Such a change has a gist, and nothing worth knowing: told
// so, an agent still finds something in every bump.
func DependenciesOnly(files []FileChange) bool {
	locked := false
	for _, f := range files {
		switch {
		case lockfile(f.Path):
			locked = true
		case !manifest(f.Path):
			return false
		}
	}
	return locked
}

// read reads an agent's reply about a change: ParseGist, and nothing
// worth knowing of a change that's only to dependencies.
func read(reply string, m Material) (string, []Know, error) {
	gist, know, err := ParseGist(reply, m.Files)
	if DependenciesOnly(m.Files) {
		know = nil
	}
	return gist, know, err
}

// material reads a subject's change from its repo's clone (bare answers
// where a repo's is). An open pull request's branch is fetched first. A
// change made in several repos is each of its pull requests' changes,
// one after another, each file and diff under its repo's name, with the
// room for the diff shared out between them.
func material(bare func(repo string) string, sub Subject) (Material, error) {
	if len(sub.Parts) == 0 {
		return materialOf(bare(sub.Repo), sub, diffInAll)
	}
	room := max(diffInAll/len(sub.Parts), 2*1024)
	var m Material
	var b strings.Builder
	var failed error
	for _, part := range sub.Parts {
		pm, err := materialOf(bare(part.Repo), part, room)
		if err != nil {
			failed = err
			continue
		}
		for _, f := range pm.Files {
			f.Repo = part.Repo
			m.Files = append(m.Files, f)
		}
		if pm.Diff != "" {
			fmt.Fprintf(&b, "=== in %s (#%d)\n%s", part.Repo, part.PR, pm.Diff)
		}
		m.Clipped = m.Clipped || pm.Clipped
	}
	if len(m.Files) == 0 && failed != nil {
		return Material{}, failed
	}
	m.Diff = b.String()
	return m, nil
}

// materialOf reads one pull request's or run of commits' change, giving
// up to room bytes of its diff.
func materialOf(bare string, sub Subject, room int) (Material, error) {
	if bare == "" {
		return Material{}, fmt.Errorf("%s isn't cloned here", sub.Repo)
	}
	var stats, patches []string
	if sub.Of == OfOpen {
		head, err := pulls.Fetch(bare, sub.PR, sub.Branch, sub.Base)
		if err != nil {
			return Material{}, err
		}
		base, err := git.Output(bare, "merge-base", pulls.BaseRef(sub.Base), head)
		if err != nil {
			return Material{}, fmt.Errorf("finding where #%d left %s: %w", sub.PR, sub.Base, err)
		}
		base = strings.TrimSpace(base)
		stat, err := git.Output(bare, "diff", "--numstat", "-M", base, head, "--")
		if err != nil {
			return Material{}, err
		}
		patch, err := git.Output(bare, "diff", "--no-color", "-M", base, head, "--")
		if err != nil {
			return Material{}, err
		}
		stats, patches = []string{stat}, []string{patch}
	} else {
		for _, sha := range sub.Commits {
			stat, err := git.Output(bare, "show", "--diff-merges=first-parent", "--format=", "--numstat", "-M", sha, "--")
			if err != nil {
				return Material{}, err
			}
			patch, err := git.Output(bare, "show", "--diff-merges=first-parent", "--format=", "--no-color", "--patch", "-M", sha, "--")
			if err != nil {
				return Material{}, err
			}
			stats, patches = append(stats, stat), append(patches, patch)
		}
	}
	var m Material
	at := map[string]int{}
	for _, stat := range stats {
		for line := range strings.SplitSeq(stat, "\n") {
			n := strings.SplitN(line, "\t", 3)
			if len(n) < 3 {
				continue
			}
			var add, del int
			_, _ = fmt.Sscan(n[0], &add) // what can't be read is none
			_, _ = fmt.Sscan(n[1], &del) // what can't be read is none
			path := renamedTo(n[2])
			i, ok := at[path]
			if !ok {
				i = len(m.Files)
				at[path] = i
				m.Files = append(m.Files, FileChange{Path: path})
			}
			m.Files[i].Additions += add
			m.Files[i].Deletions += del
		}
	}
	var b strings.Builder
	for _, patch := range patches {
		for i, section := range strings.Split("\n"+patch, "\ndiff --git ") {
			if i == 0 || strings.TrimSpace(section) == "" {
				continue
			}
			section = "diff --git " + section
			head, _, _ := strings.Cut(section, "\n")
			path := head[strings.LastIndex(head, " b/")+3:]
			switch {
			case noise(path) || strings.Contains(section, "\nBinary files "):
				continue
			case b.Len() >= room:
				m.Clipped = true
				continue
			case len(section) > diffPerFile:
				cut := strings.LastIndex(section[:diffPerFile], "\n")
				section = section[:cut] + "\n… (the rest of this file's change is left out)"
				m.Clipped = true
			}
			b.WriteString(strings.TrimRight(section, "\n") + "\n")
		}
	}
	m.Diff = b.String()
	return m, nil
}

// renamedTo is the path a numstat line is for: a rename's is where it
// went ("a/{old => new}/b" or "old => new").
func renamedTo(path string) string {
	if !strings.Contains(path, " => ") {
		return path
	}
	if open, shut := strings.Index(path, "{"), strings.Index(path, "}"); open >= 0 && shut > open {
		_, to, _ := strings.Cut(path[open+1:shut], " => ")
		return strings.ReplaceAll(path[:open]+to+path[shut+1:], "//", "/")
	}
	_, to, _ := strings.Cut(path, " => ")
	return to
}

// bodyGiven is how much of a pull request's description is given, in
// bytes.
const bodyGiven = 6 * 1024

// GistPrompt is what an agent writing a change's gist is asked: what to
// write and how to reply (read by ParseGist), then the change. name is a
// repo as people call it.
func GistPrompt(ask Ask, name func(repo string) string) string {
	sub := ask.Subject
	var b strings.Builder
	where := fmt.Sprintf("a change to %s (the repo `%s`)", name(sub.Repo), sub.Repo)
	if n := len(sub.Parts); n > 1 {
		where = fmt.Sprintf("one change made in %d repos, as a pull request in each", n)
	}
	fmt.Fprintf(&b, "You're writing two things about %s, for the other engineers who work in this group of repos. They'll read them in a list of what's happened lately, to learn what the change is and whether it touches their own work. You can't read or run anything: everything you have is below.\n\n", where)

	b.WriteString("**First, the gist:** one sentence, of 30 words at most, saying what the change does.\n")
	b.WriteString("- Say the thing itself, as it now is: \"Bundles say what they contain, with `type` and `contents` on the bundle resource.\" Not that a change was made: no \"This PR\", \"This change\", \"Updates\", \"Adds support for\".\n")
	b.WriteString("- Say more than the title does, from what the change shows. If you can only restate the title, restate it plainly.\n")
	b.WriteString("- No \"I\" or \"we\", nothing about who wrote it, no praise, and no guess at why.\n\n")

	b.WriteString("**Then, what's worth knowing, if anything is.** Most changes have nothing, and the right answer for them is `know: none`. Something is worth knowing only when an engineer who didn't write this change, working somewhere else in these repos, would be caught out, or would do their own work differently, for not knowing it. Ask of each: who, outside this change, now has to do something, stop doing something, or do it another way? If nobody, it isn't worth knowing. It's one of:\n")
	b.WriteString("- `contract`: something other code already calls or reads now has another shape or meaning: an API, a wire format, a schema, an exported function or type, a config key, a command's flags, renamed, retyped, or answering differently.\n")
	b.WriteString("- `action`: something people must do: run a migration, set a variable, regenerate something, upgrade a tool.\n")
	b.WriteString("- `convention`: how things are to be done here from now on: a pattern, a rule, a guide or template, a check in CI, a decision recorded, that others are expected to follow.\n")
	b.WriteString("- `removed`: something others may use was taken away or deprecated.\n")
	b.WriteString("- `behaviour`: something that was already there now behaves differently for its users or callers, in a way the gist doesn't say.\n")
	b.WriteString("What's only new is not worth knowing: a new flag, field, endpoint, option, screen or document that leaves what was there as it was. The gist already says it's there. Nor is a fix inside one function, a tidy-up, a dependency bumped, pinned or overridden, or anything only this change's own tests see (a test id, a fixture, a helper).\n")
	b.WriteString("Say what the change makes so, not what might follow from it. If you'd write \"may\", \"might\" or \"could\", you're guessing who's affected: leave it out.\n")
	b.WriteString("Each names the one changed file it most rests on, written exactly as it is under *Files*. What you can't point at a file for isn't worth knowing. Each is one sentence of 30 words at most, said to that engineer: what's different for them, and what to do about it if that isn't plain. Don't say the gist again, don't list what the change did, and say nothing of how well it was done. Where there's something, it's usually one thing: give the one that matters most first, and three at most.\n\n")

	b.WriteString("**Reply in exactly this form, with nothing before or after:**\n\n")
	b.WriteString("```\ngist: <the sentence>\nknow: <kind> | <file> | <the sentence>\n```\n\n")
	b.WriteString("with one `know:` line for each thing worth knowing, or the one line `know: none`.\n\n---\n\n")

	switch {
	case len(sub.Parts) > 1:
		fmt.Fprintf(&b, "**The change:** the same change made in %d repos, on a branch of the same name in each. Say what it does across them as one thing, and name a repo only where it differs from the rest. Its pull requests:\n", len(sub.Parts))
		for _, p := range sub.Parts {
			state := "open"
			if p.Of == OfLanded {
				state = "merged"
			}
			fmt.Fprintf(&b, "- `%s` #%d (%s): %s\n", p.Repo, p.PR, state, p.Title)
		}
	case sub.Kind == "commits" && len(sub.Subjects) > 1:
		fmt.Fprintf(&b, "**The change:** %d commits pushed straight to the default branch, with no pull request:\n", len(sub.Subjects))
		for _, s := range sub.Subjects {
			fmt.Fprintf(&b, "- %s\n", s)
		}
	case sub.Kind == "commits":
		fmt.Fprintf(&b, "**The change:** a commit pushed straight to the default branch, with no pull request: %s\n", sub.Title)
	case sub.Of == OfLanded:
		fmt.Fprintf(&b, "**The change:** a pull request that has merged: %s\n", sub.Title)
	default:
		fmt.Fprintf(&b, "**The change:** a pull request that's open, and hasn't merged: %s\n", sub.Title)
	}
	if body := strings.TrimSpace(sub.Body); body != "" {
		if len(body) > bodyGiven {
			body = body[:bodyGiven] + "\n… (the rest is left out)"
		}
		b.WriteString("\n**Its description, as its author wrote it** (it may be out of date, or say more than the change does: the diff is what's so):\n\n" + quoted(body) + "\n")
	}
	b.WriteString("\n**Files:**\n")
	for _, f := range ask.Material.Files {
		fmt.Fprintf(&b, "- `%s` (+%d −%d)\n", f.Name(), f.Additions, f.Deletions)
	}
	if len(ask.Touches) > 0 {
		b.WriteString("\n**Some of these files are how things are done here,** and a change to one may be a change to what people are expected to do (a `convention`), if it's more than a tidy-up:\n")
		for _, t := range ask.Touches {
			fmt.Fprintf(&b, "- %s: `%s`\n", t.Title, strings.Join(t.Files, "`, `"))
		}
	}
	if ask.Material.Diff != "" {
		b.WriteString("\n**The diff")
		if ask.Material.Clipped {
			b.WriteString(", with some of it left out for length")
		}
		b.WriteString(":**\n\n````diff\n" + ask.Material.Diff + "````\n")
	} else {
		b.WriteString("\nIts diff isn't given: the files are generated, or binary.\n")
	}
	return b.String()
}

// quoted is text as a markdown quote.
func quoted(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight("> "+l, " ")
	}
	return strings.Join(lines, "\n")
}

// ParseGist reads an agent's reply: a line "gist: …", then a line
// "know: kind | file | sentence" for each thing worth knowing, or "know:
// none". Something worth knowing that doesn't name one of the change's
// files, or whose kind isn't one, is left out: what can't be pointed at
// isn't said.
func ParseGist(reply string, files []FileChange) (string, []Know, error) {
	var gist string
	var know []Know
	for line := range strings.SplitSeq(reply, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "-*"))
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "gist:") && gist == "":
			gist = strings.TrimSpace(line[len("gist:"):])
		case strings.HasPrefix(lower, "know:"):
			parts := strings.SplitN(line[len("know:"):], "|", 3)
			if len(parts) < 3 {
				continue
			}
			kind := strings.ToLower(strings.TrimSpace(parts[0]))
			if kind == "behavior" {
				kind = KnowBehaviour
			}
			file, found := matchFile(strings.Trim(strings.TrimSpace(parts[1]), "`"), files)
			text := strings.TrimSpace(parts[2])
			ok := false
			for _, k := range KnowKinds {
				ok = ok || k == kind
			}
			if !ok || !found || text == "" || len(know) >= 3 {
				continue
			}
			know = append(know, Know{Kind: kind, In: file.Repo, File: file.Path, Text: text})
		}
	}
	if gist == "" {
		return "", nil, fmt.Errorf("its reply had no gist: %s", clip(reply, 160))
	}
	return gist, know, nil
}

// matchFile is the change's file a name is of: one written the same
// (with its repo, where the change is in several), or the only one whose
// path it is, or is the end of.
func matchFile(name string, files []FileChange) (FileChange, bool) {
	if name == "" {
		return FileChange{}, false
	}
	var found []FileChange
	for _, f := range files {
		if f.Name() == name {
			return f, true
		}
		if f.Path == name || strings.HasSuffix(f.Path, "/"+name) || strings.HasSuffix(f.Name(), ":"+name) {
			found = append(found, f)
		}
	}
	if len(found) == 1 {
		return found[0], true
	}
	return FileChange{}, false
}

func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// Ask is what an agent is asked to write a gist from.
type Ask struct {
	Subject  Subject
	Material Material
	// Touches are the watches the change touches (watch.go): it's told
	// that these files are how things are done here.
	Touches []Touch
}

// asked is what an agent is asked of a change: the change, and the
// watches its files touch.
func (s *Service) asked(sub Subject, m Material) Ask {
	ask := Ask{Subject: sub, Material: m}
	watches := s.watches()
	byRepo := map[string][]string{}
	var repos []string
	for _, f := range m.Files {
		repo := f.Repo
		if repo == "" {
			repo = sub.Repo
		}
		if _, ok := byRepo[repo]; !ok {
			repos = append(repos, repo)
		}
		byRepo[repo] = append(byRepo[repo], f.Path)
	}
	for _, repo := range repos {
		for _, touch := range touched(watches, repo, byRepo[repo]) {
			if len(sub.Parts) > 0 {
				for i := range touch.Files {
					touch.Files[i] = repo + ":" + touch.Files[i]
				}
			}
			ask.Touches = append(ask.Touches, touch)
		}
	}
	return ask
}

// narrateAtOnce is how many gists are written at a time, and
// narrateInOne the most one asking starts.
const (
	narrateAtOnce = 2
	narrateInOne  = 80
)

// recentStart is from when strands' gists are written without being
// asked for: the start of yesterday.
func (s *Service) recentStart() time.Time {
	t := s.now().AddDate(0, 0, -1)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// wants reports whether a subject's gist is to be written: it has none,
// or one of something else (it was open, and has landed; it has more
// commits).
func wants(sub Subject, g Gist, have bool) bool {
	return !have || g.Of != sub.Of || g.From != sub.From
}

// Narrate has gists written, in the background, and says how many it
// started. With no strands and a zero since, it's those of late that
// have none and haven't failed (what an open page asks, each look); with
// strands, those (failed ones too: asking is trying again); with since,
// every one in the period that has none.
func (s *Service) Narrate(snap *Snapshot, strands []string, since time.Time) int {
	if s.Writer == nil || snap == nil {
		return 0
	}
	var cfg map[string]PersonConfig
	if s.People != nil {
		cfg = s.People()
	}
	asked := map[string]bool{}
	for _, strand := range strands {
		asked[strand] = true
	}
	if len(strands) == 0 && since.IsZero() {
		since = s.recentStart()
	}
	subs := subjects(snap.Events, s.readPRs(), cfg, s.now().Location())
	// The latest first: they're what's read first.
	sort.SliceStable(subs, func(i, j int) bool { return subs[i].At.After(subs[j].At) })
	bares := map[string]string{}
	for _, repo := range s.Repos() {
		bares[repo.Name] = repo.Bare
	}
	of := make([]string, len(subs))
	for i, sub := range subs {
		of[i] = sub.Strand
	}
	gists := s.gistsOf(of)
	started := 0
	s.mu.Lock()
	if s.writing == nil {
		s.writing = map[string]bool{}
	}
	var todo []Subject
	for _, sub := range subs {
		g, have := gists[sub.Strand]
		// One that's been written of what the strand now is, is left; so
		// is one that failed, unless it's asked for by name.
		stale := wants(sub, g, have)
		switch {
		case s.writing[sub.Strand] || sub.Bot || bares[sub.Repo] == "":
			continue
		case len(strands) > 0 && !asked[sub.Strand]:
			continue
		case len(strands) == 0 && sub.At.Before(since):
			continue
		case !stale && !(asked[sub.Strand] && g.Failed != ""):
			continue
		case len(todo) >= narrateInOne:
			continue
		}
		s.writing[sub.Strand] = true
		todo = append(todo, sub)
	}
	// Only a gist starting is news: a look that starts none mustn't have
	// every open Team page read it all again.
	if len(todo) > 0 {
		s.version++
	}
	s.mu.Unlock()
	s.semOnce.Do(func() { s.sem = make(chan struct{}, narrateAtOnce) })
	for _, sub := range todo {
		started++
		go func() {
			s.sem <- struct{}{}
			defer func() { <-s.sem }()
			g := s.writeGist(sub, func(repo string) string { return bares[repo] })
			// A gist that was written stays, with why writing it again
			// failed. One that can't be kept is written again when it's
			// next asked for.
			_ = s.keepGist(g) // one that can't be kept is written again when it's next asked for
			s.mu.Lock()
			delete(s.writing, sub.Strand)
			s.version++
			s.mu.Unlock()
		}()
	}
	return started
}

// writeGist has one gist written.
func (s *Service) writeGist(sub Subject, bare func(repo string) string) Gist {
	g := Gist{Strand: sub.Strand, Of: sub.Of, From: sub.From, At: s.now(), Tries: 1}
	m, err := material(bare, sub)
	if err != nil {
		g.Failed = "its change couldn't be read: " + err.Error()
		return g
	}
	if len(m.Files) == 0 {
		g.Failed = "it changes no files"
		return g
	}
	reply, err := s.Writer(s.asked(sub, m))
	if err != nil {
		g.Failed = err.Error()
		return g
	}
	if g.Gist, g.Know, err = read(reply, m); err != nil {
		g.Failed = err.Error()
	}
	return g
}

// Try reads one strand's change and has its gist written, without
// keeping it: for seeing what an agent is given and what it says. With
// no writer, or dry, it stops at what would be asked.
func (s *Service) Try(snap *Snapshot, strand string, dry bool) (Ask, Gist, error) {
	var cfg map[string]PersonConfig
	if s.People != nil {
		cfg = s.People()
	}
	for _, sub := range subjects(snap.Events, s.readPRs(), cfg, s.now().Location()) {
		if sub.Strand != strand {
			continue
		}
		bares := map[string]string{}
		for _, repo := range s.Repos() {
			bares[repo.Name] = repo.Bare
		}
		m, err := material(func(repo string) string { return bares[repo] }, sub)
		ask := s.asked(sub, m)
		if err != nil || dry || s.Writer == nil {
			return ask, Gist{}, err
		}
		reply, err := s.Writer(ask)
		if err != nil {
			return ask, Gist{}, err
		}
		g := Gist{Strand: strand, Of: sub.Of, From: sub.From, At: s.now()}
		g.Gist, g.Know, err = read(reply, m)
		return ask, g, err
	}
	return Ask{}, Gist{}, fmt.Errorf("there's no strand %s to read: a closed pull request has none, nor has a merge git hasn't fetched (ws team -f json lists strands)", strand)
}

// Writing is how many gists are being written now.
func (s *Service) Writing() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.writing)
}

// NarrativeVersion changes whenever a gist starts being written or is
// written: an open page reads again when it does.
func (s *Service) NarrativeVersion() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.version
}

// narrate fills a timeline in with what's been written of its strands,
// and what's worth knowing across it.
func (s *Service) narrate(t *Timeline, snap *Snapshot, cfg map[string]PersonConfig) {
	t.Narrative = &Narrative{On: s.Writer != nil, Off: s.NoWriter}
	if s.Writer == nil && s.NoWriter == "" {
		t.Narrative.Off = "nothing here writes them"
	}
	can := map[string]Subject{}
	for _, sub := range subjects(snap.Events, s.readPRs(), cfg, s.now().Location()) {
		can[sub.Strand] = sub
	}
	var strands []string
	for _, d := range t.Days {
		for _, it := range d.Items {
			strands = append(strands, it.Strand)
		}
	}
	gists := s.gistsOf(strands)
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[string]bool{}
	for _, d := range t.Days {
		for i := range d.Items {
			it := &d.Items[i]
			g, have := gists[it.Strand]
			sub, could := can[it.Strand]
			if have {
				it.Gist, it.Know = g.Gist, g.Know
			}
			switch {
			case s.writing[it.Strand]:
				it.Narrative = "writing"
				if !seen[it.Strand] {
					t.Narrative.Writing++
				}
			case have && g.Failed != "" && (!could || !wants(sub, g, true)):
				it.Narrative, it.GistError = "failed", g.Failed
			case could && !it.Bot && s.Writer != nil && wants(sub, g, have):
				it.Narrative = "unwritten"
				if !seen[it.Strand] {
					t.Narrative.Unwritten++
				}
			}
			// What's worth knowing is said once, at the strand's latest row.
			if !seen[it.Strand] {
				for _, k := range it.Know {
					t.Know = append(t.Know, Known{Know: k, Strand: it.Strand, Item: it.Key, Name: it.Name, Repo: it.Repo, PR: it.PR, People: it.People, At: it.At})
				}
			}
			seen[it.Strand] = true
		}
	}
}
