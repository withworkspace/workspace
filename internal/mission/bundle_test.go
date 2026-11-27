package mission

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const bundleWritten = `## Summary
The picker is on Settings. Picking a colour applies at once
in both themes.

It survives a reload.

## Try it
- try: ` + "`npm install && cd web && npm run dev`" + `
- look for: Appearance, on http://localhost:5173/settings

## Checks
1. met: picked teal, then rose; tabs and buttons changed
   at once.
   - shows: ` + "`pick.webm`" + ` Picking three colours in a row
     - made by: ` + "`npm run shots -- accent --out pick.webm`" + `
   - shows: ` + "`light.png`" + ` Light, after picking teal
     - before: ` + "`light-ground.png`" + `
   - lines: ` + "`web/src/Appearance.tsx:41-88`" + `
   - try: ` + "`npm run shots -- accent --headed`" + `
3. can't tell: I could only drive one tab.
   - try: open Settings in two tabs
   - ask: Add a test for the storage event

## Not verified
- Safari: nothing here to run it in.

## Noticed
- The swatches' names are cut off
  at phone width.
`

func stepChecks(t *testing.T) []Check {
	t.Helper()
	var out []Check
	for _, text := range []string{"a pick applies at once, light and dark", "`npm test` passes", "a pick reaches other open tabs"} {
		c, why := parseCheck(text)
		if why != "" {
			t.Fatal(why)
		}
		out = append(out, c)
	}
	return out
}

