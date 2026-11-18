package mission

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// A verification bundle (a bundle) is what's handed over to show that work
// is as its checks say: each check with a verdict and what shows it
// (images, video, text), how to try it yourself, what wasn't verified, and
// what was noticed beside the checks. It's read on a page by someone
// deciding whether to approve it, so it's held to a length too: what was
// measured goes in a file it shows, not in its words.
// Its crew write it whole, in markdown, and ws reads it strictly
// (ParseBundle), refusing one that isn't right with each thing wrong and
// its line, as it reads a plan. It's one thing, with a ref of its own
// (b3), and it's never edited: a later one of its step takes its place.
// Its record is the mission's; its files are copied into a folder of its
// own (BundleDir) when it's handed over, so it's whole whatever becomes of
// the files it was made from.

// Bundle is a verification bundle, as kept.
type Bundle struct {
	ID string `toml:"id" json:"id"`
	// Step is the step of the mission's route it's of, by its number when
	// it was handed over, and Stage that step's text, as a stage's record
	// is known: 0 and "" where its crew were on none.
	Step  int    `toml:"step,omitempty" json:"step,omitempty"`
	Stage string `toml:"stage,omitempty" json:"stage,omitempty"`
	// Crew is who made it, and Own that they did the work it shows: it's
	// the author's own word, not someone else's.
	Crew string `toml:"crew" json:"crew"`
	Own  bool   `toml:"own,omitempty" json:"own,omitempty"`
	// Capsule (repo/name) is where the work is, and Commit the commit it
	// was at when the bundle was handed over: what it's a bundle of.
	Capsule string    `toml:"capsule,omitempty" json:"capsule,omitempty"`
	Commit  string    `toml:"commit,omitempty" json:"commit,omitempty"`
	At      time.Time `toml:"at" json:"at"`
	// Replaces is the bundle of its step, in its capsule, that it takes
	// the place of.
	Replaces string `toml:"replaces,omitempty" json:"replaces,omitempty"`

	Summary string        `toml:"summary" json:"summary"`
	Try     []BundleTry   `toml:"try,omitempty" json:"try,omitempty"`
	Checks  []BundleCheck `toml:"checks" json:"checks"`
	// NotVerified is what it says it didn't verify, and why.
	NotVerified []string `toml:"not_verified,omitempty" json:"not_verified,omitempty"`
	// Noticed is what its crew came across that isn't one of the checks:
	// it's for the user to know, and sends nothing back.
	Noticed []string `toml:"noticed,omitempty" json:"noticed,omitempty"`

	// Verdict is yours of it, as a review has one: BundleApproved, or
	// BundleSentBack with Notes, what you said wasn't right ("" until you
	// say). VerdictAt is when.
	Verdict   string       `toml:"verdict,omitempty" json:"verdict,omitempty"`
	VerdictAt *time.Time   `toml:"verdict_at,omitempty" json:"verdict_at,omitempty"`
	Notes     []BundleNote `toml:"notes,omitempty" json:"notes,omitempty"`
	// Looked are its checks you've marked as looked at, by their keys in
	// its list (c2 for its check 2, r1 for the step's check 1 that ws
	// ran): yours to keep your place with, as files viewed in a review.
	Looked []string `toml:"looked,omitempty" json:"looked,omitempty"`
}

// How a bundle stands with you.
const (
	BundleApproved = "approved"
	BundleSentBack = "sent back"
)

// BundleNote is what you said isn't right of a bundle: of one of its
// checks, by its number, or (0) of it as a whole.
type BundleNote struct {
	Check int    `toml:"check" json:"check"`
	Text  string `toml:"text" json:"text"`
}

// BundleTry is something for the user to do, to see it themselves: the
// one command to run, in backticks, or words. Look is what to look for,
// rather than something to do. (One kept before a command had to stand
// alone may have words after its command in Text.)
type BundleTry struct {
	Text    string `toml:"text" json:"text"`
	Command string `toml:"command,omitempty" json:"command,omitempty"`
	Look    bool   `toml:"look,omitempty" json:"look,omitempty"`
}

// BundleCheck is one check of a bundle: what it came to, and what shows
// it.
type BundleCheck struct {
	N int `toml:"n" json:"n"`
	// Check is the step's check, as its plan says it: "" for a claim of
	// the bundle's own (a step with no checks), which Said then is.
	Check string `toml:"check,omitempty" json:"check,omitempty"`
	// Verdict is CheckMet, CheckUnmet or CheckCantTell, and Said what was
	// found: what was seen, what's missing, or what would show it.
	Verdict string `toml:"verdict" json:"verdict"`
	Said    string `toml:"said" json:"said"`
	// Points are facts of the same kind (a result at each size, in each
	// browser), shown as a list under what it said.
	Points []string `toml:"points,omitempty" json:"points,omitempty"`
	// Shows are the files that show it; Lines, where in the capsule it's
	// done; Try, how to see it yourself; Asks, prompts to send on to crew.
	Shows []Evidence  `toml:"shows,omitempty" json:"shows,omitempty"`
	Lines []string    `toml:"lines,omitempty" json:"lines,omitempty"`
	Try   []BundleTry `toml:"try,omitempty" json:"try,omitempty"`
	Asks  []string    `toml:"asks,omitempty" json:"asks,omitempty"`

	line int
}

