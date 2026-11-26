package mission

import (
	"regexp"
	"strings"
	"unicode"
)

// Crew raise missions for what they notice outside their own, and crew on
// different missions notice the same thing: a test that fails for everyone
// was raised six times in one evening, and two crew raised the same signing
// failure two seconds apart. ws mission new refuses, from inside a mission,
// one that's like a mission in play (Alike), pointing at it, so what's
// found is noted there instead.
//
// Alike is deliberately narrow. What crew notice is named by the code it's
// about (internal/ops/refs_test.go, TestLinkNames, WS_CREW, gofmt -l's
// files), so two descriptions alike share those names; plain words alone
// (tests, fail, daemon) never make two alike. Tuned on the dogfood
// workspace's missions: every duplicate raised there is alike, and no two
// different missions are.

// Alike reports whether two descriptions of work (a mission's title and
// what it was raised as) look like the same work: they share two names
// from the code, or one and half their wording.
func Alike(a, b string) bool {
	na, wa := described(a)
	nb, wb := described(b)
	weight := 0.0
	for n, x := range na {
		if y, ok := nb[n]; ok {
			weight += min(x, y)
		}
	}
	shared := 0
	for w := range wa {
		if wb[w] {
			shared++
		}
	}
	fewer := min(len(wa), len(wb))
	return weight >= 2 || (weight >= 1 && fewer > 0 && float64(shared)/float64(fewer) >= 0.4)
}

// Says is what a mission is about, for Alike: its title, and what it was
// raised as.
func (m *Mission) Says() string {
	return m.Title + " " + m.Request
}

var (
	descToken   = regexp.MustCompile(`[A-Za-z0-9_./-]+`)
	camel       = regexp.MustCompile(`[a-z][A-Z]`)
	dotted      = regexp.MustCompile(`\w\.\w`)
	packagePath = regexp.MustCompile(`^[a-z]+/[a-z]+$`)
)

// commonWords say nothing about which work it is: most of ws's missions
// use them (internal is where most Go code is).
var commonWords = map[string]bool{}

func init() {
	for w := range strings.FieldsSeq(`a an the and or but of to in on at for from with by as is are was were be been being
		it its this that these those there their they them then than so such not no nor can could should would will shall may might must
		do does did done have has had having into onto over under out up down off only also just very more most less much many
		some any each every all both either neither own same other another which who whom whose what when where why how while because
		if else though although until unless since after before about above below again further once here instead
		make makes made run runs ran need needs like see seen says say said fail fails failed failing test tests crew mission missions work works
		internal`) {
		commonWords[w] = true
	}
}

// described is a description's names from the code (a path, an
// identifier, an environment variable, a test's name), each weighed (a
// bare package path, internal/ops, weighs half: many missions are in one),
// and its other words, stemmed.
func described(s string) (names map[string]float64, words map[string]bool) {
	names, words = map[string]float64{}, map[string]bool{}
	for _, tok := range descToken.FindAllString(s, -1) {
		tok = strings.Trim(tok, "./-")
		if len(tok) < 4 {
			continue
		}
		if !isName(tok) {
			if isLetters(tok) && !commonWords[strings.ToLower(tok)] {
				words[stem(tok)] = true
			}
			continue
		}
		parts := []string{tok}
		if strings.Contains(tok, "/") {
			for p := range strings.SplitSeq(tok, "/") {
				if isName(p) {
					parts = append(parts, p) // WS_CREW/WS_MISSION names both
				}
			}
		}
		for _, p := range parts {
			if len(p) < 4 {
				continue
			}
			k := strings.ToLower(p)
			names[k] = 1
			if packagePath.MatchString(k) {
				names[k] = 0.5
			}
		}
		for _, p := range strings.FieldsFunc(tok, func(r rune) bool { return r == '/' || r == '_' || r == '.' }) {
			if len(p) >= 4 && isLetters(p) && !commonWords[strings.ToLower(p)] {
				words[stem(p)] = true
			}
		}
	}
	return names, words
}

// isName is whether a token is named from the code rather than said: it
// has a slash, an underscore, a dot inside it, a lower letter then an upper
// one, or letters and digits both.
func isName(t string) bool {
	hasDigit := strings.IndexFunc(t, unicode.IsDigit) >= 0
	hasLetter := strings.IndexFunc(t, unicode.IsLetter) >= 0
	return strings.ContainsAny(t, "/_") || dotted.MatchString(t) || camel.MatchString(t) || (hasDigit && hasLetter)
}

func isLetters(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// stem takes the commonest endings off a word, so tests and tested are one.
func stem(w string) string {
	w = strings.ToLower(w)
	for _, suffix := range []string{"ing", "ed", "es", "s"} {
		if len(w) > len(suffix)+3 && strings.HasSuffix(w, suffix) {
			return w[:len(w)-len(suffix)]
		}
	}
	return w
}
