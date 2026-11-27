// Package artifact keeps artifacts: versioned HTML prototypes. An
// artifact is a page that shows an idea, for people to look at and use
// rather than read: the glow in dark and in light, a mock-up of a page, a
// diagram you can step through. Crew add them, and you do (a conversation
// proposes one, and you confirm it).
//
// A version is its page (index.html) and the files beside it: whatever
// the page loads, each a real file (files.go). Each artifact is a folder,
// {date}-{slug}, holding its record (artifact.toml) and a git repo of its
// own (files.git, tree.go), where each version is a commit, tagged v1,
// v2, …: whole, never changed or renumbered once written. The record says
// of each who made it, where, what it's based on and what changed. The
// folder is in .ws/artifacts on this machine. The workspace's records
// hold an index of them (records.go), which gives each a short ref (a3,
// and a3.v2 for a version); where the index and artifact.toml differ, the
// file wins.
//
// They're sketches, and this machine's. One worth keeping is promoted,
// and may then live in a repo too, for everyone with it (Home): the
// version chosen, put there each time it's promoted. The rest of its
// versions stay here.
//
// It isn't a document kind, a page, or a mission's recorded artifact
// (mission.Artifact), which is a pointer that may point at one of these.
package artifact

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"net/url"
	"path"
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
	// Home is where it lives in a repo, promoted: none for one that's on
	// this machine only. Here says that was chosen: promoted to stay on
	// this machine, whatever [artifacts] says of where promoted ones
	// live, until it's given a home.
	Home     *Home     `toml:"home,omitempty" json:"home,omitempty"`
	Here     bool      `toml:"here,omitempty" json:"here,omitempty"`
	Versions []Version `toml:"versions" json:"versions"`
}

// Home is where a promoted artifact lives in a repo, for everyone with
// it: a folder there, chosen the first time it's promoted, holding the
// version that was chosen (HomeFile) and a record of it. Promoting it
// again puts the version chosen then in the same folder, in one commit:
// where it lives isn't asked twice. Discarded, its folder there is taken
// away, and Home still says where it was, to put it back.
type Home struct {
	Repo   string `toml:"repo" json:"repo"`     // records
	Folder string `toml:"folder" json:"folder"` // artifacts/2026-11-12-horizon-glow
	// Version is the version that's there, and Commit the commit that put
	// it there.
	Version int       `toml:"version" json:"version"`
	Commit  string    `toml:"commit,omitempty" json:"commit,omitempty"`
	At      time.Time `toml:"at" json:"at"`
}

// Where is the folder as it's said to people, and as a document cites
// it: records:artifacts/2026-11-12-horizon-glow/.
func (h Home) Where() string { return h.Repo + ":" + h.Folder + "/" }

// HomeFile is the name of the version's page where an artifact lives in
// a repo, its other files beside it as they are in the version: there's
// one version there, so nothing's numbered.
const HomeFile = IndexFile

// AtHome is an artifact's record as its home in a repo has it: what it
// is, that it was promoted, and each version up to the one chosen (what
// it changed, who made it and where), whose files are beside it. What's
// only this machine's is left out.
func (a *Artifact) AtHome(p Promotion) *Artifact {
	out := &Artifact{Title: a.Title, Status: Promoted, Made: a.Made, Promoted: &p}
	for _, v := range a.Versions {
		if v.N <= p.Version {
			out.Versions = append(out.Versions, v)
		}
	}
	return out
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
}

// Restoration is an artifact put back, discarded: when, and by whom.
type Restoration struct {
	At time.Time `toml:"at" json:"at"`
	By string    `toml:"by" json:"by"`
}

// Version is one version's provenance and note. Its files are the commit
// tagged v<N> in the artifact's repo.
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
	// Files is how many files it has, and Size what they come to: 0 for
	// one from before a version said, which its repo is asked.
	Files int `toml:"files,omitzero" json:"files,omitempty"`
	Size  int `toml:"size,omitzero" json:"size,omitempty"`
}