// Evidence is a file that shows a check: an image, a video, or text (what
// a command printed, say), and what it shows.
type Evidence struct {
	// Path is the file as the bundle named it, File its name in the
	// bundle's folder once kept, Media its content type and Size its size.
	Path    string `toml:"path" json:"path"`
	File    string `toml:"file,omitempty" json:"file,omitempty"`
	Media   string `toml:"media,omitempty" json:"media,omitempty"`
	Size    int64  `toml:"size,omitempty" json:"size,omitempty"`
	Caption string `toml:"caption,omitempty" json:"caption,omitempty"`
	// Before is the same on ground, to show beside it; MadeBy, the command
	// that made it.
	Before *Evidence `toml:"before,omitempty" json:"before,omitempty"`
	MadeBy string    `toml:"made_by,omitempty" json:"made_by,omitempty"`

	line int
}

// The parts of a bundle.
const (
	BundleSummary     = "Summary"
	BundleTryIt       = "Try it"
	BundleChecks      = "Checks"
	BundleNotVerified = "Not verified"
	BundleNoticed     = "Noticed"
)

// How long a bundle's words may be, in words: its summary, a check that's
// met, a check that came to anything else (what's wrong, what would show
// it, what there is to weigh), and an item of Not verified or Noticed.
// One over is refused, as anything else wrong with it is: a bundle written
// with no limit grew with every round, each check a page of what was
// measured, and the one thing to act on was lost in it. These are what
// crew are told; one is refused at a fifth over (overLength), since an
// agent writing to a length can't count its words, and lands a few past.
const (
	BundleSummaryWords = 80
	BundleMetWords     = 50
	BundleCheckWords   = 100
	BundleItemWords    = 50
)

