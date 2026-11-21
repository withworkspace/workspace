package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Artifacts point at what a mission produced or relies on. A reference
// (mission.Artifact.Ref) is one of:
//
//	repo:path            a file on the repo's default branch (read from .ground)
//	repo/capsule:path    a file in one of the mission's capsules
//	mission:path         a file in the mission's own files/ folder
//	repo#N               a PR or issue
//	a3                   an artifact (internal/artifact): a versioned HTML prototype
//	https://…            a link
//
// Where an artifact is now (draft, in review, landed, missing) is worked out
// each time from the capsules, their PRs and the files on disk.

type ArtifactLoc int

const (
	LocLink ArtifactLoc = iota
	LocGround
	LocCapsule
	LocMission
	LocPR
	// LocArtifact is an artifact of the workspace's (a3): a versioned HTML
	// prototype, which crew's ws artifact add records.
	LocArtifact
)

type ArtifactRef struct {
	Loc     ArtifactLoc
	Repo    string
	Capsule string // the capsule's name, without its repo
	Path    string // slash-separated, relative
	Number  int
	URL     string
	ID      string // an artifact's (a3)
}

func (r ArtifactRef) String() string {
	switch r.Loc {
	case LocGround:
		return r.Repo + ":" + r.Path
	case LocCapsule:
		return r.Repo + "/" + r.Capsule + ":" + r.Path
	case LocMission:
		return "mission:" + r.Path
	case LocPR:
		return r.Repo + "#" + strconv.Itoa(r.Number)
	case LocArtifact:
		return r.ID
	}
	return r.URL
}

func (r ArtifactRef) IsFile() bool {
	return r.Loc == LocGround || r.Loc == LocCapsule || r.Loc == LocMission
}

var KindShape = regexp.MustCompile(`^[a-z][a-z0-9-]{0,23}$`)

// ParseArtifactRef reads a stored reference. It trusts its form (it was
// normalised when added) and doesn't check that repos or files exist. A
// document's ref is written as ws names things (internal/ref), but for a
// file on the default branch, which is stored as repo:path.
func ParseArtifactRef(s string) ArtifactRef {
	switch {
	case strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://"):
		return ArtifactRef{Loc: LocLink, URL: s}
	case strings.HasPrefix(s, "mission:"):
		return ArtifactRef{Loc: LocMission, Path: strings.TrimPrefix(s, "mission:")}
	}
	if repo, n, ok := ref.ParsePR(s); ok {
		return ArtifactRef{Loc: LocPR, Repo: repo, Number: n}
	}
	if id, v, ok := artifact.ParseRef(s); ok && v == 0 {
		return ArtifactRef{Loc: LocArtifact, ID: id}
	}
	// Stored whole: what's after the colon is the path, lines and all.
	if where, path, ok := strings.Cut(s, ":"); ok && path != "" {
		if r, ok := ref.Parse(where + ":x"); ok && r.InCapsule() {
			return ArtifactRef{Loc: LocCapsule, Repo: r.Repo, Capsule: r.Worktree, Path: path}
		}
		if !strings.Contains(where, "/") {
			return ArtifactRef{Loc: LocGround, Repo: where, Path: path}
		}
	}
	return ArtifactRef{Loc: LocLink, URL: s}
}

// DefaultKind is the kind an artifact gets when none is given.
func DefaultKind(r ArtifactRef) string {
	switch r.Loc {
	case LocPR:
		return "pr"
	case LocLink:
		return "link"
	case LocArtifact:
		return ArtifactKind
	}
	return "doc"
}

// ArtifactView is an artifact with where it is now.
type ArtifactView struct {
	mission.Artifact
	Parsed ArtifactRef
	// State is where it is now: draft, in review, landed, missing (files);
	// private (mission files); open, merged, pr (PRs); live, promoted,
	// discarded (artifacts); link.
	State string
	Path  string // the local file to read, for files
	URL   string // where to open or share it, when there's one
}

// ResolveArtifacts works out where each of the mission's artifacts is now,
// from its capsules (and their PRs) and the files on disk.
func ResolveArtifacts(ws *workspace.Workspace, store mission.Store, v MissionView) []ArtifactView {
	out := make([]ArtifactView, 0, len(v.M.Artifacts))
	for _, a := range v.M.Artifacts {
		out = append(out, ResolveArtifact(ws, store, v, a))
	}
	return out
}