// A bundle is read part by part: its summary, what to try, each of its
// step's checks in words with what it came to and what shows it, and what
// wasn't verified.
func TestParseBundle(t *testing.T) {
	b, err := ParseBundle(bundleWritten, stepChecks(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := "The picker is on Settings. Picking a colour applies at once\nin both themes.\n\nIt survives a reload."; b.Summary != want {
		t.Errorf("summary = %q", b.Summary)
	}
	if len(b.Try) != 2 || b.Try[0].Command != "npm install && cd web && npm run dev" || b.Try[0].Text != "`npm install && cd web && npm run dev`" || b.Try[0].Look || !b.Try[1].Look || b.Try[1].Command != "" {
		t.Errorf("try = %+v", b.Try)
	}
	if len(b.Checks) != 2 {
		t.Fatalf("checks = %+v", b.Checks)
	}
	one := b.Checks[0]
	if one.N != 1 || one.Verdict != CheckMet || one.Check != "a pick applies at once, light and dark" || one.Said != "picked teal, then rose; tabs and buttons changed at once." {
		t.Errorf("check 1 = %+v", one)
	}
	if len(one.Shows) != 2 || one.Shows[0].Path != "pick.webm" || one.Shows[0].Caption != "Picking three colours in a row" || one.Shows[0].MadeBy != "npm run shots -- accent --out pick.webm" {
		t.Errorf("check 1 shows = %+v", one.Shows)
	}
	if one.Shows[1].Before == nil || one.Shows[1].Before.Path != "light-ground.png" || one.Shows[0].Before != nil {
		t.Errorf("before = %+v", one.Shows)
	}
	if len(one.Lines) != 1 || one.Lines[0] != "web/src/Appearance.tsx:41-88" || len(one.Try) != 1 || one.Try[0].Command != "npm run shots -- accent --headed" {
		t.Errorf("check 1 lines, try = %v %+v", one.Lines, one.Try)
	}
	three := b.Checks[1]
	if three.N != 3 || three.Verdict != CheckCantTell || len(three.Asks) != 1 || three.Try[0].Command != "" {
		t.Errorf("check 3 = %+v", three)
	}
	if len(b.NotVerified) != 1 || b.Tally() != "1 met, 1 can't tell" {
		t.Errorf("not verified %v, tally %q", b.NotVerified, b.Tally())
	}
	if len(b.Noticed) != 1 || b.Noticed[0] != "The swatches' names are cut off at phone width." {
		t.Errorf("noticed = %q", b.Noticed)
	}
	if files := b.Files(); len(files) != 3 {
		t.Errorf("files = %d", len(files))
	}
}

// Where the step has no checks, the bundle's are its own, in order.
func TestParseBundle_ItsOwnChecks(t *testing.T) {
	written := "## Summary\nIt works.\n## Checks\n1. met: the page shows the picker\n   - shows: `a.png`\n2. unmet: dark keeps the old accent\n## Not verified\nNothing.\n"
	b, err := ParseBundle(written, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Checks) != 2 || b.Checks[0].Check != "" || b.Checks[1].Verdict != CheckUnmet || len(b.NotVerified) != 0 {
		t.Errorf("bundle = %+v", b)
	}
}

// A bundle that isn't right is refused, with every problem and its line.
func TestParseBundle_Refused(t *testing.T) {
	cases := []struct {
		name, written string
		checks        []Check
		want          []string
	}{
		{"no parts", "hello\n", nil, []string{"line 1: \"hello\" is before any heading", "no ## Summary", "no ## Checks", "no ## Not verified"}},
		{"a part it hasn't", "## Summary\nx\n## Proof\n- a\n## Checks\n1. unmet: y\n## Not verified\nNothing.\n", nil, []string{"line 3: \"Proof\" isn't a part of a bundle"}},
		{"twice", "## Summary\nx\n## Summary\ny\n## Checks\n1. unmet: y\n## Not verified\nNothing.\n", nil, []string{"line 3: ## Summary is there twice"}},
		{"no verdict", "## Summary\nx\n## Checks\n1. picked teal\n## Not verified\nNothing.\n", nil, []string{"line 4: check 1 doesn't say what it came to"}},
		{"met with nothing", "## Summary\nx\n## Checks\n1. met: it does\n## Not verified\nNothing.\n", nil, []string{"line 4: check 1 is met, with nothing that shows it"}},
		{"met, not how", "## Summary\nx\n## Checks\n1. met:\n   - lines: `a.go:1-2`\n## Not verified\nNothing.\n", nil, []string{"line 4: check 1 is met, but not how: say what you saw"}},
		{"out of order", "## Summary\nx\n## Checks\n2. unmet: y\n## Not verified\nNothing.\n", nil, []string{"line 4: check 2 is out of order"}},
		{"not numbered", "## Summary\nx\n## Checks\n- unmet: y\n## Not verified\nNothing.\n", nil, []string{"line 4: \"unmet: y\" isn't numbered", "## Checks has no checks"}},
		{"a field it hasn't", "## Summary\nx\n## Checks\n1. unmet: y\n   - proof: `a.png`\n## Not verified\nNothing.\n", nil, []string{"line 5: \"proof: `a.png`\" isn't something a check takes"}},
		{"no file named", "## Summary\nx\n## Checks\n1. unmet: y\n   - shows: a screenshot\n## Not verified\nNothing.\n", nil, []string{"line 5: shows: \"a screenshot\" doesn't name a file"}},
		{"before under no shows", "## Summary\nx\n## Checks\n1. unmet: y\n   - before: `a.png`\n## Not verified\nNothing.\n", nil, []string{"line 5: before: goes under the shows: it's of"}},
		{"under a shows", "## Summary\nx\n## Checks\n1. unmet: y\n   - shows: `a.png`\n     - lines: `a.go:1`\n## Not verified\nNothing.\n", nil, []string{"line 6: \"lines: `a.go:1`\" isn't something a shows: takes"}},
		{"not verified says nothing", "## Summary\nx\n## Checks\n1. unmet: y\n## Not verified\n", nil, []string{"## Not verified says nothing"}},
		{"a step's check missing", "## Summary\nx\n## Checks\n1. unmet: y\n## Not verified\nNothing.\n", stepChecks(t), []string{"check 3 of your step isn't there (a pick reaches other open tabs)"}},
		{"a command's", "## Summary\nx\n## Checks\n1. unmet: y\n2. unmet: fails\n3. unmet: z\n## Not verified\nNothing.\n", stepChecks(t), []string{"line 5: check 2 is a command (`npm test` passes), which ws runs itself"}},
		{"not the step's", "## Summary\nx\n## Checks\n1. unmet: y\n3. unmet: z\n4. unmet: q\n## Not verified\nNothing.\n", stepChecks(t), []string{"line 6: check 4 isn't one of your step's: it has 3"}},
		{"twice over", "## Summary\nx\n## Checks\n1. unmet: y\n1. unmet: z\n3. unmet: q\n## Not verified\nNothing.\n", stepChecks(t), []string{"line 5: check 1 is there twice"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseBundle(c.written, c.checks, nil)
			if err == nil || !IsMalformedBundle(err) {
				t.Fatalf("err = %v", err)
			}
			for _, want := range c.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("want %q in:\n%s", want, err)
				}
			}
		})
	}
}