// BundleFormat says how a bundle is written: what crew are told, and what
// ParseBundle holds it to. The lengths it gives are the ones above
// (TestBundleFormat_ItsLengths). Its example, and what it says of how to
// write, are in the plain sentences it asks for: crew write a bundle in
// the voice they're told in.
const BundleFormat = "A verification bundle is markdown in parts, each a `##` heading:\n\n" +
	"```markdown\n" +
	"## Summary\n" +
	"Check 3 is unmet, so this goes back. In dark mode, the chosen colour is\n" +
	"lost when the page reloads. Check 1 is met. Check 2 could not be tested,\n" +
	"because only one tab could be driven.\n\n" +
	"## Try it\n" +
	"- try: `npm install && cd web && npm run dev`\n" +
	"- look for: Appearance, on http://localhost:5173/settings\n" +
	"- look for: the row of swatches under Accent: pick one, and tabs, buttons and links change at once\n\n" +
	"## Checks\n" +
	"1. met: Picking teal and then rose changed the tabs, buttons and links immediately, in light and dark mode.\n" +
	"   - shows: `pick.webm` Picking three colours in a row\n" +
	"     - made by: `npm run shots -- accent-picker --out pick.webm`\n" +
	"   - shows: `settings-light.png` Light, after picking teal\n" +
	"     - before: `settings-light-ground.png`\n" +
	"   - lines: `web/src/settings/Appearance.tsx:41-88`\n" +
	"   - try: `npm run shots -- accent-picker --headed`\n" +
	"2. can't tell: Only one tab could be driven, so a change in a second tab was never observed.\n" +
	"   - try: open Settings in two tabs, pick in one, look at the other\n" +
	"   - ask: Add a test for the storage event\n" +
	"3. unmet: In dark mode, the chosen colour is lost when the page reloads. The Accent row shows `Blue (default)` again, so the user has to choose the colour a second time. Light mode keeps it.\n" +
	"   - point: Chrome: lost on every reload, 5 of 5.\n" +
	"   - point: Firefox: lost on 2 of 5 reloads.\n" +
	"   - shows: `reload-dark.webm` Teal chosen in dark mode, then a reload\n" +
	"   - ask: Keep the chosen accent colour after a reload in dark mode, as light mode does. The colour is read before the theme is set (`web/src/settings/Appearance.tsx:52`).\n\n" +
	"## Not verified\n" +
	"- Safari: it is not installed here.\n\n" +
	"## Noticed\n" +
	"- At phone width, the names of the swatches are cut off.\n" +
	"```\n\n" +
	"A bundle is read on a page, by someone who wasn't there and is deciding whether to approve it. Write it for them:\n\n" +
	"- Most important first, as a news report is written. The summary starts with the outcome, a check starts with its finding, and a list starts with its most important item. Detail follows, and the least important detail goes last or into a file.\n" +
	"- A check's first sentence stands alone. In about 15 words it says what was found, as the user of the work would see it. A reader who stops there knows the finding.\n" +
	"- Short, plain sentences. One fact to a sentence, in the usual order: what, did what, to what. If a sentence lists more than three things or passes about 25 words, split it or cut it.\n" +
	"- Precise, in simple words. Give the exact name, number or text on screen, and a figure as you measured it: \"7 of 20 rows\", not \"about a third\"; \"8 to 28 ms\", not \"very fast\". Use everyday words, with no idioms, figures of speech or shorthand of your own: write \"the button is hidden\", not \"the button drops out of sight\".\n" +
	"- The words of the work's user, not of your test. Name a thing as the page names it. Say what a thing is the first time you name it (\"the test query `%accent`\", \"the capsule `accent-picker`\"): the reader hasn't seen your setup. Give a size as what it is, with the figure after it: \"phone width (390px)\".\n" +
	"- A number comes with what it means for the user: what they can no longer see or do.\n" +
	"- The check's words are for the person approving: what is wrong, and how much of it. Why it happens in the code and how to put it right are for crew: they go in `ask:` and `lines:`.\n" +
	"- Keep it short: ws refuses a bundle that's over length. What you measured goes in a file the bundle shows, and the bundle gives the result.\n\n" +
	"- `## Summary` is read first. It says how it came out, in three sentences at most (80 words): what's unmet and what's wrong with it, or that every check is met; then what's left for the user. Don't go through the checks one by one (they're below), and don't tell how you went about it or what an earlier bundle found.\n" +
	"- `## Try it` is optional: how the user can see it for themselves. Each item is something to do (`try:`) or what to look for (`look for:`).\n" +
	"  - A `try:` that's run is one command in backticks, with nothing after it: the user copies it once. Where it needs setup, chain it with `&&` (`a && cd x && b`), rather than a second command or words between.\n" +
	"  - What to do or see in the UI once it's running goes in `look for:` items, one each.\n" +
	"  - A `try:` with nothing to run is in words.\n" +
	"- `## Checks` is a numbered list. Where your step has checks, each one in words is here once, by its number in the step (`ws show s<n>` lists them); a check that's a command is ws's to run, so leave it out. Where your step has none, the checks are your own: 1, 2, 3… in order, each saying what's shown. A check starts with what it came to, and says it in a sentence or two:\n" +
	"  - `met:` it's true: what you did and what you saw, once (50 words at most). It needs something that shows it: a `shows:` or a `lines:`.\n" +
	"  - `unmet:` it isn't, or not wholly: what's wrong, first, then where it shows, plainly enough for whoever puts it right (100 words at most). Show one example of it as it is on screen: the exact text in backticks, or the screenshot that has it. If the rest of the check holds, say so in one short sentence, without listing it.\n" +
	"  - `can't tell:` you couldn't show it either way: say what would (100 words at most). It's left for the user, so don't guess.\n" +
	"  - `yours:` only for a check the plan keeps for the user (a `- yours:` of your step): you don't settle it. Say what you've left to help them, and what there is to weigh (100 words at most).\n" +
	"  - Counts, timings, every size and query you tried, what a command printed: these go in a text file the check `shows:`, with what it holds as its caption. The check says what they come to, with the one or two figures that matter.\n" +
	"  - Say a thing once. What the summary or another check says isn't said again: name the check (`as in check 1`).\n" +
	"- Under a check, indented, each `- key: value`:\n" +
	"  - `point:` one of two to four facts of the same kind (a result at each size, in each browser, for each query), which are shown as a list under the check's words. Each starts with what it's of, then the fact. They count in the check's length.\n" +
	"  - `shows:` a file in backticks, then what it shows. An image (.png, .jpg, .gif, .webp, .svg, .avif), a video (.webm, .mp4, .mov), or text, such as what a command printed. Name it by its path from where you are, or in the mission's private files folder. ws copies it into the bundle, so the bundle keeps it whatever becomes of yours. Give the fewest that show it: one where one does.\n" +
	"  - under a `shows:`, further indented: `before:` the same on the default branch, in backticks, which is shown beside it; `made by:` the command that made it, in backticks.\n" +
	"  - `lines:` where in the capsule it's done, as `path:from-to`.\n" +
	"  - `try:` how the user can see this check themselves, as in Try it: one command, chained with `&&`, or words. What to look for once it's running goes in Try it's `look for:` items.\n" +
	"  - `ask:` a prompt the user could send on to crew as it's written, for something you'd suggest.\n" +
	"- `## Not verified` lists what you didn't verify, and why, a line each (50 words at most), or says `Nothing.`\n" +
	"- `## Noticed` is optional: what you came across that isn't one of the checks (something off beside the work, something older than it), a line each (50 words at most). It doesn't send the work back.\n" +
	"- Nothing else: no title, no other headings. A long item can go on over indented lines.\n\n" +
	"A bundle is of a commit: commit your work before you hand it over. It's never edited: hand over another, and it takes this one's place.\n"

// BundleFiles looks at a file a bundle names: what it is (its content
// type and size), or why it can't be kept ("" if it can). ParseBundle
// without one doesn't look at files.
type BundleFiles func(path string) (media string, size int64, why string)

// ParseBundle reads a bundle as it's written (BundleFormat) against its
// step's checks (none: its checks are its own), and refuses one that
// isn't right, saying each thing wrong and the line it's on.
func ParseBundle(written string, checks []Check, files BundleFiles) (*Bundle, error) {
	p := bundleParser{seen: map[string]bool{}, files: files}
	p.read(written)
	p.hold(checks)
	if len(p.problems) > 0 {
		return nil, &MalformedBundle{Problems: p.problems}
	}
	return &p.b, nil
}

