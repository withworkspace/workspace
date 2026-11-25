package github

import (
	"bytes"

	gh "github.com/cli/go-gh/v2"

	"github.com/withworkspace/workspace/internal/cost"
)

// ghExec runs the gh CLI, as gh.Exec does, counted when costs are measured.
func ghExec(args ...string) (stdout, stderr bytes.Buffer, err error) {
	defer cost.Begin(cost.GH)()
	return gh.Exec(args...)
}
