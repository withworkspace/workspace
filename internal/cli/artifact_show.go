package cli

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/store"
)

// artifactShowView is ws show a3 (or a3.v2): an artifact, for an agent
// about to look at it or revise it, or to say what it is. Its title and
// where it stands, where it's kept, each version with what it's based on,
// who made it and where, and its note, and how to read a version.
func artifactShowView(ctx *ops.Workspace, ref string, full bool) (*view, error) {
	id, asked, _ := artifact.ParseRef(ref)
	st := ctx.Artifacts()
	e, err := st.Get(context.Background(), id)
	if err != nil {
		return nil, err
	}
	if asked != 0 {
		if _, ok := e.Version(asked); !ok {
			return nil, fmt.Errorf("%s has no v%d: ws show %s lists its versions", id, asked, id)
		}
	}
	latest, _ := e.Latest()
	n := len(e.Versions)
	state := string(e.Status)
	cfg, now := ctx.ArtifactsConfig(), time.Now()
	expires, expirable := cfg.Expires(e)
	switch {
	case e.Gone || e.Away():
		state += ", its folder gone"
	case e.Erased:
		state += ", its files deleted for good"
	case expirable && !now.Before(expires):
		state = "expired"
	case cfg.Expiring(e, now):
		state += ", " + artifact.ExpiresWords(expires, now)
	}
	kept := "on " + e.Kept()
	head := fmt.Sprintf("Artifact %s — %q (%s, %s; %d %s, the latest v%d)", e.ID, e.Title, state, kept, n, plural(n, "version"), latest.N)
	if asked != 0 {
		head = fmt.Sprintf("Artifact %s, v%d of %d — %q (%s, %s)", e.ID, asked, n, e.Title, state, kept)
	}
	v := &view{head: head}
	v.body = append(v.body, "An artifact is a page that shows an idea, to look at and use: index.html, alone or with the files it loads beside it. Each version is whole, never changed once written.")
	dir := st.Path(e)
	switch {
	case e.Away():
		v.body = append(v.body, fmt.Sprintf("It was kept in %s (%s), from when artifacts were kept in a repo whole, and hasn't been brought onto this machine: ws doctor does it. What's below is what the records kept.", e.Place, e.Place+":"+e.Folder+"/"))
	case e.Erased:
		v.body = append(v.body, fmt.Sprintf("It was kept on this machine (.ws/artifacts/%s/), and its files have been deleted for good: what's below is what the records kept, and there are no files to read.", e.Folder))
	case e.Gone:
		v.body = append(v.body, fmt.Sprintf("Its folder has gone from .ws/artifacts (it was %s): what's below is what the records kept, and there are no files to read.", dir))
	default:
		v.body = append(v.body, fmt.Sprintf("Kept in %s, on this machine: %s is its record, and %s the repo its versions' files are in, each a commit (v<n>). Made %s.", dir, artifact.RecordFile, artifact.RepoDir, when(e.Made)))
	}
	if h := e.Home; h != nil && e.Lives() != "" {
		there := fmt.Sprintf("It lives in %s too, for everyone with %s: v%d is there (%s with its files, and a record of it), put there %s", h.Where(), h.Repo, h.Version, artifact.HomeFile, when(h.At))
		if h.Commit != "" {
			there += " in " + shortSHA(h.Commit)
		}
		there += ". Name it for other people by that folder: its ref (" + e.ID + ") is this machine's."
		if latest.N > h.Version {
			there += fmt.Sprintf(" v%d is newer: promoting it puts the version chosen there, in one commit (the user only).", latest.N)
		}
		v.body = append(v.body, there)
	}
	if d := e.Discarded; e.Status == artifact.Discarded && d != nil {
		why := "."
		if d.Reason != "" {
			why = ": " + d.Reason
			if !strings.ContainsAny(why[len(why)-1:], ".!?") {
				why += "." // "expired", or what was said without a full stop
			}
		}
		v.body = append(v.body, fmt.Sprintf("Discarded %s by %s%s It's listed no more, and takes no new versions.", when(d.At), d.By, why))
		if e.Restorable() {
			back := ""
			if e.Home != nil {
				back = ", and in " + e.Home.Where() + ", where it lived"
			}
			v.body = append(v.body, fmt.Sprintf("Its folder is in %s until %s, when it's deleted for good: ws artifact restore %s puts it back%s (the user only).", e.Where(), d.At.Add(artifact.KeepDiscarded).Local().Format("2006-01-02"), e.ID, back))
		}
	}
	if expirable && e.Status == artifact.Live {
		if !now.Before(expires) {
			v.body = append(v.body, fmt.Sprintf("Expired %s: it's had no new version for %s, and the daemon discards it, unless the user promotes it.", when(expires), cfg.ExpiryWords()))
		} else {
			v.body = append(v.body, fmt.Sprintf("Live: it expires %s, %s after its last version, unless it's given a new version or promoted.", expires.Local().Format("2006-01-02"), cfg.ExpiryWords()))
		}
	}
	if p := e.Promoted; e.Status == artifact.Promoted && p != nil {
		into := ""
		if p.Into != "" {
			into = ", into " + p.Into
		}
		v.body = append(v.body, fmt.Sprintf("Promoted %s by %s: v%d was chosen%s. It's kept for good.", when(p.At), p.By, p.Version, into))
	}

	v.body = append(v.body, "", "Versions, oldest first (each one's note says what changed in it and why; the first's, what it tries out):")
	from := map[string][]string{} // where versions came from: theirs
	var froms []string
	for _, ver := range e.Versions {
		line := fmt.Sprintf("v%d", ver.N)
		if ver.BasedOn != 0 {
			line += fmt.Sprintf(" · based on v%d", ver.BasedOn)
		}
		line += " · " + when(ver.At) + " · " + provenance(ver)
		if files, size, ok := st.VersionSize(e, ver.N); ok {
			line += " · " + artifact.FilesWords(files, size)
		} else if !e.Gone && !e.Erased && !e.Away() {
			line += " · its files are missing"
		}
		if e.Home != nil && e.Lives() != "" && e.Home.Version == ver.N {
			line += " · in " + e.Home.Repo
		}
		if ver.N == asked {
			line += "  ← asked for"
		}
		v.body = append(v.body, "  "+line)
		note := ver.Note
		if note == "" {
			note = "(no note)"
		}
		for l := range strings.SplitSeq(note, "\n") {
			v.body = append(v.body, "      "+l)
		}
		if ver.From != "" {
			if _, ok := from[ver.From]; !ok {
				froms = append(froms, ver.From)
			}
			from[ver.From] = append(from[ver.From], fmt.Sprintf("v%d", ver.N))
		}
	}

	for _, f := range froms {
		vs := strings.Join(from[f], ", ")
		if conversationRef.MatchString(f) {
			v.add(f, "the conversation "+vs+" came from")
		} else if slug, id, ok := strings.Cut(f, "/"); ok {
			v.add("ws:"+slug, "the mission whose conversation "+id+" "+vs+" came from (ws show -m "+slug+" "+id+")")
		} else {
			v.add("ws:"+f, "the mission "+vs+" came from")
		}
	}
	if !e.Gone && !e.Erased && !e.Away() {
		show := asked
		if show == 0 {
			show = latest.N
		}
		// Every version's ref, the one asked for (or the latest) first:
		// nothing to work out by analogy.
		refs := []string{artifact.VersionRef(e.ID, show)}
		for _, v := range slices.Backward(e.Versions) {
			if n := v.N; n != show {
				refs = append(refs, artifact.VersionRef(e.ID, n))
			}
		}
		// The files of the version shown, where it's more than its page.
		list, _, _, _ := st.VersionFiles(context.Background(), e.ID, show)
		many := len(list) > 1
		if many {
			v.body = append(v.body, "", fmt.Sprintf("v%d's files (%s is its page):", show, artifact.IndexFile))
			for _, f := range list {
				v.body = append(v.body, fmt.Sprintf("  %s · %s", f.Path, artifact.Size(f.Size)))
			}
		}
		v.more = append(v.more, fmt.Sprintf("ws artifact show %s   prints v%d's page (and so for each: %s)", refs[0], show, strings.Join(refs, ", ")))
		if many {
			v.more = append(v.more, fmt.Sprintf("ws artifact show %s <file>   prints another of its files, by its path above", refs[0]))
		}
		v.more = append(v.more, fmt.Sprintf("ws artifact checkout %s   puts v%d's files in a folder, to work on", refs[0], show))
		v.more = append(v.more, fmt.Sprintf("ws artifact url %s   prints the address v%d runs at, to open in a browser", refs[0], show))
		switch {
		case e.Status == artifact.Discarded:
		case agentConversation() != "" && many:
			// A conversation's agent proposes: the user adds it.
			v.more = append(v.more, fmt.Sprintf("ws artifact add --propose --to %s --on v%d --note \"…\" <folder>   proposes a version from the folder you checked out and changed, based on v%d (ws artifact add --propose says how one's written)", e.ID, show, show))
		case agentConversation() != "":
			v.more = append(v.more, fmt.Sprintf("ws artifact add --propose --to %s --on v%d --note \"…\" - <<'EOF'   proposes a version, the whole page, based on v%d; or give a folder, for one with files (ws artifact add --propose says how one's written)", e.ID, show, show))
		default:
			v.more = append(v.more, fmt.Sprintf("ws artifact add <file|folder> --to %s --note \"…\"   adds a version, based on v%d (--on for another)", e.ID, latest.N))
		}
	}
	if said, err := ctx.SaidOf(store.Key{Name: e.ID}); err == nil {
		v.addSaid(saidSection{title: "Said of it", said: said})
	}
	return v, nil
}