// A try is one command, copied once: one with a second command after it,
// or words, is refused, saying to chain what's run with && and where the
// rest goes. Under a check, which takes no look for:, that's a try of its
// own or Try it.
func TestParseBundle_TryIsOneCommand(t *testing.T) {
	in := func(tryIt, underCheck string) string {
		s := "## Summary\nx\n"
		if tryIt != "" {
			s += "## Try it\n" + tryIt + "\n"
		}
		s += "## Checks\n1. unmet: y\n"
		if underCheck != "" {
			s += "   " + underCheck + "\n"
		}
		return s + "## Not verified\nNothing.\n"
	}
	const chain, look = "chain what's to run with && into one (`a && cd x && b`)", "move the rest to - look for: items, one for each thing to do or see"
	refused := []struct {
		name, written string
		want          []string
	}{
		{"a second command", in("- try: `scripts/setup.sh ~/demo`, then `cd ~/demo && bin/app web`", ""),
			[]string{"line 4: try: has a second command in backticks (`cd ~/demo && bin/app web`) after `scripts/setup.sh ~/demo`", chain, look}},
		{"two commands, nothing between", in("- try: `just build` `bin/app web`", ""),
			[]string{"line 4: try: has a second command in backticks (`bin/app web`)", chain, look}},
		{"words after its command", in("- try: `just dev`, then open http://localhost:5173/settings", ""),
			[]string{"line 4: try: says more after its command (\"then open http://localhost:5173/settings\")", chain, look}},
		{"words after it, on the next line", in("- try: `just dev`\n  in Find (⌘K) type %greet", ""),
			[]string{"line 4: try: says more after its command (\"in Find (⌘K) type %greet\")", chain, look}},
		{"with no try:", in("- `just dev` and open the page", ""),
			[]string{"line 4: try: says more after its command (\"and open the page\")", chain, look}},
		{"under a check", in("", "- try: `just dev`, then open the page"),
			[]string{"line 5: try: says more after its command (\"then open the page\")", chain, "move the rest to a - try: of its own, in words, or to - look for: items under ## Try it"}},
		{"a second command, under a check", in("", "- try: `just build`, then `bin/app web`"),
			[]string{"line 5: try: has a second command in backticks (`bin/app web`) after `just build`", chain, "- look for: items under ## Try it"}},
	}
	for _, c := range refused {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseBundle(c.written, nil, nil)
			if err == nil || !IsMalformedBundle(err) {
				t.Fatalf("err = %v", err)
			}
			for _, want := range c.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("want %q in:\n%s", want, err)
				}
			}
		})
	}

	// What was right before still is: one command (chained or not), words
	// only, though they name something in backticks, and what to look for,
	// whatever it starts with.
	b, err := ParseBundle(in("- try: `just dev`\n- try: `scripts/setup.sh ~/demo && cd ~/demo && bin/app web`\n- try: `just dev`.\n"+
		"- try: open Settings in two tabs\n- try: open Settings, then press `Save` and `Reset`\n- look for: the row of swatches under Accent\n- look for: `teal` then `rose`, in the footer",
		"- try: `npm test`\n   - try: open the page twice"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []BundleTry{
		{Text: "`just dev`", Command: "just dev"},
		{Text: "`scripts/setup.sh ~/demo && cd ~/demo && bin/app web`", Command: "scripts/setup.sh ~/demo && cd ~/demo && bin/app web"},
		{Text: "`just dev`", Command: "just dev"},
		{Text: "open Settings in two tabs"},
		{Text: "open Settings, then press `Save` and `Reset`"},
		{Text: "the row of swatches under Accent", Look: true},
		{Text: "`teal` then `rose`, in the footer", Look: true},
	}
	if len(b.Try) != len(want) {
		t.Fatalf("try = %+v", b.Try)
	}
	for i, w := range want {
		if b.Try[i] != w {
			t.Errorf("try %d = %+v, want %+v", i+1, b.Try[i], w)
		}
	}
	if under := b.Checks[0].Try; len(under) != 2 || under[0].Command != "npm test" || under[1].Command != "" || under[1].Text != "open the page twice" {
		t.Errorf("under the check = %+v", under)
	}
}

// What crew are told is what's held to: the format's own example is read
// as it's written, and its Try it is one chained command, then what to
// look for.
func TestBundleFormat_ItsExample(t *testing.T) {
	_, after, ok := strings.Cut(BundleFormat, "```markdown\n")
	example, _, closed := strings.Cut(after, "```\n")
	if !ok || !closed {
		t.Fatal("BundleFormat has no example")
	}
	b, err := ParseBundle(example, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Try) != 3 || b.Try[0].Command != "npm install && cd web && npm run dev" || b.Try[0].Text != "`"+b.Try[0].Command+"`" || !b.Try[1].Look || !b.Try[2].Look {
		t.Errorf("try = %+v", b.Try)
	}
	if len(b.Checks) != 3 || len(b.Checks[0].Try) != 1 || b.Checks[0].Try[0].Command == "" || b.Checks[1].Try[0].Command != "" || b.Checks[2].Verdict != CheckUnmet {
		t.Errorf("checks = %+v", b.Checks)
	}
	if len(b.Noticed) != 1 || len(b.Checks[2].Points) != 2 || len(b.Checks[2].Asks) != 1 {
		t.Errorf("noticed = %q, check 3 = %+v", b.Noticed, b.Checks[2])
	}
	for _, says := range []string{"one command in backticks, with nothing after it", "chain it with `&&` (`a && cd x && b`)", "`look for:` items, one each"} {
		if !strings.Contains(BundleFormat, says) {
			t.Errorf("BundleFormat doesn't say %q", says)
		}
	}
}

// A bundle is held to a length, so that what's to act on isn't lost in
// what was measured: its summary, each check by what it came to, and each
// item of Not verified and Noticed. One a little over what crew are told
// is let by, since an agent can't count its words as it writes; one well
// over is refused, saying where the rest goes.
func TestParseBundle_Lengths(t *testing.T) {
	words := func(n int) string { return strings.TrimSpace(strings.Repeat("word ", n)) }
	written := func(summary, met, unmet, not, noticed int) string {
		return "## Summary\n" + words(summary) + "\n## Checks\n1. met: " + words(met) + "\n   - shows: `a.png`\n2. unmet: " + words(unmet) +
			"\n## Not verified\n- " + words(not) + "\n## Noticed\n- " + words(noticed) + "\n"
	}
	if _, err := ParseBundle(written(BundleSummaryWords, BundleMetWords, BundleCheckWords, BundleItemWords, BundleItemWords), nil, nil); err != nil {
		t.Errorf("at length: %v", err)
	}
	if _, err := ParseBundle(written(BundleSummaryWords+10, BundleMetWords+10, BundleCheckWords+10, BundleItemWords+10, BundleItemWords+10), nil, nil); err != nil {
		t.Errorf("a little over: %v", err)
	}
	_, err := ParseBundle(written(200, 120, 300, 90, 80), nil, nil)
	if err == nil || !IsMalformedBundle(err) {
		t.Fatalf("err = %v", err)
	}
	for _, want := range []string{
		"## Summary is 200 words, and it's 80 at most: say how it came out",
		"line 4: check 1 is 120 words, and one that's met is 50 at most: say what you did and saw, once; what you counted and timed goes in a text file it shows:",
		"line 6: check 2 is 300 words, and one that's unmet is 100 at most: say what's wrong, then where it shows",
		"line 8: this item of ## Not verified is 90 words, and one is 50 at most",
		"line 10: this item of ## Noticed is 80 words, and one is 50 at most",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("want %q in:\n%s", want, err)
		}
	}
}

