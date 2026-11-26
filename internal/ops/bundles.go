package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/withworkspace/workspace/internal/mission"
)

// Verification bundles: what crew hand over to show their work is as its
// checks say (mission.Bundle). ws reads one strictly against the checks of
// the step its crew are on, looks at each file it names, copies them into
// the bundle's own folder, and keeps it on the mission with the commit its
// capsule is at.

// How large a bundle's file may be: an image or a video, and text.
const (
	bundleMediaMax = 50 << 20
	bundleTextMax  = 1 << 20
)

// BundleText is the content type a bundle's text is kept as.
const BundleText = "text/plain; charset=utf-8"

// HandOverBundle keeps the bundle crew wrote, for the step they're on: read
// against that step's checks and refused, with every problem and its line,
// when it isn't right. dir is where its crew stand, for the files it names.
// A bundle is of a commit, so it's refused while its capsule has changes
// that aren't committed.
func (w *Workspace) HandOverBundle(slug, crewID, written, dir string) (mission.Bundle, error) {
	h, err := w.HandOver(slug, crewID, written, dir)
	return h.Bundle, err
}

// HandedOver is a bundle handed over, and what follows of it.
type HandedOver struct {
	Bundle mission.Bundle
	// Next is what ws does next, for a verifier's: its bundle is its done,
	// so its step is settled on it.
	Next string
}

// HandOver is HandOverBundle, saying what follows. A verifier's bundle (one
// from crew on a Verify step) isn't the author's own: it's held to the
// commit its capsule was at when the verifier started, since a verifier
// changes nothing, and handing it over is the verifier's done, on which ws
// settles the step.
func (w *Workspace) HandOver(slug, crewID, written, dir string) (HandedOver, error) {
	b, verifier, err := w.keepBundle(slug, crewID, written, dir)
	if err != nil || !verifier {
		return HandedOver{Bundle: b}, err
	}
	if err := w.Missions.SetCrewStatus(slug, crewID, mission.CrewDone, "handed over "+b.ID+": "+b.Tally()); err != nil {
		return HandedOver{Bundle: b}, wrap(err)
	}
	return HandedOver{Bundle: b, Next: w.CrewDone(slug, crewID)}, nil
}

// keepBundle reads and keeps the bundle crew wrote, and says whether its
// crew are a verifier.
func (w *Workspace) keepBundle(slug, crewID, written, dir string) (mission.Bundle, bool, error) {
	none := mission.Bundle{}
	m, err := w.load(slug)
	if err != nil {
		return none, false, err
	}
	c, ok := m.CrewByID(crewID)
	if !ok {
		return none, false, fmt.Errorf("%w: mission %s has no crew %s", ErrNotFound, m.Slug, crewID)
	}
	if c.Role != "" {
		return none, false, refused("bundle-is-works", "a bundle is handed over by crew working on the mission, for the step they're on")
	}
	b := mission.Bundle{Crew: c.ID}
	var checks []mission.Check
	var at mission.StageState
	verifier := false
	brief, _ := w.Missions.ReadBrief(m.Slug)
	states, _ := m.StageProgress(brief)
	if st, ok := mission.StageFor(states, c.ID); ok && !st.Implicit {
		b.Step, b.Stage, at = st.N, st.Text, st
		if st.Step != nil {
			checks = st.Step.Checks
			verifier = st.Step.Type == mission.StepVerify
		}
	}
	b.Own = !verifier
	if c.Capsule != "" && c.Capsule != RootPlace {
		repo, name, _ := strings.Cut(c.Capsule, "/")
		capsule := filepath.Join(w.WS().RepoDir(repo), name)
		if info, err := os.Stat(capsule); err == nil && info.IsDir() {
			if changed := gitOut(capsule, "status", "--porcelain", "--untracked-files=no"); changed != "" {
				if verifier {
					return none, true, refused("verifier-changed", "%s has changes to its files (%s): a verifier changes nothing. Put them back as they were (git checkout), keep what you saw in the mission's private files folder, and hand it over again", c.Capsule, firstLine(changed))
				}
				return none, false, refused("bundle-uncommitted", "a bundle is of a commit, and %s has changes that aren't committed (%s): commit them, then hand it over again", c.Capsule, firstLine(changed))
			}
			b.Capsule, b.Commit = c.Capsule, gitOut(capsule, "rev-parse", "HEAD")
			if verifier {
				if err := verifierMayHandOver(at, b.Capsule, b.Commit); err != nil {
					return none, true, err
				}
			}
		}
	}
	// Each file it names, once: where it is, for copying once it reads.
	found := map[string]string{}
	files := func(path string) (string, int64, string) {
		abs, media, size, why := w.bundleFile(m.Slug, dir, path)
		if why == "" {
			found[path] = abs
		}
		return media, size, why
	}
	read, err := mission.ParseBundle(written, checks, files)
	if err != nil {
		return none, verifier, err
	}
	b.Summary, b.Try, b.Checks, b.NotVerified, b.Noticed = read.Summary, read.Try, read.Checks, read.NotVerified, read.Noticed
	b, err = w.Missions.KeepBundle(m.Slug, b, func(to string, b *mission.Bundle) error {
		names := map[string]string{} // a file copied, by where it was
		for _, e := range b.Files() {
			from := found[e.Path]
			if name, ok := names[from]; ok {
				e.File = name
				continue
			}
			name := freeName(to, filepath.Base(from))
			if err := copyFile(from, filepath.Join(to, name)); err != nil {
				return fmt.Errorf("keeping %s: %w", e.Path, err)
			}
			e.File, names[from] = name, name
		}
		return nil
	})
	return b, verifier, err
}