// RecordFile is the name of an artifact's record in its folder.
const RecordFile = "artifact.toml"

// VersionFile is the name version n's file had in its artifact's folder,
// from when a version was one file there: what's still so named is put
// in the artifact's repo the first time it's touched (tree.go).
func VersionFile(n int) string { return "v" + strconv.Itoa(n) + ".html" }

var versionFile = regexp.MustCompile(`^v([1-9][0-9]*)\.html$`)

// IsVersionFile is whether name is one of those (v3.html), and which.
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

// Checked is what checking a version found: its files, as it keeps them,
// its page's <title>, if it has one, and what won't work in the sandboxed
// frame the web UI runs it in.
type Checked struct {
	Files    Files
	Title    string
	Warnings []string
}

var (
	titleTag = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	// A tag that loads something from elsewhere (src="https://…", or
	// "//…"). A link (<a href>) loads nothing.
	loadsFrom = regexp.MustCompile(`(?is)<(script|link|img|iframe|source|video|audio|embed|object|track|input)\b[^>]*?\b(?:src|href|data|srcset|poster)\s*=\s*["']?\s*((?:https?:)?//[^"'\s>]+)`)
	// A tag that loads a file of the artifact's own (src="app.js").
	loadsOwn = regexp.MustCompile(`(?is)<(script|link|img|source|video|audio|track)\b[^>]*?\s(?:src|href|poster)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+))`)
	scheme   = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)
	cssFrom  = regexp.MustCompile(`(?i)(?:@import\s+(?:url\(\s*)?|url\(\s*)["']?\s*((?:https?:)?//[^"')\s]+)`)
	// A call to somewhere else: fetch("https://…"), xhr.open("GET", "//…"),
	// fetch("/api/…").
	fetchFrom = regexp.MustCompile("(?:\\bfetch|\\.open)\\s*\\(\\s*(?:[\"'][A-Za-z]+[\"']\\s*,\\s*)?[\"'`]((?:https?:)?//[^\"'`\\s)]+|/[^\"'`\\s)]*)")
	sockets   = regexp.MustCompile(`\b(WebSocket|EventSource|navigator\.sendBeacon)\s*\(`)
	storage   = regexp.MustCompile(`\b(localStorage|sessionStorage|indexedDB|document\.cookie)\b`)
	looksHTML = regexp.MustCompile(`(?i)^<(?:!--|!doctype\s+html\b|(?:html|head|body|meta|title|style|script|link|div|main|section|svg|canvas|p|pre|h[1-6]|ul|ol|table|header|nav|article|template|span|form|button|img)\b)`)
	doctype   = regexp.MustCompile(`(?is)^\s*(<!--.*?-->\s*)*<!doctype\s+html\b`)
)