// MalformedBundle is a bundle ParseBundle refused: what's wrong with it.
type MalformedBundle struct{ Problems []string }

func (m *MalformedBundle) Error() string {
	if len(m.Problems) == 1 {
		return "the bundle isn't right: " + m.Problems[0]
	}
	return "the bundle isn't right:\n- " + strings.Join(m.Problems, "\n- ")
}

// IsMalformedBundle reports whether err is a bundle refused for its form.
func IsMalformedBundle(err error) bool {
	var m *MalformedBundle
	return errors.As(err, &m)
}

var (
	bundleVerdict = regexp.MustCompile(`(?i)^\**\s*(met|unmet|not met|can['’]?t tell|cannot tell|can not tell|yours)\s*\**\s*[:.]?\s*\**\s*(.*)$`)
	bundleTicked  = regexp.MustCompile("^`([^`]+)`\\s*(.*)$")
	bundleKey     = regexp.MustCompile(`^([A-Za-z]+(?: [A-Za-z]+)?)\s*:\s*(.*)$`)
	bundleSpan    = regexp.MustCompile("`[^`]+`")
)

// bundleOpen is the item being read, continued over indented lines.
type bundleOpen struct {
	line   int
	indent int
	kind   string // try-it, not-verified, noticed, check, field or sub
	number string // a check's
	text   string
}

type bundleParser struct {
	files BundleFiles

	b        Bundle
	part     string
	seen     map[string]bool
	summary  []string
	none     bool // Not verified says there's nothing
	open     *bundleOpen
	check    *BundleCheck // the check being read
	shows    *Evidence    // the shows: its sub-fields go under, and
	showsAt  int          // how far it was indented
	problems []string
}

func (p *bundleParser) problem(line int, format string, args ...any) {
	p.problems = append(p.problems, fmt.Sprintf("line %d: ", line)+fmt.Sprintf(format, args...))
}

func (p *bundleParser) read(written string) {
	lines := strings.Split(strings.ReplaceAll(written, "\r\n", "\n"), "\n")
	for i, raw := range lines {
		n := i + 1
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if p.part == BundleSummary && len(p.summary) > 0 {
				p.summary = append(p.summary, "")
			}
			continue
		}
		indent := indentOf(line)
		if m := planHeading.FindStringSubmatch(trimmed); m != nil && indent == 0 {
			p.close()
			p.endCheck()
			p.heading(n, m[2])
			continue
		}
		switch p.part {
		case "":
			p.problem(n, "%q is before any heading: a bundle starts with ## %s", clipPlan(trimmed), BundleSummary)
		case BundleSummary:
			p.summary = append(p.summary, trimmed)
		default:
			m := planItem.FindStringSubmatch(trimmed)
			if m == nil {
				if indent > 0 && p.open != nil {
					p.open.text += " " + trimmed
					continue
				}
				p.close()
				if p.part == BundleNotVerified && isNothing(trimmed) {
					p.none = true
					continue
				}
				if p.part == BundleNoticed && isNothing(trimmed) {
					continue
				}
				p.problem(n, "%q isn't an item of a list: ## %s is a list", clipPlan(trimmed), p.part)
				continue
			}
			p.close()
			p.item(n, indent, m[1], m[2])
		}
	}
	p.close()
	p.endCheck()
	p.b.Summary = strings.TrimSpace(strings.Join(p.summary, "\n"))
	if !p.seen[BundleSummary] {
		p.problems = append(p.problems, fmt.Sprintf("it has no ## %s: say how it came out, in three sentences at most", BundleSummary))
	} else if p.b.Summary == "" {
		p.problems = append(p.problems, fmt.Sprintf("## %s says nothing: say how it came out, in three sentences at most", BundleSummary))
	} else if n := wordsIn(p.b.Summary); overLength(n, BundleSummaryWords) {
		p.problems = append(p.problems, fmt.Sprintf("## %s is %d words, and it's %d at most: say how it came out (what's unmet and what's wrong with it, or that every check is met, then what's left for the user), and leave the rest to the checks", BundleSummary, n, BundleSummaryWords))
	}
	if !p.seen[BundleChecks] {
		p.problems = append(p.problems, fmt.Sprintf("it has no ## %s: a numbered list, each check with what it came to", BundleChecks))
	}
	if !p.seen[BundleNotVerified] {
		p.problems = append(p.problems, fmt.Sprintf("it has no ## %s: list what you didn't verify and why, or say Nothing.", BundleNotVerified))
	} else if len(p.b.NotVerified) == 0 && !p.none {
		p.problems = append(p.problems, fmt.Sprintf("## %s says nothing: list what you didn't verify and why, or say Nothing.", BundleNotVerified))
	}
}

// indentOf is how far a line is indented, a tab as four spaces.
func indentOf(line string) int {
	n := 0
	for _, r := range line {
		switch r {
		case ' ':
			n++
		case '\t':
			n += 4
		default:
			return n
		}
	}
	return n
}

// wordsIn is how many words s is, as a bundle's lengths are counted.
func wordsIn(s string) int {
	return len(strings.Fields(s))
}

// overLength reports whether n words is too many where most is the
// length told: more than a fifth over.
func overLength(n, most int) bool {
	return n > most+most/5
}

