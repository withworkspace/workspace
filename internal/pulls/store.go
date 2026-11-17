package pulls

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/withworkspace/workspace/internal/review"
)

// What ws keeps of a pull request, in the workspace's records
// (records.go): as last seen, what it says it does, your last review of it
// here, whether it's docked and by whom; and your reviews of it (as a
// mission's: drafts, submitted reviews, viewed files), in the reviews'
// own. A stack isn't kept: it's how PRs are shown, and its layers change.

// Root is where ws kept what it knew of pull requests, as files, before
// the records.
func Root(root string) string { return filepath.Join(root, ".ws", "prs") }

// Dir is a pull request's folder under Root: what its reviews are kept
// beside.
func Dir(root, repo string, number int) string {
	return filepath.Join(Root(root), repo, strconv.Itoa(number))
}

// Reviews is your reviews of a pull request.
func Reviews(root, repo string, number int) review.Store {
	return review.At(root, Dir(root, repo, number), Key(repo, number))
}

// Record is what's kept of a pull request (pr.json, as it was a file).
type Record struct {
	// Title and Branch are as last seen, for saying what a capsule docked
	// for it was for once it's gone from GitHub's list.
	Title  string `json:"title,omitempty"`
	Branch string `json:"branch,omitempty"`
	// Body is what it says it does, as last read: its words, for finding
	// it by.
	Body string `json:"body,omitempty"`
	// Head is its commit as last fetched.
	Head string `json:"head,omitempty"`
	// Reviewed is your last review of it, made here.
	Reviewed *Reviewed `json:"reviewed,omitempty"`
	// Docked is the capsule it's checked out in here, when ws knows who
	// docked it.
	Docked *Docked `json:"docked,omitempty"`
}

// Reviewed is a review of yours: the commit it was of (kept in the clone
// as refs/ws/pr/<number>/reviewed/<id>, so it's there after a force-push),
// where that left its base, and each file's fingerprint then.
type Reviewed struct {
	ID      string    `json:"id"`
	Verdict string    `json:"verdict,omitempty"`
	Head    string    `json:"head"`
	Base    string    `json:"base"`
	At      time.Time `json:"at"`
	URL     string    `json:"url,omitempty"`
	// Prints are each file's change as you reviewed it (review.Fingerprints).
	Prints map[string]string `json:"prints,omitempty"`
}

// Docked is a PR checked out here. By is "ws" for a capsule ws docked to
// answer a question (its to tidy up: not boarded, removed once the PR
// merges or closes), "you" once it's yours.
type Docked struct {
	Capsule string    `json:"capsule"`
	Branch  string    `json:"branch"`
	By      string    `json:"by"`
	At      time.Time `json:"at"`
}

// Who docks a capsule.
const (
	ByWS  = "ws"
	ByYou = "you"
)

// Kept names a PR ws keeps something of.
type Kept struct {
	Repo   string
	Number int
	Record *Record
}

// Litter is Root, if it's there: nothing's kept in files now, so what's
// there is from before the records, which nothing reads (ws doctor).
func Litter(root string) []string {
	if _, err := os.Stat(Root(root)); err == nil {
		return []string{Root(root)}
	}
	return nil
}