// Check checks a version's files: that there's a page among them
// (index.html) that's HTML, that each is at a path a version can keep,
// and that they're within the limits. It says what in them won't work in
// the frame: anything loaded from elsewhere (a script, a font, an image),
// a file the page loads that isn't among them, calls to the network, and
// storage, which the sandbox gives it none of.
func Check(files Files) (Checked, error) {
	if len(files) == 1 && files[0].Path == IndexFile && len(bytes.TrimSpace(files[0].Data)) == 0 {
		return Checked{}, errors.New("it has nothing in it")
	}
	files, err := checkFiles(files)
	if err != nil {
		return Checked{}, err
	}
	page, _ := files.Index()
	if len(bytes.TrimSpace(page)) == 0 {
		return Checked{}, fmt.Errorf("its %s has nothing in it", IndexFile)
	}
	if !utf8.Valid(page) || bytes.IndexByte(page, 0) >= 0 {
		return Checked{}, fmt.Errorf("its %s isn't text: a version's page is an HTML file", IndexFile)
	}
	start := bytes.TrimLeft(bytes.TrimPrefix(page, []byte("\xef\xbb\xbf")), " \t\r\n")
	if !looksHTML.Match(start) {
		return Checked{}, fmt.Errorf("its %s isn't HTML: a version's page is an HTML file (<!doctype html>, then its page)", IndexFile)
	}
	c := Checked{Files: files}
	if m := titleTag.FindSubmatch(page); m != nil {
		c.Title = strings.Join(strings.Fields(html.UnescapeString(string(m[1]))), " ")
	}
	if !doctype.Match(start) {
		c.Warnings = append(c.Warnings, "missing <!doctype html> at the start: renders in quirks mode")
	}
	seen := map[string]bool{}
	warn := func(s string) {
		if !seen[s] {
			seen[s] = true
			c.Warnings = append(c.Warnings, s)
		}
	}
	has := map[string]bool{}
	for _, f := range files {
		has[f.Path] = true
	}
	for _, f := range files {
		// What's said of the page is "its"; of another file, by its name.
		its, it := "its", "it"
		if f.Path != IndexFile {
			its, it = f.Path+"'s", f.Path
		}
		switch strings.ToLower(path.Ext(f.Path)) {
		case ".html", ".htm":
			for _, m := range loadsFrom.FindAllSubmatch(f.Data, -1) {
				warn(fmt.Sprintf("%s <%s> loads %s from outside, which is blocked: add it to the artifact's files", its, strings.ToLower(string(m[1])), clip(string(m[2]))))
			}
			for _, m := range loadsOwn.FindAllSubmatch(f.Data, -1) {
				ref := string(m[2]) + string(m[3]) + string(m[4])
				if w := ownFile(path.Dir(f.Path), ref, has); w != "" {
					warn(fmt.Sprintf("%s <%s> loads %s, %s", its, strings.ToLower(string(m[1])), clip(strings.TrimSpace(ref)), w))
				}
			}
			for _, m := range cssFrom.FindAllSubmatch(f.Data, -1) {
				warn(fmt.Sprintf("%s CSS loads %s from outside, which is blocked: add it to the artifact's files", its, clip(string(m[1]))))
			}
		case ".css":
			for _, m := range cssFrom.FindAllSubmatch(f.Data, -1) {
				warn(fmt.Sprintf("%s loads %s from outside, which is blocked: add it to the artifact's files", f.Path, clip(string(m[1]))))
			}
			continue
		case ".js", ".mjs":
		default:
			continue
		}
		for _, m := range fetchFrom.FindAllSubmatch(f.Data, -1) {
			warn(fmt.Sprintf("%s fetches %s, which is blocked: add the data to the artifact's files (fetch(\"data.json\") reads one)", it, clip(string(m[1]))))
		}
		for _, m := range sockets.FindAllSubmatch(f.Data, -1) {
			warn(fmt.Sprintf("%s calls %s, which is blocked: show what it would send on the page", it, m[1]))
		}
		for _, m := range storage.FindAllSubmatch(f.Data, -1) {
			warn(fmt.Sprintf("%s uses %s, which is unavailable (throws, or is empty): keep state in variables", it, m[1]))
		}
	}
	return c, nil
}

// ownFile is what's wrong with a file a page in dir loads by ref, and ""
// where it's one of the version's (has), or isn't a file of its own at
// all (a data: URL, a link elsewhere, which is said by itself).
func ownFile(dir, ref string, has map[string]bool) string {
	ref = strings.TrimSpace(ref)
	if i := strings.IndexAny(ref, "?#"); i >= 0 {
		ref = ref[:i]
	}
	if ref == "" || scheme.MatchString(ref) || strings.HasPrefix(ref, "//") {
		return ""
	}
	if strings.HasPrefix(ref, "/") {
		return "an absolute path, which is blocked: use a path relative to the page (" + strings.TrimLeft(ref, "/") + ")"
	}
	if un, err := url.PathUnescape(ref); err == nil {
		ref = un
	}
	p := path.Join(dir, ref)
	if p == ".." || strings.HasPrefix(p, "../") {
		return "which is outside the artifact: add it to the artifact's files"
	}
	if has[p] {
		return ""
	}
	return "which is missing from the artifact's files: add the folder that has it (ws artifact add <folder>)"
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