// isNothing reports whether a part says there's nothing in it.
func isNothing(s string) bool {
	return isNone(s) || nothing(s)
}

func (p *bundleParser) heading(n int, title string) {
	part := bundlePartOf(title)
	switch {
	case part == "":
		p.part = "?"
		p.problem(n, "%q isn't a part of a bundle: its parts are ## %s, ## %s, ## %s, ## %s and ## %s", clipPlan(title), BundleSummary, BundleTryIt, BundleChecks, BundleNotVerified, BundleNoticed)
	case p.seen[part]:
		p.part = part
		p.problem(n, "## %s is there twice: once", part)
	default:
		p.part, p.seen[part] = part, true
	}
}

func bundlePartOf(title string) string {
	switch strings.ToLower(strings.Trim(strings.TrimSpace(title), "*_:. ")) {
	case "summary":
		return BundleSummary
	case "try it", "try":
		return BundleTryIt
	case "checks":
		return BundleChecks
	case "not verified", "unverified":
		return BundleNotVerified
	case "noticed", "also noticed":
		return BundleNoticed
	}
	return ""
}

// item starts an item of the part being read: something to try, a check,
// one of a check's fields, or something not verified or noticed.
func (p *bundleParser) item(n, indent int, number, text string) {
	switch p.part {
	case BundleTryIt:
		p.open = &bundleOpen{line: n, indent: indent, kind: "try-it", text: text}
	case BundleNotVerified:
		p.open = &bundleOpen{line: n, indent: indent, kind: "not-verified", text: text}
	case BundleNoticed:
		p.open = &bundleOpen{line: n, indent: indent, kind: "noticed", text: text}
	case BundleChecks:
		switch {
		case indent < 2 && number == "":
			p.endCheck()
			p.problem(n, "%q isn't numbered: a check is numbered (1. met: …), and what's under it is indented", clipPlan(text))
		case indent < 2:
			p.endCheck()
			p.open = &bundleOpen{line: n, indent: indent, kind: "check", number: number, text: text}
		case p.check == nil:
			p.problem(n, "%q is under no check: it goes under a numbered check, indented", clipPlan(text))
		case p.shows != nil && indent > p.showsAt:
			p.open = &bundleOpen{line: n, indent: indent, kind: "sub", text: text}
		default:
			p.shows = nil
			p.open = &bundleOpen{line: n, indent: indent, kind: "field", text: text}
		}
	}
}

// close reads the item that was open, now it's whole.
func (p *bundleParser) close() {
	it := p.open
	p.open = nil
	if it == nil {
		return
	}
	text := strings.TrimSpace(it.text)
	switch it.kind {
	case "try-it":
		if t, ok := p.try(it.line, text, false); ok {
			p.b.Try = append(p.b.Try, t)
		}
	case "not-verified":
		if isNothing(text) {
			p.none = true
			return
		}
		p.itemLength(it.line, BundleNotVerified, text)
		p.b.NotVerified = append(p.b.NotVerified, text)
	case "noticed":
		if isNothing(text) {
			return
		}
		p.itemLength(it.line, BundleNoticed, text)
		p.b.Noticed = append(p.b.Noticed, text)
	case "check":
		n, _ := strconv.Atoi(it.number)
		c := BundleCheck{N: n, line: it.line}
		m := bundleVerdict.FindStringSubmatch(text)
		if m == nil {
			p.problem(it.line, "check %d doesn't say what it came to: it starts with met:, unmet: or can't tell: (or yours:, for a check that's the user's) (%q)", n, clipPlan(text))
		} else {
			c.Verdict, c.Said = bundleVerdictOf(m[1]), strings.TrimSpace(m[2])
		}
		p.b.Checks = append(p.b.Checks, c)
		p.check = &p.b.Checks[len(p.b.Checks)-1]
	case "field":
		p.field(it, text)
	case "sub":
		p.sub(it, text)
	}
}

// itemLength holds an item of Not verified or Noticed to a line.
func (p *bundleParser) itemLength(line int, part, text string) {
	if n := wordsIn(text); overLength(n, BundleItemWords) {
		p.problem(line, "this item of ## %s is %d words, and one is %d at most: a line saying what, and why", part, n, BundleItemWords)
	}
}

func bundleVerdictOf(s string) string {
	switch s = strings.ToLower(s); {
	case s == "met":
		return CheckMet
	case s == "unmet" || s == "not met":
		return CheckUnmet
	case s == "yours":
		return CheckYours
	}
	return CheckCantTell
}

