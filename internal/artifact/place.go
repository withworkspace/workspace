package artifact

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Where a workspace keeps its artifacts: [artifacts] in ws.toml (and its
// path in ws.local.toml), read. Without a path, or with path = ".ws",
// they're on this machine, in .ws/artifacts/{date}-{slug}/. With a repo's
// folder ("records:artifacts/{date}-{slug}/"), they're kept in git: each
// one you add is a commit landed on its default branch, crew's wait in
// their capsule and land with it, and ws reads them from the repo's
// ground and those capsules. Wherever new ones go, ws reads both places.

// Messages are the first lines of the commits that keep artifacts in a
// repo, with {title}, {slug}, {date}, {number} and {version} filled in.
type Messages struct {
	Add, Revise, Promote, Discard, Restore, Clear string
}

// DefaultMessages are ws's own, for a workspace whose [artifacts] says
// none.
var DefaultMessages = Messages{
	Add:     "Add artifact: {title}",
	Revise:  "Revise artifact {slug}: v{version}",
	Promote: "Promote artifact {slug}",
	Discard: "Discard artifact {slug}",
	Restore: "Restore artifact {slug}",
	Clear:   "Clear expired artifacts",
}

// DefaultExpiry is how long a live artifact lasts after its last version,
// where [artifacts] doesn't say.
const DefaultExpiry = 30 * 24 * time.Hour

// Config is [artifacts], read: the repo they're kept in, if any, and
// where in it; where new ones go; the commits' messages; how long a live
// one lasts; and the guide a conversation's agent is shown.
type Config struct {
	// Repo is the repo the workspace keeps them in, by its canonical name:
	// "" for none, and they're all on this machine.
	Repo string
	// Dir is the folder they're in, in Repo ("artifacts"), and Name a
	// folder's name in it ("{date}-{slug}").
	Dir, Name string
	// Path is [artifacts] path as it's written: "records:artifacts/{date}-{slug}/".
	Path string
	// Local is new ones going on this machine though there's a repo for
	// them: ws.local.toml says path = ".ws".
	Local    bool
	Messages Messages
	// ExpireAfter is how long a live one lasts after its last version: 0
	// for never.
	ExpireAfter time.Duration
	Guide       string
	// Problems are what's wrong with what [artifacts] says, which ws
	// doctor says. Where its path can't be used, there's no Repo, and new
	// ones go on this machine.
	Problems []string
}

// InRepo reports whether new artifacts go in a repo.
func (c Config) InRepo() bool { return c.Repo != "" && !c.Local }

// Place is where new artifacts go: the repo's name, or Local.
func (c Config) Place() string {
	if c.InRepo() {
		return c.Repo
	}
	return Local
}

// Said is [artifacts] as ws.toml and ws.local.toml say it, before it's
// read: what ReadConfig reads.
type Said struct {
	Path, Local                                   string
	Add, Revise, Promote, Discard, Restore, Clear string
	ExpireAfter, Guide                            string
}

// ReadConfig reads what [artifacts] says, given how to name a repo
// (canonical: its canonical name, or false for none the workspace has).
// What it can't use is in its Problems, and is left out: a path that
// can't be used keeps them on this machine.
func ReadConfig(s Said, canonical func(string) (string, bool)) Config {
	c := Config{Messages: DefaultMessages, ExpireAfter: DefaultExpiry, Guide: strings.TrimSpace(s.Guide)}
	for _, m := range []struct {
		to   *string
		said string
	}{{&c.Messages.Add, s.Add}, {&c.Messages.Revise, s.Revise}, {&c.Messages.Promote, s.Promote}, {&c.Messages.Discard, s.Discard}, {&c.Messages.Restore, s.Restore}, {&c.Messages.Clear, s.Clear}} {
		if t := strings.Join(strings.Fields(m.said), " "); t != "" {
			*m.to = t
		}
	}
	if e := strings.TrimSpace(s.ExpireAfter); e != "" {
		d, err := ParseExpiry(e)
		if err != nil {
			c.Problems = append(c.Problems, fmt.Sprintf("expire_after = %q: %v", e, err))
		} else {
			c.ExpireAfter = d
		}
	}
	team, local := strings.TrimSpace(s.Path), strings.TrimSpace(s.Local)
	read := func(file, p string) (repo, dir, name string, ok bool) {
		if p == "" || p == Local {
			return "", "", "", true
		}
		r, d, n, err := ParsePath(p)
		if err == nil {
			var known bool
			if r, known = canonical(r); !known {
				err = fmt.Errorf("%s isn't one of the workspace's repos", strings.SplitN(p, ":", 2)[0])
			}
		}
		if err != nil {
			c.Problems = append(c.Problems, fmt.Sprintf("%s's path = %q: %v", file, p, err))
			return "", "", "", false
		}
		return r, d, n, true
	}
	if repo, dir, name, ok := read("ws.toml", team); ok && repo != "" {
		c.Repo, c.Dir, c.Name, c.Path = repo, dir, name, team
	}
	if local != "" {
		repo, dir, name, ok := read("ws.local.toml", local)
		switch {
		case !ok:
		case repo == "":
			c.Local = true
		default:
			c.Repo, c.Dir, c.Name, c.Path, c.Local = repo, dir, name, local, false
		}
	}
	return c
}

// ParseExpiry reads expire_after: a number of days ("30d"), or "never"
// (0).
func ParseExpiry(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "never" {
		return 0, nil
	}
	days, ok := strings.CutSuffix(s, "d")
	n, err := strconv.Atoi(days)
	if !ok || err != nil || n < 1 {
		return 0, fmt.Errorf("it's a number of days, as 30d, or never")
	}
	return time.Duration(n) * 24 * time.Hour, nil
}

var namePart = regexp.MustCompile(`\{([a-z]+)\}`)

// ParsePath reads [artifacts] path: a repo, a folder in it, and the name
// of an artifact's folder there, with {date} and {slug} (or {number}),
// ending in / because it's a folder: "records:artifacts/{date}-{slug}/"
// is the repo records, the folder artifacts, and the name
// {date}-{slug}.
func ParsePath(p string) (repo, dir, name string, err error) {
	repo, rest, ok := strings.Cut(strings.TrimSpace(p), ":")
	switch {
	case !ok || repo == "":
		return "", "", "", fmt.Errorf("it doesn't say which repo: it's repo:folder/{date}-{slug}/, or .ws")
	case !strings.HasSuffix(rest, "/"):
		return "", "", "", fmt.Errorf("it's a folder, so it ends in /: %s:%s/", repo, rest)
	}
	rest = strings.TrimSuffix(rest, "/")
	if rest == "" || strings.HasPrefix(rest, "/") || slices.Contains(strings.Split(rest, "/"), "..") || slices.Contains(strings.Split(rest, "/"), ".") {
		return "", "", "", fmt.Errorf("it leads out of %s, or nowhere in it", repo)
	}
	dir, name = path.Dir(rest), path.Base(rest)
	if dir == "." {
		dir = ""
	}
	if namePart.MatchString(dir) {
		return "", "", "", fmt.Errorf("only an artifact's own folder, the last part, is named from it: %s has {…} before that", dir+"/")
	}
	seen := map[string]bool{}
	for _, m := range namePart.FindAllStringSubmatch(name, -1) {
		switch m[1] {
		case "date", "slug", "number":
		default:
			return "", "", "", fmt.Errorf("{%s} isn't something a folder's named from: it's {date}, {slug} or {number}", m[1])
		}
		if seen[m[1]] {
			return "", "", "", fmt.Errorf("it has {%s} twice", m[1])
		}
		seen[m[1]] = true
	}
	if !seen["slug"] && !seen["number"] {
		return "", "", "", fmt.Errorf("its folder's name, %s, has neither {slug} nor {number}: every artifact would be the same folder", name)
	}
	return repo, dir, name, nil
}

// Fields are what an artifact's folder and messages are filled in from.
type Fields struct {
	Date, Slug, Number, Title string
	Version                   int
}

// Fill is a folder's name, or a message, with what's known filled in.
func (f Fields) Fill(s string) string {
	return namePart.ReplaceAllStringFunc(s, func(p string) string {
		v := map[string]string{"{date}": f.Date, "{slug}": f.Slug, "{number}": f.Number, "{title}": f.Title}[p]
		if p == "{version}" && f.Version > 0 {
			v = strconv.Itoa(f.Version)
		}
		if v == "" {
			return p
		}
		return v
	})
}

// Subject is one of the messages filled in, on one line.
func (f Fields) Subject(msg string) string { return strings.Join(strings.Fields(f.Fill(msg)), " ") }

// nameShape is a folder's name as a pattern, and what each group is.
func nameShape(name string) (*regexp.Regexp, []string) {
	var re strings.Builder
	var fields []string
	re.WriteString("^")
	last := 0
	for _, m := range namePart.FindAllStringSubmatchIndex(name, -1) {
		re.WriteString(regexp.QuoteMeta(name[last:m[0]]))
		last = m[1]
		f := name[m[2]:m[3]]
		switch f {
		case "date":
			re.WriteString(`(\d{4}-\d{2}-\d{2})`)
		case "number":
			re.WriteString(`(\d+)`)
		default:
			re.WriteString(`(.+?)`)
		}
		fields = append(fields, f)
	}
	re.WriteString(regexp.QuoteMeta(name[last:]) + "$")
	return regexp.MustCompile(re.String()), fields
}

// MatchName is what a folder's name says, by the shape the workspace names
// them (name): its date, slug and number. False where it isn't of that
// shape.
func MatchName(name, folder string) (Fields, bool) {
	re, fields := nameShape(name)
	m := re.FindStringSubmatch(folder)
	if m == nil {
		return Fields{}, false
	}
	var f Fields
	for i, field := range fields {
		switch field {
		case "date":
			f.Date = m[i+1]
		case "slug":
			f.Slug = m[i+1]
		case "number":
			f.Number = m[i+1]
		}
	}
	return f, true
}

// SlugOf is an artifact's folder's slug, for its messages: what its name
// says it is, or its name, where it says none.
func SlugOf(name, folder string) string {
	folder = path.Base(folder)
	if f, ok := MatchName(name, folder); ok && f.Slug != "" {
		return f.Slug
	}
	if f, ok := MatchName("{date}-{slug}", folder); ok {
		return f.Slug
	}
	return folder
}

// NameFolder is a new artifact's folder's name, by the shape the
// workspace names them (name), made on the day of made from title: the
// first not taken, -2, -3 past one that is. A {number} is one past the
// highest of the names there are (numbers).
func NameFolder(name string, made time.Time, title string, numbers []string, taken func(string) bool) (string, error) {
	f := Fields{Date: made.Local().Format("2006-01-02"), Slug: Slug(title), Title: title}
	if strings.Contains(name, "{number}") {
		high := 0
		for _, n := range numbers {
			if g, ok := MatchName(name, n); ok {
				if v, err := strconv.Atoi(g.Number); err == nil && v > high {
					high = v
				}
			}
		}
		for i := 1; i <= 100; i++ {
			f.Number = fmt.Sprintf("%04d", high+i)
			if base := f.Fill(name); !taken(base) {
				return base, nil
			}
		}
		return "", fmt.Errorf("couldn't number a folder for %q", title)
	}
	base := f.Fill(name)
	for i := 1; i <= 100; i++ {
		n := base
		if i > 1 {
			n = base + "-" + strconv.Itoa(i)
		}
		if !taken(n) {
			return n, nil
		}
	}
	return "", fmt.Errorf("couldn't name a folder for %q", title)
}