// The lengths crew are told are the ones a bundle is held to.
func TestBundleFormat_ItsLengths(t *testing.T) {
	for _, says := range []string{
		fmt.Sprintf("three sentences at most (%d words)", BundleSummaryWords),
		fmt.Sprintf("what you saw, once (%d words at most)", BundleMetWords),
		fmt.Sprintf("whoever puts it right (%d words at most)", BundleCheckWords),
		fmt.Sprintf("say what would (%d words at most)", BundleCheckWords),
		fmt.Sprintf("what there is to weigh (%d words at most)", BundleCheckWords),
		fmt.Sprintf("and why, a line each (%d words at most)", BundleItemWords),
		fmt.Sprintf("older than it), a line each (%d words at most)", BundleItemWords),
	} {
		if !strings.Contains(BundleFormat, says) {
			t.Errorf("BundleFormat doesn't say %q", says)
		}
	}
}

// A check takes a few points: facts of the same kind, kept as a list
// under what it said. They count in its length, and are part of what it
// found, as the crew the work goes back to are told it.
func TestParseBundle_Points(t *testing.T) {
	written := "## Summary\nx\n## Checks\n1. unmet: the colour is lost on a reload.\n   - point: Chrome: 5 of 5.\n   - point: Firefox: 2 of 5,\n     in dark mode.\n   - shows: `a.png`\n## Not verified\nNothing.\n"
	b, err := ParseBundle(written, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	c := b.Checks[0]
	if len(c.Points) != 2 || c.Points[1] != "Firefox: 2 of 5, in dark mode." || len(c.Shows) != 1 {
		t.Errorf("check = %+v", c)
	}
	if want := "the colour is lost on a reload. Chrome: 5 of 5.; Firefox: 2 of 5, in dark mode."; c.Found() != want {
		t.Errorf("found = %q", c.Found())
	}
	long := "## Summary\nx\n## Checks\n1. unmet: " + strings.TrimSpace(strings.Repeat("word ", 80)) + "\n   - point: " + strings.TrimSpace(strings.Repeat("word ", 60)) + "\n   - point:\n## Not verified\nNothing.\n"
	_, err = ParseBundle(long, nil, nil)
	if err == nil {
		t.Fatal("a check over length with its points, and an empty point, were taken")
	}
	for _, want := range []string{"line 4: check 1 is 140 words, and one that's unmet is 100 at most", "line 6: point: says nothing"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("want %q in:\n%s", want, err)
		}
	}
}