// try reads something to try: one command in backticks, or words, or
// (look for:) what to look for. A command stands alone, so that it's
// copied once: a second one after it, or words, is refused, with where
// they go instead (under a check, which takes no look for:, that's a try
// of their own, or Try it).
func (p *bundleParser) try(line int, text string, check bool) (BundleTry, bool) {
	t := BundleTry{}
	if m := bundleKey.FindStringSubmatch(text); m != nil {
		switch strings.ToLower(m[1]) {
		case "try":
			text = strings.TrimSpace(m[2])
		case "look for":
			text, t.Look = strings.TrimSpace(m[2]), true
		}
	}
	if text == "" {
		p.problem(line, "it says nothing to try: say what to do, with the command first in backticks where there is one")
		return t, false
	}
	t.Text = text
	m := bundleTicked.FindStringSubmatch(text)
	if m == nil || t.Look {
		return t, true
	}
	t.Command = strings.TrimSpace(m[1])
	rest := strings.Trim(m[2], ".,;: ")
	if rest == "" {
		t.Text = "`" + m[1] + "`"
		return t, true
	}
	fix := "chain what's to run with && into one (`a && cd x && b`), and move the rest to - look for: items, one for each thing to do or see"
	if check {
		fix = "chain what's to run with && into one (`a && cd x && b`), and move the rest to a - try: of its own, in words, or to - look for: items under ## " + BundleTryIt
	}
	if second := bundleSpan.FindString(rest); second != "" {
		p.problem(line, "try: has a second command in backticks (%s) after `%s`: a try: is one command, copied once, so %s", second, t.Command, fix)
	} else {
		p.problem(line, "try: says more after its command (%q): a try: is one command, copied once, with nothing after it, so %s", clipPlan(rest), fix)
	}
	return t, false
}

// field reads one of a check's fields.
func (p *bundleParser) field(it *bundleOpen, text string) {
	m := bundleKey.FindStringSubmatch(text)
	key, value := "", ""
	if m != nil {
		key, value = strings.ToLower(m[1]), strings.TrimSpace(m[2])
	}
	switch key {
	case "shows":
		e, ok := p.evidence(it.line, "shows", value)
		if !ok {
			return
		}
		p.check.Shows = append(p.check.Shows, e)
		p.shows, p.showsAt = &p.check.Shows[len(p.check.Shows)-1], it.indent
	case "point":
		if value == "" {
			p.problem(it.line, "point: says nothing: one of a few facts of the same kind, starting with what it's of")
			return
		}
		p.check.Points = append(p.check.Points, value)
	case "lines":
		if value = strings.Trim(value, "` "); value == "" {
			p.problem(it.line, "lines: says nothing: where in the capsule it's done, as `path:from-to`")
			return
		}
		p.check.Lines = append(p.check.Lines, value)
	case "try":
		if t, ok := p.try(it.line, value, true); ok {
			p.check.Try = append(p.check.Try, t)
		}
	case "ask":
		if value == "" {
			p.problem(it.line, "ask: says nothing: a prompt the user could send on to crew as it's written")
			return
		}
		p.check.Asks = append(p.check.Asks, value)
	case "before", "made by":
		p.problem(it.line, "%s: goes under the shows: it's of, indented further than it", key)
	default:
		p.problem(it.line, "%q isn't something a check takes: under a check go - point:, - shows:, - lines:, - try: and - ask:", clipPlan(text))
	}
}

// sub reads what's under a shows: the same before, or what made it.
func (p *bundleParser) sub(it *bundleOpen, text string) {
	m := bundleKey.FindStringSubmatch(text)
	key, value := "", ""
	if m != nil {
		key, value = strings.ToLower(m[1]), strings.TrimSpace(m[2])
	}
	switch key {
	case "before":
		if p.shows.Before != nil {
			p.problem(it.line, "before: is there twice for %s: once", p.shows.Path)
			return
		}
		if e, ok := p.evidence(it.line, "before", value); ok {
			p.shows.Before = &e
		}
	case "made by":
		if value = strings.Trim(value, "` "); value == "" {
			p.problem(it.line, "made by: says nothing: the command that made it, in backticks")
			return
		}
		p.shows.MadeBy = value
	default:
		p.problem(it.line, "%q isn't something a shows: takes: under it go - before: and - made by:", clipPlan(text))
	}
}

// evidence reads a file a check shows: its path in backticks, then what
// it shows.
func (p *bundleParser) evidence(line int, key, value string) (Evidence, bool) {
	m := bundleTicked.FindStringSubmatch(value)
	if m == nil {
		p.problem(line, "%s: %q doesn't name a file: its path goes first, in backticks, then what it shows", key, clipPlan(value))
		return Evidence{}, false
	}
	e := Evidence{Path: strings.TrimSpace(m[1]), Caption: strings.TrimSpace(strings.TrimLeft(m[2], ":-–— ")), line: line}
	if p.files != nil {
		media, size, why := p.files(e.Path)
		if why != "" {
			p.problem(line, "%s: %s: %s", key, e.Path, why)
			return Evidence{}, false
		}
		e.Media, e.Size = media, size
	}
	return e, true
}

func (p *bundleParser) endCheck() {
	p.check, p.shows = nil, nil
}