// bundleFile is a file a bundle names: where it is, what it is and how
// large, or why it can't be kept. A path is from where its crew stand, or
// in the mission's private files folder (mission:path, or a path found
// there).
func (w *Workspace) bundleFile(slug, dir, path string) (abs, media string, size int64, why string) {
	var tried []string
	switch rest, inMission := strings.CutPrefix(path, "mission:"); {
	case inMission:
		tried = []string{filepath.Join(w.Missions.FilesDir(slug), filepath.FromSlash(rest))}
	case filepath.IsAbs(path):
		tried = []string{filepath.Clean(path)}
	default:
		if dir != "" {
			tried = append(tried, filepath.Join(dir, filepath.FromSlash(path)))
		}
		tried = append(tried, filepath.Join(w.Missions.FilesDir(slug), filepath.FromSlash(path)))
	}
	var info os.FileInfo
	for _, p := range tried {
		if i, err := os.Stat(p); err == nil {
			abs, info = p, i
			break
		}
	}
	switch {
	case info == nil:
		return "", "", 0, "there's no such file, from where you are or in the mission's private files folder (" + w.Missions.FilesDir(slug) + ")"
	case !info.Mode().IsRegular():
		return "", "", 0, "it isn't a file"
	case info.Size() == 0:
		return "", "", 0, "it's empty"
	}
	size = info.Size()
	if media = MediaType(abs); media != "" {
		if size > bundleMediaMax {
			return "", "", 0, fmt.Sprintf("it's %s, and a bundle's image or video is %s at most: a shorter recording, or a smaller one", sizeWords(size), sizeWords(bundleMediaMax))
		}
		return abs, media, size, ""
	}
	if size > bundleTextMax {
		return "", "", 0, fmt.Sprintf("it's %s of text, and a bundle's is %s at most: keep the part that shows it", sizeWords(size), sizeWords(bundleTextMax))
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", "", 0, "it can't be read: " + err.Error()
	}
	if !utf8.Valid(data) || strings.ContainsRune(string(data), 0) {
		return "", "", 0, "it isn't an image (.png, .jpg, .gif, .webp, .svg, .avif), a video (.webm, .mp4, .mov) or text"
	}
	return abs, BundleText, size, ""
}