// Noticed is optional, and may say there's nothing.
func TestParseBundle_Noticed(t *testing.T) {
	for _, noticed := range []string{"", "## Noticed\nNothing.\n", "## Noticed\n- Nothing.\n"} {
		b, err := ParseBundle("## Summary\nx\n## Checks\n1. unmet: y\n## Not verified\nNothing.\n"+noticed, nil, nil)
		if err != nil || len(b.Noticed) != 0 {
			t.Errorf("%q: %v, %q", noticed, err, b.Noticed)
		}
	}
	if _, err := ParseBundle("## Summary\nx\n## Checks\n1. unmet: y\n## Not verified\nNothing.\n## Noticed\nthe label is cut off\n", nil, nil); err == nil || !strings.Contains(err.Error(), "isn't an item of a list: ## Noticed is a list") {
		t.Errorf("err = %v", err)
	}
}

// A file the bundle names is looked at as it's read: one that can't be
// kept is a problem on its line, with why.
func TestParseBundle_Files(t *testing.T) {
	files := func(path string) (string, int64, string) {
		if path == "gone.png" {
			return "", 0, "there's no such file"
		}
		return "image/png", 12, ""
	}
	written := "## Summary\nx\n## Checks\n1. met: y\n   - shows: `a.png` it\n     - before: `gone.png`\n## Not verified\nNothing.\n"
	_, err := ParseBundle(written, nil, files)
	if err == nil || !strings.Contains(err.Error(), "line 6: before: gone.png: there's no such file") {
		t.Fatalf("err = %v", err)
	}
	b, err := ParseBundle(strings.Replace(written, "gone.png", "b.png", 1), nil, files)
	if err != nil {
		t.Fatal(err)
	}
	if e := b.Checks[0].Shows[0]; e.Media != "image/png" || e.Size != 12 || e.Before.Media != "image/png" {
		t.Errorf("evidence = %+v", e)
	}
}

