package ops

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/space"
	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/team"
	"github.com/withworkspace/workspace/internal/workspace"
)

// What lands on a repo's default branch is heard of as its ground moves
// (the daemon follows every ground: FollowGrounds), whoever moved it: a
// teammate's merge, your own landing, a lift bringing the ground up. Each
// ground's head is kept with the events it was heard as (events_heard),
// so a move is heard once, and one made while the daemon was down is
// heard when it's next looked at.
//
// A move is one commits.landed: the commits, oldest first, each by its
// author as a person; and with it, for each of the repo's spaces, a
// doc.changed for each document the commits touched, and a thing.added or
// thing.changed for each thing (a folder with a thing.toml).
//
// A ground heard of for the first time is only noted: what was there
// before isn't news. Nor is a ground that's been put somewhere that isn't
// ahead of where it was (a branch rewritten).

// groundSettle is how long after a ground's head moves what landed on it
// is heard: a pull writes its files together.
var groundSettle = 500 * time.Millisecond

// hearGroundsAsTheyMove hears what lands on each repo the keeper says its
// ground moved for (told on moved), as it does, until stop closes: the
// follow of the grounds hears every one besides, for what this missed.
func (w *Workspace) hearGroundsAsTheyMove(stop <-chan struct{}, moved <-chan string) {
	db, err := w.Records()
	if err != nil {
		return
	}
	due := map[string]bool{}
	var settle <-chan time.Time
	for {
		select {
		case <-stop:
			return
		case repo := <-moved:
			due[repo] = true
			if settle == nil {
				settle = time.After(groundSettle)
			}
		case <-settle:
			settle = nil
			for repo := range due {
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				_, err := w.hearGround(ctx, db, repo)
				cancel()
				logFailed(err)
			}
			clear(due)
		}
	}
}

// groundHeardMax is the most commits one move is heard as: past it, the
// latest are.
const groundHeardMax = 500

// HearGrounds hears what's landed on each repo's default branch since it
// was last heard of, keeping it as events: what it kept.
func (w *Workspace) HearGrounds(ctx context.Context) ([]events.Event, error) {
	db, err := w.Records()
	if err != nil {
		return nil, err
	}
	ws := w.WS()
	var out []events.Event
	var errs []string
	for _, repo := range ws.RepoNames {
		if ctx.Err() != nil {
			break
		}
		got, err := w.hearGround(ctx, db, repo)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", repo, err))
		}
		out = append(out, got...)
	}
	if len(errs) > 0 {
		return out, fmt.Errorf("hearing the grounds: %s", strings.Join(errs, "; "))
	}
	return out, nil
}

