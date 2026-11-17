// Package artifact keeps artifacts: versioned HTML prototypes. An
// artifact is one HTML file, with its CSS and JS inline, that shows an
// idea, for people to look at and use rather than read: the glow in dark
// and in light, a mock-up of a page, a diagram you can step through. Crew
// add them, and you do (a conversation proposes one, and you confirm it).
//
// Each artifact is a folder, {date}-{slug}, holding its record
// (artifact.toml) and its versions (v1.html, v2.html, …). A version is a
// whole file, never changed or renumbered once written, and says who made
// it, where, what it's based on and what changed. The folder is in
// .ws/artifacts on this machine. The workspace's records hold an index of
// them (records.go), which gives each a short ref (a3, and a3.v2 for a
// version); where the index and artifact.toml differ, the file wins.
//
// It isn't a document kind, a page, or a mission's recorded artifact
// (mission.Artifact), which is a pointer that may point at one of these.
package artifact

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/BurntSushi/toml"
	"github.com/withworkspace/workspace/internal/slug"
)

// Status is where an artifact stands.
type Status string

const (
	// Live is the default: it takes new versions, and expires (Expiry).
	Live Status = "live"
	// Promoted is kept for good, with the version chosen and what it went
	// into. It still takes new versions.
	Promoted Status = "promoted"
	// Discarded is gone from the list, with the reason. Its record stays,
	// so its ref still says what it was.
	Discarded Status = "discarded"
)

// Artifact is an artifact's record, as artifact.toml holds it.
type Artifact struct {
	Title  string    `toml:"title" json:"title"`
	Status Status    `toml:"status" json:"status"`
	Made   time.Time `toml:"made" json:"made"`
	// Promoted says which version was chosen, and what it went into.
	Promoted *Promotion `toml:"promoted,omitempty" json:"promoted,omitempty"`
	// Discarded says when, by whom and why it was discarded.
	Discarded *Discard `toml:"discarded,omitempty" json:"discarded,omitempty"`
	// Restored says when it was last put back, discarded: its expiry
	// counts from then, if that's later than its last version.
	Restored *Restoration `toml:"restored,omitempty" json:"restored,omitempty"`
	Versions []Version    `toml:"versions" json:"versions"`
}

// Promotion is an artifact kept for good: when, by whom, the version
// chosen, and what it went into (a mission's slug, a document's ref, or
// nothing).
type Promotion struct {
	At      time.Time `toml:"at" json:"at"`
	By      string    `toml:"by" json:"by"`
	Version int       `toml:"version" json:"version"`
	Into    string    `toml:"into" json:"into"`
}

// Discard is an artifact discarded: when, by whom, and why ("expired",
// or what you said, or nothing).
type Discard struct {
	At     time.Time `toml:"at" json:"at"`
	By     string    `toml:"by" json:"by"`
	Reason string    `toml:"reason,omitempty" json:"reason,omitempty"`
	// Commit is, in a repo, the last commit its folder was in, before the
	// one that took it away: where history keeps it, and where restoring
	// it takes it from. Only the index has it: the record went with the
	// folder.
	Commit string `toml:"-" json:"commit,omitempty"`
}

// Restoration is an artifact put back, discarded: when, and by whom.
type Restoration struct {
	At time.Time `toml:"at" json:"at"`
	By string    `toml:"by" json:"by"`
}

// Version is one version's provenance and note. Its file is v<N>.html.
type Version struct {
	N int `toml:"n" json:"n"`
	// BasedOn is the version it revises, normally the one before; 0 for
	// the first, which revises nothing.
	BasedOn int       `toml:"based_on,omitzero" json:"basedOn,omitempty"`
	At      time.Time `toml:"at" json:"at"`
	// By is who added it: "you" (adding it, or confirming a proposal), or
	// crew (c1).
	By string `toml:"by" json:"by"`
	// From is where it was made: a conversation (w9, or slug/t3 for a
	// mission's, with Proposal its proposal's number) or a mission (its
	// slug, with By its crew). It's empty for one added by hand.
	From     string `toml:"from,omitempty" json:"from,omitempty"`
	Proposal int    `toml:"proposal,omitzero" json:"proposal,omitempty"`
	// FromTitle is the conversation's or mission's title as it was then:
	// they exist only on the machine that had them.
	FromTitle string `toml:"from_title,omitempty" json:"fromTitle,omitempty"`
	// Note is what changed and why, in a line or two; the first version's
	// says what the artifact is trying out.
	Note string `toml:"note,omitempty" json:"note,omitempty"`

	// Capsule is where a version kept in a repo waits until it lands
	// (records/glow): crew's, in flight. Was is its number there, where
	// it'll land as another (N), one past a version that landed first.
	// Neither is in artifact.toml: they're where ws found it.
	Capsule string `toml:"-" json:"capsule,omitempty"`
	Was     int    `toml:"-" json:"was,omitempty"`
}