// A bundle kept has a folder of its own, filled before its record's
// written, a ref numbered in its mission, and takes the place of the last
// of its step and capsule.
func TestKeepBundle(t *testing.T) {
	s := owned(t)
	fixedNow(t)
	m, err := s.Create("Accent picker")
	if err != nil {
		t.Fatal(err)
	}
	fill := func(dir string, b *Bundle) error {
		return os.WriteFile(filepath.Join(dir, "a.png"), []byte("png"), 0o644)
	}
	one, err := s.KeepBundle(m.Slug, Bundle{Step: 2, Stage: "Check it", Crew: "c1", Capsule: "web/accent", Commit: "abc", Summary: "x",
		Checks: []BundleCheck{{N: 1, Verdict: CheckUnmet, Said: "y"}}}, fill)
	if err != nil {
		t.Fatal(err)
	}
	if one.ID != "b1" || one.Replaces != "" || one.At.IsZero() {
		t.Errorf("one = %+v", one)
	}
	if _, err := os.Stat(filepath.Join(s.BundleDir(m.Slug, "b1"), "a.png")); err != nil {
		t.Error(err)
	}
	// A folder with no record (a write that didn't finish) is passed over.
	if err := os.Mkdir(s.BundleDir(m.Slug, "b2"), 0o755); err != nil {
		t.Fatal(err)
	}
	two, err := s.KeepBundle(m.Slug, Bundle{Step: 2, Stage: "Check it", Crew: "c2", Capsule: "web/accent", Commit: "def", Summary: "x",
		Checks: []BundleCheck{{N: 1, Verdict: CheckMet, Said: "y"}}}, fill)
	if err != nil {
		t.Fatal(err)
	}
	if two.ID != "b3" || two.Replaces != "b1" {
		t.Errorf("two = %+v", two)
	}
	other, err := s.KeepBundle(m.Slug, Bundle{Step: 4, Stage: "Check the API", Crew: "c3", Capsule: "api/accent", Summary: "x"}, nil)
	if err != nil || other.Replaces != "" {
		t.Errorf("other = %+v, %v", other, err)
	}
	got, _ := s.Load(m.Slug)
	if len(got.Bundles) != 3 {
		t.Fatalf("bundles = %+v", got.Bundles)
	}
	if by, ok := got.ReplacedBy("b1"); !ok || by.ID != "b3" {
		t.Errorf("replaced by = %+v", by)
	}
	events, _ := s.ReadLog(m.Slug, 0)
	last := events[len(events)-2]
	if said, _ := Sentence(last); said != "c2 handed over bundle b3 for step 2: 1 met, in place of b1" {
		t.Errorf("said = %q", said)
	}
	// One whose files can't be kept leaves nothing behind.
	if _, err := s.KeepBundle(m.Slug, Bundle{Crew: "c1", Summary: "x"}, func(string, *Bundle) error { return os.ErrPermission }); err == nil {
		t.Error("kept without its files")
	}
	if _, err := os.Stat(s.BundleDir(m.Slug, "b5")); !os.IsNotExist(err) {
		t.Errorf("left a folder: %v", err)
	}
}

