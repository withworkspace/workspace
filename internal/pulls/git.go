package pulls

import (
	"fmt"

	"github.com/withworkspace/workspace/internal/git"
)

// A PR's commits are read from git, not GitHub's API: ws fetches its head
// into the repo's clone, out of the way of branches, so reviewing needs no
// capsule and the files around a change can be read in full.

// HeadRef is where a PR's head is kept in its repo's clone.
func HeadRef(number int) string { return fmt.Sprintf("refs/ws/pr/%d/head", number) }

// ReviewedRef is where the commit a review was of is kept, so it's still
// there after a force-push.
func ReviewedRef(number int, id string) string {
	return fmt.Sprintf("refs/ws/pr/%d/reviewed/%s", number, id)
}

// BaseRef is the remote branch a PR is for, as the clone tracks it.
func BaseRef(branch string) string { return "origin/" + branch }

// Fetch brings a PR's head and the branch it's for into the clone at dir,
// and answers the head's commit. GitHub keeps every PR's head at
// refs/pull/<number>/head, forks' included; a remote without it (not
// GitHub) is asked for the PR's branch instead.
func Fetch(dir string, number int, branch, base string) (string, error) {
	head := HeadRef(number)
	specs := []string{}
	if base != "" {
		specs = append(specs, fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", base, base))
	}
	_, err := git.Line(dir, append([]string{"fetch", "--quiet", "--no-tags", "origin", fmt.Sprintf("+refs/pull/%d/head:%s", number, head)}, specs...)...)
	if err != nil && branch != "" {
		_, err = git.Line(dir, append([]string{"fetch", "--quiet", "--no-tags", "origin", fmt.Sprintf("+refs/heads/%s:%s", branch, head)}, specs...)...)
	}
	if err != nil {
		return "", fmt.Errorf("could not fetch #%d: %w", number, err)
	}
	return git.Line(dir, "rev-parse", "--verify", head+"^{commit}")
}

// FetchCommit brings one commit into the clone by its id, kept at ref: a
// commit you reviewed on GitHub that a force-push has since taken out of
// the branch. GitHub serves any commit it still has.
func FetchCommit(dir, sha, ref string) error {
	_, err := git.Line(dir, "fetch", "--quiet", "--no-tags", "origin", "+"+sha+":"+ref)
	return err
}

// Keep points ref at a commit in the clone.
func Keep(dir, ref, sha string) error {
	_, err := git.Line(dir, "update-ref", ref, sha)
	return err
}