// RecordFile is the name of an artifact's record in its folder.
const RecordFile = "artifact.toml"

// VersionFile is the name of version n's file in its artifact's folder.
func VersionFile(n int) string { return "v" + strconv.Itoa(n) + ".html" }

var versionFile = regexp.MustCompile(`^v([1-9][0-9]*)\.html$`)

// IsVersionFile is whether name is a version's file (v3.html), and which.
func IsVersionFile(name string) (int, bool) {
	m := versionFile.FindStringSubmatch(name)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	return n, err == nil
}

// Latest is its latest version: the highest numbered.
func (a *Artifact) Latest() (Version, bool) {
	if len(a.Versions) == 0 {
		return Version{}, false
	}
	latest := a.Versions[0]
	for _, v := range a.Versions[1:] {
		if v.N > latest.N {
			latest = v
		}
	}
	return latest, true
}

// Version is version n, if it has one.
func (a *Artifact) Version(n int) (Version, bool) {
	for _, v := range a.Versions {
		if v.N == n {
			return v, true
		}
	}
	return Version{}, false
}

// LastAt is when its latest version was added. Its making, for one with
// none.
func (a *Artifact) LastAt() time.Time {
	if v, ok := a.Latest(); ok {
		return v.At
	}
	return a.Made
}

// Since is what its expiry counts from: its latest version, or its
// restoring, if that's later (one put back isn't expired again at once).
func (a *Artifact) Since() time.Time {
	at := a.LastAt()
	if a.Restored != nil && a.Restored.At.After(at) {
		at = a.Restored.At
	}
	return at
}

// Parse reads an artifact.toml.
func Parse(data []byte) (*Artifact, error) {
	var a Artifact
	if _, err := toml.Decode(string(data), &a); err != nil {
		return nil, err
	}
	if a.Status == "" {
		a.Status = Live
	}
	return &a, nil
}

// Encode writes an artifact's record as artifact.toml has it.
func Encode(a *Artifact) ([]byte, error) {
	var b bytes.Buffer
	enc := toml.NewEncoder(&b)
	enc.Indent = "" // as the plan lays it out: [[versions]] and its keys flush left
	if err := enc.Encode(a); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// --- Refs

var refShape = regexp.MustCompile(`^(a[1-9][0-9]{0,6})(?:\.v([1-9][0-9]{0,4}))?$`)

// IsRef reports whether s names an artifact (a3) or a version of one
// (a3.v2).
func IsRef(s string) bool { return refShape.MatchString(s) }

// ParseRef reads an artifact's ref: its id (a3), and the version it names
// (0 for none).
func ParseRef(s string) (id string, version int, ok bool) {
	m := refShape.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return "", 0, false
	}
	if m[2] != "" {
		version, _ = strconv.Atoi(m[2])
	}
	return m[1], version, true
}

// VersionRef is how version n of artifact id is named: a3.v2.
func VersionRef(id string, n int) string { return id + ".v" + strconv.Itoa(n) }

// ParseVersion reads a version as --on names one: 2, or v2.
func ParseVersion(s string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(s), "v"))
	return n, err == nil && n > 0
}

// --- Its folder

// slugRoom is how long a folder's slug may be.
const slugRoom = 48

// FolderName is an artifact's folder's name, from the day it was made and
// its title: 2026-11-12-horizon-glow. The name stays if the title changes.
func FolderName(made time.Time, title string) string {
	return made.Local().Format("2006-01-02") + "-" + Slug(title)
}

// Slug is a title made into a folder's slug: horizon-glow.
func Slug(title string) string { return slug.Make(title, slugRoom, "artifact") }

// --- The checks

// MaxSize is the most a version may be. Images go in as data: URLs, so
// it's they that make one big.
const MaxSize = 1 << 20

// Checked is what checking a version found: its <title>, if it has one,
// and what won't work in the sandboxed frame the web UI runs it in.
type Checked struct {
	Title    string
	Warnings []string
}