func (w *Workspace) hearGround(ctx context.Context, db *store.DB, repo string) ([]events.Event, error) {
	ws := w.WS()
	ground := ws.MainWorktree(repo)
	if !dirThere(ground) {
		return nil, nil
	}
	head, err := git.ReadLine(ctx, ground, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if err != nil {
		return nil, nil // no commit yet
	}
	key := "ground:" + repo
	var last string
	if err := db.Read(ctx, func(tx *store.Tx) error {
		last, err = events.HeardTo(tx, key)
		return err
	}); err != nil {
		return nil, err
	}
	if last == head {
		return nil, nil
	}
	var heard []events.Event
	if last != "" {
		if _, err := git.ReadLine(ctx, ground, "merge-base", "--is-ancestor", last, head); err == nil {
			if heard, err = w.landedBetween(ctx, repo, last, head); err != nil {
				return nil, err
			}
		}
	}
	err = db.Write(ctx, func(tx *store.Tx) error {
		// Heard to somewhere else meanwhile (two looks at once): theirs
		// stands, and this one's is let go.
		if now, err := events.HeardTo(tx, key); err != nil || now != last {
			heard = nil
			return err
		}
		for i := range heard {
			e, err := events.Append(tx, heard[i])
			if err != nil {
				return err
			}
			heard[i] = e
		}
		return events.SetHeardTo(tx, key, head)
	})
	return heard, err
}

// landed is one commit, as heard.
type landed struct {
	sha, subject string
	author       team.Event
	at           time.Time
	paths        []string // as it touched them
	added        []string // those it added
	deleted      []string // those it deleted
}

// landedBetween is what landed on repo's ground from last to head: one
// commits.landed, and what it changed in the repo's spaces.
func (w *Workspace) landedBetween(ctx context.Context, repo, last, head string) ([]events.Event, error) {
	ws := w.WS()
	ground := ws.MainWorktree(repo)
	out, err := git.Read(ctx, ground, "log", "--reverse", "--no-merges", "-z", "--name-status", "--no-renames",
		fmt.Sprintf("--max-count=%d", groundHeardMax), "--format=%x1e%H%x1f%s%x1f%an%x1f%ae%x1f%cI", last+".."+head)
	if err != nil {
		return nil, err
	}
	commits := parseLanded(string(out))
	if len(commits) == 0 {
		return nil, nil
	}
	me := w.gitMe(ctx, ground)
	by := w.peopleOf(commits, me)
	branch := firstNonEmpty(ws.DefaultBranch, "main")

	landedE := events.Event{Kind: events.CommitsLanded, Origin: events.Heard, Repo: repo, Branch: branch, At: commits[len(commits)-1].at}
	seen := map[string]bool{}
	for _, c := range commits {
		landedE.Commits = append(landedE.Commits, events.Commit{SHA: c.sha, Subject: c.subject, By: by[c.sha]})
		for _, p := range c.paths {
			if !seen[p] && len(landedE.Paths) < events.PathsMax {
				seen[p] = true
				landedE.Paths = append(landedE.Paths, p)
			}
		}
	}
	tip := commits[len(commits)-1]
	landedE.By, landedE.Text = by[tip.sha], tip.subject
	if n := len(commits); n > 1 {
		landedE.Text = fmt.Sprintf("%d commits, the last: %s", n, tip.subject)
	}
	heard := []events.Event{landedE}

	sp, ok := ws.SpaceOf(repo)
	if !ok {
		return heard, nil
	}
	heard = append(heard, docsChanged(repo, sp, commits, by)...)
	heard = append(heard, w.thingsChanged(ctx, repo, sp, head, commits, by)...)
	return heard, nil
}

// docsChanged is a doc.changed for each of the space's documents the
// commits touched, by the last to: added, changed or deleted.
func docsChanged(repo string, sp workspace.Space, commits []landed, by map[string]string) []events.Event {
	type touch struct {
		c      landed
		status string
		first  bool // it was added in these commits
	}
	last := map[string]*touch{}
	var order []string
	for _, c := range commits {
		for _, p := range c.paths {
			if !sp.Has(p) {
				continue
			}
			t := last[p]
			if t == nil {
				t = &touch{first: slices.Contains(c.added, p)}
				last[p] = t
				order = append(order, p)
			}
			t.c, t.status = c, "changed"
		}
	}
	var out []events.Event
	for _, p := range order {
		t := last[p]
		switch {
		case slices.Contains(t.c.deleted, p):
			t.status = "deleted"
		case t.first:
			t.status = "added"
		}
		out = append(out, events.Event{
			Kind: events.DocChanged, Origin: events.Heard, By: by[t.c.sha], At: t.c.at,
			Repo: repo, Doc: repo + ":" + p, Title: strings.TrimSuffix(path.Base(p), path.Ext(p)), Status: t.status,
			Text:    t.c.subject,
			Commits: []events.Commit{{SHA: t.c.sha, Subject: t.c.subject, By: by[t.c.sha]}},
		})
	}
	return out
}

// thingsChanged is a thing.added or thing.changed for each of the space's
// things (a folder with a thing.toml, as it is at head) the commits
// touched, by the last to.
func (w *Workspace) thingsChanged(ctx context.Context, repo string, sp workspace.Space, head string, commits []landed, by map[string]string) []events.Event {
	ground := w.WS().MainWorktree(repo)
	homes := map[string]string{} // a folder → its thing.toml's text at head, "" for none
	homeOf := func(p string) (dir, manifest string) {
		for dir := path.Dir(p); dir != "." && dir != "/"; dir = path.Dir(dir) {
			if !strings.HasPrefix(dir+"/", sp.Folder) {
				return "", ""
			}
			text, ok := homes[dir]
			if !ok {
				b, err := git.Read(ctx, ground, "show", head+":"+dir+"/"+protocol.ManifestName)
				if err == nil {
					text = string(b)
				}
				homes[dir] = text
			}
			if text != "" {
				return dir, text
			}
		}
		return "", ""
	}
	type touch struct {
		c        landed
		manifest string
		added    bool
	}
	last := map[string]*touch{}
	var order []string
	for _, c := range commits {
		for _, p := range c.paths {
			dir, manifest := homeOf(p)
			if dir == "" {
				continue
			}
			t := last[dir]
			if t == nil {
				t = &touch{manifest: manifest}
				last[dir] = t
				order = append(order, dir)
			}
			t.c = c
			if slices.Contains(c.added, dir+"/"+protocol.ManifestName) {
				t.added = true
			}
		}
	}
	var out []events.Event
	for _, dir := range order {
		t := last[dir]
		kind, title, _, _, ok := space.ReadManifest([]byte(t.manifest))
		if !ok {
			continue
		}
		e := events.Event{
			Kind: events.ThingChanged, Origin: events.Heard, By: by[t.c.sha], At: t.c.at,
			Repo: repo, Thing: space.Thing{Repo: repo, Path: dir + "/"}.Name(), ThingKind: kind, Title: title,
			Text:    t.c.subject,
			Commits: []events.Commit{{SHA: t.c.sha, Subject: t.c.subject, By: by[t.c.sha]}},
		}
		if t.added {
			e.Kind = events.ThingAdded
		}
		out = append(out, e)
	}
	return out
}

// parseLanded reads git log's commits, each with the paths it touched, as
// landedBetween asks for them: a record separator before each commit, its
// fields between unit separators, then -z name-status pairs.
func parseLanded(out string) []landed {
	var commits []landed
	for _, rec := range strings.Split(out, "\x1e") {
		if strings.TrimSpace(rec) == "" {
			continue
		}
		head, rest, _ := strings.Cut(rec, "\n")
		f := strings.Split(head, "\x1f")
		if len(f) < 5 {
			continue
		}
		c := landed{sha: f[0], subject: f[1], author: team.Event{Name: f[2], Email: f[3]}}
		c.at, _ = time.Parse(time.RFC3339, strings.TrimSpace(f[4]))
		parts := strings.Split(strings.Trim(rest, "\x00\n"), "\x00")
		for i := 0; i+1 < len(parts); i += 2 {
			status, p := strings.TrimSpace(parts[i]), parts[i+1]
			if status == "" || p == "" {
				continue
			}
			c.paths = append(c.paths, p)
			switch status[0] {
			case 'A':
				c.added = append(c.added, p)
			case 'D':
				c.deleted = append(c.deleted, p)
			}
		}
		commits = append(commits, c)
	}
	return commits
}

// gitMe is who you are in git where repo's ground is: your name and
// email, as your commits say them.
func (w *Workspace) gitMe(ctx context.Context, dir string) team.Event {
	name, _ := git.ReadLine(ctx, dir, "config", "user.name")
	email, _ := git.ReadLine(ctx, dir, "config", "user.email")
	return team.Event{Name: name, Email: email}
}

// peopleOf is who each commit's by, as By says it: you, where its author
// is who you are in git; else the person the roster (the team's look,
// with [team.people]) knows them as.
func (w *Workspace) peopleOf(commits []landed, me team.Event) map[string]string {
	var known []team.Event
	var people map[string]team.PersonConfig
	if t := w.Team(); t != nil {
		if snap := t.Peek(); snap != nil {
			known = snap.Events
		}
		if t.People != nil {
			people = t.People()
		}
	}
	all := slices.Clone(known)
	for _, c := range commits {
		all = append(all, c.author)
	}
	roster := team.Resolve(all, people)
	out := map[string]string{}
	for _, c := range commits {
		switch {
		case me.Email != "" && strings.EqualFold(c.author.Email, me.Email):
			out[c.sha] = events.ByYou
		case roster.Of(c.author) != nil:
			out[c.sha] = events.ByPerson(roster.Of(c.author).Key)
		default:
			out[c.sha] = events.ByPerson(firstNonEmpty(c.author.Email, firstNonEmpty(c.author.Name, "someone")))
		}
	}
	return out
}