func ResolveArtifact(ws *workspace.Workspace, store mission.Store, v MissionView, a mission.Artifact) ArtifactView {
	r := ParseArtifactRef(a.Ref)
	av := ArtifactView{Artifact: a, Parsed: r}
	if av.Kind == "" {
		av.Kind = DefaultKind(r)
	}
	repoURL := ws.RemoteOf(r.Repo).URL()
	onGround := func() (string, bool) {
		p := filepath.Join(ws.MainWorktree(r.Repo), filepath.FromSlash(r.Path))
		return p, fileExists(p)
	}
	landed := func(p string) {
		av.State, av.Path = "landed", p
		av.URL = repoURL + "/blob/" + ws.DefaultBranch + "/" + r.Path
	}
	switch r.Loc {
	case LocLink:
		av.State, av.URL = "link", r.URL
	case LocPR:
		av.State, av.URL = "pr", fmt.Sprintf("%s/pull/%d", repoURL, r.Number)
		if av.Kind == "issue" {
			av.URL = fmt.Sprintf("%s/issues/%d", repoURL, r.Number)
		}
		for _, c := range v.Capsules {
			if c.Repo == r.Repo && c.PR != nil && c.PR.Number == r.Number {
				av.State = map[bool]string{true: "merged", false: "open"}[c.PRS == mission.PRMerged]
			}
		}
	case LocArtifact:
		// As the records last had it: a view of many doesn't look in
		// each folder.
		av.State = "missing"
		if e, ok := (artifact.Store{Root: ws.Root}).Known(r.ID); ok && !e.Gone {
			av.State = string(e.Status)
		}
	case LocMission:
		av.Path = filepath.Join(store.FilesDir(v.M.Slug), filepath.FromSlash(r.Path))
		av.State = map[bool]string{true: "private", false: "missing"}[fileExists(av.Path)]
	case LocGround:
		if p, ok := onGround(); ok {
			landed(p)
		} else {
			av.State, av.Path = "missing", p
		}
	case LocCapsule:
		groundPath, groundHas := onGround()
		var capsule *CapsuleView
		for i := range v.Capsules {
			if c := &v.Capsules[i]; c.Repo == r.Repo && c.Name == r.Capsule && c.Exists {
				capsule = c
			}
		}
		if capsule == nil { // burned (or detached): it's on the default branch now, or nowhere
			if groundHas {
				landed(groundPath)
			} else {
				av.State = "missing"
			}
			break
		}
		capPath := filepath.Join(capsule.Path, filepath.FromSlash(r.Path))
		capHas := fileExists(capPath)
		// Landed: its PR merged, or (a repo with none) its commits were
		// pushed to the default branch.
		done := capsule.PRS == mission.PRMerged || capsule.Landed != nil
		switch {
		case done && groundHas:
			landed(groundPath)
		case done && capHas:
			landed(capPath) // .ground hasn't caught up yet
		case !capHas && groundHas:
			landed(groundPath)
		case !capHas:
			av.State = "missing"
		case capsule.PR != nil:
			av.State, av.Path, av.URL = "in review", capPath, capsule.PR.URL
		default:
			av.State, av.Path = "draft", capPath
		}
	}
	return av
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// NormalizeArtifact turns what someone typed (in dir, for a relative
// path) into a reference for mission
// m: a reference, a URL (a workspace repo's PR or issue URL becomes repo#N),
// or a path, classified by where it sits. Files must exist; a capsule must
// be one of the mission's. Errors say what to do instead.
func (w *Workspace) NormalizeArtifact(m *mission.Mission, input, dir string) (ArtifactRef, error) {
	s := strings.TrimSpace(input)
	switch {
	case s == "":
		return ArtifactRef{}, errors.New("say what to record: a file, a reference like records:adr/0001.md or api#88, or a URL")
	case strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://"):
		if g, ok := ref.ParseGitHub(s); ok && (g.What == "pull" || g.What == "issues") && len(g.Segs) > 0 {
			if repo, ok := w.WS().RepoAt(g.Org, g.Repo); ok {
				if n, err := strconv.Atoi(g.Segs[0]); err == nil && n > 0 {
					return ArtifactRef{Loc: LocPR, Repo: repo, Number: n}, nil
				}
			}
		}
		return ArtifactRef{Loc: LocLink, URL: s}, nil
	case strings.HasPrefix(s, "mission:"):
		rel, err := cleanArtifactPath(strings.TrimPrefix(s, "mission:"))
		if err != nil {
			return ArtifactRef{}, err
		}
		r := ArtifactRef{Loc: LocMission, Path: rel}
		return r, w.artifactFileExists(m, r)
	}
	if id, _, ok := artifact.ParseRef(s); ok {
		if _, err := w.Artifacts().Get(context.Background(), id); err != nil {
			return ArtifactRef{}, fmt.Errorf("there's no artifact %s here: ws artifact list lists them", id)
		}
		return ArtifactRef{Loc: LocArtifact, ID: id}, nil
	}
	if name, n, ok := ref.ParsePR(s); ok {
		if repo, ok := w.WS().ResolveAlias(name); ok {
			return ArtifactRef{Loc: LocPR, Repo: repo, Number: n}, nil
		}
	}
	if where, path, ok := strings.Cut(s, ":"); ok {
		repoName, capsule, hasCapsule := strings.Cut(where, "/")
		if repo, ok := w.WS().ResolveAlias(repoName); ok {
			rel, err := cleanArtifactPath(path)
			if err != nil {
				return ArtifactRef{}, err
			}
			r := ArtifactRef{Loc: LocGround, Repo: repo, Path: rel}
			if hasCapsule {
				r.Loc, r.Capsule = LocCapsule, capsule
				if !m.HasCapsule(repo, capsule) {
					return ArtifactRef{}, fmt.Errorf("%s/%s isn't one of mission %s's capsules: attach it with ws mission attach %s %s", repo, capsule, m.Slug, repo, capsule)
				}
			}
			return r, w.artifactFileExists(m, r)
		}
	}
	return w.artifactFromPath(m, s, dir)
}

// artifactFromPath classifies a file path by where it sits: one of the
// mission's capsules, a repo's .ground, or the mission's files folder.
func (w *Workspace) artifactFromPath(m *mission.Mission, p, dir string) (ArtifactRef, error) {
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	if _, err := os.Stat(p); err != nil {
		return ArtifactRef{}, fmt.Errorf("no file %s: record a file that exists, a reference (repo:path, repo#N) or a URL", p)
	}
	abs := physicalPath(p)
	store := w.Missions
	if rel, ok := under(physicalPath(store.FilesDir(m.Slug)), abs); ok {
		return ArtifactRef{Loc: LocMission, Path: filepath.ToSlash(rel)}, nil
	}
	rel, ok := under(physicalPath(w.WS().ReposDir()), abs)
	if ok {
		parts := strings.SplitN(filepath.ToSlash(rel), "/", 3)
		if len(parts) == 3 && slices.Contains(w.WS().RepoNames, parts[0]) {
			repo, tree, file := parts[0], parts[1], parts[2]
			switch {
			case tree == workspace.GroundDir:
				return ArtifactRef{Loc: LocGround, Repo: repo, Path: file}, nil
			case strings.HasPrefix(tree, "."):
				// .bare, .silo: not somewhere work is recorded from.
			case m.HasCapsule(repo, tree):
				return ArtifactRef{Loc: LocCapsule, Repo: repo, Capsule: tree, Path: file}, nil
			default:
				return ArtifactRef{}, fmt.Errorf("%s/%s isn't one of mission %s's capsules: attach it with ws mission attach %s %s, or record the file once it's on the default branch (%s:%s)", repo, tree, m.Slug, repo, tree, repo, file)
			}
		}
	}
	return ArtifactRef{}, fmt.Errorf("%s isn't in one of the mission's capsules, a .ground, or the mission's files (%s): copy it into the mission's files to keep it privately, or record a URL", p, store.FilesDir(m.Slug))
}

// under reports whether p is inside dir, and its path relative to dir.
func under(dir, p string) (string, bool) {
	rel, err := filepath.Rel(dir, p)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

// cleanArtifactPath checks a path inside a reference stays inside where it
// points.
func cleanArtifactPath(p string) (string, error) {
	c := filepath.ToSlash(filepath.Clean(strings.TrimPrefix(p, "/")))
	if c == "." || c == ".." || strings.HasPrefix(c, "../") {
		return "", fmt.Errorf("%q isn't a path inside the repo or the mission's files", p)
	}
	return c, nil
}

// artifactFileExists checks that a file reference points at a file, saying
// where it looked if it doesn't.
func (w *Workspace) artifactFileExists(m *mission.Mission, r ArtifactRef) error {
	var path string
	switch r.Loc {
	case LocGround:
		path = filepath.Join(w.WS().MainWorktree(r.Repo), filepath.FromSlash(r.Path))
	case LocCapsule:
		path = filepath.Join(w.WS().RepoDir(r.Repo), r.Capsule, filepath.FromSlash(r.Path))
	case LocMission:
		path = filepath.Join(w.Missions.FilesDir(m.Slug), filepath.FromSlash(r.Path))
	default:
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("no file %s (for %s)", path, r)
	}
	return nil
}

// physicalPath is p with its symlinks resolved, where they can be.
func physicalPath(p string) string {
	if real, err := filepath.EvalSymlinks(p); err == nil {
		return real
	}
	return p
}

// ArtifactLine is one artifact, for agents: kind, state, reference, label,
// and where to read and open it.
func ArtifactLine(av ArtifactView) string {
	line := fmt.Sprintf("[%s, %s] %s", av.Kind, av.State, av.Ref)
	if av.Label != "" {
		line += " — " + av.Label
	}
	if av.Path != "" && av.State != "missing" {
		line += " — read: " + av.Path
	}
	if av.URL != "" && av.Parsed.Loc != LocLink {
		line += " — " + av.URL
	}
	if av.Parsed.Loc == LocArtifact && av.State != "missing" {
		line += " — ws show " + av.Ref
	}
	return line
}