// hold holds the bundle's checks to its step's: each of the step's checks
// in words there once, by its number, or, for a step with none, the
// bundle's own in order. And each to what its verdict needs.
func (p *bundleParser) hold(checks []Check) {
	if p.seen[BundleChecks] && len(p.b.Checks) == 0 {
		p.problems = append(p.problems, fmt.Sprintf("## %s has no checks: a numbered list, each with what it came to", BundleChecks))
	}
	seen := map[int]bool{}
	for i := range p.b.Checks {
		c := &p.b.Checks[i]
		switch {
		case len(checks) == 0:
			if c.N != i+1 {
				p.problem(c.line, "check %d is out of order: your step has no checks, so these are your own, numbered 1, 2, 3… in order (this is number %d)", c.N, i+1)
			}
		case c.N < 1 || c.N > len(checks):
			p.problem(c.line, "check %d isn't one of your step's: it has %d (ws show s<n> lists them)", c.N, len(checks))
			continue
		case seen[c.N]:
			p.problem(c.line, "check %d is there twice: once", c.N)
			continue
		case checks[c.N-1].Run():
			p.problem(c.line, "check %d is a command (%s), which ws runs itself: leave it out", c.N, clipPlan(checks[c.N-1].Text))
			continue
		default:
			c.Check = checks[c.N-1].Text
		}
		seen[c.N] = true
		if c.Verdict == "" {
			continue // said already
		}
		yours := c.N >= 1 && c.N <= len(checks) && checks[c.N-1].Yours
		switch {
		case yours && c.Verdict != CheckYours:
			p.problem(c.line, "check %d is the user's to settle, not yours: say yours:, with what you've left to help them (a - shows:, a - try:)", c.N)
			continue
		case !yours && c.Verdict == CheckYours:
			p.problem(c.line, "check %d is yours to settle: met:, unmet: or can't tell: (yours: is for a check the plan keeps for the user)", c.N)
			continue
		}
		if c.Said == "" {
			p.problem(c.line, "check %d is %s, but not how: %s", c.N, c.Verdict, bundleSays(c.Verdict))
		}
		if n, most := wordsIn(c.Found()), bundleCheckWords(c.Verdict); overLength(n, most) {
			p.problem(c.line, "check %d is %d words, and one that's %s is %d at most: %s", c.N, n, c.Verdict, most, bundleShorter(c.Verdict))
		}
		if c.Verdict == CheckMet && len(c.Shows) == 0 && len(c.Lines) == 0 {
			p.problem(c.line, "check %d is met, with nothing that shows it: give it a - shows: (an image, a video, or what a command printed) or the - lines: it's done in", c.N)
		}
	}
	for i, c := range checks {
		if !c.Run() && !seen[i+1] && p.seen[BundleChecks] {
			p.problems = append(p.problems, fmt.Sprintf("check %d of your step isn't there (%s): %d. met:, unmet: or can't tell:, with what you found", i+1, clipPlan(c.Text), i+1))
		}
	}
}

// bundleSays is what a verdict has to say.
func bundleSays(verdict string) string {
	switch verdict {
	case CheckMet:
		return "say what you saw"
	case CheckUnmet:
		return "say what's missing or wrong"
	case CheckYours:
		return "say what you've left to help them settle it"
	}
	return "say what would show it"
}

// bundleCheckWords is how many words a check that came to verdict may be.
func bundleCheckWords(verdict string) int {
	if verdict == CheckMet {
		return BundleMetWords
	}
	return BundleCheckWords
}

// bundleShorter is how a check that's over length is made shorter.
func bundleShorter(verdict string) string {
	file := "what you counted and timed goes in a text file it shows:, and what another check says isn't said again"
	switch verdict {
	case CheckMet:
		return "say what you did and saw, once; " + file
	case CheckUnmet:
		return "say what's wrong, then where it shows, and give what holds a clause; " + file
	case CheckYours:
		return "say what you've left to help them and what there is to weigh; " + file
	}
	return "say what would show it; " + file
}

// Found is what the check found, whole and in a line: what it said, then
// its points.
func (c BundleCheck) Found() string {
	if len(c.Points) == 0 {
		return c.Said
	}
	return strings.TrimSpace(c.Said + " " + strings.Join(c.Points, "; "))
}

// With is the bundle's checks that came to verdict.
func (b Bundle) With(verdict string) []BundleCheck {
	var out []BundleCheck
	for _, c := range b.Checks {
		if c.Verdict == verdict {
			out = append(out, c)
		}
	}
	return out
}

// Tally is the bundle's checks in a few words: "2 met, 1 can't tell".
func (b Bundle) Tally() string {
	var parts []string
	for _, v := range []string{CheckMet, CheckUnmet, CheckCantTell, CheckYours} {
		if n := len(b.With(v)); n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, v))
		}
	}
	return strings.Join(parts, ", ")
}

// Files are the bundle's files, each once: what it shows, and the befores
// beside them.
func (b *Bundle) Files() []*Evidence {
	var out []*Evidence
	for i := range b.Checks {
		for j := range b.Checks[i].Shows {
			e := &b.Checks[i].Shows[j]
			out = append(out, e)
			if e.Before != nil {
				out = append(out, e.Before)
			}
		}
	}
	return out
}

// IsBundleRef reports whether ref is a bundle's: b3.
func IsBundleRef(ref string) bool { return bundleRef.MatchString(ref) }

var bundleRef = regexp.MustCompile(`^b[1-9][0-9]*$`)

// Bundle is the mission's bundle with id.
func (m *Mission) Bundle(id string) (Bundle, bool) {
	for _, b := range m.Bundles {
		if b.ID == id {
			return b, true
		}
	}
	return Bundle{}, false
}

// ReplacedBy is the bundle that took b's place, if one has.
func (m *Mission) ReplacedBy(id string) (Bundle, bool) {
	for _, b := range m.Bundles {
		if b.Replaces == id {
			return b, true
		}
	}
	return Bundle{}, false
}