// A Verify step says what's to be shown: checks its verifier settles,
// commands ws runs, and (yours:) checks that are the user's. A bundle for
// it says yours: of those, and of no other.
func TestVerifyStep_ChecksAndYours(t *testing.T) {
	plan := "## Z\n- x\n## Stops early\n- None.\n## Steps\n1. Work: fix it.\n   - in: `web`\n" +
		"2. Verify: the fix, running.\n   - in: `web`\n   - on: claude:opus\n   - limit: 30m\n   - check: the page loads\n   - check: `npm test` passes\n   - yours: it reads well at phone width\n3. Review: it.\n"
	p, err := ParsePlan(plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	st := p.Steps[1]
	if st.Type != StepVerify || st.Who != WhoCrew || st.Capsule != "web" || st.Model != "claude:opus" || len(st.Checks) != 3 {
		t.Fatalf("step = %+v", st)
	}
	if st.Checks[0].Yours || !st.Checks[1].Run() || !st.Checks[2].Yours || st.Checks[2].Run() {
		t.Errorf("checks = %+v", st.Checks)
	}
	if whole := p.Stages()[1].Whole(); !strings.Contains(whole, "3. it reads well at phone width (yours)") || strings.Contains(whole, "once you report done") {
		t.Errorf("whole = %q", whole)
	}
	for written, want := range map[string]string{
		strings.Replace(plan, "   - check: the page loads\n   - check: `npm test` passes\n   - yours: it reads well at phone width\n", "", 1): "step 2 verifies nothing",
		strings.Replace(plan, "- yours: it reads well at phone width", "- yours: `npm test` passes", 1):                                       "is a command: write the user's checks in words",
		strings.Replace(plan, "   - limit: 30m\n", "   - carrying on from: 1\n", 1):                                                           "carrying on from",
		strings.Replace(plan, "1. Work: fix it.\n   - in: `web`\n", "1. Work: fix it.\n   - in: `web`\n   - yours: it's tidy\n", 1):           "yours",
	} {
		if _, err := ParsePlan(written, nil); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("want %q refused, got %v", want, err)
		}
	}

	ok := "## Summary\nx\n## Checks\n1. met: it loads\n   - shows: `a.png`\n3. yours: at 390 px wide\n   - shows: `b.png`\n## Not verified\nNothing.\n"
	b, err := ParseBundle(ok, st.Checks, nil)
	if err != nil {
		t.Fatal(err)
	}
	if b.Checks[1].Verdict != CheckYours || b.Tally() != "1 met, 1 yours" {
		t.Errorf("bundle = %+v", b.Checks)
	}
	for written, want := range map[string]string{
		strings.Replace(ok, "3. yours:", "3. met:", 1):                  "check 3 is the user's to settle, not yours",
		strings.Replace(ok, "1. met:", "1. yours:", 1):                  "check 1 is yours to settle",
		strings.Replace(ok, "3. yours: at 390 px wide", "3. yours:", 1): "check 3 is yours, but not how: say what you've left to help them settle it",
	} {
		if _, err := ParseBundle(written, st.Checks, nil); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("want %q refused, got %v", want, err)
		}
	}
	// A bundle of its own checks has none that are the user's.
	if _, err := ParseBundle("## Summary\nx\n## Checks\n1. yours: is it nice\n## Not verified\nNothing.\n", nil, nil); err == nil || !strings.Contains(err.Error(), "check 1 is yours to settle") {
		t.Errorf("a yours of its own: %v", err)
	}
}

// A Verify step isn't offered to you to accept on its verifier's done: ws
// settles it on the bundle.
func TestVerifyStep_NotYoursToAccept(t *testing.T) {
	p, err := ParsePlan("## Z\n- x\n## Stops early\n- None.\n## Steps\n1. Verify: it runs.\n   - check: it runs\n2. Review: it.\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC)
	m := &Mission{Slug: "cache", Plan: p, Crew: []Crew{{ID: "c1", Status: CrewDone, StatusAt: &at, Ended: &at, Note: "handed over b1", Started: at}},
		Stages: []StageRecord{{Text: p.Steps[0].Text, N: 1, Of: 2, Crew: []string{"c1"}, Started: &at,
			Checks: []CheckRound{{N: 1, Crew: "c1", Done: at.Format(time.RFC3339) + "|handed over b1", Results: []CheckResult{{Result: CheckUnmet}}}}}}}
	for _, h := range Resolve(Snapshot{M: m, State: Active, OrchestratorOff: true}) {
		if h.Kind == KindAccept {
			t.Errorf("offered to accept a Verify step: %+v", h)
		}
	}
}