var (
	conversationRef = regexp.MustCompile(`^w[1-9][0-9]*$`)
	// missionTalkRef is a mission's conversation, as a version's from
	// names it: glow/t3.
	missionTalkRef = regexp.MustCompile(`^[^/\s]+/t[1-9][0-9]*$`)
)

// provenance says who added a version, and where: by c1, crew on mission
// glow ("The glow"); by you, accepting proposal 2 in conversation w9
// ("…"), or in a mission's (glow/t3); by you, by hand.
func provenance(v artifact.Version) string {
	title := ""
	if v.FromTitle != "" {
		title = fmt.Sprintf(" (%q)", v.FromTitle)
	}
	switch {
	case v.From == "" && v.By == "you":
		return "by you, by hand"
	case v.From == "":
		return "by " + v.By
	case missionTalkRef.MatchString(v.From) && v.Proposal != 0:
		slug, id, _ := strings.Cut(v.From, "/")
		return fmt.Sprintf("by %s, accepting proposal %d in conversation %s on mission %s%s", v.By, v.Proposal, id, slug, title)
	case conversationRef.MatchString(v.From) && v.Proposal != 0:
		return fmt.Sprintf("by %s, accepting proposal %d in conversation %s%s", v.By, v.Proposal, v.From, title)
	case conversationRef.MatchString(v.From):
		return fmt.Sprintf("by %s, from conversation %s%s", v.By, v.From, title)
	case v.By == "you":
		return fmt.Sprintf("by you, for mission %s%s", v.From, title)
	}
	return fmt.Sprintf("by %s, crew on mission %s%s", v.By, v.From, title)
}