// sizeWords is a size in words: 3 KB, 12 MB.
func sizeWords(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%d MB", (n+(1<<19))>>20)
	case n >= 1<<10:
		return fmt.Sprintf("%d KB", (n+(1<<9))>>10)
	}
	return fmt.Sprintf("%d bytes", n)
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// freeName is a name for a file in dir that nothing there has: name, made
// plain, with -2, -3… before its extension where it's taken.
func freeName(dir, name string) string {
	name = strings.Trim(unsafeName.ReplaceAllString(name, "-"), "-.")
	if name == "" {
		name = "file"
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for n := 1; ; n++ {
		try := name
		if n > 1 {
			try = fmt.Sprintf("%s-%d%s", stem, n, ext)
		}
		if _, err := os.Lstat(filepath.Join(dir, try)); os.IsNotExist(err) {
			return try
		}
	}
}

// BundleView is a bundle as it's shown: with where its step is now, who
// made it, what took its place, and what ws ran and had judged of its
// step's checks.
type BundleView struct {
	mission.Bundle
	// StepN is its step's number now (0 if the plan no longer has it),
	// and StepName how it's listed.
	StepN    int
	StepName string
	// By is what its crew ran on (claude on sonnet).
	By string
	// ReplacedBy is the bundle that took its place.
	ReplacedBy string
	// StepChecks are its step's checks as the plan has them now, and Ran
	// the latest round of them in its capsule at its commit: the commands
	// ws ran, and what the judge said of those in words. A round of
	// another commit isn't of what the bundle shows.
	StepChecks []mission.Check
	Ran        *mission.CheckRound
	// Asked: the mission is at a step of yours that looks at it (a Review
	// step, or its own step with its checks yours to say), so it's yours
	// to approve, and approving it passes that step (AskedAt).
	Asked   bool
	AskedAt int
	// Dir is the folder its files are kept in.
	Dir string
}

// Bundles is the mission's bundles as they're shown, oldest first.
func (w *Workspace) Bundles(m *mission.Mission) []BundleView {
	if len(m.Bundles) == 0 {
		return nil
	}
	brief, _ := w.Missions.ReadBrief(m.Slug)
	states, _ := m.StageProgress(brief)
	out := make([]BundleView, 0, len(m.Bundles))
	for _, b := range m.Bundles {
		v := BundleView{Bundle: b, Dir: w.Missions.BundleDir(m.Slug, b.ID)}
		if cur, _, _, ok := w.waitsOn(m, b.ID); ok {
			v.Asked, v.AskedAt = true, cur.N
		}
		if c, ok := m.CrewByID(b.Crew); ok {
			v.By = c.Agent()
		}
		if by, ok := m.ReplacedBy(b.ID); ok {
			v.ReplacedBy = by.ID
		}
		for _, st := range states {
			if b.Stage == "" || st.Text != b.Stage {
				continue
			}
			v.StepN, v.StepName = st.N, st.Short()
			if st.Step != nil {
				v.StepChecks = st.Step.Checks
			}
			if st.Record != nil {
				for _, r := range slices.Backward(st.Record.Checks) {
					if r.Capsule == b.Capsule && r.Commit == b.Commit {
						v.Ran = &r
						break
					}
				}
			}
			break
		}
		out = append(out, v)
	}
	return out
}

// BundleFile is a file of one of the mission's bundles, by its name in
// the bundle's folder: where it is, and its content type. Only a file the
// bundle's record names, so this can't be pointed at any other.
func (w *Workspace) BundleFile(slug, id, name string) (path, media string, err error) {
	m, err := w.load(slug)
	if err != nil {
		return "", "", err
	}
	b, ok := m.Bundle(id)
	if !ok {
		return "", "", fmt.Errorf("%w: mission %s has no bundle %s", ErrNotFound, m.Slug, id)
	}
	for _, e := range b.Files() {
		if e.File == name && name != "" {
			return filepath.Join(w.Missions.BundleDir(m.Slug, b.ID), e.File), e.Media, nil
		}
	}
	return "", "", fmt.Errorf("%w: bundle %s has no file %s", ErrNotFound, id, name)
}

// BundleDone is your verdict of a bundle recorded, and what follows.
type BundleDone struct {
	Slug   string
	Bundle mission.Bundle
	Next   string
}

func (d BundleDone) String() string {
	said := "Approved " + d.Bundle.ID
	if d.Bundle.Verdict == mission.BundleSentBack {
		said = "Sent " + d.Bundle.ID + " back"
	}
	if d.Next != "" {
		said += ": " + d.Next
	}
	return said + "."
}

// yourBundle is a bundle of the mission's that's yours to give a verdict
// of: you're the one asking, and nothing has taken its place.
func (w *Workspace) yourBundle(slug, id string, by Actor) (*mission.Mission, mission.Bundle, error) {
	if !by.IsYou() {
		return nil, mission.Bundle{}, refused("bundle-is-yours", "a bundle is the user's to approve or send back, as a review is: say what you found in your handoff, or ask them")
	}
	m, err := w.load(slug)
	if err != nil {
		return nil, mission.Bundle{}, err
	}
	b, ok := m.Bundle(id)
	if !ok {
		return nil, mission.Bundle{}, fmt.Errorf("%w: mission %s has no bundle %s", ErrNotFound, m.Slug, id)
	}
	if by, ok := m.ReplacedBy(b.ID); ok {
		return nil, mission.Bundle{}, refused("bundle-replaced", "%s has taken %s's place: it's the one to look at", by.ID, b.ID)
	}
	return m, b, nil
}

// waitsOn is the step of yours the mission is at, when it looks at bundle
// id: the step approving the bundle passes, with everything it looks at.
// That's a Review step, or the bundle's own step when its checks are
// yours to say: ws has asked you (checks: the task it asked with), or they
// came round not all met and its crew have left it there, which is yours
// to pass whether or not anyone's asked yet.
func (w *Workspace) waitsOn(m *mission.Mission, id string) (cur mission.StageState, looks []mission.Look, checks *mission.Task, ok bool) {
	cur, ok = currentStep(w.Missions, m)
	if !ok {
		return mission.StageState{}, nil, nil, false
	}
	brief, _ := w.Missions.ReadBrief(m.Slug)
	states, _ := m.StageProgress(brief)
	if cur.Step.Type == mission.StepReview {
		looks = m.Looks(states, cur.N)
	} else if t, open := m.StepTask(cur.N); open && t.Verb == mission.TaskChecks {
		looks, checks = m.ChecksLooks(states, cur.N), &t
	} else if _, _, left := m.ChecksUnmet(brief); left {
		// One you've sent back isn't there to approve: its crew are to
		// hand over another.
		if b, ok := m.Bundle(id); ok && b.Verdict != mission.BundleSentBack {
			looks = m.ChecksLooks(states, cur.N)
		}
	}
	if slices.ContainsFunc(looks, func(l mission.Look) bool { return l.Bundle == id }) {
		return cur, looks, checks, true
	}
	return mission.StageState{}, nil, nil, false
}

// ApproveBundle is your approval of a bundle, as you'd approve a review.
// Where the mission is at a Review step that looks at it, and every bundle
// that step looks at is approved, the step passes, as going on does: the
// bundle is what's to approve, with the work it shows beside it. So does
// its own step when its checks are yours to say, as passing it as it is
// does.
func (w *Workspace) ApproveBundle(slug, id string, by Actor) (BundleDone, error) {
	m, b, err := w.yourBundle(slug, id, by)
	if err != nil {
		return BundleDone{}, err
	}
	if b.Verdict == mission.BundleApproved {
		return BundleDone{}, refused("bundle-approved", "%s is approved already", b.ID)
	}
	b, err = w.Missions.SetBundleVerdict(m.Slug, b.ID, mission.BundleApproved, nil)
	if err != nil {
		return BundleDone{}, wrap(err)
	}
	w.youActed(m.Slug, by)
	out := BundleDone{Slug: m.Slug, Bundle: b}
	m, err = w.load(m.Slug)
	if err != nil {
		return out, nil
	}
	cur, looks, checks, ok := w.waitsOn(m, b.ID)
	if !ok {
		return out, nil
	}
	var waits []string
	for _, l := range looks {
		if other, ok := m.Bundle(l.Bundle); ok && other.Verdict != mission.BundleApproved {
			waits = append(waits, other.ID)
		}
	}
	if len(waits) > 0 {
		out.Next = fmt.Sprintf("step %d also looks at %s, and passes once that's approved too", cur.N, strings.Join(waits, ", "))
		return out, nil
	}
	why := "you approved " + b.ID
	if cur.Step.Type != mission.StepReview {
		why += ", its checks as they were"
	}
	if checks != nil {
		// What ws asked of its checks is answered: kept for the
		// orchestrator, the task closing as the step passes.
		logFailed(w.Missions.AnswerTask(m.Slug, checks.N, fmt.Sprintf("%s: approved %s", mission.ChecksPass, b.ID), by.By(), true))
	}
	acc, err := w.AcceptStage(m.Slug, AsYou, why)
	if err != nil {
		out.Next = fmt.Sprintf("step %d isn't passed yet: %v", cur.N, err)
		return out, nil
	}
	out.Next = fmt.Sprintf("step %d passed; %s", cur.N, acc.Next)
	return out, nil
}

// SendBackBundle sends a bundle back with what you said isn't right, a
// check each (0: of it as a whole): it's recorded on the bundle, and goes
// to the crew who made the work, with each check and what the bundle
// showed for it, as a Work step's unmet checks do. They put it right and
// hand over another, which takes this one's place. A Review step looking
// at it stays yours. A Work step whose checks were yours to say goes back
// with it, as sending it back from what ws asked does: its crew report
// done again, and the checks run again.
func (w *Workspace) SendBackBundle(slug, id string, notes []mission.BundleNote, by Actor) (BundleDone, error) {
	m, b, err := w.yourBundle(slug, id, by)
	if err != nil {
		return BundleDone{}, err
	}
	var kept []mission.BundleNote
	for _, n := range notes {
		n.Text = strings.TrimSpace(n.Text)
		if n.Text == "" {
			continue
		}
		if n.Check != 0 && !slices.ContainsFunc(b.Checks, func(c mission.BundleCheck) bool { return c.N == n.Check }) {
			return BundleDone{}, invalid("%s has no check %d: it has %s", b.ID, n.Check, bundleCheckNumbers(b))
		}
		kept = append(kept, n)
	}
	if len(kept) == 0 {
		return BundleDone{}, invalid("say what's not right, of a check (by its number) or of the bundle as a whole: that's what goes back to its crew")
	}
	b, err = w.Missions.SetBundleVerdict(m.Slug, b.ID, mission.BundleSentBack, kept)
	if err != nil {
		return BundleDone{}, wrap(err)
	}
	w.youActed(m.Slug, by)
	out := BundleDone{Slug: m.Slug, Bundle: b}
	brief, _ := w.Missions.ReadBrief(m.Slug)
	states, _ := m.StageProgress(brief)
	st, ok := mission.StageFor(states, b.Crew)
	for _, s := range states {
		if b.Stage != "" && s.Text == b.Stage {
			st, ok = s, true
		}
	}
	if !ok {
		out.Next = "it has no step to send it back at: tell " + b.Crew + " yourself"
		return out, nil
	}
	text := bundleBackText(b)
	if cur, _, checks, waits := w.waitsOn(m, b.ID); waits && checks != nil && cur.N == st.N && cur.Step.Type == mission.StepWork {
		logFailed(w.Missions.AnswerTask(m.Slug, checks.N, fmt.Sprintf("%s: %s isn't right yet", mission.ChecksSendBack, b.ID), by.By(), false))
		text += fmt.Sprintf(" Then report done again with a handoff (ws mission done): ws runs step %d's checks again.", st.N)
	}
	said, err := w.carryOn(m, st, b.Crew, text, by)
	if err != nil {
		out.Next = fmt.Sprintf("couldn't reach %s (%v): tell them yourself", b.Crew, err)
		return out, nil
	}
	out.Next = said
	return out, nil
}

func bundleCheckNumbers(b mission.Bundle) string {
	var ns []string
	for _, c := range b.Checks {
		ns = append(ns, fmt.Sprint(c.N))
	}
	return strings.Join(ns, ", ")
}

// bundleBackText is what crew are told when the bundle they handed over
// comes back to them.
func bundleBackText(b mission.Bundle) string {
	var t strings.Builder
	fmt.Fprintf(&t, "The user looked at the bundle you handed over (%s) and sent it back: it's not right yet.\n\n", b.ID)
	for _, n := range b.Notes {
		if n.Check == 0 {
			fmt.Fprintf(&t, "- Of it as a whole: %s\n", n.Text)
			continue
		}
		for _, c := range b.Checks {
			if c.N != n.Check {
				continue
			}
			fmt.Fprintf(&t, "- Check %d (%s): %s\n", c.N, firstNonEmpty(oneLine(c.Check), oneLine(c.Said)), n.Text)
			var showed []string
			for _, e := range c.Shows {
				showed = append(showed, firstNonEmpty(e.Caption, e.Path))
			}
			if len(showed) > 0 {
				fmt.Fprintf(&t, "  The bundle showed: %s.\n", strings.Join(showed, "; "))
			}
		}
	}
	fmt.Fprintf(&t, "\nPut it right, commit, and hand over a new bundle (ws mission bundle --set -; ws show %s has this one): it takes %s's place, and the user looks again.", b.ID, b.ID)
	return t.String()
}

// MarkBundleLooked marks one of a bundle's checks as looked at by you, or
// not: yours to keep your place with, as a file viewed in a review.
func (w *Workspace) MarkBundleLooked(slug, id, key string, looked bool, by Actor) error {
	if !by.IsYou() {
		return refused("bundle-is-yours", "what's been looked at in a bundle is the user's to mark")
	}
	return wrap(w.Missions.MarkBundleLooked(slug, id, key, looked))
}
