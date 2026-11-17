package cli

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/mission"
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
	case e.Gone:
		state += ", its folder gone"
	case e.Erased:
		state += ", its files deleted for good"
	case expirable && !now.Before(expires):
		state = "expired"
	case cfg.Expiring(e, now):
		state += ", " + artifact.ExpiresWords(expires, now)
	}
	kept := "on this machine"
	switch {
	case e.InRepo() && e.Status == artifact.Discarded:
		kept = "in " + e.Place + "'s history"
	case e.InFlight() != "":
		state += ", in flight"
		kept = "in " + e.InFlight() + ", not landed"
	case e.InRepo():
		kept = "in " + e.Place
	}
	head := fmt.Sprintf("Artifact %s — %q (%s, %s; %d %s, the latest v%d)", e.ID, e.Title, state, kept, n, plural(n, "version"), latest.N)
	if asked != 0 {
		head = fmt.Sprintf("Artifact %s, v%d of %d — %q (%s, %s)", e.ID, asked, n, e.Title, state, kept)
	}
	v := &view{head: head}
	v.body = append(v.body, "An artifact is one HTML file, with its CSS and JS inline, that shows an idea, to look at and use. Each version is a whole file, never changed once written.")
	dir := st.Path(e)
	history := e.InRepo() && e.Status == artifact.Discarded
	switch {
	case history:
		v.body = append(v.body, fmt.Sprintf("It was kept in %s, the repo the workspace keeps artifacts in, at %s, until a commit on its default branch took it away. History keeps it, at %s (git show %s:%s/v<n>.html is a version). Made %s.", e.Place, e.Place+":"+e.Folder+"/", keptAt(e), shortCommit(e), e.Folder, when(e.Made)))
	case e.Erased:
		v.body = append(v.body, fmt.Sprintf("It was kept on this machine (.ws/artifacts/%s/), and its files have been deleted for good: what's below is what the records kept, and there are no files to read.", e.Folder))
	case e.Gone && e.InRepo():
		v.body = append(v.body, fmt.Sprintf("Its folder has gone from %s (it was %s), on its default branch and in the mission capsules of it: what's below is what the records kept, and there are no files to read.", e.Place, e.Where()))
	case e.Gone:
		v.body = append(v.body, fmt.Sprintf("Its folder has gone from .ws/artifacts (it was %s): what's below is what the records kept, and there are no files to read.", dir))
	case e.InFlight() != "":
		c, _ := st.Repo.CapsuleOf(e.InFlight())
		v.body = append(v.body, fmt.Sprintf("Kept in %s, the repo the workspace keeps artifacts in, but not landed: it's in %s (%s), mission %s's capsule, and lands with it. %s is its record, and v<n>.html each version. Made %s. Name it for other people by its folder, %s, once it's landed: a3 is this machine's.", e.Place, e.InFlight(), dir, c.Mission, artifact.RecordFile, when(e.Made), e.Place+":"+e.Folder+"/"))
	case e.InRepo():
		v.body = append(v.body, fmt.Sprintf("Kept in %s, the repo the workspace keeps artifacts in, at %s: on its default branch, as last fetched (%s). %s is its record, and v<n>.html each version. Made %s. Name it for other people by its folder, %s: its ref (%s) is this machine's.", e.Place, e.Where(), dir, artifact.RecordFile, when(e.Made), e.Where(), e.ID))
	default:
		v.body = append(v.body, fmt.Sprintf("Kept in %s (this machine only): %s is its record, and v<n>.html each version. Made %s.", dir, artifact.RecordFile, when(e.Made)))
	}
	if fly := e.Flying(); len(fly) > 0 && e.InFlight() == "" {
		var vs []string
		for _, f := range fly {
			vs = append(vs, fmt.Sprintf("v%d", f.N))
		}
		v.body = append(v.body, fmt.Sprintf("In flight: %s, crew's, in a capsule and not landed (each says which below).", strings.Join(vs, ", ")))
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
		switch {
		case e.Restorable() && !e.InRepo():
			v.body = append(v.body, fmt.Sprintf("Its folder is in %s until %s, when it's deleted for good: ws artifact restore %s puts it back (the user only).", e.Where(), d.At.Add(artifact.KeepDiscarded).Local().Format("2006-01-02"), e.ID))
		case e.Restorable():
			v.body = append(v.body, fmt.Sprintf("ws artifact restore %s puts it back, from %s, in a new commit (the user only).", e.ID, keptAt(e)))
		}
	}
	if expirable && e.Status == artifact.Live {
		if !now.Before(expires) {
			how := "the daemon discards it"
			if e.InRepo() {
				how = "it waits for the user to clear it (ws artifact clear)"
			}
			v.body = append(v.body, fmt.Sprintf("Expired %s: it's had no new version for %s, and %s, unless they promote it.", when(expires), cfg.ExpiryWords(), how))
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
		if ver.Capsule != "" {
			line += " · in flight, in " + ver.Capsule
			if ver.Was != 0 {
				line += fmt.Sprintf(" (as v%d there: v%d landed first, so it lands as v%d)", ver.Was, ver.Was, ver.N)
			}
		}
		if size, ok := st.VersionSize(e, ver.N); ok {
			line += " · " + artifact.Size(size)
		} else if !e.Gone && !e.Erased {
			line += " · its file is missing"
		}
		if ver.N == asked {
			line += "  ← asked for"
		}
		v.body = append(v.body, "  "+line)
		note := ver.Note
		if note == "" {
			note = "(no note)"
		}
		for _, l := range strings.Split(note, "\n") {
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
	if !e.Gone && !e.Erased {
		show := asked
		if show == 0 {
			show = latest.N
		}
		// Every version's ref, the one asked for (or the latest) first:
		// nothing to work out by analogy.
		refs := []string{artifact.VersionRef(e.ID, show)}
		for i := len(e.Versions) - 1; i >= 0; i-- {
			if n := e.Versions[i].N; n != show {
				refs = append(refs, artifact.VersionRef(e.ID, n))
			}
		}
		v.more = append(v.more, fmt.Sprintf("ws artifact show %s   prints v%d's HTML (and so for each: %s)", refs[0], show, strings.Join(refs, ", ")))
		if !history {
			v.more = append(v.more, fmt.Sprintf("%s   is its file (v<n>.html beside it, each version's)", st.VersionPath(e, show)))
		}
	}
	switch {
	case e.Status == artifact.Discarded:
	case agentConversation() != "":
		// A conversation's agent proposes: the user adds it.
		v.more = append(v.more, fmt.Sprintf("ws artifact add --propose --to %s --on v%d --note \"…\" - <<'EOF'   proposes a version, the whole file, based on v%d (ws artifact add --propose says how one's written)", e.ID, latest.N, latest.N))
	default:
		// Yours goes on the default branch: based on what's landed there.
		on := latest.N
		if latest.Capsule != "" && os.Getenv(mission.EnvCrew) == "" {
			on = 0
			for _, ver := range e.Versions {
				if ver.Capsule == "" && ver.N > on {
					on = ver.N
				}
			}
		}
		v.more = append(v.more, fmt.Sprintf("ws artifact add <file> --to %s --note \"…\"   adds a version, based on v%d (--on for another)", e.ID, on))
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

// shortCommit is the commit history keeps an artifact discarded in a repo
// in, short.
func shortCommit(e artifact.Entry) string {
	if e.Discarded == nil {
		return ""
	}
	c := e.Discarded.Commit
	if len(c) > 7 {
		c = c[:7]
	}
	return c
}

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