// StandingBundles are the mission's bundles of the step with this text
// that nothing has taken the place of, oldest first: one for each capsule
// the step ran in.
func (m *Mission) StandingBundles(stage string) []Bundle {
	var out []Bundle
	for _, b := range m.Bundles {
		if _, replaced := m.ReplacedBy(b.ID); b.Stage == stage && stage != "" && !replaced {
			out = append(out, b)
		}
	}
	return out
}

// SetBundleVerdict records your verdict of a bundle: approved, or sent
// back with what you said wasn't right. It's logged.
func (s Store) SetBundleVerdict(slug, id, verdict string, notes []BundleNote) (Bundle, error) {
	var out Bundle
	if _, err := s.Update(slug, func(m *Mission) error {
		for i := range m.Bundles {
			if m.Bundles[i].ID == id {
				at := now()
				m.Bundles[i].Verdict, m.Bundles[i].VerdictAt, m.Bundles[i].Notes = verdict, &at, notes
				out = m.Bundles[i]
				return nil
			}
		}
		return fmt.Errorf("mission %s has no bundle %s", slug, id)
	}); err != nil {
		return Bundle{}, err
	}
	text := id + " approved"
	if verdict == BundleSentBack {
		var said []string
		for _, n := range notes {
			if n.Check > 0 {
				said = append(said, fmt.Sprintf("check %d: %s", n.Check, n.Text))
			} else {
				said = append(said, n.Text)
			}
		}
		text = id + " sent back: " + strings.Join(said, "; ")
	}
	repo, capsule, _ := strings.Cut(out.Capsule, "/")
	return out, s.AppendLog(slug, Event{Kind: EventBundle, Crew: "you", Repo: repo, Capsule: capsule, Text: text, What: verdict})
}

// MarkBundleLooked marks one of a bundle's checks, by its key in the
// bundle's list, as looked at by you, or not.
func (s Store) MarkBundleLooked(slug, id, key string, looked bool) error {
	_, err := s.Update(slug, func(m *Mission) error {
		for i := range m.Bundles {
			if b := &m.Bundles[i]; b.ID == id {
				at := -1
				for j, k := range b.Looked {
					if k == key {
						at = j
					}
				}
				switch {
				case looked && at < 0:
					b.Looked = append(b.Looked, key)
				case !looked && at >= 0:
					b.Looked = append(b.Looked[:at], b.Looked[at+1:]...)
				}
				return nil
			}
		}
		return fmt.Errorf("mission %s has no bundle %s", slug, id)
	})
	return err
}

// BundlesDir is where a mission's bundles keep their files, a folder
// each.
func (s Store) BundlesDir(slug string) string { return filepath.Join(s.missionDir(slug), "bundles") }

// BundleDir is the folder a bundle's files are kept in.
func (s Store) BundleDir(slug, id string) string { return filepath.Join(s.BundlesDir(slug), id) }

// KeepBundle keeps a bundle on its mission: a folder of its own is made,
// numbered after the mission's last, files fills it (and names each file
// it kept on the bundle), and then its record is written, saying which
// bundle of its step and capsule it takes the place of. The files go
// first, then the record, so a record never names a file that isn't
// there; a folder with no record is litter.
func (s Store) KeepBundle(slug string, b Bundle, files func(dir string, b *Bundle) error) (Bundle, error) {
	m, err := s.Load(slug)
	if err != nil {
		return Bundle{}, err
	}
	if err := os.MkdirAll(s.BundlesDir(slug), 0o755); err != nil {
		return Bundle{}, err
	}
	dir := ""
	for n := len(m.Bundles) + 1; ; n++ {
		b.ID = "b" + strconv.Itoa(n)
		dir = s.BundleDir(slug, b.ID)
		err := os.Mkdir(dir, 0o755)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrExist) {
			return Bundle{}, err
		}
	}
	if files != nil {
		if err := files(dir, &b); err != nil {
			os.RemoveAll(dir)
			return Bundle{}, err
		}
	}
	if b.At.IsZero() {
		b.At = now()
	}
	if _, err := s.Update(slug, func(m *Mission) error {
		if _, taken := m.Bundle(b.ID); taken {
			return fmt.Errorf("bundle %s was kept meanwhile: hand it over again", b.ID)
		}
		b.Replaces = ""
		for i := len(m.Bundles) - 1; i >= 0 && b.Stage != ""; i-- {
			if was := m.Bundles[i]; was.Stage == b.Stage && was.Capsule == b.Capsule {
				b.Replaces = was.ID
				break
			}
		}
		m.Bundles = append(m.Bundles, b)
		return nil
	}); err != nil {
		os.RemoveAll(dir)
		return Bundle{}, err
	}
	text := b.ID
	if b.Step > 0 {
		text += fmt.Sprintf(" for step %d", b.Step)
	}
	text += ": " + b.Tally()
	if b.Replaces != "" {
		text += ", in place of " + b.Replaces
	}
	repo, capsule, _ := strings.Cut(b.Capsule, "/")
	return b, s.AppendLog(slug, Event{Kind: EventBundle, Crew: b.Crew, Repo: repo, Capsule: capsule, Text: text})
}