var (
	titleTag = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	// A tag that loads something from elsewhere (src="https://…", or
	// "//…"). A link (<a href>) loads nothing.
	loadsFrom = regexp.MustCompile(`(?is)<(script|link|img|iframe|source|video|audio|embed|object|track|input)\b[^>]*?\b(?:src|href|data|srcset|poster)\s*=\s*["']?\s*((?:https?:)?//[^"'\s>]+)`)
	cssFrom   = regexp.MustCompile(`(?i)(?:@import\s+(?:url\(\s*)?|url\(\s*)["']?\s*((?:https?:)?//[^"')\s]+)`)
	network   = regexp.MustCompile(`\b(fetch|XMLHttpRequest|WebSocket|EventSource|navigator\.sendBeacon)\s*\(`)
	storage   = regexp.MustCompile(`\b(localStorage|sessionStorage|indexedDB|document\.cookie)\b`)
	looksHTML = regexp.MustCompile(`(?i)^<(?:!--|!doctype\s+html\b|(?:html|head|body|meta|title|style|script|link|div|main|section|svg|canvas|p|pre|h[1-6]|ul|ol|table|header|nav|article|template|span|form|button|img)\b)`)
	doctype   = regexp.MustCompile(`(?is)^\s*(<!--.*?-->\s*)*<!doctype\s+html\b`)
)

// Check checks that html is one HTML file, under the size limit, and says
// what in it won't work in the frame: anything loaded from elsewhere (a
// script, a font, an image), calls to the network, and storage, which the
// sandbox gives it none of.
func Check(htmlText []byte) (Checked, error) {
	if len(bytes.TrimSpace(htmlText)) == 0 {
		return Checked{}, errors.New("it has nothing in it")
	}
	if len(htmlText) > MaxSize {
		return Checked{}, fmt.Errorf("it's %s, and a version can be at most %s: use smaller images, or fewer (they're in it as data: URLs)", Size(len(htmlText)), Size(MaxSize))
	}
	if !utf8.Valid(htmlText) || bytes.IndexByte(htmlText, 0) >= 0 {
		return Checked{}, errors.New("it isn't text: an artifact is one HTML file, with its CSS and JS inline")
	}
	start := bytes.TrimLeft(bytes.TrimPrefix(htmlText, []byte("\xef\xbb\xbf")), " \t\r\n")
	if !looksHTML.Match(start) {
		return Checked{}, errors.New("it isn't HTML: an artifact is one HTML file (<!doctype html>, then its page), with its CSS and JS inline")
	}
	var c Checked
	if m := titleTag.FindSubmatch(htmlText); m != nil {
		c.Title = strings.Join(strings.Fields(html.UnescapeString(string(m[1]))), " ")
	}
	if !doctype.Match(start) {
		c.Warnings = append(c.Warnings, "it doesn't start with <!doctype html>, so it's drawn in quirks mode: start it with that")
	}
	seen := map[string]bool{}
	warn := func(s string) {
		if !seen[s] {
			seen[s] = true
			c.Warnings = append(c.Warnings, s)
		}
	}
	for _, m := range loadsFrom.FindAllSubmatch(htmlText, -1) {
		warn(fmt.Sprintf("its <%s> loads %s, which the frame won't fetch: put it in the file (inline, or as a data: URL)", strings.ToLower(string(m[1])), clip(string(m[2]))))
	}
	for _, m := range cssFrom.FindAllSubmatch(htmlText, -1) {
		warn(fmt.Sprintf("its CSS loads %s, which the frame won't fetch: put it in the file (inline, or as a data: URL)", clip(string(m[1]))))
	}
	for _, m := range network.FindAllSubmatch(htmlText, -1) {
		warn(fmt.Sprintf("it calls %s, and nothing leaves the frame: show the data it would fetch in the file", m[1]))
	}
	for _, m := range storage.FindAllSubmatch(htmlText, -1) {
		warn(fmt.Sprintf("it uses %s, which the frame doesn't give it (it throws, or is empty): keep what it needs in variables", m[1]))
	}
	return c, nil
}

func clip(s string) string {
	if r := []rune(s); len(r) > 60 {
		return string(r[:59]) + "…"
	}
	return s
}

// Size says a size in bytes as a person would.
func Size(n int) string {
	switch {
	case n >= 1<<20:
		return strings.TrimSuffix(strconv.FormatFloat(float64(n)/(1<<20), 'f', 1, 64), ".0") + " MB"
	case n >= 1<<10:
		return strconv.Itoa((n+512)>>10) + " KB"
	}
	return strconv.Itoa(n) + " bytes"
}

// CleanTitle is a title on one line, without the spaces around it.
func CleanTitle(s string) string { return strings.Join(strings.Fields(s), " ") }

// CleanNote is a note without the blank lines around it.
func CleanNote(s string) string { return strings.TrimSpace(s) }
