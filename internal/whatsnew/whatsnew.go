// Package whatsnew is what ws says is new after an upgrade: a few lines a
// release, written by hand. Nothing's said that nobody wrote, and a
// release nobody wrote of says nothing.
//
// An entry is a file of its own in entries/ (entries/README.md says how
// to write one), added in the pull request that makes what it tells of.
// It doesn't say which release it's in: that's its tag's to say. A build
// works it out (scripts/whatsnew): an entry is in the first release whose
// tag holds the commit that added it, and one in no release yet is
// unreleased. What it found is gen/whatsnew.json, which the binary holds.
// A build that didn't run it (go build alone) holds none, and says
// nothing's new.
//
// Only the entries of the latest Keep releases that have any are held, so
// the binary doesn't grow with every release: what's before is on the
// releases page. Pictures aren't held either: they're the release's own
// files, fetched when they're shown.
package whatsnew

import (
	"bufio"
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/withworkspace/workspace/internal/release"
)

// Keep is how many releases' entries a build holds: the latest that have
// any.
const Keep = 10

// TitleMax and BodyMax are how long an entry's title and its few lines
// can be, in characters: it's read in a moment, after an upgrade.
const (
	TitleMax = 60
	BodyMax  = 320
)

//go:embed all:gen
var gen embed.FS

// Entry is one thing that's new.
type Entry struct {
	// Slug is its file's name, without .md.
	Slug string `json:"slug"`
	// Version is the release it's in: "" for one that isn't in a release
	// yet.
	Version string `json:"version,omitempty"`
	Title   string `json:"title"`
	// Body is a sentence or two, in markdown.
	Body string `json:"body"`
	// Link is where in the web UI to see it (/settings/plugins): "" for
	// nowhere in particular.
	Link string `json:"link,omitempty"`
	// Picture is a picture of it, as the release's file is named
	// (AssetName): "" for none.
	Picture string `json:"picture,omitempty"`
}

// Index is what a build holds: the entries of the latest Keep releases
// that have any, and any that are unreleased, newest first.
type Index struct {
	// Before is the latest release whose entries aren't held ("" for
	// none left out): what's new in it and before is on the releases page.
	Before  string  `json:"before,omitempty"`
	Entries []Entry `json:"entries"`
	// Dir is the entries' folder in the clone the build was made from: a
	// build of a clone shows their pictures from there, as they'll read
	// before they're published. A release's build doesn't use it.
	Dir string `json:"dir,omitempty"`
}

// Held is what this build holds.
func Held() Index {
	var ix Index
	data, err := gen.ReadFile("gen/whatsnew.json")
	if err != nil {
		return ix
	}
	if err := json.Unmarshal(data, &ix); err != nil {
		return Index{}
	}
	return ix
}

// After is what's new after a release, up to and including another: the
// entries of the releases between, newest first. Unreleased entries are
// in no release, so never among them.
func (ix Index) After(seen, upTo string) []Entry {
	var out []Entry
	for _, e := range ix.Entries {
		if e.Version == "" || !release.Newer(e.Version, seen) || release.Newer(e.Version, upTo) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// UpTo is the entries of a release and those before it that are held.
func (ix Index) UpTo(upTo string) []Entry {
	var out []Entry
	for _, e := range ix.Entries {
		if e.Version != "" && !release.Newer(e.Version, upTo) {
			out = append(out, e)
		}
	}
	return out
}

// Missing is whether some of what's new after seen isn't held: seen is
// before the latest release whose entries were left out.
func (ix Index) Missing(seen string) bool {
	return ix.Before != "" && release.Newer(ix.Before, seen)
}

// AssetName is what an entry's picture is called among its release's
// files.
func AssetName(picture string) string { return "whatsnew-" + picture }

// Parse reads an entry's file: a head of title, and link and picture if
// it has them, between --- lines, then its few lines.
//
//	---
//	title: Plugins are in Settings
//	link: /settings/plugins
//	picture: plugins.png
//	---
//	Add, update and remove them from **Settings › Plugins**.
func Parse(slug string, data []byte) (Entry, error) {
	e := Entry{Slug: slug}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return e, fmt.Errorf("%s: it starts with a --- line, then its title", slug)
	}
	head, body, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		return e, fmt.Errorf("%s: its head doesn't end with a --- line", slug)
	}
	sc := bufio.NewScanner(bytes.NewBufferString(head))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return e, fmt.Errorf("%s: %q in its head isn't key: value", slug, line)
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "title":
			e.Title = value
		case "link":
			e.Link = value
		case "picture":
			e.Picture = value
		default:
			return e, fmt.Errorf("%s: %q isn't something an entry says (title, link, picture)", slug, key)
		}
	}
	e.Body = strings.TrimSpace(body)
	switch {
	case e.Title == "":
		return e, fmt.Errorf("%s: it has no title", slug)
	case e.Body == "":
		return e, fmt.Errorf("%s: it says nothing under its head", slug)
	case len([]rune(e.Title)) > TitleMax:
		return e, fmt.Errorf("%s: its title is %d characters, and %d is the most", slug, len([]rune(e.Title)), TitleMax)
	case len([]rune(e.Body)) > BodyMax:
		return e, fmt.Errorf("%s: it's %d characters, and %d is the most: say less, or link to where there's more", slug, len([]rune(e.Body)), BodyMax)
	case e.Link != "" && !strings.HasPrefix(e.Link, "/"):
		return e, fmt.Errorf("%s: its link is a path in the web UI (/settings/plugins), not %q", slug, e.Link)
	case e.Picture != "" && (strings.ContainsAny(e.Picture, "/\\") || !isPicture(e.Picture)):
		return e, fmt.Errorf("%s: its picture is a .png, .jpg or .webp file beside it in entries/, not %q", slug, e.Picture)
	}
	return e, nil
}

func isPicture(name string) bool {
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp"} {
		if strings.HasSuffix(strings.ToLower(name), ext) {
			return true
		}
	}
	return false
}

// Arrange puts entries in the order they're shown, newest first
// (unreleased before any release, then each release's in its entries'
// names), and keeps those of the latest keep releases that have any.
func Arrange(entries []Entry, keep int) Index {
	slices.SortStableFunc(entries, func(a, b Entry) int {
		switch {
		case a.Version == b.Version:
			return strings.Compare(a.Slug, b.Slug)
		case a.Version == "":
			return -1
		case b.Version == "":
			return 1
		case release.Newer(a.Version, b.Version):
			return -1
		}
		return 1
	})
	ix := Index{Entries: []Entry{}}
	var versions []string
	for _, e := range entries {
		if e.Version != "" && !slices.Contains(versions, e.Version) {
			if len(versions) == keep {
				ix.Before = e.Version
				break
			}
			versions = append(versions, e.Version)
		}
		ix.Entries = append(ix.Entries, e)
	}
	return ix
}

// Notes is a release's entries as its notes on GitHub say them, above the
// list of pull requests GitHub writes: "" for a release with none. base
// is the repo's page (https://github.com/withworkspace/workspace), where
// its pictures are.
func Notes(entries []Entry, version, base string) string {
	var b strings.Builder
	for _, e := range entries {
		if e.Version != version {
			continue
		}
		if b.Len() == 0 {
			b.WriteString("## What's new\n")
		}
		fmt.Fprintf(&b, "\n### %s\n\n%s\n", e.Title, e.Body)
		if e.Picture != "" {
			fmt.Fprintf(&b, "\n![%s](%s/releases/download/%s/%s)\n", e.Title, base, version, AssetName(e.Picture))
		}
	}
	return b.String()
}
